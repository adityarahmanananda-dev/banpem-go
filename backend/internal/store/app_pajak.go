package store

import (
	"context"
	"strings"
	"time"
)

// TaxValueForJenis mengambil nilai pajak suatu invoice untuk jenis pajak
// tertentu: PPN -> nilai_ppn; PPh 21 -> nilai_pph jika jenis_pph mengandung
// '21'; PPh 22 -> '22'; PPh 23 -> '23'.
func TaxValueForJenis(inv Invoice, jenis string) int64 {
	switch jenis {
	case "PPN":
		return inv.NilaiPPN
	case "PPh 21":
		if inv.JenisPPH != nil && strings.Contains(*inv.JenisPPH, "21") {
			return inv.NilaiPPH
		}
	case "PPh 22":
		if inv.JenisPPH != nil && strings.Contains(*inv.JenisPPH, "22") {
			return inv.NilaiPPH
		}
	case "PPh 23":
		if inv.JenisPPH != nil && strings.Contains(*inv.JenisPPH, "23") {
			return inv.NilaiPPH
		}
	}
	return 0
}

// InferJenisPajak menyimpulkan jenis pajak dari uraian entri setor.
func InferJenisPajak(uraian string) string {
	u := strings.ToUpper(uraian)
	switch {
	case strings.Contains(u, "PPN"):
		return "PPN"
	case strings.Contains(u, "21"):
		return "PPh 21"
	case strings.Contains(u, "22"):
		return "PPh 22"
	case strings.Contains(u, "23"):
		return "PPh 23"
	}
	return "Pajak"
}

type SetorInput struct {
	BantuanID  int64
	JenisPajak string
	InvoiceIDs []int64
	Tanggal    time.Time
	NTBN       string
	NTPN       string
	Keterangan string
}

// CreateSetorPajak membuat 1 entri BKU 'setor_pajak', N baris REKAP_PAJAK,
// dan 1 entri Bank 'setor_pajak'.
func (s *Store) CreateSetorPajak(ctx context.Context, in SetorInput) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var nominal int64
		for _, iid := range in.InvoiceIDs {
			inv, err := getInvoiceQ(ctx, tx, iid)
			if err != nil {
				return err
			}
			nominal += TaxValueForJenis(inv, in.JenisPajak)
		}
		if nominal <= 0 {
			return ErrNoTaxValue
		}
		uraian := strings.TrimSpace(in.Keterangan)
		if uraian == "" {
			uraian = "Penyetoran " + in.JenisPajak
		}
		nomorBukti := ""
		if in.NTPN != "" {
			nomorBukti = "NTPN=\n" + in.NTPN
		} else if in.NTBN != "" {
			nomorBukti = "NTB=\n" + in.NTBN
		}
		tgl := in.Tanggal
		ledgerID, err := AppendBKULedger(ctx, tx, LedgerEntry{
			BantuanID: in.BantuanID, Tanggal: &tgl, NomorBukti: nomorBukti, Uraian: uraian,
			Debit: nominal, Kredit: 0, JenisTransaksi: "setor_pajak",
		})
		if err != nil {
			return err
		}
		for _, iid := range in.InvoiceIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO rekap_pajak
				(bantuan_id, tanggal_posting, ntbn, ntpn, invoice_id, jasa_giro, setor_ledger_id, jenis_pajak)
				VALUES ($1,$2,$3,$4,$5,0,$6,$7)`,
				in.BantuanID, tgl, in.NTBN, in.NTPN, iid, ledgerID, in.JenisPajak); err != nil {
				return err
			}
		}
		_, err = AppendBankLedger(ctx, tx, LedgerEntry{
			BantuanID: in.BantuanID, Tanggal: &tgl, NomorBukti: nomorBukti, Uraian: uraian,
			Debit: nominal, Kredit: 0, JenisTransaksi: "setor_pajak",
		})
		return err
	})
}

var ErrNoTaxValue = errNoTaxValue()

func errNoTaxValue() error { return &storedError{"tidak ada nilai pajak untuk invoice yang dipilih"} }

type storedError struct{ msg string }

func (e *storedError) Error() string { return e.msg }

// UpdateSetorPajak mengedit entri setor: update ledger, rekap dihapus &
// diinsert ulang, dan entri bank terkait di-update.
func (s *Store) UpdateSetorPajak(ctx context.Context, setorID int64, in SetorInput) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var oldDebit int64
		var oldTanggal time.Time
		var oldUraian string
		err := tx.QueryRow(ctx, `SELECT tanggal, uraian, debit FROM trx_ledger WHERE id=$1`, setorID).Scan(&oldTanggal, &oldUraian, &oldDebit)
		if err != nil {
			return err
		}
		oldJenis := in.JenisPajak
		var j string
		if err := tx.QueryRow(ctx, `SELECT jenis_pajak FROM rekap_pajak WHERE setor_ledger_id=$1 AND jenis_pajak<>'' ORDER BY id LIMIT 1`, setorID).Scan(&j); err == nil && j != "" {
			oldJenis = j
		}
		var nominal int64
		for _, iid := range in.InvoiceIDs {
			inv, err := getInvoiceQ(ctx, tx, iid)
			if err != nil {
				return err
			}
			nominal += TaxValueForJenis(inv, oldJenis)
		}
		if nominal <= 0 {
			return ErrNoTaxValue
		}
		uraian := strings.TrimSpace(in.Keterangan)
		if uraian == "" {
			uraian = "Penyetoran " + oldJenis
		}
		nomorBukti := ""
		if in.NTPN != "" {
			nomorBukti = "NTPN=\n" + in.NTPN
		} else if in.NTBN != "" {
			nomorBukti = "NTB=\n" + in.NTBN
		}
		tgl := in.Tanggal
		if _, err := tx.Exec(ctx, `UPDATE trx_ledger SET tanggal=$1, nomor_bukti=$2, uraian=$3, debit=$4, kredit=0 WHERE id=$5`,
			tgl, nomorBukti, uraian, nominal, setorID); err != nil {
			return err
		}
		if err := RebuildLedger(ctx, tx, in.BantuanID, "bku"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM rekap_pajak WHERE setor_ledger_id=$1`, setorID); err != nil {
			return err
		}
		for _, iid := range in.InvoiceIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO rekap_pajak
				(bantuan_id, tanggal_posting, ntbn, ntpn, invoice_id, jasa_giro, setor_ledger_id, jenis_pajak)
				VALUES ($1,$2,$3,$4,$5,0,$6,$7)`,
				in.BantuanID, tgl, in.NTBN, in.NTPN, iid, setorID, oldJenis); err != nil {
				return err
			}
		}
		var bankID int64
		err = tx.QueryRow(ctx, `SELECT id FROM trx_bank_ledger
			WHERE bantuan_id=$1 AND jenis_transaksi='setor_pajak' AND tanggal=$2 AND uraian=$3 AND debit=$4
			ORDER BY id LIMIT 1`, in.BantuanID, oldTanggal, oldUraian, oldDebit).Scan(&bankID)
		if err == nil {
			if _, err := tx.Exec(ctx, `UPDATE trx_bank_ledger SET tanggal=$1, nomor_bukti=$2, uraian=$3, debit=$4, kredit=0 WHERE id=$5`,
				tgl, nomorBukti, uraian, nominal, bankID); err != nil {
				return err
			}
		}
		return RebuildLedger(ctx, tx, in.BantuanID, "bank")
	})
}

// DeleteSetorPajak menghapus entri BKU, rekap pajak, dan entri bank setor.
func (s *Store) DeleteSetorPajak(ctx context.Context, setorID int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var bantuanID int64
		var oldTanggal time.Time
		var oldUraian string
		var oldDebit int64
		err := tx.QueryRow(ctx, `SELECT bantuan_id, tanggal, uraian, debit FROM trx_ledger WHERE id=$1`, setorID).Scan(&bantuanID, &oldTanggal, &oldUraian, &oldDebit)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM rekap_pajak WHERE setor_ledger_id=$1`, setorID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_ledger WHERE id=$1`, setorID); err != nil {
			return err
		}
		if err := RebuildLedger(ctx, tx, bantuanID, "bku"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_bank_ledger
			WHERE bantuan_id=$1 AND jenis_transaksi='setor_pajak' AND tanggal=$2 AND uraian=$3 AND debit=$4`,
			bantuanID, oldTanggal, oldUraian, oldDebit); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, bantuanID, "bank")
	})
}

// ListSetorJenisPerInvoice mengembalikan peta invoice_id -> daftar jenis pajak
// yang sudah disetor (untuk deteksi "sudah disetor" di form setor).
func (s *Store) ListSetorJenisPerInvoice(ctx context.Context, bantuanID int64) (map[int64][]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT rp.invoice_id, l.uraian
		FROM rekap_pajak rp JOIN trx_ledger l ON l.id = rp.setor_ledger_id
		WHERE rp.bantuan_id=$1 AND rp.invoice_id IS NOT NULL AND rp.setor_ledger_id IS NOT NULL`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var iid int64
		var uraian string
		if err := rows.Scan(&iid, &uraian); err != nil {
			return nil, err
		}
		j := InferJenisPajak(uraian)
		found := false
		for _, x := range out[iid] {
			if x == j {
				found = true
				break
			}
		}
		if !found {
			out[iid] = append(out[iid], j)
		}
	}
	return out, rows.Err()
}

// ListSetorEntries menampilkan entri TRX_LEDGER 'setor_pajak'.
func (s *Store) ListSetorEntries(ctx context.Context, bantuanID int64) ([]Ledger, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi, nomor_bupot
		FROM trx_ledger WHERE bantuan_id=$1 AND jenis_transaksi='setor_pajak' ORDER BY nomor, id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ledger
	for rows.Next() {
		var e Ledger
		if err := rows.Scan(&e.ID, &e.BantuanID, &e.Nomor, &e.Tanggal, &e.NomorBukti, &e.Uraian, &e.Debit, &e.Kredit, &e.Saldo, &e.InvoiceID, &e.JenisTransaksi, &e.NomorBupot); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateJasaGiro menyimpan jasa giro + entri BKU 'jasa_giro' (debit = kredit,
// sehingga saldo tidak berubah karena jasa giro hanya numpang lewat).
func (s *Store) CreateJasaGiro(ctx context.Context, bantuanID int64, tanggal time.Time, nominal int64, uraian string) error {
	return s.WithTx(ctx, func(tx Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO jasa_giro (bantuan_id, tanggal, nominal, uraian) VALUES ($1,$2,$3,$4)`,
			bantuanID, tanggal, nominal, uraian); err != nil {
			return err
		}
		tgl := tanggal
		_, err := AppendBKULedger(ctx, tx, LedgerEntry{
			BantuanID: bantuanID, Tanggal: &tgl, NomorBukti: "", Uraian: uraian,
			Debit: nominal, Kredit: nominal, JenisTransaksi: "jasa_giro",
		})
		return err
	})
}

func (s *Store) ListJasaGiro(ctx context.Context, bantuanID int64) ([]JasaGiro, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, bantuan_id, tanggal, nominal, uraian FROM jasa_giro WHERE bantuan_id=$1 ORDER BY tanggal, id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JasaGiro
	for rows.Next() {
		var j JasaGiro
		if err := rows.Scan(&j.ID, &j.BantuanID, &j.Tanggal, &j.Nominal, &j.Uraian); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// DeleteJasaGiro menghapus jasa giro beserta entri BKU 'jasa_giro' terkait.
func (s *Store) DeleteJasaGiro(ctx context.Context, jgID int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var bantuanID int64
		var tanggal time.Time
		var nominal int64
		var uraian string
		err := tx.QueryRow(ctx, `SELECT bantuan_id, tanggal, nominal, uraian FROM jasa_giro WHERE id=$1`, jgID).
			Scan(&bantuanID, &tanggal, &nominal, &uraian)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM jasa_giro WHERE id=$1`, jgID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `WITH d AS (
				SELECT id FROM trx_ledger
				WHERE bantuan_id=$1 AND jenis_transaksi='jasa_giro' AND tanggal=$2 AND kredit=$3 AND uraian=$4
				ORDER BY id DESC LIMIT 1
			) DELETE FROM trx_ledger WHERE id IN (SELECT id FROM d)`,
			bantuanID, tanggal, nominal, uraian); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, bantuanID, "bku")
	})
}

// TotalSetorPajak menjumlahkan debit entri setor_pajak.
func (s *Store) TotalSetorPajak(ctx context.Context, bantuanID int64) int64 {
	var v int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(debit),0) FROM trx_ledger WHERE bantuan_id=$1 AND jenis_transaksi='setor_pajak'`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

// TotalJasaGiro menjumlahkan nominal jasa giro.
func (s *Store) TotalJasaGiro(ctx context.Context, bantuanID int64) int64 {
	var v int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(nominal),0) FROM jasa_giro WHERE bantuan_id=$1`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

// TotalPemungutan menjumlahkan total pajak semua tagihan (PPN + PPh).
func (s *Store) TotalPemungutan(ctx context.Context, bantuanID int64) int64 {
	var v int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(nilai_ppn),0) + COALESCE(SUM(nilai_pph),0) FROM trx_invoice WHERE bantuan_id=$1`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}
