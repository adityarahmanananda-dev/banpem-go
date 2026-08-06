package server

import (
	"log"
	"net/http"
	"time"

	"ebku/internal/export"
	"ebku/internal/store"
)

// exportParams membaca parameter export dari query string.
func exportParams(r *http.Request) (tgl time.Time, orientasi, versi string) {
	tgl = store.ParseDate(r.URL.Query().Get("tanggal_cetak"))
	if tgl.IsZero() {
		tgl = time.Now()
	}
	orientasi = r.URL.Query().Get("orientasi")
	if orientasi != "portrait" {
		orientasi = "landscape"
	}
	versi = r.URL.Query().Get("versi")
	if versi != "real" {
		versi = "rencana"
	}
	return
}

func (s *Server) serveExcel(w http.ResponseWriter, rep export.Report, filename string) {
	data, err := export.ExcelBytes(rep)
	if err != nil {
		log.Println("export excel:", err)
		http.Error(w, "gagal membuat excel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.xlsx"`)
	w.Write(data)
}

// servePDF mengirim laporan PDF. Orientasi mengikuti pilihan user pada query
// string (?orientasi=portrait|landscape); bila tidak ada pilihan, memakai
// default rep.Landscape.
func (s *Server) servePDF(w http.ResponseWriter, rep export.Report, filename string, r *http.Request) {
	switch r.URL.Query().Get("orientasi") {
	case "portrait":
		rep.Landscape = false
	case "landscape":
		rep.Landscape = true
	}
	data, err := export.PDF(rep)
	if err != nil {
		log.Println("export pdf:", err)
		http.Error(w, "gagal membuat pdf", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.pdf"`)
	w.Write(data)
}

func (s *Server) serveWord(w http.ResponseWriter, rep export.Report, filename string, landscape bool) {
	data, err := export.WordBytes(rep, landscape)
	if err != nil {
		log.Println("export word:", err)
		http.Error(w, "gagal membuat word", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.docx"`)
	w.Write(data)
}

func (s *Server) ledgerExport(w http.ResponseWriter, r *http.Request, table, kind string) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	tgl, _, versi := exportParams(r)
	vi := s.versiInfo(r.Context(), id, versi)
	rep, err := s.buildLedgerReport(r.Context(), &b, table, vi.Base, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	kindName := "BKU"
	if table == "bank" {
		kindName = "BukuBank"
	}
	fname := reportFilename(kindName, b.Nama, tgl)
	switch kind {
	case "excel":
		s.serveExcel(w, rep, fname)
	case "pdf":
		s.servePDF(w, rep, fname, r)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}

func (s *Server) handleBKUExportExcel(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bku", "excel")
}
func (s *Server) handleBKUExportPDF(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bku", "pdf")
}
func (s *Server) handleBKUExportWord(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bku", "word")
}

func (s *Server) handleBankExportExcel(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bank", "excel")
}
func (s *Server) handleBankExportPDF(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bank", "pdf")
}
func (s *Server) handleBankExportWord(w http.ResponseWriter, r *http.Request) {
	s.ledgerExport(w, r, "bank", "word")
}

func (s *Server) handleRekapPajakExportExcel(w http.ResponseWriter, r *http.Request) {
	s.rekapPajakExport(w, r, "excel")
}
func (s *Server) handleRekapPajakExportPDF(w http.ResponseWriter, r *http.Request) {
	s.rekapPajakExport(w, r, "pdf")
}
func (s *Server) handleRekapPajakExportWord(w http.ResponseWriter, r *http.Request) {
	s.rekapPajakExport(w, r, "word")
}

func (s *Server) rekapPajakExport(w http.ResponseWriter, r *http.Request, kind string) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	tgl, _, _ := exportParams(r)
	rep, err := s.buildRekapPajak(r.Context(), &b, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	fname := reportFilename("RekapPajak", b.Nama, tgl)
	switch kind {
	case "excel":
		s.serveExcel(w, rep, fname)
	case "pdf":
		s.servePDF(w, rep, fname, r)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}

func (s *Server) handleRekapBelanjaExportPDF(w http.ResponseWriter, r *http.Request) {
	s.rekapBelanjaExport(w, r, "pdf")
}
func (s *Server) handleRekapBelanjaExportWord(w http.ResponseWriter, r *http.Request) {
	s.rekapBelanjaExport(w, r, "word")
}

func (s *Server) rekapBelanjaExport(w http.ResponseWriter, r *http.Request, kind string) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	tgl, _, _ := exportParams(r)
	rep, err := s.buildRekapBelanja(r.Context(), &b, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	fname := reportFilename("RekapBelanja", b.Nama, tgl)
	switch kind {
	case "pdf":
		s.servePDF(w, rep, fname, r)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}

// handleTagihanPDF/Word dan handleTagihanDaftarPDF/Word mengekspor laporan
// Daftar Tagihan.
func (s *Server) handleTagihanPDF(w http.ResponseWriter, r *http.Request) {
	s.daftarTagihanExport(w, r, "pdf")
}
func (s *Server) handleTagihanWord(w http.ResponseWriter, r *http.Request) {
	s.daftarTagihanExport(w, r, "word")
}
func (s *Server) handleTagihanDaftarPDF(w http.ResponseWriter, r *http.Request) {
	s.daftarTagihanExport(w, r, "pdf")
}
func (s *Server) handleTagihanDaftarWord(w http.ResponseWriter, r *http.Request) {
	s.daftarTagihanExport(w, r, "word")
}

func (s *Server) daftarTagihanExport(w http.ResponseWriter, r *http.Request, kind string) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	tgl, _, _ := exportParams(r)
	rep, err := s.buildDaftarTagihan(r.Context(), &b, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	fname := reportFilename("DaftarTagihan", b.Nama, tgl)
	switch kind {
	case "pdf":
		s.servePDF(w, rep, fname, r)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}

func (s *Server) handleRABExportExcel(w http.ResponseWriter, r *http.Request) {
	s.rabExport(w, r, "excel")
}
func (s *Server) handleRABExportPDF(w http.ResponseWriter, r *http.Request) {
	s.rabExport(w, r, "pdf")
}
func (s *Server) handleRABExportWord(w http.ResponseWriter, r *http.Request) {
	s.rabExport(w, r, "word")
}

// rabExport mengekspor laporan Rencana Anggaran Biaya.
func (s *Server) rabExport(w http.ResponseWriter, r *http.Request, kind string) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	tgl, _, _ := exportParams(r)
	rep, err := s.buildRABReport(r.Context(), &b, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	fname := reportFilename("RAB", b.Nama, tgl)
	switch kind {
	case "excel":
		s.serveExcel(w, rep, fname)
	case "pdf":
		s.servePDF(w, rep, fname, r)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}
