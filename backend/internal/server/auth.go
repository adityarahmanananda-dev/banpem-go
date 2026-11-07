package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ebku/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "ebku_session"
	csrfCookieName    = "ebku_csrf"
	csrfHeaderName    = "X-CSRF-Token"
)

type ctxKey int

const (
	userCtxKey ctxKey = iota
	csrfCtxKey
)

func userFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userCtxKey).(*store.User)
	return u
}

func csrfFromContext(ctx context.Context) string {
	s, _ := ctx.Value(csrfCtxKey).(string)
	return s
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		xff = strings.TrimSpace(xff)
		if xff != "" {
			return xff
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func isPublicPath(path string) bool {
	return path == "/login" || path == "/healthz" || strings.HasPrefix(path, "/static/")
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsAny(next, "\\\r\n") {
		return "/"
	}
	return next
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"font-src 'self' data:; "+
				"img-src 'self' data:; "+
				"connect-src 'self'; object-src 'none'; worker-src 'none'; "+
				"base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(10 << 20)
		if r.URL.Path == "/import-database" {
			limit = store.MaxDumpBytes + (1 << 20)
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		u := userFromContext(r.Context())
		user := "-"
		if u != nil {
			user = u.Username
		}
		log.Printf("request_id=%s %s %s %d %s user=%s ip=%s",
			requestIDFromContext(r.Context()), r.Method, r.URL.Path, rec.status,
			time.Since(start).Round(time.Millisecond), user, clientIP(r))
	})
}

// withCSRF memakai pola double-submit cookie. Token disimpan di cookie
// HttpOnly dan diinjeksi ke halaman sebagai meta tag / hidden input.
func (s *Server) withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		token := ""
		if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
			token = c.Value
		}
		if token == "" {
			var err error
			token, err = randomToken(32)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   s.CookieSecure,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int((12 * time.Hour).Seconds()),
			})
		}
		ctx := context.WithValue(r.Context(), csrfCtxKey, token)
		r = r.WithContext(ctx)

		if !isSafeMethod(r.Method) {
			got := r.Header.Get(csrfHeaderName)
			if got == "" {
				got = r.FormValue("csrf_token")
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "CSRF token tidak valid. Muat ulang halaman lalu coba lagi.", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" && metricsTokenValid(r) {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookieName)
		if err != nil || c.Value == "" {
			s.redirectLogin(w, r)
			return
		}
		sess, u, err := s.Store.GetSessionUser(r.Context(), c.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.CookieSecure})
			s.redirectLogin(w, r)
			return
		}
		if u.Role == store.RoleViewer && !isSafeMethod(r.Method) && r.URL.Path != "/logout" {
			http.Error(w, "Akses ditolak: role viewer hanya boleh membaca data.", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, &u)
		ctx = context.WithValue(ctx, csrfCtxKey, sess.CSRFToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) redirectLogin(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	target := r.URL.RequestURI()
	http.Redirect(w, r, "/login?next="+url.QueryEscape(target), http.StatusSeeOther)
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := userFromContext(r.Context())
		if u == nil || u.Role != store.RoleAdmin {
			http.Error(w, "Akses ditolak: hanya admin.", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string][]time.Time{}}
}

// allow mencatat percobaan login. Maksimal 5 percobaan per 15 menit per
// kombinasi IP+username.
func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-15 * time.Minute)
	var keep []time.Time
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 5 {
		l.attempts[key] = keep
		return false
	}
	l.attempts[key] = append(keep, time.Now())
	return true
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if userFromContext(r.Context()) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	data := struct {
		baseView
		Next  string
		Error string
	}{
		baseView: baseView{Title: "Masuk", Active: "login"},
		Next:     safeNext(r.URL.Query().Get("next")),
	}
	s.render(w, r, "login.html", data)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	next := safeNext(r.FormValue("next"))
	key := clientIP(r) + "|" + strings.ToLower(username)
	if !s.loginLimiter.allow(key) {
		s.renderLoginError(w, r, next, "Terlalu banyak percobaan. Coba lagi dalam 15 menit.", http.StatusTooManyRequests)
		return
	}
	u, err := s.Store.GetUserByUsername(r.Context(), username)
	if err != nil || !u.Active || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.metrics.incLoginFailure()
		s.auditAnon(r, username, "login_failed", map[string]any{"reason": "invalid credentials"})
		s.renderLoginError(w, r, next, "Username atau password salah.", http.StatusUnauthorized)
		return
	}
	s.loginLimiter.reset(key)
	token, err := randomToken(32)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	csrf, err := randomToken(32)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(s.SessionTTL)
	if err := s.Store.CreateSession(r.Context(), u.ID, token, csrf, expires, r.UserAgent(), clientIP(r)); err != nil {
		s.fail(w, r, err, "/login")
		return
	}
	_ = s.Store.DeleteExpiredSessions(r.Context())
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode,
		Expires: expires, MaxAge: int(s.SessionTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName, Value: csrf, Path: "/", HttpOnly: true,
		Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode,
		Expires: expires, MaxAge: int(s.SessionTTL.Seconds()),
	})
	s.auditForUser(r, &u, nil, "login", "session", nil, map[string]any{"ip": clientIP(r)})
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) renderLoginError(w http.ResponseWriter, r *http.Request, next, msg string, status int) {
	data := struct {
		baseView
		Next  string
		Error string
	}{
		baseView: baseView{Title: "Masuk", Active: "login"},
		Next:     next,
		Error:    msg,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	s.render(w, r, "login.html", data)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.Store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.CookieSecure})
	s.audit(r, nil, "logout", "session", nil, nil)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleUsersGet(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers(r.Context())
	if err != nil {
		s.fail(w, r, err, "/admin/users")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Users []store.User
	}{
		baseView: baseView{Title: "Manajemen User", Active: "users", FlashType: ft, FlashMsg: fm},
		Users:    users,
	}
	s.render(w, r, "users.html", data)
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	username := formStr(r, "username")
	password := r.FormValue("password")
	role := formStr(r, "role")
	if len(strings.TrimSpace(password)) < 10 {
		s.setFlash(w, "danger", "Password minimal 10 karakter.")
		s.redirect(w, r, "/admin/users")
		return
	}
	u, err := s.Store.CreateUser(r.Context(), username, password, role)
	if err != nil {
		msg := "Gagal membuat user."
		if errors.Is(err, store.ErrUserExists) {
			msg = "Username sudah dipakai."
		} else if errors.Is(err, store.ErrInvalidRole) {
			msg = "Role tidak valid."
		}
		s.setFlash(w, "danger", msg)
		s.redirect(w, r, "/admin/users")
		return
	}
	id := u.ID
	s.audit(r, nil, "create", "user", &id, map[string]any{"username": u.Username, "role": u.Role})
	s.setFlash(w, "success", "User berhasil dibuat.")
	s.redirect(w, r, "/admin/users")
}

func (s *Server) handleUserToggle(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	active := formBool(r, "active")
	me := userFromContext(r.Context())
	if me != nil && me.ID == id && !active {
		s.setFlash(w, "danger", "Tidak bisa menonaktifkan akun sendiri.")
		s.redirect(w, r, "/admin/users")
		return
	}
	if err := s.Store.SetUserActive(r.Context(), id, active); err != nil {
		msg := "Gagal mengubah status user."
		if errors.Is(err, store.ErrLastAdmin) {
			msg = "Tidak boleh menonaktifkan admin terakhir."
		}
		s.setFlash(w, "danger", msg)
		s.redirect(w, r, "/admin/users")
		return
	}
	s.audit(r, nil, "update", "user", &id, map[string]any{"active": active})
	s.setFlash(w, "success", "Status user diperbarui.")
	s.redirect(w, r, "/admin/users")
}

func (s *Server) handleUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	password := r.FormValue("password")
	if len(strings.TrimSpace(password)) < 10 {
		s.setFlash(w, "danger", "Password minimal 10 karakter.")
		s.redirect(w, r, "/admin/users")
		return
	}
	if err := s.Store.UpdateUserPassword(r.Context(), id, password); err != nil {
		s.setFlash(w, "danger", "Gagal reset password.")
		s.redirect(w, r, "/admin/users")
		return
	}
	s.audit(r, nil, "reset_password", "user", &id, nil)
	s.setFlash(w, "success", "Password user diperbarui. Semua session user tersebut telah dihapus.")
	s.redirect(w, r, "/admin/users")
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListErrorLogs(r.Context(), 250)
	if err != nil {
		s.fail(w, r, err, "/admin/errors")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows []store.ErrorLogEntry
	}{
		baseView: baseView{Title: "Error Log", Active: "errors", FlashType: ft, FlashMsg: fm},
		Rows:     rows,
	}
	s.render(w, r, "errors.html", data)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListAuditRows(r.Context(), 250)
	if err != nil {
		s.fail(w, r, err, "/admin/audit")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows []store.AuditRow
	}{
		baseView: baseView{Title: "Audit Log", Active: "audit", FlashType: ft, FlashMsg: fm},
		Rows:     rows,
	}
	s.render(w, r, "audit.html", data)
}

// ---- audit helpers ----

func (s *Server) audit(r *http.Request, bantuanID *int64, action, entity string, entityID *int64, details map[string]any) {
	u := userFromContext(r.Context())
	s.auditForUser(r, u, bantuanID, action, entity, entityID, details)
}

func (s *Server) auditForUser(r *http.Request, u *store.User, bantuanID *int64, action, entity string, entityID *int64, details map[string]any) {
	if u == nil {
		return
	}
	e := store.AuditEntry{
		UserID: &u.ID, Username: u.Username, Role: u.Role,
		BantuanID: bantuanID, Action: action, Entity: entity, EntityID: entityID,
		Details: details, IP: clientIP(r),
	}
	if err := s.Store.InsertAudit(r.Context(), e); err != nil {
		log.Printf("audit gagal: %v", err)
	}
}

func (s *Server) auditAnon(r *http.Request, username, action string, details map[string]any) {
	e := store.AuditEntry{
		Username: username, Action: action, Entity: "session",
		Details: details, IP: clientIP(r),
	}
	if err := s.Store.InsertAudit(r.Context(), e); err != nil {
		log.Printf("audit gagal: %v", err)
	}
}
