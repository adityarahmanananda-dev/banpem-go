package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role aplikasi. Admin dapat mengelola user, import/export database, dan audit.
// Bendahara dapat mengelola data keuangan. Viewer hanya boleh membaca.
const (
	RoleAdmin     = "admin"
	RoleBendahara = "bendahara"
	RoleViewer    = "viewer"
)

var (
	ErrAdminPasswordRequired = errors.New("ADMIN_PASSWORD wajib diisi saat membuat user admin pertama")
	ErrInvalidRole           = errors.New("role tidak valid")
	ErrLastAdmin             = errors.New("tidak boleh menonaktifkan admin terakhir")
	ErrUserExists            = errors.New("username sudah dipakai")
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        int64
	TokenHash string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuditEntry struct {
	UserID    *int64
	Username  string
	Role      string
	BantuanID *int64
	Action    string
	Entity    string
	EntityID  *int64
	Details   map[string]any
	IP        string
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validRole(role string) bool {
	switch role {
	case RoleAdmin, RoleBendahara, RoleViewer:
		return true
	}
	return false
}

// EnsureAdmin membuat user admin pertama bila tabel users masih kosong.
// Password wajib diisi lewat env saat instalasi pertama.
func (s *Store) EnsureAdmin(ctx context.Context, username, password string) (bool, error) {
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if len(strings.TrimSpace(password)) < 10 {
		return false, ErrAdminPasswordRequired
	}
	username = normalizeUsername(username)
	if username == "" {
		username = "admin"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (username, password_hash, role, active)
		VALUES ($1,$2,$3,TRUE)`, username, string(hash), RoleAdmin)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT id, username, password_hash, role, active, created_at, updated_at
		FROM users WHERE username=$1`, normalizeUsername(username)).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT id, username, password_hash, role, active, created_at, updated_at
		FROM users WHERE id=$1`, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, username, password_hash, role, active, created_at, updated_at
		FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Active,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, username, password, role string) (User, error) {
	username = normalizeUsername(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return User{}, errors.New("username dan password wajib diisi")
	}
	if !validRole(role) {
		return User{}, ErrInvalidRole
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	var u User
	err = s.Pool.QueryRow(ctx, `INSERT INTO users (username, password_hash, role, active)
		VALUES ($1,$2,$3,TRUE)
		RETURNING id, username, password_hash, role, active, created_at, updated_at`,
		username, string(hash), role).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	return u, nil
}

func (s *Store) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role=$1 AND active=TRUE`, RoleAdmin).Scan(&n)
	return n, err
}

func (s *Store) SetUserActive(ctx context.Context, id int64, active bool) error {
	u, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if !active && u.Role == RoleAdmin && u.Active {
		n, err := s.CountActiveAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	_, err = s.Pool.Exec(ctx, `UPDATE users SET active=$1, updated_at=now() WHERE id=$2`, active, id)
	return err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int64, password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password wajib diisi")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2`,
		string(hash), id)
	if err == nil {
		// Paksa login ulang di semua perangkat setelah password berubah.
		_, _ = s.Pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, id)
	}
	return err
}

func (s *Store) CreateSession(ctx context.Context, userID int64, token, csrf string, expires time.Time, userAgent, ip string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO sessions
		(token_hash, user_id, csrf_token, expires_at, user_agent, ip)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		hashToken(token), userID, csrf, expires, userAgent, ip)
	return err
}

func (s *Store) GetSessionUser(ctx context.Context, token string) (Session, User, error) {
	var sess Session
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT s.id, s.token_hash, s.user_id, s.csrf_token, s.expires_at, s.created_at,
		u.id, u.username, u.password_hash, u.role, u.active, u.created_at, u.updated_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.expires_at > now() AND u.active=TRUE`,
		hashToken(token)).Scan(
		&sess.ID, &sess.TokenHash, &sess.UserID, &sess.CSRFToken, &sess.ExpiresAt, &sess.CreatedAt,
		&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	return sess, u, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hashToken(token))
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}

func (s *Store) InsertAudit(ctx context.Context, e AuditEntry) error {
	details := []byte(`{}`)
	if e.Details != nil {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return err
		}
		details = b
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO audit_log
		(user_id, username, role, bantuan_id, action, entity, entity_id, details, ip)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.UserID, e.Username, e.Role, e.BantuanID, e.Action, e.Entity, e.EntityID, details, e.IP)
	return err
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	rows, err := s.Pool.Query(ctx, `SELECT user_id, username, role, bantuan_id, action, entity,
		entity_id, details, ip, created_at
		FROM audit_log ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var details []byte
		var created time.Time
		if err := rows.Scan(&e.UserID, &e.Username, &e.Role, &e.BantuanID, &e.Action,
			&e.Entity, &e.EntityID, &details, &e.IP, &created); err != nil {
			return nil, err
		}
		if len(details) > 0 {
			_ = json.Unmarshal(details, &e.Details)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AuditRow adalah baris audit lengkap untuk halaman admin.
type AuditRow struct {
	AuditEntry
	CreatedAt time.Time
}

func (s *Store) ListAuditRows(ctx context.Context, limit int) ([]AuditRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	rows, err := s.Pool.Query(ctx, `SELECT user_id, username, role, bantuan_id, action, entity,
		entity_id, details, ip, created_at
		FROM audit_log ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var e AuditRow
		var details []byte
		if err := rows.Scan(&e.UserID, &e.Username, &e.Role, &e.BantuanID, &e.Action,
			&e.Entity, &e.EntityID, &details, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		if len(details) > 0 {
			_ = json.Unmarshal(details, &e.Details)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
