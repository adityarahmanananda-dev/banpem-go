// Banpem-GO: aplikasi web Buku Kas Umum keuangan bantuan pemerintah di sekolah.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"ebku/internal/migrations"
	"ebku/internal/server"
	"ebku/internal/store"
)

func main() {
	ctx := context.Background()
	dbURL := env("DATABASE_URL", "postgres://ebku:ebku@db:5432/ebku?sslmode=disable")
	backupDir := env("BACKUP_DIR", "backups")
	addr := env("ADDR", ":8080")

	st, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer st.Close()

	if err := st.Migrate(ctx, migrations.FS); err != nil {
		log.Fatalf("migrasi database gagal: %v", err)
	}
	created, err := st.EnsureAdmin(ctx, env("ADMIN_USERNAME", "admin"), os.Getenv("ADMIN_PASSWORD"))
	if err != nil {
		if errors.Is(err, store.ErrAdminPasswordRequired) {
			log.Fatal("Belum ada user. Set ADMIN_PASSWORD (minimal 10 karakter) di environment untuk membuat admin pertama.")
		}
		log.Fatalf("gagal menyiapkan admin: %v", err)
	}
	if created {
		log.Printf("User admin pertama dibuat dari ADMIN_USERNAME/ADMIN_PASSWORD. Segera ganti password setelah login.")
	}
	_ = st.DeleteExpiredSessions(ctx)
	// Resync seluruh ledger bersifat opsional karena menulis ulang semua baris
	// dan bisa memperlambat startup. Aktifkan hanya saat perbaikan data lama.
	if envBool("RESYNC_LEDGER_ON_STARTUP", false) {
		if err := st.ResyncAllLedger(ctx); err != nil {
			log.Printf("peringatan: sinkronisasi urutan buku gagal: %v", err)
		}
	}
	if envBool("BACKUP_ON_STARTUP", true) {
		if _, err := st.BackupFile(ctx, backupDir); err != nil {
			log.Printf("peringatan: backup otomatis gagal: %v", err)
		}
	}

	srv, err := server.New(st, backupDir)
	if err != nil {
		log.Fatalf("inisialisasi server gagal: %v", err)
	}
	srv.CookieSecure = envBool("COOKIE_SECURE", false)
	srv.ReportCity = env("REPORT_CITY", "Jakarta")
	srv.AlertWebhookURL = os.Getenv("ALERT_WEBHOOK_URL")
	if hours := envInt("SESSION_TTL_HOURS", 12); hours > 0 {
		srv.SessionTTL = time.Duration(hours) * time.Hour
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Printf("Banpem-GO berjalan di %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server gagal: %v", err)
		}
	}()
	<-stopCtx.Done()
	log.Printf("menghentikan server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown gagal: %v", err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "ya" || v == "on"
}

func envInt(k string, def int) int {
	v, err := strconv.Atoi(os.Getenv(k))
	if err != nil {
		return def
	}
	return v
}
