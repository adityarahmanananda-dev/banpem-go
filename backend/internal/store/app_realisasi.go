package store

import (
	"context"
	"strings"
)

// PivotRow adalah satu baris rekap realisasi (diagregasi per kombinasi level
// Kegiatan/Sub Kegiatan/Aktivitas/Komponen yang dipilih).
type PivotRow struct {
	Kegiatan  string
	SubKeg    string
	Aktivitas string
	Komponen  string
	// Pagu adalah jumlah pagu anggaran komponen-komponen pada grup (dihitung
	// sekali per komponen, tidak berlipat walau komponen punya banyak realisasi).
	Pagu int64
	// Realisasi adalah total nilai realisasi (bruto) pada grup.
	Realisasi int64
	// Sisa = Pagu - Realisasi.
	Sisa int64
}

// KomponenRealisasi adalah komponen beserta nilai realisasinya.
type KomponenRealisasi struct {
	Komponen
	Realisasi int64
}

// AktivitasRealisasi adalah aktivitas beserta pagu dan realisasi komponennya.
type AktivitasRealisasi struct {
	Aktivitas
	Komponens []KomponenRealisasi
	Pagu      int64
	Realisasi int64
}

// SubRealisasi adalah sub kegiatan beserta agregasi aktivitasnya.
type SubRealisasi struct {
	SubKegiatan
	Aktivitass []AktivitasRealisasi
	Pagu       int64
	Realisasi  int64
}

// KegiatanRealisasi adalah kegiatan beserta agregasi subnya.
type KegiatanRealisasi struct {
	Kegiatan
	Subs      []SubRealisasi
	Pagu      int64
	Realisasi int64
}

// ListRealisasiTree mengembalikan pohon Kegiatan->Sub->Aktivitas->Komponen
// dengan nilai Pagu (dari komponen) dan Realisasi (dari trx_invoice_realisasi)
// teragregasi di tiap level, mirip tampilan RAB.
func (s *Store) ListRealisasiTree(ctx context.Context, bantuanID int64) ([]KegiatanRealisasi, error) {
	rows, err := s.Pool.Query(ctx, `SELECT ko.id, COALESCE(SUM(r.bruto),0)
		FROM trx_invoice_realisasi r
		JOIN trx_invoice i ON i.id = r.invoice_id
		JOIN komponen ko ON ko.id = r.komponen_id
		WHERE i.bantuan_id=$1
		GROUP BY ko.id`, bantuanID)
	if err != nil {
		return nil, err
	}
	real := map[int64]int64{}
	for rows.Next() {
		var id, v int64
		if err := rows.Scan(&id, &v); err != nil {
			rows.Close()
			return nil, err
		}
		real[id] = v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	kegs, err := s.ListKegiatanTree(ctx, bantuanID)
	if err != nil {
		return nil, err
	}
	var out []KegiatanRealisasi
	for _, k := range kegs {
		kr := KegiatanRealisasi{Kegiatan: k.Kegiatan}
		for _, sk := range k.Subs {
			sr := SubRealisasi{SubKegiatan: sk.SubKegiatan}
			for _, a := range sk.Aktivitass {
				ar := AktivitasRealisasi{Aktivitas: a.Aktivitas}
				for _, ko := range a.Komponens {
					krk := KomponenRealisasi{Komponen: ko, Realisasi: real[ko.ID]}
					ar.Komponens = append(ar.Komponens, krk)
					ar.Pagu += ko.Pagu
					ar.Realisasi += krk.Realisasi
				}
				sr.Aktivitass = append(sr.Aktivitass, ar)
				sr.Pagu += ar.Pagu
				sr.Realisasi += ar.Realisasi
			}
			kr.Subs = append(kr.Subs, sr)
			kr.Pagu += sr.Pagu
			kr.Realisasi += sr.Realisasi
		}
		out = append(out, kr)
	}
	return out, nil
}

// ListRealisasiPivot mengagregasi realisasi belanja berdasarkan level yang
// dipilih (groups berisi urutan "kegiatan", "sub", "aktivitas", "komponen").
// groups kosong => default semua level (paling detail). Mengembalikan baris
// + grand total. Pagu dihitung per komponen (tidak terhitung ganda) dengan
// cara mengagregasi realisasi ke level komponen terlebih dahulu.
func (s *Store) ListRealisasiPivot(ctx context.Context, bantuanID int64, groups []string) ([]PivotRow, PivotRow, error) {
	if len(groups) == 0 {
		groups = []string{"kegiatan", "sub", "aktivitas", "komponen"}
	}
	rows, err := s.Pool.Query(ctx, `SELECT k.nama, sk.nama, a.nama, ko.nama, ko.pagu,
		COALESCE(SUM(r.bruto),0)
		FROM trx_invoice_realisasi r
		JOIN trx_invoice i ON i.id = r.invoice_id
		JOIN komponen ko ON ko.id = r.komponen_id
		JOIN aktivitas a ON a.id = ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE i.bantuan_id=$1
		GROUP BY ko.id, k.id, sk.id, a.id, k.nama, sk.nama, a.nama, ko.nama
		ORDER BY k.id, sk.id, a.id, ko.id`, bantuanID)
	if err != nil {
		return nil, PivotRow{}, err
	}
	defer rows.Close()

	type komp struct {
		names [4]string
		pagu  int64
		real  int64
	}
	var krows []komp
	for rows.Next() {
		var k komp
		if err := rows.Scan(&k.names[0], &k.names[1], &k.names[2], &k.names[3], &k.pagu, &k.real); err != nil {
			return nil, PivotRow{}, err
		}
		krows = append(krows, k)
	}
	if err := rows.Err(); err != nil {
		return nil, PivotRow{}, err
	}

	var idx []int
	for _, g := range groups {
		switch g {
		case "kegiatan":
			idx = append(idx, 0)
		case "sub":
			idx = append(idx, 1)
		case "aktivitas":
			idx = append(idx, 2)
		case "komponen":
			idx = append(idx, 3)
		}
	}
	keyOf := func(k komp) string {
		parts := make([]string, len(idx))
		for i, j := range idx {
			parts[i] = k.names[j]
		}
		return strings.Join(parts, "\x00")
	}

	var out []PivotRow
	var total PivotRow
	prev := ""
	for _, k := range krows {
		kk := keyOf(k)
		if kk != prev {
			var p PivotRow
			for _, j := range idx {
				switch j {
				case 0:
					p.Kegiatan = k.names[j]
				case 1:
					p.SubKeg = k.names[j]
				case 2:
					p.Aktivitas = k.names[j]
				case 3:
					p.Komponen = k.names[j]
				}
			}
			p.Pagu = k.pagu
			p.Realisasi = k.real
			out = append(out, p)
			prev = kk
		} else {
			last := &out[len(out)-1]
			last.Pagu += k.pagu
			last.Realisasi += k.real
		}
		total.Pagu += k.pagu
		total.Realisasi += k.real
	}
	for i := range out {
		out[i].Sisa = out[i].Pagu - out[i].Realisasi
	}
	total.Sisa = total.Pagu - total.Realisasi
	return out, total, nil
}
