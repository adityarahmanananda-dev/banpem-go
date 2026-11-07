package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ebku/internal/store"
)

type durationMetric struct {
	sum     float64
	count   int64
	buckets [11]int64
}

type appMetrics struct {
	mu            sync.Mutex
	requests      map[string]int64
	durations     map[string]*durationMetric
	loginFailures int64
	start         time.Time
}

var metricBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func newAppMetrics() *appMetrics {
	return &appMetrics{
		requests:  map[string]int64{},
		durations: map[string]*durationMetric{},
		start:     time.Now(),
	}
}

func (m *appMetrics) observe(method, path string, status int, d time.Duration) {
	key := method + "\x00" + path + "\x00" + strconv.Itoa(status)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[key]++
	dm := m.durations[key]
	if dm == nil {
		dm = &durationMetric{}
		m.durations[key] = dm
	}
	sec := d.Seconds()
	dm.sum += sec
	dm.count++
	for i, b := range metricBuckets {
		if sec <= b {
			dm.buckets[i]++
		}
	}
}

func (m *appMetrics) incLoginFailure() {
	m.mu.Lock()
	m.loginFailures++
	m.mu.Unlock()
}

func metricPath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if _, err := strconv.ParseInt(part, 10, 64); err == nil {
			parts[i] = ":id"
		}
	}
	out := strings.Join(parts, "/")
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.metrics.observe(r.Method, metricPath(r.URL.Path), rec.status, time.Since(start))
	})
}

func metricsTokenValid(r *http.Request) bool {
	token := os.Getenv("METRICS_TOKEN")
	if token == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if token := os.Getenv("METRICS_TOKEN"); token != "" {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		u := userFromContext(r.Context())
		if u == nil || u.Role != store.RoleAdmin {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	dbUp := 1
	if err := s.Store.Ping(ctx); err != nil {
		dbUp = 0
	}

	m := s.metrics
	m.mu.Lock()
	requests := make(map[string]int64, len(m.requests))
	for k, v := range m.requests {
		requests[k] = v
	}
	durations := make(map[string]durationMetric, len(m.durations))
	for k, v := range m.durations {
		durations[k] = *v
	}
	loginFailures := m.loginFailures
	start := m.start
	m.mu.Unlock()

	var b strings.Builder
	b.WriteString("# HELP banpem_http_requests_total Total HTTP requests.\n")
	b.WriteString("# TYPE banpem_http_requests_total counter\n")
	keys := make([]string, 0, len(requests))
	for k := range requests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.Split(k, "\x00")
		if len(parts) != 3 {
			continue
		}
		fmt.Fprintf(&b, "banpem_http_requests_total{method=%q,path=%q,status=%q} %d\n",
			parts[0], parts[1], parts[2], requests[k])
	}

	b.WriteString("# HELP banpem_http_request_duration_seconds HTTP request duration.\n")
	b.WriteString("# TYPE banpem_http_request_duration_seconds histogram\n")
	for _, k := range keys {
		d, ok := durations[k]
		if !ok {
			continue
		}
		parts := strings.Split(k, "\x00")
		if len(parts) != 3 {
			continue
		}
		labels := fmt.Sprintf("method=%q,path=%q,status=%q", parts[0], parts[1], parts[2])
		for i, bucket := range metricBuckets {
			fmt.Fprintf(&b, "banpem_http_request_duration_seconds_bucket{%s,le=%q} %d\n",
				labels, strconv.FormatFloat(bucket, 'f', -1, 64), d.buckets[i])
		}
		fmt.Fprintf(&b, "banpem_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, d.count)
		fmt.Fprintf(&b, "banpem_http_request_duration_seconds_sum{%s} %f\n", labels, d.sum)
		fmt.Fprintf(&b, "banpem_http_request_duration_seconds_count{%s} %d\n", labels, d.count)
	}

	fmt.Fprintf(&b, "# HELP banpem_db_up Database reachable (1=up, 0=down).\n")
	fmt.Fprintf(&b, "# TYPE banpem_db_up gauge\nbanpem_db_up %d\n", dbUp)
	fmt.Fprintf(&b, "# HELP banpem_login_failures_total Failed login attempts.\n")
	fmt.Fprintf(&b, "# TYPE banpem_login_failures_total counter\nbanpem_login_failures_total %d\n", loginFailures)
	fmt.Fprintf(&b, "# HELP process_start_time_seconds Start time since Unix epoch.\n")
	fmt.Fprintf(&b, "# TYPE process_start_time_seconds gauge\nprocess_start_time_seconds %d\n", start.Unix())
	fmt.Fprintf(&b, "# HELP go_goroutines Number of goroutines.\n")
	fmt.Fprintf(&b, "# TYPE go_goroutines gauge\ngo_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "# HELP banpem_build_info Build information.\n")
	fmt.Fprintf(&b, "# TYPE banpem_build_info gauge\nbanpem_build_info{version=%q} 1\n", "dev")

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}
