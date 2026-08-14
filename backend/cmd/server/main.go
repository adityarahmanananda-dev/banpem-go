// Banpem-GO: aplikasi web Buku Kas Umum keuangan bantuan pemerintah di sekolah.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

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
	if _, err := st.BackupFile(ctx, backupDir); err != nil {
		log.Printf("peringatan: backup otomatis gagal: %v", err)
	}

	srv, err := server.New(st, backupDir)
	if err != nil {
		log.Fatalf("inisialisasi server gagal: %v", err)
	}
	log.Printf("Banpem-GO berjalan di %s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
