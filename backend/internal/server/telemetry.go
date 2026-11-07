package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"ebku/internal/store"
)

type requestIDKeyType ctxKey

const requestIDKey requestIDKeyType = 100

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// withRequestID memberikan X-Request-ID untuk korelasi log/error tracking.
func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				msg := "panic: " + toString(rec)
				log.Printf("panic request_id=%s: %s\n%s", requestIDFromContext(r.Context()), msg, stack)
				s.recordError(r, msg, stack)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return "unknown panic"
}

func (s *Server) recordError(r *http.Request, message, stack string) {
	u := userFromContext(r.Context())
	e := store.ErrorLogEntry{
		RequestID: requestIDFromContext(r.Context()),
		Method:    r.Method,
		Path:      r.URL.Path,
		Message:   message,
		Stack:     stack,
	}
	if u != nil {
		e.UserID = &u.ID
		e.Username = u.Username
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Store.InsertErrorLog(ctx, e); err != nil {
			log.Printf("gagal menyimpan error_log: %v", err)
		}
	}()
	s.alert("error", message, r)
}

// isExpectedError membedakan error validasi/bisnis dari error tak terduga
// sehingga error_log tidak dipenuhi error 4xx yang normal.
func isExpectedError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	targets := []error{
		store.ErrNotFound, store.ErrInvalidInput, store.ErrInvalidDate,
		store.ErrInUse, store.ErrInvoiceHasTaxDeposit, store.ErrTaxAlreadyDeposited,
		store.ErrRealisasiMismatch, store.ErrBudgetExceeded, store.ErrDuplicateInvoiceNo,
		store.ErrPencairanExceedsTotal, store.ErrUserExists, store.ErrInvalidRole,
		store.ErrLastAdmin, store.ErrAdminPasswordRequired,
	}
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "tidak valid") || strings.Contains(msg, "tidak ditemukan") ||
		strings.Contains(msg, "wajib diisi")
}

// alert mengirim webhook JSON bila ALERT_WEBHOOK_URL diset. Dibatasi satu
// alert per jenis per 5 menit agar tidak membanjiri endpoint.
func (s *Server) alert(kind, message string, r *http.Request) {
	if strings.TrimSpace(s.AlertWebhookURL) == "" {
		return
	}
	key := kind
	s.alertMu.Lock()
	if last, ok := s.lastAlert[key]; ok && time.Since(last) < 5*time.Minute {
		s.alertMu.Unlock()
		return
	}
	s.lastAlert[key] = time.Now()
	s.alertMu.Unlock()

	payload := map[string]any{
		"type":       kind,
		"message":    message,
		"request_id": requestIDFromContext(r.Context()),
		"path":       r.URL.Path,
		"time":       time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.AlertWebhookURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			log.Printf("alert webhook gagal: %v", err)
			return
		}
		_ = resp.Body.Close()
	}()
}
