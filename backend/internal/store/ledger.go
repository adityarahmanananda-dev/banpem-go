package store

import (
	"context"
	"sort"
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

// SaldoAsOf menghitung saldo buku (bku/bank) pada tanggal tertentu dengan
// aturan yang sama seperti RebuildLedger: saldo awal dipakai hanya bila entri
// 'saldo_awal' sudah tercatat pada atau sebelum tanggal tersebut.
func (s *Store) SaldoAsOf(ctx context.Context, bantuanID int64, table string, tgl time.Time) int64 {
	sel := `SELECT jenis_transaksi, debit, kredit FROM trx_ledger WHERE bantuan_id=$1 AND tanggal<=$2 ORDER BY nomor, id`
	if table == "bank" {
		sel = `SELECT jenis_transaksi, debit, kredit FROM trx_bank_ledger WHERE bantuan_id=$1 AND tanggal<=$2 ORDER BY nomor, id`
	}
	base := SaldoAwalValue(ctx, s.Pool, bantuanID)
	rows, err := s.Pool.Query(ctx, sel, bantuanID, tgl)
	if err != nil {
		return 0
	}
	defer rows.Close()
	running := int64(0)
	for rows.Next() {
		var jenis string
		var d, k int64
		if err := rows.Scan(&jenis, &d, &k); err != nil {
			return running
		}
		if jenis == "saldo_awal" {
			running = base
		} else {
			running = running - d + k
		}
	}
	return running
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

// syncLedgerItem adalah satu baris ledger untuk sinkronisasi urutan input.
type syncLedgerItem struct {
	id      int64
	jenis   string
	debit   int64
	kredit  int64
	sortRow int
}

// syncKey mengembalikan kunci urutan alami baris buku:
// saldo_awal selalu paling atas (0); lalu baris milik tagihan mengikuti urutan
// input tagihan (sort_order); entri lain (manual/setor_pajak/jasa_giro) mengikuti
// urutan pembuatannya (id).
func (it syncLedgerItem) syncKey() (p, s, t int) {
	if it.jenis == "saldo_awal" {
		return 0, 0, 0
	}
	if it.sortRow > 0 {
		return 1, it.sortRow, int(it.id)
	}
	return 2, int(it.id), int(it.id)
}

// SyncLedgerInputOrder menata ulang nomor baris buku (bku/bank) mengikuti
// urutan input ascending: baris 'saldo_awal' tetap nomor 1, lalu baris dari
// tagihan berurutan sesuai sort_order tagihan, lalu entri lainnya berurutan
// id. Saldo dihitung ulang mulai dari saldo_awal.
func SyncLedgerInputOrder(ctx context.Context, q Querier, bantuanID int64, table string) error {
	sel := `SELECT l.id, l.jenis_transaksi, l.debit, l.kredit, COALESCE(i.sort_order, 0)
		FROM trx_ledger l LEFT JOIN trx_invoice i ON i.id = l.invoice_id
		WHERE l.bantuan_id=$1`
	upd := `UPDATE trx_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
	if table == "bank" {
		sel = `SELECT l.id, l.jenis_transaksi, l.debit, l.kredit, COALESCE(i.sort_order, 0)
			FROM trx_bank_ledger l LEFT JOIN trx_invoice i ON i.id = l.invoice_id
			WHERE l.bantuan_id=$1`
		upd = `UPDATE trx_bank_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
	}
	rows, err := q.Query(ctx, sel, bantuanID)
	if err != nil {
		return err
	}
	var items []syncLedgerItem
	for rows.Next() {
		var it syncLedgerItem
		if err := rows.Scan(&it.id, &it.jenis, &it.debit, &it.kredit, &it.sortRow); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	sort.SliceStable(items, func(a, b int) bool {
		pa, sa, ta := items[a].syncKey()
		pb, sb, tb := items[b].syncKey()
		if pa != pb {
			return pa < pb
		}
		if sa != sb {
			return sa < sb
		}
		return ta < tb
	})
	base := SaldoAwalValue(ctx, q, bantuanID)
	running := base
	for i, it := range items {
		var saldo int64
		if it.jenis == "saldo_awal" {
			saldo = base
			running = base
		} else {
			running = running - it.debit + it.kredit
			saldo = running
		}
		if _, err := q.Exec(ctx, upd, i+1, saldo, it.id); err != nil {
			return err
		}
	}
	return nil
}

// ResyncAllLedger menyinkronkan urutan seluruh buku (BKU & Bank) semua bantuan
// ke urutan input. Dipanggil saat server start agar data lama ikut terurut.
func (s *Store) ResyncAllLedger(ctx context.Context) error {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM bantuan ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := SyncLedgerInputOrder(ctx, s.Pool, id, "bku"); err != nil {
			return err
		}
		if err := SyncLedgerInputOrder(ctx, s.Pool, id, "bank"); err != nil {
			return err
		}
	}
	return nil
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
