package server

import (
	"net/http"
	"strings"
	"time"

	"ebku/internal/store"
)

// dateRangeFromQuery membaca filter dari/sampai. Nilai invalid diabaikan
// (halaman tetap tampil tanpa filter) agar GET yang di-bookmark tidak error.
func dateRangeFromQuery(r *http.Request) (*time.Time, *time.Time) {
	var from, to *time.Time
	if s := strings.TrimSpace(r.URL.Query().Get("dari")); s != "" {
		if t, err := store.ParseDateStrict(s); err == nil {
			from = &t
		}
	}
	if s := strings.TrimSpace(r.URL.Query().Get("sampai")); s != "" {
		if t, err := store.ParseDateStrict(s); err == nil {
			to = &t
		}
	}
	if from != nil && to != nil && from.After(*to) {
		from, to = to, from
	}
	return from, to
}

func dateInRange(t time.Time, from, to *time.Time) bool {
	if from != nil && t.Before(*from) {
		return false
	}
	if to != nil && t.After(*to) {
		return false
	}
	return true
}

func datePtrInRange(t *time.Time, from, to *time.Time) bool {
	if t == nil {
		return from == nil && to == nil
	}
	return dateInRange(*t, from, to)
}
