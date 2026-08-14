package store

import (
	"context"
	"time"

	"ebku/internal/money"
)

const batasPencairanAwal = int64(10000000000) // 100.000.000 rupiah dalam sen

// Tahap1Nominal menghitung nominal tahap 1: jika nominal > 100 juta, 70%.
func Tahap1Nominal(nominal int64) int64 {
	if nominal > batasPencairanAwal {
		return money.MulDiv(nominal, 70, 100)
	}
	return nominal
}

func (s *Store) CreateBantuan(ctx context.Context, b Bantuan) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO bantuan
			(nama, nama_sekolah, nama_rekening, nomor_rekening, bank, npwp, kepala_sekolah, nip_kepala_sekolah, bendahara, nip_bendahara, nominal)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
			b.Nama, b.NamaSekolah, b.NamaRekening, b.NomorRekening, b.Bank, b.NPWP,
			b.KepalaSekolah, b.NIPKepalaSekolah, b.Bendahara, b.NIPBendahara, b.Nominal).Scan(&id)
		if err != nil {
			return err
		}
		if b.Nominal != nil && *b.Nominal > 0 {
			tanggal := time.Now()
			first := Tahap1Nominal(*b.Nominal)
			if _, err := tx.Exec(ctx, `INSERT INTO pencairan_hibah (bantuan_id, tahap, tanggal, nominal, keterangan)
				VALUES ($1,1,$2,$3,'Pencairan awal hibah')`, id, tanggal, first); err != nil {
				return err
			}
			// Pencairan pertama otomatis menjadi saldo awal.
			if err := setSaldoAwalTx(ctx, tx, id, first, tanggal); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

func (s *Store) UpdateBantuan(ctx context.Context, b Bantuan) error {
	return s.WithTx(ctx, func(tx Tx) error {
		_, err := tx.Exec(ctx, `UPDATE bantuan SET
			nama=$1, nama_sekolah=$2, nama_rekening=$3, nomor_rekening=$4, bank=$5, npwp=$6,
			kepala_sekolah=$7, nip_kepala_sekolah=$8, bendahara=$9, nip_bendahara=$10, nominal=$11
			WHERE id=$12`,
			b.Nama, b.NamaSekolah, b.NamaRekening, b.NomorRekening, b.Bank, b.NPWP,
			b.KepalaSekolah, b.NIPKepalaSekolah, b.Bendahara, b.NIPBendahara, b.Nominal, b.ID)
		return err
	})
}

func (s *Store) GetBantuan(ctx context.Context, id int64) (Bantuan, error) {
	var b Bantuan
	err := s.Pool.QueryRow(ctx, `SELECT id, nama, nama_sekolah, nama_rekening, nomor_rekening, bank, npwp,
		kepala_sekolah, nip_kepala_sekolah, bendahara, nip_bendahara, nominal, created_at
		FROM bantuan WHERE id=$1`, id).Scan(
		&b.ID, &b.Nama, &b.NamaSekolah, &b.NamaRekening, &b.NomorRekening, &b.Bank, &b.NPWP,
		&b.KepalaSekolah, &b.NIPKepalaSekolah, &b.Bendahara, &b.NIPBendahara, &b.Nominal, &b.CreatedAt)
	return b, err
}

func (s *Store) ListBantuan(ctx context.Context) ([]Bantuan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, nama, nama_sekolah, nama_rekening, nomor_rekening, bank, npwp,
		kepala_sekolah, nip_kepala_sekolah, bendahara, nip_bendahara, nominal, created_at
		FROM bantuan ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bantuan
	for rows.Next() {
		var b Bantuan
		if err := rows.Scan(&b.ID, &b.Nama, &b.NamaSekolah, &b.NamaRekening, &b.NomorRekening, &b.Bank, &b.NPWP,
			&b.KepalaSekolah, &b.NIPKepalaSekolah, &b.Bendahara, &b.NIPBendahara, &b.Nominal, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetSaldoAwal mengambil baris saldo_awal (nil bila belum ada).
func (s *Store) GetSaldoAwal(ctx context.Context, bantuanID int64) (*SaldoAwal, error) {
	var sa SaldoAwal
	err := s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, saldo_awal, tanggal FROM saldo_awal WHERE bantuan_id=$1`, bantuanID).
		Scan(&sa.ID, &sa.BantuanID, &sa.Saldo, &sa.Tanggal)
	if err != nil {
		return nil, err
	}
	return &sa, nil
}

func upsertSaldoAwalLedger(ctx context.Context, q Querier, bantuanID int64, table string, tanggal time.Time, saldo int64) error {
	var id int64
	sel := `SELECT id FROM trx_ledger WHERE bantuan_id=$1 AND jenis_transaksi='saldo_awal'`
	upd := `UPDATE trx_ledger SET tanggal=$1, nomor_bukti='', uraian='Saldo Awal', debit=0, kredit=0, saldo=$2 WHERE id=$3`
	ins := `INSERT INTO trx_ledger (bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi)
		VALUES ($1,1,$2,'','Saldo Awal',0,0,$3,NULL,'saldo_awal')`
	shift := `UPDATE trx_ledger SET nomor = nomor + 1 WHERE bantuan_id=$1`
	if table == "bank" {
		sel = `SELECT id FROM trx_bank_ledger WHERE bantuan_id=$1 AND jenis_transaksi='saldo_awal'`
		upd = `UPDATE trx_bank_ledger SET tanggal=$1, nomor_bukti='', uraian='Saldo Awal', debit=0, kredit=0, saldo=$2 WHERE id=$3`
		ins = `INSERT INTO trx_bank_ledger (bantuan_id, nomor, tanggal, nomor_bukti, uraian, debit, kredit, saldo, invoice_id, jenis_transaksi)
			VALUES ($1,1,$2,'','Saldo Awal',0,0,$3,NULL,'saldo_awal')`
		shift = `UPDATE trx_bank_ledger SET nomor = nomor + 1 WHERE bantuan_id=$1`
	}
	err := q.QueryRow(ctx, sel, bantuanID).Scan(&id)
	if err == nil {
		_, err = q.Exec(ctx, upd, tanggal, saldo, id)
		return err
	}
	if _, err := q.Exec(ctx, shift, bantuanID); err != nil {
		return err
	}
	_, err = q.Exec(ctx, ins, bantuanID, tanggal, saldo)
	return err
}

// setSaldoAwalTx menyimpan saldo awal dan memastikan entri 'saldo_awal' nomor 1
// di BKU dan Buku Kas Bank, dalam transaksi yang sedang berjalan.
func setSaldoAwalTx(ctx context.Context, q Querier, bantuanID, saldo int64, tanggal time.Time) error {
	if _, err := q.Exec(ctx, `INSERT INTO saldo_awal (bantuan_id, saldo_awal, tanggal) VALUES ($1,$2,$3)
		ON CONFLICT (bantuan_id) DO UPDATE SET saldo_awal=EXCLUDED.saldo_awal, tanggal=EXCLUDED.tanggal`,
		bantuanID, saldo, tanggal); err != nil {
		return err
	}
	if err := upsertSaldoAwalLedger(ctx, q, bantuanID, "bku", tanggal, saldo); err != nil {
		return err
	}
	if err := RebuildLedger(ctx, q, bantuanID, "bku"); err != nil {
		return err
	}
	if err := upsertSaldoAwalLedger(ctx, q, bantuanID, "bank", tanggal, saldo); err != nil {
		return err
	}
	return RebuildLedger(ctx, q, bantuanID, "bank")
}

// SetSaldoAwal menyimpan saldo awal dan memastikan entri 'saldo_awal' nomor 1
// di BKU dan Buku Kas Bank.
func (s *Store) SetSaldoAwal(ctx context.Context, bantuanID, saldo int64, tanggal time.Time) error {
	return s.WithTx(ctx, func(tx Tx) error {
		return setSaldoAwalTx(ctx, tx, bantuanID, saldo, tanggal)
	})
}

// EnsureAutoPencairan membuat pencairan tahap 1 otomatis bila nominal>0 dan
// belum ada pencairan sama sekali.
func (s *Store) EnsureAutoPencairan(ctx context.Context, bantuanID int64) error {
	b, err := s.GetBantuan(ctx, bantuanID)
	if err != nil {
		return err
	}
	if b.Nominal == nil || *b.Nominal <= 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx Tx) error {
		var cnt int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM pencairan_hibah WHERE bantuan_id=$1`, bantuanID).Scan(&cnt); err != nil {
			return err
		}
		if cnt > 0 {
			return nil
		}
		first := Tahap1Nominal(*b.Nominal)
		tanggal := time.Now()
		if _, err := tx.Exec(ctx, `INSERT INTO pencairan_hibah (bantuan_id, tahap, tanggal, nominal, keterangan)
			VALUES ($1,1,$2,$3,'Pencairan awal hibah')`, bantuanID, tanggal, first); err != nil {
			return err
		}
		return setSaldoAwalTx(ctx, tx, bantuanID, first, tanggal)
	})
}

func (s *Store) ListPencairan(ctx context.Context, bantuanID int64) ([]Pencairan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, bantuan_id, tahap, tanggal, nominal, keterangan
		FROM pencairan_hibah WHERE bantuan_id=$1 ORDER BY tahap, id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pencairan
	for rows.Next() {
		var p Pencairan
		if err := rows.Scan(&p.ID, &p.BantuanID, &p.Tahap, &p.Tanggal, &p.Nominal, &p.Keterangan); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPencairan(ctx context.Context, id int64) (Pencairan, error) {
	var p Pencairan
	err := s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, tahap, tanggal, nominal, keterangan FROM pencairan_hibah WHERE id=$1`, id).
		Scan(&p.ID, &p.BantuanID, &p.Tahap, &p.Tanggal, &p.Nominal, &p.Keterangan)
	return p, err
}

func (s *Store) TotalTurun(ctx context.Context, bantuanID int64) int64 {
	var v int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(nominal),0) FROM pencairan_hibah WHERE bantuan_id=$1`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

// FirstPencairanNominal mengambil nominal pencairan pertama (tahap terkecil).
func (s *Store) FirstPencairanNominal(ctx context.Context, bantuanID int64) int64 {
	var v int64
	err := s.Pool.QueryRow(ctx, `SELECT nominal FROM pencairan_hibah WHERE bantuan_id=$1 ORDER BY tahap, id LIMIT 1`, bantuanID).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

func (s *Store) CreatePencairan(ctx context.Context, bantuanID int64, p Pencairan) error {
	return s.WithTx(ctx, func(tx Tx) error {
		var maxTahap int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(tahap),0) FROM pencairan_hibah WHERE bantuan_id=$1`, bantuanID).Scan(&maxTahap); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pencairan_hibah (bantuan_id, tahap, tanggal, nominal, keterangan)
			VALUES ($1,$2,$3,$4,$5)`, bantuanID, maxTahap+1, p.Tanggal, p.Nominal, p.Keterangan); err != nil {
			return err
		}
		// Pencairan pertama otomatis menjadi saldo awal bila saldo awal belum
		// pernah diatur (apapun nilai nominal bantuan).
		if maxTahap == 0 && p.Tanggal != nil {
			var cnt int
			if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM saldo_awal WHERE bantuan_id=$1`, bantuanID).Scan(&cnt); err == nil && cnt == 0 {
				if err := setSaldoAwalTx(ctx, tx, bantuanID, p.Nominal, *p.Tanggal); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Store) UpdatePencairan(ctx context.Context, p Pencairan) error {
	_, err := s.Pool.Exec(ctx, `UPDATE pencairan_hibah SET tahap=$1, tanggal=$2, nominal=$3, keterangan=$4 WHERE id=$5`,
		p.Tahap, p.Tanggal, p.Nominal, p.Keterangan, p.ID)
	return err
}

func (s *Store) DeletePencairan(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM pencairan_hibah WHERE id=$1`, id)
	return err
}
