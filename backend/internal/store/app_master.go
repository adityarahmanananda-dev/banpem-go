package store

import (
	"context"
)

func (s *Store) CreateKegiatan(ctx context.Context, bantuanID int64, nama string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO kegiatan (bantuan_id, nama) VALUES ($1,$2) RETURNING id`, bantuanID, nama).Scan(&id)
	return id, err
}

func (s *Store) DeleteKegiatan(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM kegiatan WHERE id=$1`, id)
	return err
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

func (s *Store) GetKegiatan(ctx context.Context, id int64) (Kegiatan, error) {
	var k Kegiatan
	err := s.Pool.QueryRow(ctx, `SELECT id, bantuan_id, nama FROM kegiatan WHERE id=$1`, id).Scan(&k.ID, &k.BantuanID, &k.Nama)
	return k, err
}

func (s *Store) CreateSubKegiatan(ctx context.Context, kegiatanID int64, nama string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO sub_kegiatan (kegiatan_id, nama) VALUES ($1,$2) RETURNING id`, kegiatanID, nama).Scan(&id)
	return id, err
}

func (s *Store) DeleteSubKegiatan(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sub_kegiatan WHERE id=$1`, id)
	return err
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

func (s *Store) GetSubKegiatan(ctx context.Context, id int64) (SubKegiatan, error) {
	var k SubKegiatan
	err := s.Pool.QueryRow(ctx, `SELECT id, kegiatan_id, nama FROM sub_kegiatan WHERE id=$1`, id).Scan(&k.ID, &k.KegiatanID, &k.Nama)
	return k, err
}

func (s *Store) CreateKomponen(ctx context.Context, subKegiatanID int64, nama string, pagu int64) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO komponen (sub_kegiatan_id, nama, pagu) VALUES ($1,$2,$3) RETURNING id`, subKegiatanID, nama, pagu).Scan(&id)
	return id, err
}

func (s *Store) UpdateKomponen(ctx context.Context, id int64, nama string, pagu int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE komponen SET nama=$2, pagu=$3 WHERE id=$1`, id, nama, pagu)
	return err
}

func (s *Store) DeleteKomponen(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM komponen WHERE id=$1`, id)
	return err
}

func (s *Store) ListKomponen(ctx context.Context, subKegiatanID int64) ([]Komponen, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, sub_kegiatan_id, nama, pagu FROM komponen WHERE sub_kegiatan_id=$1 ORDER BY id`, subKegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Komponen
	for rows.Next() {
		var k Komponen
		if err := rows.Scan(&k.ID, &k.SubKegiatanID, &k.Nama, &k.Pagu); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListKomponenBantuan mengambil komponen untuk bantuan + sub kegiatan tertentu.
func (s *Store) ListKomponenBantuan(ctx context.Context, bantuanID, subKegiatanID int64) ([]Komponen, error) {
	rows, err := s.Pool.Query(ctx, `SELECT ko.id, ko.sub_kegiatan_id, ko.nama, ko.pagu
		FROM komponen ko
		JOIN sub_kegiatan sk ON sk.id = ko.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE k.bantuan_id=$1 AND ko.sub_kegiatan_id=$2 ORDER BY ko.id`, bantuanID, subKegiatanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Komponen
	for rows.Next() {
		var k Komponen
		if err := rows.Scan(&k.ID, &k.SubKegiatanID, &k.Nama, &k.Pagu); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetKomponen(ctx context.Context, id int64) (Komponen, error) {
	var k Komponen
	err := s.Pool.QueryRow(ctx, `SELECT id, sub_kegiatan_id, nama, pagu FROM komponen WHERE id=$1`, id).Scan(&k.ID, &k.SubKegiatanID, &k.Nama, &k.Pagu)
	return k, err
}

type KomponenChain struct {
	KegiatanID   int64
	KegiatanNama string
	SubID        int64
	SubNama      string
	KomponenID   int64
	KomponenNama string
}

// GetKomponenChain mengambil rantai master data sebuah komponen.
func (s *Store) GetKomponenChain(ctx context.Context, komponenID int64) (KomponenChain, error) {
	var c KomponenChain
	err := s.Pool.QueryRow(ctx, `SELECT k.id, k.nama, sk.id, sk.nama, ko.id, ko.nama
		FROM komponen ko
		JOIN sub_kegiatan sk ON sk.id = ko.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE ko.id=$1`, komponenID).Scan(
		&c.KegiatanID, &c.KegiatanNama, &c.SubID, &c.SubNama, &c.KomponenID, &c.KomponenNama)
	return c, err
}

type KegiatanTree struct {
	Kegiatan
	Subs []SubTree
	Pagu int64
}

type SubTree struct {
	SubKegiatan
	Komponens []Komponen
	Pagu      int64
}

// ListKegiatanTree mengambil pohon Kegiatan->Sub->Komponen untuk halaman
// Data Kegiatan.
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
			ks, err := s.ListKomponen(ctx, sk.ID)
			if err != nil {
				return nil, err
			}
			for _, ko := range ks {
				st.Pagu += ko.Pagu
			}
			st.Komponens = ks
			t.Pagu += st.Pagu
			t.Subs = append(t.Subs, st)
		}
		out = append(out, t)
	}
	return out, nil
}
