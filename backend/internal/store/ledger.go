package store

import (
	"context"
	"time"
)

// SaldoAwalValue mengambil saldo awal dari tabel saldo_awal (0 bila belum ada).
func SaldoAwalValue(ctx context.Context, q Querier, bantuanID int64) int64 {
	var v int64
	err := q.QueryRow(ctx, `SELECT saldo_awal FROM saldo_awal WHERE bantuan_id=$1`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

type ledgerRow struct {
	ID     int64
	Jenis  string
	Debit  int64
	Kredit int64
}

func tableSQL(table string) (string, string) {
	if table == "bank" {
		return `SELECT id, jenis_transaksi, debit, kredit FROM trx_bank_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`,
			`UPDATE trx_bank_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
	}
	return `SELECT id, jenis_transaksi, debit, kredit FROM trx_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`,
		`UPDATE trx_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
}

// RebuildLedger menomori ulang semua entri (1,2,3,...) dan menghitung ulang
// saldo mulai dari saldo_awal. table = "bku" | "bank".
func RebuildLedger(ctx context.Context, q Querier, bantuanID int64, table string) error {
	sel, upd := tableSQL(table)
	base := SaldoAwalValue(ctx, q, bantuanID)
	rows, err := q.Query(ctx, sel, bantuanID)
	if err != nil {
		return err
	}
	var items []ledgerRow
	for rows.Next() {
		var it ledgerRow
		if err := rows.Scan(&it.ID, &it.Jenis, &it.Debit, &it.Kredit); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	running := base
	for i, it := range items {
		var saldo int64
		if it.Jenis == "saldo_awal" {
			saldo = base
			running = base
		} else {
			running = running - it.Debit + it.Kredit
			saldo = running
		}
		if _, err := q.Exec(ctx, upd, i+1, saldo, it.ID); err != nil {
			return err
		}
	}
	return nil
}

// LedgerEntry mewakili satu entri yang akan ditambahkan.
type LedgerEntry struct {
	BantuanID      int64
	Tanggal        *time.Time
	NomorBukti     string
	Uraian         string
	Debit          int64
	Kredit         int64
	InvoiceID      *int64
	JenisTransaksi string
	NomorBupot     *string
}

// AppendBKULedger menambah entri BKU di akhir (nomor = MAX+1, saldo = saldo
// entri sebelumnya - debit + kredit) dan mengembalikan id + saldo barunya.
func AppendBKULedger(ctx context.Context, q Querier, e LedgerEntry) (int64, error) {
	var nomor int
	err := q.QueryRow(ctx, `SELECT COALESCE(MAX(nomor),0) FROM trx_ledger WHERE bantuan_id=$1`, e.BantuanID).Scan(&nomor)
	if err != nil {
		return 0, err
	}
	nomor++
	var lastSaldo int64
	err = q.QueryRow(ctx, `SELECT saldo FROM trx_ledger WHERE bantuan_id=$1 ORDER BY nomor DESC LIMIT 1`, e.BantuanID).Scan(&lastSaldo)
	if err != nil {
		lastSaldo = SaldoAwalValue(ctx, q, e.BantuanID)
	}
	saldo := lastSaldo - e.Debit + e.Kredit
	var id int64
	err = q.QueryRow(ctx, `INSERT INTO trx_ledger
		(bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi, nomor_bupot)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		e.BantuanID, nomor, e.Tanggal, e.NomorBukti, e.Uraian, e.Debit, e.Kredit, saldo, e.InvoiceID, e.JenisTransaksi, e.NomorBupot).Scan(&id)
	return id, err
}

// AppendBankLedger menambah entri Buku Kas Bank di akhir.
func AppendBankLedger(ctx context.Context, q Querier, e LedgerEntry) (int64, error) {
	var nomor int
	err := q.QueryRow(ctx, `SELECT COALESCE(MAX(nomor),0) FROM trx_bank_ledger WHERE bantuan_id=$1`, e.BantuanID).Scan(&nomor)
	if err != nil {
		return 0, err
	}
	nomor++
	var lastSaldo int64
	err = q.QueryRow(ctx, `SELECT saldo FROM trx_bank_ledger WHERE bantuan_id=$1 ORDER BY nomor DESC LIMIT 1`, e.BantuanID).Scan(&lastSaldo)
	if err != nil {
		lastSaldo = SaldoAwalValue(ctx, q, e.BantuanID)
	}
	saldo := lastSaldo - e.Debit + e.Kredit
	var id int64
	err = q.QueryRow(ctx, `INSERT INTO trx_bank_ledger
		(bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		e.BantuanID, nomor, e.Tanggal, e.NomorBukti, e.Uraian, e.Debit, e.Kredit, saldo, e.InvoiceID, e.JenisTransaksi).Scan(&id)
	return id, err
}

// ListLedger membaca semua entri suatu buku secara urut. table = "bku" | "bank".
func ListLedger(ctx context.Context, q Querier, bantuanID int64, table string) ([]Ledger, error) {
	var out []Ledger
	var err error
	if table == "bank" {
		err = listLedgerBank(ctx, q, bantuanID, &out)
	} else {
		err = listLedgerBKU(ctx, q, bantuanID, &out)
	}
	return out, err
}

func listLedgerBKU(ctx context.Context, q Querier, bantuanID int64, out *[]Ledger) error {
	rows, err := q.Query(ctx, `SELECT id, bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi, nomor_bupot
		FROM trx_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`, bantuanID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e Ledger
		if err := rows.Scan(&e.ID, &e.BantuanID, &e.Nomor, &e.Tanggal, &e.NomorBukti, &e.Uraian, &e.Debit, &e.Kredit, &e.Saldo, &e.InvoiceID, &e.JenisTransaksi, &e.NomorBupot); err != nil {
			return err
		}
		*out = append(*out, e)
	}
	return rows.Err()
}

func listLedgerBank(ctx context.Context, q Querier, bantuanID int64, out *[]Ledger) error {
	rows, err := q.Query(ctx, `SELECT id, bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi
		FROM trx_bank_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`, bantuanID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e Ledger
		if err := rows.Scan(&e.ID, &e.BantuanID, &e.Nomor, &e.Tanggal, &e.NomorBukti, &e.Uraian, &e.Debit, &e.Kredit, &e.Saldo, &e.InvoiceID, &e.JenisTransaksi); err != nil {
			return err
		}
		*out = append(*out, e)
	}
	return rows.Err()
}
