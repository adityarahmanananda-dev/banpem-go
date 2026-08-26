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
	// PPNOverride/PPHOverride membiarkan user mengoreksi hasil perhitungan
	// pajak (mis. selisih pembulatan). Nilai pointer bila diisi dipakai
	// menggantikan nilai PPN/PPh hasil hitung_pajak.
	PPNOverride *int64
	PPHOverride *int64
}

// hitungPajakEfektif menerapkan koreksi user (bila diisi) pada hasil hitung
// pajak. Nilai PPN/PPh yang dikoreksi tetap menjaga bruto tidak berubah.
func hitungPajakEfektif(in InvoiceInput, res tax.Result) (ppn, pph int64) {
	ppn, pph = res.PPN, res.PPH
	if in.PPNOverride != nil {
		ppn = *in.PPNOverride
	}
	if in.PPHOverride != nil {
		pph = *in.PPHOverride
	}
	return ppn, pph
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
		ppn, pph := hitungPajakEfektif(in, res)
		admin := AdminFee(b.Bank, in.Bank)
		nettoFinal := in.Bruto - ppn - pph - admin

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
			ppn, pph, jenisPPH, nettoFinal, in.NamaRekening, in.NomorRekening,
			in.Bank, in.NPWP, in.NomorBupotPPN, in.NomorBupotPPH, in.BiayaAdminDibebankan,
			sortOrder).Scan(&iid)
		if err != nil {
			return err
		}
		if err := saveRealisasi(ctx, tx, iid, in.Realisasi, ppn, pph); err != nil {
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
		if ppn > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPN,
				Uraian: "Pemungutan PPN atas " + in.Uraian, Debit: 0, Kredit: ppn,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_ppn",
			}); err != nil {
				return err
			}
		}
		if pph > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPH,
				Uraian: "Pemungutan " + jenisPPH + " atas " + in.Uraian, Debit: 0, Kredit: pph,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_pph",
			}); err != nil {
				return err
			}
		}

		// Entri Bank
		nettoBank := in.Bruto - ppn - pph - admin
		if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
			BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: nettoBank, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if admin > 0 {
			if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
				BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: "Biaya Admin untuk " + in.Uraian,
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
		ppn, pph := hitungPajakEfektif(in, res)
		admin := AdminFee(b.Bank, in.Bank)
		nettoFinal := in.Bruto - ppn - pph - admin
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
			in.FlagPph, in.KategoriNarasumber, res.DPP, res.DPPNilaiLain, ppn,
			pph, jenisPPH, nettoFinal, in.NamaRekening, in.NomorRekening,
			in.Bank, in.NPWP, in.NomorBupotPPN, in.NomorBupotPPH, in.BiayaAdminDibebankan, iid); err != nil {
			return err
		}
		if err := saveRealisasi(ctx, tx, iid, in.Realisasi, ppn, pph); err != nil {
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
		if ppn > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPN,
				Uraian: "Pemungutan PPN atas " + in.Uraian, Debit: 0, Kredit: ppn,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_ppn",
			}); err != nil {
				return err
			}
		}
		if pph > 0 {
			if _, err := AppendBKULedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBupotPPH,
				Uraian: "Pemungutan " + jenisPPH + " atas " + in.Uraian, Debit: 0, Kredit: pph,
				InvoiceID: &iidPtr, JenisTransaksi: "pungut_pph",
			}); err != nil {
				return err
			}
		}
		nettoBank := in.Bruto - ppn - pph - admin
		if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
			BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: in.Uraian,
			Debit: nettoBank, Kredit: 0, InvoiceID: &iidPtr, JenisTransaksi: "pembayaran",
		}); err != nil {
			return err
		}
		if admin > 0 {
			if _, err := AppendBankLedger(ctx, tx, LedgerEntry{
				BantuanID: inv.BantuanID, Tanggal: &tgl, NomorBukti: in.NomorBukti, Uraian: "Biaya Admin untuk " + in.Uraian,
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

// RekapPenggunaanCol adalah satu kolom angka distribusi: nilai kegiatan/sub
// kegiatan/aktivitas/komponen di anggaran yang menjadi kolom tersendiri.
type RekapPenggunaanCol struct {
	Level string // "kegiatan" | "sub" | "aktivitas" | "komponen"
	Name  string
}

// RekapPenggunaanRow adalah satu baris Rekap Penggunaan Dana (per tagihan).
// Values memetakan Level+"\x00"+Nama ke nominal bruto tagihan pada nilai itu.
type RekapPenggunaanRow struct {
	ID         int64
	Tanggal    time.Time
	NomorBukti string
	Uraian     string
	Bruto      int64
	NilaiPPN   int64
	PPh21      int64
	PPh22      int64
	PPh23      int64
	Values     map[string]int64
}

// RekapPenggunaanData berisi kolom distribusi (dari anggaran) + baris per
// tagihan untuk Rekap Penggunaan Dana.
type RekapPenggunaanData struct {
	Tree []KegiatanTree
	Cols []RekapPenggunaanCol
	Rows []RekapPenggunaanRow
}

// ListRekapPenggunaan menyusun data Rekap Penggunaan Dana: kolom distribusi
// diambil dari hierarki anggaran (kegiatan/sub kegiatan/aktivitas/komponen)
// dengan urutan RAB, sedangkan nilai tiap sel = bruto realisasi tagihan pada
// nilai tersebut. Tagihan tanpa realisasi tetap tampil (nilai 0).
func (s *Store) ListRekapPenggunaan(ctx context.Context, bantuanID int64) (RekapPenggunaanData, error) {
	var out RekapPenggunaanData
	tree, err := s.ListKegiatanTree(ctx, bantuanID)
	if err != nil {
		return out, err
	}
	out.Tree = tree
	addCol := func(level, name string) {
		if name == "" {
			return
		}
		for _, c := range out.Cols {
			if c.Level == level && c.Name == name {
				return
			}
		}
		out.Cols = append(out.Cols, RekapPenggunaanCol{Level: level, Name: name})
	}
	for _, k := range tree {
		addCol("kegiatan", k.Nama)
		for _, sk := range k.Subs {
			addCol("sub", sk.Nama)
			for _, a := range sk.Aktivitass {
				addCol("aktivitas", a.Nama)
				for _, ko := range a.Komponens {
					addCol("komponen", ko.Nama)
				}
			}
		}
	}

	rows, err := s.Pool.Query(ctx, `SELECT i.id, i.tanggal, i.nomor_bukti, i.uraian, i.bruto,
		COALESCE(k.nama,''), COALESCE(sk.nama,''), COALESCE(a.nama,''), COALESCE(ko.nama,''),
		COALESCE(r.bruto,0), COALESCE(i.nilai_ppn,0), COALESCE(i.nilai_pph,0), COALESCE(i.jenis_pph,'')
		FROM trx_invoice i
		LEFT JOIN trx_invoice_realisasi r ON r.invoice_id = i.id
		LEFT JOIN komponen ko ON ko.id = r.komponen_id
		LEFT JOIN aktivitas a ON a.id = ko.aktivitas_id
		LEFT JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		LEFT JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE i.bantuan_id=$1
		ORDER BY i.sort_order, i.id, ko.id`, bantuanID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var last *RekapPenggunaanRow
	for rows.Next() {
		var id int64
		var tgl time.Time
		var bukti, uraian, kegiatan, sub, aktivitas, komponen string
		var invBruto, rBruto, ppn, pph int64
		var jenisPPH string
		if err := rows.Scan(&id, &tgl, &bukti, &uraian, &invBruto,
			&kegiatan, &sub, &aktivitas, &komponen, &rBruto, &ppn, &pph, &jenisPPH); err != nil {
			return out, err
		}
		if last == nil || last.ID != id {
			rw := RekapPenggunaanRow{
				ID: id, Tanggal: tgl, NomorBukti: bukti, Uraian: uraian,
				Bruto: invBruto, NilaiPPN: ppn, Values: map[string]int64{},
			}
			setPPhValues(&rw, jenisPPH, pph)
			out.Rows = append(out.Rows, rw)
			last = &out.Rows[len(out.Rows)-1]
		}
		if rBruto > 0 {
			if kegiatan != "" {
				last.Values["kegiatan\x00"+kegiatan] += rBruto
			}
			if sub != "" {
				last.Values["sub\x00"+sub] += rBruto
			}
			if aktivitas != "" {
				last.Values["aktivitas\x00"+aktivitas] += rBruto
			}
			if komponen != "" {
				last.Values["komponen\x00"+komponen] += rBruto
			}
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// setPPhValues menempatkan nilai PPh ke PPh 21/22/23 sesuai jenis_pph.
func setPPhValues(r *RekapPenggunaanRow, jenis string, val int64) {
	if strings.Contains(jenis, "21") {
		r.PPh21 = val
	}
	if strings.Contains(jenis, "22") {
		r.PPh22 = val
	}
	if strings.Contains(jenis, "23") {
		r.PPh23 = val
	}
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
