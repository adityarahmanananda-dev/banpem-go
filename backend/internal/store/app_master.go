package store

import (
	"context"
)

func (s *Store) CreateKegiatan(ctx context.Context, bantuanID int64, nama string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO kegiatan (bantuan_id, nama) VALUES ($1,$2) RETURNING id`, bantuanID, nama).Scan(&id)
	return id, err
}

func (s *Store) DeleteKegiatan(ctx context.Context, bantuanID, id int64) error {
	return s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM trx_invoice_realisasi r
			JOIN komponen ko ON ko.id=r.komponen_id
			JOIN aktivitas a ON a.id=ko.aktivitas_id
			JOIN sub_kegiatan sk ON sk.id=a.sub_kegiatan_id
			WHERE sk.kegiatan_id=$1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrInUse
		}
		_, err := tx.Exec(ctx, `DELETE FROM kegiatan WHERE id=$1 AND bantuan_id=$2`, id, bantuanID)
		return err
	})
}

func (s *Store) ListKegiatan(ctx context.Context, bantuanID int64) ([]Kegiatan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, bantuan_id, nama FROM kegiatan WHERE bantuan_id=$1 ORDER BY id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Kegiatan
	for rows.Next() {
		var k Kegiatan
		if err := rows.Scan(&k.ID, &k.BantuanID, &k.Nama); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetKegiatan(ctx context.Context, bantuanID, id int64) (Kegiatan, error) {
	var k Kegiatan
	err := s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, nama FROM kegiatan
		WHERE id=$1 AND bantuan_id=$2`, id, bantuanID).Scan(&k.ID, &k.BantuanID, &k.Nama)
	return k, err
}

func (s *Store) CreateSubKegiatan(ctx context.Context, bantuanID, kegiatanID int64, nama string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO sub_kegiatan (kegiatan_id, nama)
		SELECT id, $2 FROM kegiatan WHERE id=$1 AND bantuan_id=$3 RETURNING id`,
		kegiatanID, nama, bantuanID).Scan(&id)
	return id, err
}

func (s *Store) DeleteSubKegiatan(ctx context.Context, bantuanID, id int64) error {
	return s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM trx_invoice_realisasi r
			JOIN komponen ko ON ko.id=r.komponen_id
			JOIN aktivitas a ON a.id=ko.aktivitas_id
			WHERE a.sub_kegiatan_id=$1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrInUse
		}
		_, err := tx.Exec(ctx, `DELETE FROM sub_kegiatan sk
			USING kegiatan k WHERE sk.id=$1 AND sk.kegiatan_id=k.id AND k.bantuan_id=$2`, id, bantuanID)
		return err
	})
}

func (s *Store) ListSubKegiatan(ctx context.Context, kegiatanID int64) ([]SubKegiatan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, kegiatan_id, nama FROM sub_kegiatan WHERE kegiatan_id=$1 ORDER BY id`, kegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubKegiatan
	for rows.Next() {
		var k SubKegiatan
		if err := rows.Scan(&k.ID, &k.KegiatanID, &k.Nama); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListSubKegiatanBantuan mengambil sub kegiatan untuk bantuan + kegiatan
// tertentu (cascading dropdown tagihan).
func (s *Store) ListSubKegiatanBantuan(ctx context.Context, bantuanID, kegiatanID int64) ([]SubKegiatan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT sk.id, sk.kegiatan_id, sk.nama
		FROM sub_kegiatan sk JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE k.bantuan_id=$1 AND sk.kegiatan_id=$2 ORDER BY sk.id`, bantuanID, kegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubKegiatan
	for rows.Next() {
		var k SubKegiatan
		if err := rows.Scan(&k.ID, &k.KegiatanID, &k.Nama); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetSubKegiatan(ctx context.Context, bantuanID, id int64) (SubKegiatan, error) {
	var k SubKegiatan
	err := s.Pool.QueryRow(ctx, `SELECT sk.id, sk.kegiatan_id, sk.nama
		FROM sub_kegiatan sk JOIN kegiatan k ON k.id=sk.kegiatan_id
		WHERE sk.id=$1 AND k.bantuan_id=$2`, id, bantuanID).Scan(&k.ID, &k.KegiatanID, &k.Nama)
	return k, err
}

func (s *Store) CreateAktivitas(ctx context.Context, bantuanID, subKegiatanID int64, nama string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO aktivitas (sub_kegiatan_id, nama)
		SELECT sk.id, $2 FROM sub_kegiatan sk
		JOIN kegiatan k ON k.id=sk.kegiatan_id
		WHERE sk.id=$1 AND k.bantuan_id=$3 RETURNING id`,
		subKegiatanID, nama, bantuanID).Scan(&id)
	return id, err
}

func (s *Store) DeleteAktivitas(ctx context.Context, bantuanID, id int64) error {
	return s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM trx_invoice_realisasi r
			JOIN komponen ko ON ko.id=r.komponen_id
			WHERE ko.aktivitas_id=$1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrInUse
		}
		_, err := tx.Exec(ctx, `DELETE FROM aktivitas a
			USING sub_kegiatan sk, kegiatan k
			WHERE a.id=$1 AND a.sub_kegiatan_id=sk.id AND sk.kegiatan_id=k.id AND k.bantuan_id=$2`,
			id, bantuanID)
		return err
	})
}

func (s *Store) ListAktivitas(ctx context.Context, subKegiatanID int64) ([]Aktivitas, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, sub_kegiatan_id, nama FROM aktivitas WHERE sub_kegiatan_id=$1 ORDER BY id`, subKegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Aktivitas
	for rows.Next() {
		var a Aktivitas
		if err := rows.Scan(&a.ID, &a.SubKegiatanID, &a.Nama); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAktivitasBantuan mengambil aktivitas untuk bantuan + sub kegiatan tertentu
// (cascading dropdown tagihan).
func (s *Store) ListAktivitasBantuan(ctx context.Context, bantuanID, subKegiatanID int64) ([]Aktivitas, error) {
	rows, err := s.Pool.Query(ctx, `SELECT a.id, a.sub_kegiatan_id, a.nama
		FROM aktivitas a
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE k.bantuan_id=$1 AND a.sub_kegiatan_id=$2 ORDER BY a.id`, bantuanID, subKegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Aktivitas
	for rows.Next() {
		var a Aktivitas
		if err := rows.Scan(&a.ID, &a.SubKegiatanID, &a.Nama); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAktivitas(ctx context.Context, bantuanID, id int64) (Aktivitas, error) {
	var a Aktivitas
	err := s.Pool.QueryRow(ctx, `SELECT a.id, a.sub_kegiatan_id, a.nama
		FROM aktivitas a
		JOIN sub_kegiatan sk ON sk.id=a.sub_kegiatan_id
		JOIN kegiatan k ON k.id=sk.kegiatan_id
		WHERE a.id=$1 AND k.bantuan_id=$2`, id, bantuanID).Scan(&a.ID, &a.SubKegiatanID, &a.Nama)
	return a, err
}

func (s *Store) CreateKomponen(ctx context.Context, bantuanID, aktivitasID int64, nama string, pagu int64) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO komponen (sub_kegiatan_id, aktivitas_id, nama, pagu)
		SELECT a.sub_kegiatan_id, a.id, $2, $3
		FROM aktivitas a
		JOIN sub_kegiatan sk ON sk.id=a.sub_kegiatan_id
		JOIN kegiatan k ON k.id=sk.kegiatan_id
		WHERE a.id=$1 AND k.bantuan_id=$4 RETURNING id`,
		aktivitasID, nama, pagu, bantuanID).Scan(&id)
	return id, err
}

// CreateKomponens membuat beberapa komponen dalam satu transaksi agar tidak
// ada penyimpanan parsial saat salah satu baris gagal.
func (s *Store) CreateKomponens(ctx context.Context, bantuanID, aktivitasID int64, inputs []KomponenInput) (int, error) {
	created := 0
	err := s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		for _, in := range inputs {
			var id int64
			if err := tx.QueryRow(ctx, `INSERT INTO komponen (sub_kegiatan_id, aktivitas_id, nama, pagu)
				SELECT a.sub_kegiatan_id, a.id, $2, $3
				FROM aktivitas a
				JOIN sub_kegiatan sk ON sk.id=a.sub_kegiatan_id
				JOIN kegiatan k ON k.id=sk.kegiatan_id
				WHERE a.id=$1 AND k.bantuan_id=$4 RETURNING id`,
				aktivitasID, in.Nama, in.Pagu, bantuanID).Scan(&id); err != nil {
				return err
			}
			created++
		}
		return nil
	})
	return created, err
}

type KomponenInput struct {
	Nama string
	Pagu int64
}

func (s *Store) UpdateKomponen(ctx context.Context, bantuanID, id int64, nama string, pagu int64) error {
	return s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		var realisasi int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(bruto),0) FROM trx_invoice_realisasi WHERE komponen_id=$1`,
			id).Scan(&realisasi); err != nil {
			return err
		}
		if pagu > 0 && realisasi > pagu {
			return ErrBudgetExceeded
		}
		_, err := tx.Exec(ctx, `UPDATE komponen ko SET nama=$3, pagu=$4
			FROM aktivitas a, sub_kegiatan sk, kegiatan k
			WHERE ko.id=$2 AND ko.aktivitas_id=a.id AND a.sub_kegiatan_id=sk.id
			  AND sk.kegiatan_id=k.id AND k.bantuan_id=$1`,
			bantuanID, id, nama, pagu)
		return err
	})
}

func (s *Store) DeleteKomponen(ctx context.Context, bantuanID, id int64) error {
	return s.WithBantuanTx(ctx, bantuanID, func(tx Tx) error {
		var used bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM trx_invoice_realisasi WHERE komponen_id=$1)`, id).Scan(&used); err != nil {
			return err
		}
		if used {
			return ErrInUse
		}
		_, err := tx.Exec(ctx, `DELETE FROM komponen ko
			USING aktivitas a, sub_kegiatan sk, kegiatan k
			WHERE ko.id=$1 AND ko.aktivitas_id=a.id AND a.sub_kegiatan_id=sk.id
			  AND sk.kegiatan_id=k.id AND k.bantuan_id=$2`, id, bantuanID)
		return err
	})
}

func (s *Store) ListKomponen(ctx context.Context, aktivitasID int64) ([]Komponen, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, sub_kegiatan_id, aktivitas_id, nama, pagu FROM komponen WHERE aktivitas_id=$1 ORDER BY id`, aktivitasID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Komponen
	for rows.Next() {
		var k Komponen
		if err := rows.Scan(&k.ID, &k.SubKegiatanID, &k.AktivitasID, &k.Nama, &k.Pagu); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListKomponenBantuan mengambil komponen untuk bantuan + aktivitas tertentu.
func (s *Store) ListKomponenBantuan(ctx context.Context, bantuanID, aktivitasID int64) ([]Komponen, error) {
	rows, err := s.Pool.Query(ctx, `SELECT ko.id, ko.sub_kegiatan_id, ko.aktivitas_id, ko.nama, ko.pagu
		FROM komponen ko
		JOIN aktivitas a ON a.id = ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE k.bantuan_id=$1 AND ko.aktivitas_id=$2 ORDER BY ko.id`, bantuanID, aktivitasID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Komponen
	for rows.Next() {
		var k Komponen
		if err := rows.Scan(&k.ID, &k.SubKegiatanID, &k.AktivitasID, &k.Nama, &k.Pagu); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetKomponen(ctx context.Context, bantuanID, id int64) (Komponen, error) {
	var k Komponen
	err := s.Pool.QueryRow(ctx, `SELECT ko.id, ko.sub_kegiatan_id, ko.aktivitas_id, ko.nama, ko.pagu
		FROM komponen ko
		JOIN aktivitas a ON a.id=ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id=a.sub_kegiatan_id
		JOIN kegiatan k ON k.id=sk.kegiatan_id
		WHERE ko.id=$1 AND k.bantuan_id=$2`, id, bantuanID).Scan(
		&k.ID, &k.SubKegiatanID, &k.AktivitasID, &k.Nama, &k.Pagu)
	return k, err
}

type KomponenChain struct {
	BantuanID     int64
	KegiatanID    int64
	KegiatanNama  string
	SubID         int64
	SubNama       string
	AktivitasID   int64
	AktivitasNama string
	KomponenID    int64
	KomponenNama  string
}

// GetKomponenChain mengambil rantai master data sebuah komponen.
func (s *Store) GetKomponenChain(ctx context.Context, komponenID int64) (KomponenChain, error) {
	var c KomponenChain
	err := s.Pool.QueryRow(ctx, `SELECT k.bantuan_id, k.id, k.nama, sk.id, sk.nama, a.id, a.nama, ko.id, ko.nama
		FROM komponen ko
		JOIN aktivitas a ON a.id = ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE ko.id=$1`, komponenID).Scan(
		&c.BantuanID, &c.KegiatanID, &c.KegiatanNama, &c.SubID, &c.SubNama,
		&c.AktivitasID, &c.AktivitasNama, &c.KomponenID, &c.KomponenNama)
	return c, err
}

type KegiatanTree struct {
	Kegiatan
	Subs []SubTree
	Pagu int64
}

type SubTree struct {
	SubKegiatan
	Aktivitass []AktivitasNode
	Pagu       int64
}

type AktivitasNode struct {
	Aktivitas
	Komponens []Komponen
	Pagu      int64
}

// ListKegiatanTree mengambil pohon Kegiatan->Sub->Aktivitas->Komponen untuk
// halaman Data Kegiatan.
func (s *Store) ListKegiatanTree(ctx context.Context, bantuanID int64) ([]KegiatanTree, error) {
	kegs, err := s.ListKegiatan(ctx, bantuanID)
	if err != nil {
		return nil, err
	}
	var out []KegiatanTree
	for _, k := range kegs {
		t := KegiatanTree{Kegiatan: k}
		subs, err := s.ListSubKegiatan(ctx, k.ID)
		if err != nil {
			return nil, err
		}
		for _, sk := range subs {
			st := SubTree{SubKegiatan: sk}
			acts, err := s.ListAktivitas(ctx, sk.ID)
			if err != nil {
				return nil, err
			}
			for _, a := range acts {
				an := AktivitasNode{Aktivitas: a}
				ks, err := s.ListKomponen(ctx, a.ID)
				if err != nil {
					return nil, err
				}
				for _, ko := range ks {
					an.Pagu += ko.Pagu
				}
				an.Komponens = ks
				st.Pagu += an.Pagu
				st.Aktivitass = append(st.Aktivitass, an)
			}
			t.Pagu += st.Pagu
			t.Subs = append(t.Subs, st)
		}
		out = append(out, t)
	}
	return out, nil
}
