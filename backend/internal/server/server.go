// Package server menyediakan lapisan HTTP aplikasi e-BKU.
package server

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ebku/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	Store     *store.Store
	Tpls      map[string]*template.Template
	BackupDir string
}

func New(st *store.Store, backupDir string) (*Server, error) {
	files, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	tpls := map[string]*template.Template{}
	for _, f := range files {
		name := strings.TrimPrefix(f, "templates/")
		if name == "base.html" {
			continue
		}
		t, err := template.New(name).Funcs(funcMap()).ParseFS(templatesFS, "templates/base.html", f)
		if err != nil {
			return nil, err
		}
		tpls[name] = t
	}
	return &Server{Store: st, Tpls: tpls, BackupDir: backupDir}, nil
}

// Router membangun seluruh rute aplikasi.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /export-database", s.handleExportDB)
	mux.HandleFunc("POST /import-database", s.handleImportDB)

	mux.HandleFunc("GET /bantuan/tambah", s.handleBantuanTambahGet)
	mux.HandleFunc("POST /bantuan/tambah", s.handleBantuanTambahPost)
	mux.HandleFunc("GET /bantuan/{id}/menu/{jenis}", s.handleBantuanMenu)
	mux.HandleFunc("GET /bantuan/{id}/edit", s.handleBantuanEditGet)
	mux.HandleFunc("POST /bantuan/{id}/edit", s.handleBantuanEditPost)
	mux.HandleFunc("GET /bantuan/{id}/saldo-awal", s.handleSaldoAwalGet)
	mux.HandleFunc("POST /bantuan/{id}/saldo-awal", s.handleSaldoAwalPost)

	mux.HandleFunc("GET /bantuan/{id}/tagihan", s.handleTagihanGet)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/tambah", s.handleTagihanTambahGet)
	mux.HandleFunc("POST /bantuan/{id}/tagihan", s.handleTagihanPost)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/edit/{iid}", s.handleTagihanEditGet)
	mux.HandleFunc("POST /bantuan/{id}/tagihan/edit/{iid}", s.handleTagihanEditPost)
	mux.HandleFunc("POST /bantuan/{id}/tagihan/hapus/{iid}", s.handleTagihanHapus)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/daftar", s.handleTagihanDaftar)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/daftar/pdf", s.handleTagihanDaftarPDF)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/daftar/word", s.handleTagihanDaftarWord)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/detail/{iid}", s.handleTagihanDetail)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/pdf", s.handleTagihanPDF)
	mux.HandleFunc("GET /bantuan/{id}/tagihan/word", s.handleTagihanWord)

	mux.HandleFunc("GET /bantuan/{id}/bku", s.handleBKU)
	mux.HandleFunc("POST /bantuan/{id}/bku/reorder", s.handleBKUReorder)
	mux.HandleFunc("GET /bantuan/{id}/bku/tambah", s.handleBKUTambahGet)
	mux.HandleFunc("POST /bantuan/{id}/bku/tambah", s.handleBKUTambahPost)
	mux.HandleFunc("GET /bantuan/{id}/bku/edit/{eid}", s.handleBKUEditGet)
	mux.HandleFunc("POST /bantuan/{id}/bku/edit/{eid}", s.handleBKUEditPost)
	mux.HandleFunc("POST /bantuan/{id}/bku/hapus/{eid}", s.handleBKUHapus)
	mux.HandleFunc("GET /bantuan/{id}/bku/detail/{eid}", s.handleBKUDetail)

	mux.HandleFunc("GET /bantuan/{id}/bku/setor-pajak", s.handleSetorGet)
	mux.HandleFunc("POST /bantuan/{id}/bku/setor-pajak", s.handleSetorPost)
	mux.HandleFunc("GET /bantuan/{id}/bku/setor-pajak/detail/{sid}", s.handleSetorDetail)
	mux.HandleFunc("GET /bantuan/{id}/bku/setor-pajak/edit/{sid}", s.handleSetorEditGet)
	mux.HandleFunc("POST /bantuan/{id}/bku/setor-pajak/edit/{sid}", s.handleSetorEditPost)
	mux.HandleFunc("POST /bantuan/{id}/bku/setor-pajak/hapus/{sid}", s.handleSetorHapus)

	mux.HandleFunc("GET /bantuan/{id}/jasa-giro", s.handleJasaGiro)
	mux.HandleFunc("GET /bantuan/{id}/jasa-giro/tambah", s.handleJasaGiroTambahGet)
	mux.HandleFunc("POST /bantuan/{id}/jasa-giro/tambah", s.handleJasaGiroTambahPost)
	mux.HandleFunc("POST /bantuan/{id}/jasa-giro/hapus/{jid}", s.handleJasaGiroHapus)

	mux.HandleFunc("GET /bantuan/{id}/pencairan", s.handlePencairan)
	mux.HandleFunc("GET /bantuan/{id}/pencairan/tambah", s.handlePencairanTambahGet)
	mux.HandleFunc("POST /bantuan/{id}/pencairan/tambah", s.handlePencairanTambahPost)
	mux.HandleFunc("GET /bantuan/{id}/pencairan/edit/{pid}", s.handlePencairanEditGet)
	mux.HandleFunc("POST /bantuan/{id}/pencairan/edit/{pid}", s.handlePencairanEditPost)
	mux.HandleFunc("POST /bantuan/{id}/pencairan/hapus/{pid}", s.handlePencairanHapus)

	mux.HandleFunc("GET /bantuan/{id}/bank", s.handleBank)
	mux.HandleFunc("POST /bantuan/{id}/bank/reorder", s.handleBankReorder)
	mux.HandleFunc("POST /bantuan/{id}/bank/hapus/{eid}", s.handleBankHapus)
	mux.HandleFunc("GET /bantuan/{id}/bank/export/excel", s.handleBankExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/bank/export/pdf", s.handleBankExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/bank/export/word", s.handleBankExportWord)
	mux.HandleFunc("GET /bantuan/{id}/laporan/excel", s.handleBKUExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/laporan/pdf", s.handleBKUExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/laporan/word", s.handleBKUExportWord)
	mux.HandleFunc("GET /bantuan/{id}/laporan/rekap/excel", s.handleRekapPajakExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/laporan/rekap/pdf", s.handleRekapPajakExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/laporan/rekap/word", s.handleRekapPajakExportWord)

	mux.HandleFunc("GET /bantuan/{id}/rekap-pajak", s.handleRekapPajak)
	mux.HandleFunc("POST /bantuan/{id}/rekap-pajak/reorder", s.handleInvoiceReorder)
	mux.HandleFunc("GET /bantuan/{id}/rekap-belanja", s.handleRekapBelanja)
	mux.HandleFunc("POST /bantuan/{id}/rekap-belanja/reorder", s.handleInvoiceReorder)
	mux.HandleFunc("GET /bantuan/{id}/rekap-belanja/export-pdf", s.handleRekapBelanjaExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/rekap-belanja/export-word", s.handleRekapBelanjaExportWord)
	mux.HandleFunc("GET /bantuan/{id}/rekap-realisasi", s.handleRekapRealisasi)
	mux.HandleFunc("GET /bantuan/{id}/rekap-realisasi/export/excel", s.handleRekapRealisasiExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/rekap-realisasi/export/word", s.handleRekapRealisasiExportWord)

	mux.HandleFunc("GET /bantuan/{id}/rekap-penggunaan-dana", s.handleRekapPenggunaanDana)
	mux.HandleFunc("GET /bantuan/{id}/rekap-penggunaan-dana/export/excel", s.handleRekapPenggunaanDanaExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/rekap-penggunaan-dana/export/pdf", s.handleRekapPenggunaanDanaExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/rekap-penggunaan-dana/export/word", s.handleRekapPenggunaanDanaExportWord)

	mux.HandleFunc("GET /bantuan/{id}/bap-kas", s.handleBAPKas)
	mux.HandleFunc("GET /bantuan/{id}/bap-kas/export/pdf", s.handleBAPKasExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/bap-kas/export/word", s.handleBAPKasExportWord)

	mux.HandleFunc("GET /bantuan/{id}/kegiatan", s.handleKegiatan)
	mux.HandleFunc("POST /bantuan/{id}/kegiatan/tambah", s.handleKegiatanTambah)
	mux.HandleFunc("POST /bantuan/{id}/kegiatan/hapus/{kid}", s.handleKegiatanHapus)
	mux.HandleFunc("GET /bantuan/{id}/rab/export/excel", s.handleRABExportExcel)
	mux.HandleFunc("GET /bantuan/{id}/rab/export/pdf", s.handleRABExportPDF)
	mux.HandleFunc("GET /bantuan/{id}/rab/export/word", s.handleRABExportWord)
	mux.HandleFunc("POST /bantuan/{id}/sub-kegiatan/tambah", s.handleSubKegiatanTambah)
	mux.HandleFunc("POST /bantuan/{id}/sub-kegiatan/hapus/{sid}", s.handleSubKegiatanHapus)
	mux.HandleFunc("POST /bantuan/{id}/aktivitas/tambah", s.handleAktivitasTambah)
	mux.HandleFunc("POST /bantuan/{id}/aktivitas/hapus/{aid}", s.handleAktivitasHapus)
	mux.HandleFunc("POST /bantuan/{id}/komponen/tambah", s.handleKomponenTambah)
	mux.HandleFunc("GET /bantuan/{id}/komponen/edit/{cid}", s.handleKomponenEditGet)
	mux.HandleFunc("POST /bantuan/{id}/komponen/edit/{cid}", s.handleKomponenEditPost)
	mux.HandleFunc("POST /bantuan/{id}/komponen/hapus/{cid}", s.handleKomponenHapus)

	mux.HandleFunc("GET /api/bantuan/{id}/sub-kegiatan", s.handleAPISubKegiatan)
	mux.HandleFunc("GET /api/bantuan/{id}/aktivitas", s.handleAPIAktivitas)
	mux.HandleFunc("GET /api/bantuan/{id}/komponen", s.handleAPIKomponen)
	mux.HandleFunc("GET /api/bantuan/{id}/hitung-pajak", s.handleAPIHitungPajak)

	return withRecovery(mux)
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func formStr(r *http.Request, name string) string {
	return strings.TrimSpace(r.FormValue(name))
}

func formInt(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue(name)), 10, 64)
	return v
}

func formBool(r *http.Request, name string) bool {
	v := strings.ToLower(strings.TrimSpace(r.FormValue(name)))
	return v == "1" || v == "ya" || v == "true" || v == "on"
}

func (s *Server) setFlash(w http.ResponseWriter, typ, msg string) {
	v := url.QueryEscape(typ + "|" + msg)
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: v, Path: "/", MaxAge: 120, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (s *Server) getFlash(w http.ResponseWriter, r *http.Request) (string, string) {
	c, err := r.Cookie("flash")
	if err != nil {
		return "", ""
	}
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1})
	raw, err := url.QueryUnescape(c.Value)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := s.Tpls[name]
	if !ok {
		log.Printf("render %s: template tidak ditemukan", name)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error, url string) {
	log.Printf("error: %v", err)
	s.setFlash(w, "danger", "Terjadi kesalahan. Silakan coba lagi.")
	s.redirect(w, r, url)
}

// bantuanCtx mengambil data bantuan untuk konteks halaman.
func (s *Server) bantuanCtx(r *http.Request) (*store.Bantuan, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
