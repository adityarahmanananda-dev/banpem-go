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
	name := "ebku_export_" + time.Now().Format("20060102_150405") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Write(data)
}

func (s *Server) handleImportDB(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.setFlash(w, "danger", "Gagal membaca upload.")
		s.redirect(w, r, "/")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.setFlash(w, "danger", "Pilih file backup (.db).")
		s.redirect(w, r, "/")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".db") {
		s.setFlash(w, "danger", "Ekstensi file harus .db.")
		s.redirect(w, r, "/")
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		s.setFlash(w, "danger", "Gagal membaca file.")
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
		s.setFlash(w, "danger", err.Error())
		s.redirect(w, r, "/")
		return
	}
	s.setFlash(w, "success", "Database berhasil di-restore.")
	s.redirect(w, r, "/")
}
