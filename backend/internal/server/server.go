// Package server menyediakan lapisan HTTP aplikasi e-BKU.
package server

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"ebku/internal/money"
	"ebku/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	Store           *store.Store
	Tpls            map[string]*template.Template
	BackupDir       string
	SessionTTL      time.Duration
	CookieSecure    bool
	ReportCity      string
	AlertWebhookURL string
	loginLimiter    *loginLimiter
	metrics         *appMetrics
	alertMu         sync.Mutex
	lastAlert       map[string]time.Time
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
	return &Server{
		Store: st, Tpls: tpls, BackupDir: backupDir,
		SessionTTL: 12 * time.Hour, ReportCity: "Jakarta",
		loginLimiter: newLoginLimiter(),
		metrics:      newAppMetrics(),
		lastAlert:    map[string]time.Time{},
	}, nil
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
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Store.Ping(ctx); err != nil {
			s.alert("database_down", "health check database gagal: "+err.Error(), r)
			http.Error(w, "database tidak siap", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /login", s.handleLoginGet)
	mux.HandleFunc("POST /login", s.handleLoginPost)
	mux.HandleFunc("POST /logout", s.handleLogout)

	mux.HandleFunc("GET /admin/users", s.requireAdmin(s.handleUsersGet))
	mux.HandleFunc("POST /admin/users", s.requireAdmin(s.handleUserCreate))
	mux.HandleFunc("POST /admin/users/{id}/toggle", s.requireAdmin(s.handleUserToggle))
	mux.HandleFunc("POST /admin/users/{id}/password", s.requireAdmin(s.handleUserPassword))
	mux.HandleFunc("GET /admin/audit", s.requireAdmin(s.handleAudit))
	mux.HandleFunc("GET /admin/errors", s.requireAdmin(s.handleErrors))
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /search", s.handleSearch)
	mux.HandleFunc("GET /export-database", s.requireAdmin(s.handleExportDB))
	mux.HandleFunc("POST /import-database", s.requireAdmin(s.handleImportDB))

	mux.HandleFunc("GET /pihak-ketiga", s.handlePihakKetigaList)
	mux.HandleFunc("GET /pihak-ketiga/tambah", s.handlePihakKetigaCreateGet)
	mux.HandleFunc("POST /pihak-ketiga/tambah", s.handlePihakKetigaCreatePost)
	mux.HandleFunc("GET /pihak-ketiga/{id}", s.handlePihakKetigaDetail)
	mux.HandleFunc("GET /pihak-ketiga/{id}/edit", s.handlePihakKetigaEditGet)
	mux.HandleFunc("POST /pihak-ketiga/{id}/edit", s.handlePihakKetigaEditPost)
	mux.HandleFunc("POST /pihak-ketiga/{id}/hapus", s.handlePihakKetigaDelete)
	mux.HandleFunc("POST /pihak-ketiga/{id}/rekening/tambah", s.handlePihakKetigaRekeningAdd)
	mux.HandleFunc("POST /pihak-ketiga/{id}/rekening/{rid}/edit", s.handlePihakKetigaRekeningEdit)
	mux.HandleFunc("POST /pihak-ketiga/{id}/rekening/{rid}/hapus", s.handlePihakKetigaRekeningDelete)

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

	var h http.Handler = mux
	h = s.withRequestLog(h)
	h = s.withAuth(h)
	h = s.withCSRF(h)
	h = s.withMetrics(h)
	h = s.withRequestID(h)
	h = s.withBodyLimit(h)
	h = s.withSecurityHeaders(h)
	return s.withRecovery(h)
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

// formFloat membaca angka desimal dari form (mendukung koma desimal & '%').
func formFloat(r *http.Request, name string) float64 {
	return parseFloatStr(r.FormValue(name))
}

// parseFloatStr membaca angka desimal (mendukung koma desimal & '%').
func parseFloatStr(raw string) float64 {
	s := strings.TrimSpace(strings.ReplaceAll(raw, ",", "."))
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
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
	t, ok := s.Tpls[name]
	if !ok {
		log.Printf("render %s: template tidak ditemukan", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data = injectBaseFields(data, userFromContext(r.Context()), csrfFromContext(r.Context()), r.URL.Query().Get("q"))
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		log.Printf("render %s request_id=%s: %v", name, requestIDFromContext(r.Context()), err)
		s.recordError(r, "render "+name+": "+err.Error(), "")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// injectBaseFields mengisi User/CSRF pada baseView yang di-embed oleh semua
// struct data halaman. Reflection dipakai agar tidak perlu mengubah puluhan
// literal baseView di setiap handler.
func injectBaseFields(data any, u *store.User, csrf, searchQuery string) any {
	if data == nil {
		return data
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return data
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return data
	}
	// Handler umumnya mengirim struct by value. Buat salinan addressable agar
	// field promoted dari baseView bisa diisi tanpa mengubah semua handler.
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	if f := out.FieldByName("User"); f.IsValid() && f.CanSet() {
		f.Set(reflect.ValueOf(u))
	}
	if f := out.FieldByName("CSRF"); f.IsValid() && f.CanSet() {
		f.SetString(csrf)
	}
	if f := out.FieldByName("SearchQuery"); f.IsValid() && f.CanSet() {
		f.SetString(searchQuery)
	}
	return out.Interface()
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error, url string) {
	log.Printf("error request_id=%s: %v", requestIDFromContext(r.Context()), err)
	if !isExpectedError(err) {
		s.recordError(r, err.Error(), "")
	}
	msg := "Terjadi kesalahan. Silakan coba lagi."
	switch {
	case errors.Is(err, store.ErrInUse):
		msg = "Data tidak bisa dihapus karena masih dipakai transaksi."
	case errors.Is(err, store.ErrInvoiceHasTaxDeposit):
		msg = "Tagihan sudah memiliki setoran pajak. Hapus/edit setoran pajak terlebih dahulu."
	case errors.Is(err, store.ErrTaxAlreadyDeposited):
		msg = "Pajak tagihan tersebut sudah disetor."
	case errors.Is(err, store.ErrRealisasiMismatch):
		msg = "Total realisasi harus sama dengan bruto tagihan."
	case errors.Is(err, store.ErrBudgetExceeded):
		msg = "Realisasi/pagu melebihi batas komponen."
	case errors.Is(err, store.ErrDuplicateInvoiceNo):
		msg = "Nomor bukti tagihan sudah dipakai."
	case errors.Is(err, store.ErrPencairanExceedsTotal):
		msg = "Total pencairan melebihi nominal bantuan."
	case errors.Is(err, store.ErrInvalidInput), errors.Is(err, store.ErrNotFound):
		msg = "Input/data tidak valid."
	case errors.Is(err, store.ErrInvalidDate):
		msg = "Tanggal tidak valid."
	case errors.Is(err, money.ErrInvalidAmount), errors.Is(err, money.ErrNegativeAmount),
		errors.Is(err, money.ErrZeroAmount):
		msg = "Nominal tidak valid atau harus lebih dari 0."
	}
	s.setFlash(w, "danger", msg)
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
