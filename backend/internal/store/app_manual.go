package store

import (
	"context"
)

// CreateManualLedger menambah entri manual BKU di akhir.
func (s *Store) CreateManualLedger(ctx context.Context, bantuanID int64, e LedgerEntry) error {
	return s.WithTx(ctx, func(tx Tx) error {
		_, err := AppendBKULedger(ctx, tx, e)
		return err
	})
}

// UpdateManualLedger mengedit entri manual BKU lalu recount saldo.
func (s *Store) UpdateManualLedger(ctx context.Context, eid int64, e LedgerEntry) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var bantuanID int64
		if err := tx.QueryRow(ctx, `SELECT bantuan_id FROM trx_ledger WHERE id=$1 AND invoice_id IS NULL`, eid).Scan(&bantuanID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE trx_ledger SET tanggal=$1, nomor_bukti=$2, uraian=$3, debit=$4, kredit=$5 WHERE id=$6`,
			e.Tanggal, e.NomorBukti, e.Uraian, e.Debit, e.Kredit, eid); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, bantuanID, "bku")
	})
}

// DeleteManualLedger menghapus entri manual BKU lalu recount saldo.
func (s *Store) DeleteManualLedger(ctx context.Context, eid int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var bantuanID int64
		if err := tx.QueryRow(ctx, `SELECT bantuan_id FROM trx_ledger WHERE id=$1 AND invoice_id IS NULL`, eid).Scan(&bantuanID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_ledger WHERE id=$1`, eid); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, bantuanID, "bku")
	})
}

// GetLedgerEntry mengambil satu entri dari tabel buku tertentu.
func (s *Store) GetLedgerEntry(ctx context.Context, id int64, table string) (Ledger, error) {
	var e Ledger
	var err error
	if table == "bank" {
		err = s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi
			FROM trx_bank_ledger WHERE id=$1`, id).Scan(
			&e.ID, &e.BantuanID, &e.Nomor, &e.Tanggal, &e.NomorBukti, &e.Uraian, &e.Debit, &e.Kredit, &e.Saldo, &e.InvoiceID, &e.JenisTransaksi)
	} else {
		err = s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi, nomor_bupot
			FROM trx_ledger WHERE id=$1`, id).Scan(
			&e.ID, &e.BantuanID, &e.Nomor, &e.Tanggal, &e.NomorBukti, &e.Uraian, &e.Debit, &e.Kredit, &e.Saldo, &e.InvoiceID, &e.JenisTransaksi, &e.NomorBupot)
	}
	return e, err
}

// DeleteBankLedger menghapus entri Buku Kas Bank lalu recount saldo.
func (s *Store) DeleteBankLedger(ctx context.Context, eid int64) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var bantuanID int64
		if err := tx.QueryRow(ctx, `SELECT bantuan_id FROM trx_bank_ledger WHERE id=$1`, eid).Scan(&bantuanID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trx_bank_ledger WHERE id=$1`, eid); err != nil {
			return err
		}
		return RebuildLedger(ctx, tx, bantuanID, "bank")
	})
}

type ledgerReorderItem struct {
	id     int64
	jenis  string
	debit  int64
	kredit int64
}

// ReorderLedger mengatur ulang urutan entri BKU/Bank sesuai daftar id
// (entri saldo_awal selalu dipaksa nomor 1) lalu menghitung ulang saldo.
func (s *Store) ReorderLedger(ctx context.Context, bantuanID int64, table string, ids []int64) error {
	sel := `SELECT id, jenis_transaksi, debit, kredit FROM trx_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`
	upd := `UPDATE trx_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
	if table == "bank" {
		sel = `SELECT id, jenis_transaksi, debit, kredit FROM trx_bank_ledger WHERE bantuan_id=$1 ORDER BY nomor, id`
		upd = `UPDATE trx_bank_ledger SET nomor=$1, saldo=$2 WHERE id=$3`
	}
	return s.WithTx(ctx, func(tx Tx) error {
		rows, err := tx.Query(ctx, sel, bantuanID)
		if err != nil {
			return err
		}
		var items []ledgerReorderItem
		for rows.Next() {
			var it ledgerReorderItem
			if err := rows.Scan(&it.id, &it.jenis, &it.debit, &it.kredit); err != nil {
				rows.Close()
				return err
			}
			items = append(items, it)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		byID := map[int64]ledgerReorderItem{}
		for _, it := range items {
			byID[it.id] = it
		}
		seen := map[int64]bool{}
		var order []ledgerReorderItem
		for _, it := range items {
			if it.jenis == "saldo_awal" {
				order = append(order, it)
				seen[it.id] = true
				break
			}
		}
		for _, id := range ids {
			if it, ok := byID[id]; ok && !seen[id] {
				order = append(order, it)
				seen[id] = true
			}
		}
		for _, it := range items {
			if !seen[it.id] {
				order = append(order, it)
				seen[it.id] = true
			}
		}
		base := SaldoAwalValue(ctx, tx, bantuanID)
		running := base
		for i, it := range order {
			var saldo int64
			if it.jenis == "saldo_awal" {
				saldo = base
				running = base
			} else {
				running = running - it.debit + it.kredit
				saldo = running
			}
			if _, err := tx.Exec(ctx, upd, i+1, saldo, it.id); err != nil {
				return err
			}
		}
		return nil
	})
}
