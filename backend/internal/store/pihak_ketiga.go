package store

import (
	"context"
	"strings"
)

type PihakKetiga struct {
	ID       int64
	Nama     string
	NPWP     string
	Rekening []PihakKetigaRekening
}

type PihakKetigaRekening struct {
	ID            int64
	PihakKetigaID int64
	NamaRekening  string
	NomorRekening string
	Bank          string
}

func (s *Store) ListPihakKetiga(ctx context.Context) ([]PihakKetiga, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, nama, npwp FROM pihak_ketiga ORDER BY lower(nama), id`)
	if err != nil {
		return nil, err
	}
	var out []PihakKetiga
	index := map[int64]int{}
	for rows.Next() {
		var p PihakKetiga
		if err := rows.Scan(&p.ID, &p.Nama, &p.NPWP); err != nil {
			rows.Close()
			return nil, err
		}
		index[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.Pool.Query(ctx, `SELECT id, pihak_ketiga_id, nama_rekening, nomor_rekening, bank
		FROM pihak_ketiga_rekening ORDER BY lower(bank), lower(nama_rekening), nomor_rekening, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r PihakKetigaRekening
		if err := rows.Scan(&r.ID, &r.PihakKetigaID, &r.NamaRekening, &r.NomorRekening, &r.Bank); err != nil {
			return nil, err
		}
		if i, ok := index[r.PihakKetigaID]; ok {
			out[i].Rekening = append(out[i].Rekening, r)
		}
	}
	return out, rows.Err()
}

func (s *Store) GetPihakKetiga(ctx context.Context, id int64) (PihakKetiga, error) {
	var p PihakKetiga
	if err := s.Pool.QueryRow(ctx, `SELECT id, nama, npwp FROM pihak_ketiga WHERE id=$1`, id).
		Scan(&p.ID, &p.Nama, &p.NPWP); err != nil {
		return PihakKetiga{}, ErrNotFound
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, pihak_ketiga_id, nama_rekening, nomor_rekening, bank
		FROM pihak_ketiga_rekening WHERE pihak_ketiga_id=$1 ORDER BY lower(bank), lower(nama_rekening), nomor_rekening, id`, id)
	if err != nil {
		return PihakKetiga{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var r PihakKetigaRekening
		if err := rows.Scan(&r.ID, &r.PihakKetigaID, &r.NamaRekening, &r.NomorRekening, &r.Bank); err != nil {
			return PihakKetiga{}, err
		}
		p.Rekening = append(p.Rekening, r)
	}
	return p, rows.Err()
}

func (s *Store) CreatePihakKetiga(ctx context.Context, nama, npwp string) (int64, error) {
	nama = strings.TrimSpace(nama)
	if nama == "" {
		return 0, ErrInvalidInput
	}
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO pihak_ketiga (nama, npwp) VALUES ($1,$2) RETURNING id`,
		nama, strings.TrimSpace(npwp)).Scan(&id)
	return id, err
}

func (s *Store) UpdatePihakKetiga(ctx context.Context, id int64, nama, npwp string) error {
	nama = strings.TrimSpace(nama)
	if nama == "" {
		return ErrInvalidInput
	}
	_, err := s.Pool.Exec(ctx, `UPDATE pihak_ketiga SET nama=$1, npwp=$2, updated_at=now() WHERE id=$3`,
		nama, strings.TrimSpace(npwp), id)
	return err
}

func (s *Store) DeletePihakKetiga(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM pihak_ketiga WHERE id=$1`, id)
	return err
}

func (s *Store) AddPihakKetigaRekening(ctx context.Context, partyID int64, nama, nomor, bank string) (int64, error) {
	if partyID <= 0 {
		return 0, ErrInvalidInput
	}
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO pihak_ketiga_rekening
		(pihak_ketiga_id, nama_rekening, nomor_rekening, bank)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		partyID, strings.TrimSpace(nama), strings.TrimSpace(nomor), strings.TrimSpace(bank)).Scan(&id)
	return id, err
}

func (s *Store) UpdatePihakKetigaRekening(ctx context.Context, id, partyID int64, nama, nomor, bank string) error {
	res, err := s.Pool.Exec(ctx, `UPDATE pihak_ketiga_rekening
		SET nama_rekening=$1, nomor_rekening=$2, bank=$3
		WHERE id=$4 AND pihak_ketiga_id=$5`,
		strings.TrimSpace(nama), strings.TrimSpace(nomor), strings.TrimSpace(bank), id, partyID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePihakKetigaRekening(ctx context.Context, id, partyID int64) error {
	res, err := s.Pool.Exec(ctx, `DELETE FROM pihak_ketiga_rekening WHERE id=$1 AND pihak_ketiga_id=$2`, id, partyID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// resolvePihakKetiga menimpa snapshot rekening/NPWP invoice dari master data
// bila user memilih pihak ketiga/rekening. Ini mencegah client memalsukan
// nomor rekening saat memilih master.
func resolvePihakKetiga(ctx context.Context, q Querier, in *InvoiceInput) error {
	if in.PihakKetigaID == nil && in.PihakKetigaRekeningID == nil {
		return nil
	}
	var partyID int64
	if in.PihakKetigaID != nil {
		partyID = *in.PihakKetigaID
	}
	if in.PihakKetigaRekeningID != nil {
		var ridParty int64
		if err := q.QueryRow(ctx, `SELECT pihak_ketiga_id FROM pihak_ketiga_rekening WHERE id=$1`,
			*in.PihakKetigaRekeningID).Scan(&ridParty); err != nil {
			return ErrNotFound
		}
		if partyID != 0 && partyID != ridParty {
			return ErrInvalidInput
		}
		partyID = ridParty
	}
	var nama, npwp string
	if err := q.QueryRow(ctx, `SELECT nama, npwp FROM pihak_ketiga WHERE id=$1`, partyID).
		Scan(&nama, &npwp); err != nil {
		return ErrNotFound
	}
	in.PihakKetigaID = &partyID
	in.NPWP = npwp
	if in.PihakKetigaRekeningID != nil {
		var rNama, rNomor, rBank string
		if err := q.QueryRow(ctx, `SELECT nama_rekening, nomor_rekening, bank
			FROM pihak_ketiga_rekening WHERE id=$1 AND pihak_ketiga_id=$2`,
			*in.PihakKetigaRekeningID, partyID).Scan(&rNama, &rNomor, &rBank); err != nil {
			return ErrNotFound
		}
		in.NamaRekening = rNama
		in.NomorRekening = rNomor
		in.Bank = rBank
	} else if strings.TrimSpace(in.NamaRekening) == "" {
		in.NamaRekening = nama
	}
	return nil
}
