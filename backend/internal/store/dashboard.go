package store

import (
	"context"
	"time"
)

type MonthlyBelanja struct {
	Bulan time.Time
	Total int64
}

// MonthlyBelanja mengembalikan total bruto tagihan per bulan untuk N bulan
// terakhir (termasuk bulan berjalan). bantuanID=0 berarti semua bantuan.
// Bulan tanpa transaksi diisi 0.
func (s *Store) MonthlyBelanja(ctx context.Context, bantuanID int64, months int) ([]MonthlyBelanja, error) {
	if months <= 0 || months > 60 {
		months = 12
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)
	rows, err := s.Pool.Query(ctx, `SELECT date_trunc('month', tanggal)::date, COALESCE(SUM(bruto),0)
		FROM trx_invoice
		WHERE tanggal >= $1 AND ($2 = 0 OR bantuan_id = $2)
		GROUP BY 1 ORDER BY 1`, start, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[string]int64{}
	for rows.Next() {
		var bulan time.Time
		var total int64
		if err := rows.Scan(&bulan, &total); err != nil {
			return nil, err
		}
		values[bulan.Format("2006-01")] = total
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]MonthlyBelanja, 0, months)
	for i := 0; i < months; i++ {
		b := start.AddDate(0, i, 0)
		out = append(out, MonthlyBelanja{Bulan: b, Total: values[b.Format("2006-01")]})
	}
	return out, nil
}
