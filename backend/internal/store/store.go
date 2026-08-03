package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

// Migrate menjalankan semua file *.sql di fsys secara urut (idempotent).
func (s *Store) Migrate(ctx context.Context, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return err
		}
		if _, err := s.Pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("migrasi %s: %w", f, err)
		}
	}
	return nil
}

// Querier dipakai agar fungsi bisa menerima pool maupun transaksi.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Tx adalah alias transaksi pgx.
type Tx = pgx.Tx

func (s *Store) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ParseDate mengubah "YYYY-MM-DD" menjadi time.Time.
func ParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Now()
	}
	return t
}

// TodayString mengembalikan tanggal hari ini "YYYY-MM-DD".
func TodayString() string {
	return time.Now().Format("2006-01-02")
}
