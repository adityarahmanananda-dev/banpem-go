package server

import (
	"io"
	"net/http"
	"strings"
	"time"

	"ebku/internal/store"
)

func (s *Server) handleExportDB(w http.ResponseWriter, r *http.Request) {
	data, err := s.Store.DumpDatabase(r.Context())
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	name := "ebku_export_" + time.Now().Format("20060102_150405") + ".sql"
	w.Header().Set("Content-Type", "application/sql; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	s.audit(r, nil, "export", "database", nil, map[string]any{"bytes": len(data)})
	_, _ = w.Write(data)
}

func (s *Server) handleImportDB(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(store.MaxDumpBytes + (1 << 20)); err != nil {
		s.setFlash(w, "danger", "Gagal membaca upload (maksimal 64 MB).")
		s.redirect(w, r, "/")
		return
	}
	if formStr(r, "confirm") != "RESTORE" {
		s.setFlash(w, "danger", "Ketik RESTORE pada kolom konfirmasi untuk melanjutkan.")
		s.redirect(w, r, "/")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.setFlash(w, "danger", "Pilih file backup (.sql atau .db).")
		s.redirect(w, r, "/")
		return
	}
	defer file.Close()
	lower := strings.ToLower(header.Filename)
	if !strings.HasSuffix(lower, ".sql") && !strings.HasSuffix(lower, ".db") {
		s.setFlash(w, "danger", "Ekstensi file harus .sql atau .db.")
		s.redirect(w, r, "/")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, store.MaxDumpBytes+1))
	if err != nil {
		s.setFlash(w, "danger", "Gagal membaca file.")
		s.redirect(w, r, "/")
		return
	}
	if len(data) > store.MaxDumpBytes {
		s.setFlash(w, "danger", "File backup terlalu besar (maksimal 64 MB).")
		s.redirect(w, r, "/")
		return
	}
	if err := store.ValidateDump(data); err != nil {
		s.setFlash(w, "danger", "File tidak valid: "+err.Error())
		s.redirect(w, r, "/")
		return
	}
	if _, err := s.Store.BackupFile(r.Context(), s.BackupDir); err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.ApplyDump(r.Context(), data); err != nil {
		s.setFlash(w, "danger", "Restore gagal. Backup sebelum restore tetap tersimpan.")
		s.redirect(w, r, "/")
		return
	}
	s.audit(r, nil, "restore", "database", nil, map[string]any{"bytes": len(data), "file": header.Filename})
	s.setFlash(w, "success", "Database berhasil di-restore.")
	s.redirect(w, r, "/")
}
