package store

import (
	"context"
	"strings"
	"time"

	"ebku/internal/money"
	"ebku/internal/tax"
)

// AdminFee mengembalikan 2900 (dalam sen) bila bank penerima berbeda dari bank
// bantuan (case-insensitive); selain itu 0.
func AdminFee(bankBantuan, bankPenerima string) int64 {
	if bankBantuan != "" && bankPenerima != "" &&
		strings.ToLower(bankPenerima) != strings.ToLower(bankBantuan) {
		return 290000
	}
	return 0
}

type RealisasiInput struct {
	KomponenID int64
	Bruto      int64
}

type InvoiceInput struct {
	Tanggal              time.Time
	NomorBukti           string
	Uraian               string
	Bruto                int64
	JenisMenu            int
	FlagPpn              int
	FlagPph              int
	KategoriNarasumber   *int
	NamaRekening         string
	NomorRekening        string
	Bank                 string
	NPWP                 string
	NomorBupotPPN        string
	NomorBupotPPH        string
	BiayaAdminDibebankan string
	Realisasi            []RealisasiInput
}

func getBantuanQ(ctx context.Context, q Querier, id int64) (Bantuan, error) {
	var b Bantuan
	err := q.QueryRow(ctx, `SELECT id, nama, nama_sekolah, nama_rekening, nomor_rekening, bank, npwp,
		kepala_sekolah, nip_kepala_sekolah, bendahara, nip_bendahara, nominal, created_at
		FROM bantuan WHERE id=$1`, id).Scan(
		&b.ID, &b.Nama, &b.NamaSekolah, &b.NamaRekening, &b.NomorRekening, &b.Bank, &b.NPWP,
		&b.KepalaSekolah, &b.NIPKepalaSekolah, &b.Bendahara, &b.NIPBendahara, &b.Nominal, &b.CreatedAt)
	return b, err
}

func getInvoiceQ(ctx context.Context, q Querier, id int64) (Invoice, error) {
	var i Invoice
	err := q.QueryRow(ctx, `SELECT id, bantuan_id, tanggal, nomor_bukti, uraian, bruto,
		jenis_menu, flag_ppn, flag_pph, kategori_narasumber, dpp, dpp_nilai_lain,
		nilai_ppn, nilai_pph, jenis_pph, nilai_netto, nama_rekening, nomor_rekening,
		bank, npwp, nomor_bupot_ppn, nomor_bupot_pph, biaya_admin_dibebankan, sort_order
		FROM trx_invoice WHERE id=$1`, id).Scan(
		&i.ID, &i.BantuanID, &i.Tanggal, &i.NomorBukti, &i.Uraian, &i.Bruto,
		&i.JenisMenu, &i.FlagPpn, &i.FlagPph, &i.KategoriNarasumber, &i.DPP, &i.DPPNilaiLain,
		&i.NilaiPPN, &i.NilaiPPH, &i.JenisPPH, &i.NilaiNetto, &i.NamaRekening, &i.NomorRekening,
		&i.Bank, &i.NPWP, &i.NomorBupotPPN, &i.NomorBupotPPH, &i.BiayaAdminDibebankan, &i.SortOrder)
	return i, err
}

func saveRealisasi(ctx context.Context, q Querier, iid int64, rows []RealisasiInput, ppnTotal, pphTotal int64) error {
	var total int64
	for _, r := range rows {
		if r.KomponenID <= 0 {
			continue
		}
		total += r.Bruto
	}
	if total <= 0 {
		return nil
	}
	for _, r := range rows {
		if r.KomponenID <= 0 {
			continue
		}
		rPPN := money.MulDiv(ppnTotal, r.Bruto, total)
		rPPH := money.MulDiv(pphTotal, r.Bruto, total)
		rNetto := r.Bruto - rPPN - rPPH
		if _, err := q.Exec(ctx, `INSERT INTO trx_invoice_realisasi
			(invoice_id, komponen_id, bruto, nilai_ppn, nilai_pph, nilai_netto)
			VALUES ($1,$2,$3,$4,$5,$6)`, iid, r.KomponenID, r.Bruto, rPPN, rPPH, rNetto); err != nil {
			return err
		}
	}
	return nil
}

// CreateInvoice membuat tagihan baru lengkap dengan entri BKU + Bank dan
// realisasi, semuanya dalam satu transaksi.
func (s *Store) CreateInvoice(ctx context.Context, bantuanID int64, in InvoiceInput) (int64, error) {
	var iid int64
	err := s.WithTx(ctx, func(tx Tx) error {
		b, err := getBantuanQ(ctx, tx, bantuanID)
		if err != nil {
			return err
		}
		res := tax.Hitung(in.Bruto, in.JenisMenu, in.FlagPpn, in.FlagPph, in.KategoriNarasumber)
		admin := AdminFee(b.Bank, in.Bank)
		nettoFinal := res.Netto - admin

		var sortOrder int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order),0) FROM trx_invoice WHERE bantuan_id=$1`, bantuanID).Scan(&sortOrder); err != nil {
			return err
		}
		sortOrder++

		jenisPPH := "Pemungutan PPh 23"
		if res.JenisPPH == nil {
			jenisPPH = ""
		} else {
			jenisPPH = *res.JenisPPH
		}

		err = tx.QueryRow(ctx, `INSERT INTO trx_invoice
			(bantuan_id, tanggal, nomor_bukti, uraian, bruto, jenis_menu, flag_ppn, flag_pph,
			 kategori_narasumber, dpp, dpp_nilai_lain, nilai_ppn, nilai_pph, jenis_pph,
			 nilai_netto, nama_rekening, nomor_rekening, bank, npwp, nomor_bupot_ppn,
			 nomor_bupot_pph, biaya_admin_dibebankan, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
			RETURNING id`,
			bantuanID, in.Tanggal, in.NomorBukti, in.Uraian, in.Bruto, in.JenisMenu,
			in.FlagPpn, in.FlagPph, in.KategoriNarasumber, res.DPP, res.DPPNilaiLain,
			res.PPN, res.PPH, jenisPPH, nettoFinal, in.NamaRekening, in.NomorRekening,
			in.Bank, in.NPWP, in.NomorBupotPPN, in.NomorBupotPPH, in.BiayaAdminDibebankan,
			sortOrder).Scan(&iid)
		if err != nil {
			return err
		}
		if err := saveRealisasi(ctx, tx, iid, in.Realisasi, res.PPN, res.PPH); err != nil {
			return err
		}
		iidPtr := iid
		tgl := in.Tanggal

		// Entri BKU (tanpa biaya admin)
		if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
			BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: in.Bruto, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if res.PPN > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPN,
				Uraian: "Pemungutan PPN atas " + in.Uraian, Debit: 0, Kredit: res.PPN,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_ppn",
			}); err != nil {
				return err
			}
		}
		if res.PPH > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPH,
				Uraian: "Pemungutan " + jenisPPH + " atas " + in.Uraian, Debit: 0, Kredit: res.PPH,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_pph",
			}); err != nil {
				return err
			}
		}

		// Entri Bank
		nettoBank := in.Bruto - res.PPN - res.PPH - admin
		if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
			BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: nettoBank, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if admin > 0 {
			if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: "", Uraian: "Biaya Admin untuk " + in.Uraian,
				Debit: admin, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "biaya_admin",
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return iid, err
}

// UpdateInvoice = hapus entri lama lalu buat ulang dari nominal terbaru.
// Tidak ada jurnal koreksi.
func (s *Store) UpdateInvoice(ctx context.Context, iid int64, in InvoiceInput) error {
	return s.WithTx(ctx, func(tx Tx) error {
		inv, err := getInvoiceQ(ctx, tx, iid)
		if err != nil {
			return err
		}
		b, err := getBantuanQ(ctx, tx, inv.BantuanID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_ledger WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_bank_ledger WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_invoice_realisasi WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if err := RebuildLedger(ctx, tx, inv.BantuanID, "bku"); err != nil {
			return err
		}
		if err := RebuildLedger(ctx, tx, inv.BantuanID, "bank"); err != nil {
			return err
		}

		res := tax.Hitung(in.Bruto, in.JenisMenu, in.FlagPpn, in.FlagPph, in.KategoriNarasumber)
		admin := AdminFee(b.Bank, in.Bank)
		nettoFinal := res.Netto - admin
		jenisPPH := ""
		if res.JenisPPH != nil {
			jenisPPH = *res.JenisPPH
		}
		if _, err := tx.Exec(ctx, `UPDATE trx_invoice SET
			tanggal=$1, nomor_bukti=$2, uraian=$3, bruto=$4, jenis_menu=$5, flag_ppn=$6,
			flag_pph=$7, kategori_narasumber=$8, dpp=$9, dpp_nilai_lain=$10, nilai_ppn=$11,
			nilai_pph=$12, jenis_pph=$13, nilai_netto=$14, nama_rekening=$15, nomor_rekening=$16,
			bank=$17, npwp=$18, nomor_bupot_ppn=$19, nomor_bupot_pph=$20, biaya_admin_dibebankan=$21
			WHERE id=$22`,
			in.Tanggal, in.NomorBukti, in.Uraian, in.Bruto, in.JenisMenu, in.FlagPpn,
			in.FlagPph, in.KategoriNarasumber, res.DPP, res.DPPNilaiLain, res.PPN,
			res.PPH, jenisPPH, nettoFinal, in.NamaRekening, in.NomorRekening,
			in.Bank, in.NPWP, in.NomorBupotPPN, in.NomorBupotPPH, in.BiayaAdminDibebankan, iid); err != nil {
			return err
		}
		if err := saveRealisasi(ctx, tx, iid, in.Realisasi, res.PPN, res.PPH); err != nil {
			return err
		}
		iidPtr := iid
		tgl := in.Tanggal
		if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
			BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: in.Bruto, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if res.PPN > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPN,
				Uraian: "Pemungutan PPN atas " + in.Uraian, Debit: 0, Kredit: res.PPN,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_ppn",
			}); err != nil {
				return err
			}
		}
		if res.PPH > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPH,
				Uraian: "Pemungutan " + jenisPPH + " atas " + in.Uraian, Debit: 0, Kredit: res.PPH,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_pph",
			}); err != nil {
				return err
			}
		}
		nettoBank := in.Bruto - res.PPN - res.PPH - admin
		if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
			BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: nettoBank, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if admin > 0 {
			if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: "", Uraian: "Biaya Admin untuk " + in.Uraian,
				Debit: admin, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "biaya_admin",
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteInvoice menghapus tagihan beserta entri ledger, realisasi, dan rekap
// pajak terkait, lalu menyusun ulang nomor & saldo.
func (s *Store) DeleteInvoice(ctx context.Context, iid int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		inv, err := getInvoiceQ(ctx, tx, iid)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_ledger WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_bank_ledger WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_invoice_realisasi WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM rekap_pajak WHERE invoice_id=$1`, iid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_invoice WHERE id=$1`, iid); err != nil {
			return err
		}
		if err := RebuildLedger(ctx, tx, inv.BantuanID, "bku"); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, inv.BantuanID, "bank")
	})
}

func (s *Store) GetInvoice(ctx context.Context, id int64) (Invoice, error) {
	return getInvoiceQ(ctx, s.Pool, id)
}

func (s *Store) ListInvoices(ctx context.Context, bantuanID int64, order string) ([]Invoice, error) {
	sqlOrder := "ORDER BY tanggal, id"
	if order == "sort" {
		sqlOrder = "ORDER BY sort_order, id"
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, bantuan_id, tanggal, nomor_bukti, uraian, bruto,
		jenis_menu, flag_ppn, flag_pph, kategori_narasumber, dpp, dpp_nilai_lain,
		nilai_ppn, nilai_pph, jenis_pph, nilai_netto, nama_rekening, nomor_rekening,
		bank, npwp, nomor_bupot_ppn, nomor_bupot_pph, biaya_admin_dibebankan, sort_order
		FROM trx_invoice WHERE bantuan_id=$1 `+sqlOrder, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invoice
	for rows.Next() {
		var i Invoice
		if err := rows.Scan(&i.ID, &i.BantuanID, &i.Tanggal, &i.NomorBukti, &i.Uraian, &i.Bruto,
			&i.JenisMenu, &i.FlagPpn, &i.FlagPph, &i.KategoriNarasumber, &i.DPP, &i.DPPNilaiLain,
			&i.NilaiPPN, &i.NilaiPPH, &i.JenisPPH, &i.NilaiNetto, &i.NamaRekening, &i.NomorRekening,
			&i.Bank, &i.NPWP, &i.NomorBupotPPN, &i.NomorBupotPPH, &i.BiayaAdminDibebankan, &i.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) ListRealisasi(ctx context.Context, invoiceID int64) ([]Realisasi, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, invoice_id, komponen_id, bruto, nilai_ppn, nilai_pph, nilai_netto
		FROM trx_invoice_realisasi WHERE invoice_id=$1 ORDER BY id`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Realisasi
	for rows.Next() {
		var r Realisasi
		if err := rows.Scan(&r.ID, &r.InvoiceID, &r.KomponenID, &r.Bruto, &r.NilaiPPN, &r.NilaiPPH, &r.NilaiNetto); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type BelanjaRow struct {
	SortOrder  int
	InvoiceID  int64
	Tanggal    time.Time
	Uraian     string
	Penyedia   string
	Bank       string
	NoRek      string
	NPWP       string
	Kegiatan   string
	SubKeg     string
	Aktivitas  string
	Komponen   string
	Bruto      int64
	PPN        int64
	PPH        int64
	Netto      int64
	BiayaAdmin int64
}

// ListBelanja mengembalikan realisasi belanja per TAGIHAN (diagregasi dari
// baris realisasi per komponen), bergabung dengan master data, urut
// invoice.sort_order.
func (s *Store) ListBelanja(ctx context.Context, bantuanID int64) ([]BelanjaRow, error) {
	rows, err := s.Pool.Query(ctx, `SELECT i.sort_order, i.id, i.tanggal, i.uraian, i.nama_rekening,
		i.bank, i.nomor_rekening, i.npwp,
		string_agg(DISTINCT k.nama, E'\n' ORDER BY k.nama),
		string_agg(DISTINCT sk.nama, E'\n' ORDER BY sk.nama),
		string_agg(DISTINCT a.nama, E'\n' ORDER BY a.nama),
		string_agg(DISTINCT ko.nama, E'\n' ORDER BY ko.nama),
		SUM(r.bruto), SUM(r.nilai_ppn), SUM(r.nilai_pph), SUM(r.nilai_netto),
		MAX(CASE WHEN lower(i.bank) <> lower(ba.bank) THEN 290000 ELSE 0 END)
		FROM trx_invoice_realisasi r
		JOIN trx_invoice i ON i.id = r.invoice_id
		JOIN bantuan ba ON ba.id = i.bantuan_id
		JOIN komponen ko ON ko.id = r.komponen_id
		JOIN aktivitas a ON a.id = ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE i.bantuan_id=$1
		GROUP BY i.sort_order, i.id, i.tanggal, i.uraian, i.nama_rekening, i.bank, i.nomor_rekening, i.npwp
		ORDER BY i.sort_order, i.id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BelanjaRow
	for rows.Next() {
		var r BelanjaRow
		if err := rows.Scan(&r.SortOrder, &r.InvoiceID, &r.Tanggal, &r.Uraian, &r.Penyedia,
			&r.Bank, &r.NoRek, &r.NPWP, &r.Kegiatan, &r.SubKeg, &r.Aktivitas, &r.Komponen,
			&r.Bruto, &r.PPN, &r.PPH, &r.Netto, &r.BiayaAdmin); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetInvoiceOrder menyimpan urutan invoice (drag-and-drop rekap belanja).
func (s *Store) SetInvoiceOrder(ctx context.Context, bantuanID int64, ids []int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		for idx, id := range ids {
			if _, err := tx.Exec(ctx, `UPDATE trx_invoice SET sort_order=$1 WHERE id=$2 AND bantuan_id=$3`, idx+1, id, bantuanID); err != nil {
				return err
			}
		}
		return nil
	})
}
