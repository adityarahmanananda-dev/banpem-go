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
	Bruto     int64
	PPN       int64
	PPH       int64
	Netto     int64
}

func pivotCol(g string) string {
	switch g {
	case "kegiatan":
		return "k.nama"
	case "sub":
		return "sk.nama"
	case "aktivitas":
		return "a.nama"
	default:
		return "ko.nama"
	}
}

// ListRealisasiPivot mengagregasi realisasi belanja berdasarkan level yang
// dipilih (groups berisi urutan "kegiatan", "sub", "aktivitas", "komponen").
// groups kosong => default semua level (paling detail). Mengembalikan baris
// + grand total.
func (s *Store) ListRealisasiPivot(ctx context.Context, bantuanID int64, groups []string) ([]PivotRow, PivotRow, error) {
	if len(groups) == 0 {
		groups = []string{"kegiatan", "sub", "aktivitas", "komponen"}
	}
	var sel []string
	for _, g := range groups {
		sel = append(sel, pivotCol(g))
	}
	groupBy := strings.Join(sel, ", ")
	orderBy := strings.Join(sel, ", ")

	rows, err := s.Pool.Query(ctx, `SELECT `+groupBy+`,
		COALESCE(SUM(r.bruto),0), COALESCE(SUM(r.nilai_ppn),0), COALESCE(SUM(r.nilai_pph),0), COALESCE(SUM(r.nilai_netto),0)
		FROM trx_invoice_realisasi r
		JOIN trx_invoice i ON i.id = r.invoice_id
		JOIN komponen ko ON ko.id = r.komponen_id
		JOIN aktivitas a ON a.id = ko.aktivitas_id
		JOIN sub_kegiatan sk ON sk.id = a.sub_kegiatan_id
		JOIN kegiatan k ON k.id = sk.kegiatan_id
		WHERE i.bantuan_id=$1
		GROUP BY `+groupBy+`
		ORDER BY `+orderBy, bantuanID)
	if err != nil {
		return nil, PivotRow{}, err
	}
	defer rows.Close()

	var out []PivotRow
	var total PivotRow
	for rows.Next() {
		var row PivotRow
		var dst []any
		for _, g := range groups {
			switch g {
			case "kegiatan":
				dst = append(dst, &row.Kegiatan)
			case "sub":
				dst = append(dst, &row.SubKeg)
			case "aktivitas":
				dst = append(dst, &row.Aktivitas)
			case "komponen":
				dst = append(dst, &row.Komponen)
			}
		}
		dst = append(dst, &row.Bruto, &row.PPN, &row.PPH, &row.Netto)
		if err := rows.Scan(dst...); err != nil {
			return nil, PivotRow{}, err
		}
		out = append(out, row)
		total.Bruto += row.Bruto
		total.PPN += row.PPN
		total.PPH += row.PPH
		total.Netto += row.Netto
	}
	return out, total, rows.Err()
}
