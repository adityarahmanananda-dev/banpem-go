package store

import (
	"context"
	"time"
)

type ErrorLogEntry struct {
	ID        int64
	RequestID string
	UserID    *int64
	Username  string
	Method    string
	Path      string
	Message   string
	Stack     string
	CreatedAt time.Time
}

func (s *Store) InsertErrorLog(ctx context.Context, e ErrorLogEntry) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO error_log
		(request_id, user_id, username, method, path, message, stack)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.RequestID, e.UserID, e.Username, e.Method, e.Path, e.Message, e.Stack)
	return err
}

func (s *Store) ListErrorLogs(ctx context.Context, limit int) ([]ErrorLogEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, request_id, user_id, username, method, path, message, stack, created_at
		FROM error_log ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ErrorLogEntry
	for rows.Next() {
		var e ErrorLogEntry
		if err := rows.Scan(&e.ID, &e.RequestID, &e.UserID, &e.Username, &e.Method, &e.Path,
			&e.Message, &e.Stack, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
