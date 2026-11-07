package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidDate = errors.New("tanggal tidak valid (format YYYY-MM-DD)")

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

// Ping memeriksa koneksi database (dipakai oleh /healthz).
func (s *Store) Ping(ctx context.Context) error {
	return s.Pool.Ping(ctx)
}

// Migrate menjalankan migration yang belum pernah diterapkan. Setiap file
// dicatat di schema_migrations dan dijalankan dalam transaksi. Advisory lock
// mencegah dua instance menjalankan migration bersamaan.
func (s *Store) Migrate(ctx context.Context, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('banpem-go-migrations'))`); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('banpem-go-migrations'))`)
	}()
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	for _, f := range files {
		var applied bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`,
			f).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(b)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrasi %s: %w", f, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, f); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
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

// WithBantuanTx menjalankan transaksi dengan advisory lock per bantuan.
// Lock ini mencegah dua request bersamaan menghitung nomor/saldo dari nilai
// yang sama (race condition MAX(nomor)+1 dan semacamnya).
func (s *Store) WithBantuanTx(ctx context.Context, bantuanID int64, fn func(tx pgx.Tx) error) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bantuanID); err != nil {
			return err
		}
		return fn(tx)
	})
}

// ParseDate mengubah "YYYY-MM-DD" menjadi time.Time.
// Nilai kosong/invalid tetap memakai waktu sekarang untuk kompatibilitas
// pemanggil lama. Untuk input form, gunakan ParseDateStrict.
func ParseDate(s string) time.Time {
	t, err := ParseDateStrict(s)
	if err != nil {
		return time.Now()
	}
	return t
}

// ParseDateStrict mengembalikan error bila tanggal kosong atau tidak valid.
func ParseDateStrict(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, ErrInvalidDate
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, ErrInvalidDate
	}
	return t, nil
}

// TodayString mengembalikan tanggal hari ini "YYYY-MM-DD".
func TodayString() string {
	return time.Now().Format("2006-01-02")
}
