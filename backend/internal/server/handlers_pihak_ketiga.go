package server

import (
	"net/http"
	"strings"

	"ebku/internal/store"
)

func (s *Server) handlePihakKetigaList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListPihakKetiga(r.Context())
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows []store.PihakKetiga
	}{
		baseView: baseView{Title: "Pihak Ketiga", Active: "pihak-ketiga", FlashType: ft, FlashMsg: fm},
		Rows:     rows,
	}
	s.render(w, r, "pihak_ketiga.html", data)
}

func (s *Server) handlePihakKetigaCreateGet(w http.ResponseWriter, r *http.Request) {
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		P      store.PihakKetiga
		IsEdit bool
	}{
		baseView: baseView{Title: "Tambah Pihak Ketiga", Active: "pihak-ketiga", FlashType: ft, FlashMsg: fm},
		IsEdit:   false,
	}
	s.render(w, r, "pihak_ketiga_form.html", data)
}

func (s *Server) handlePihakKetigaCreatePost(w http.ResponseWriter, r *http.Request) {
	nama := formStr(r, "nama")
	npwp := formStr(r, "npwp")
	if nama == "" {
		s.setFlash(w, "danger", "Nama pihak ketiga wajib diisi.")
		s.redirect(w, r, "/pihak-ketiga/tambah")
		return
	}
	id, err := s.Store.CreatePihakKetiga(r.Context(), nama, npwp)
	if err != nil {
		s.setFlash(w, "danger", "Gagal menyimpan pihak ketiga. Pastikan nama belum dipakai.")
		s.redirect(w, r, "/pihak-ketiga/tambah")
		return
	}
	s.audit(r, nil, "create", "pihak_ketiga", &id, map[string]any{"nama": nama})
	s.setFlash(w, "success", "Pihak ketiga berhasil ditambahkan. Tambahkan rekeningnya di halaman detail.")
	s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
}

func (s *Server) handlePihakKetigaEditGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	p, err := s.Store.GetPihakKetiga(r.Context(), id)
	if err != nil {
		s.fail(w, r, store.ErrNotFound, "/pihak-ketiga")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		P      store.PihakKetiga
		IsEdit bool
	}{
		baseView: baseView{Title: "Edit Pihak Ketiga", Active: "pihak-ketiga", FlashType: ft, FlashMsg: fm},
		P:        p,
		IsEdit:   true,
	}
	s.render(w, r, "pihak_ketiga_form.html", data)
}

func (s *Server) handlePihakKetigaEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	nama := formStr(r, "nama")
	if nama == "" {
		s.setFlash(w, "danger", "Nama pihak ketiga wajib diisi.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id)+"/edit")
		return
	}
	if err := s.Store.UpdatePihakKetiga(r.Context(), id, nama, formStr(r, "npwp")); err != nil {
		s.setFlash(w, "danger", "Gagal memperbarui pihak ketiga. Pastikan nama belum dipakai.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id)+"/edit")
		return
	}
	s.audit(r, nil, "update", "pihak_ketiga", &id, map[string]any{"nama": nama})
	s.setFlash(w, "success", "Pihak ketiga berhasil diperbarui.")
	s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
}

func (s *Server) handlePihakKetigaDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	if err := s.Store.DeletePihakKetiga(r.Context(), id); err != nil {
		s.setFlash(w, "danger", "Gagal menghapus pihak ketiga.")
		s.redirect(w, r, "/pihak-ketiga")
		return
	}
	s.audit(r, nil, "delete", "pihak_ketiga", &id, nil)
	s.setFlash(w, "success", "Pihak ketiga berhasil dihapus. Tagihan lama tetap menyimpan snapshot rekeningnya.")
	s.redirect(w, r, "/pihak-ketiga")
}

func (s *Server) handlePihakKetigaDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	p, err := s.Store.GetPihakKetiga(r.Context(), id)
	if err != nil {
		s.fail(w, r, store.ErrNotFound, "/pihak-ketiga")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		P store.PihakKetiga
	}{
		baseView: baseView{Title: "Rekening Pihak Ketiga", Active: "pihak-ketiga", FlashType: ft, FlashMsg: fm},
		P:        p,
	}
	s.render(w, r, "pihak_ketiga_detail.html", data)
}

func (s *Server) handlePihakKetigaRekeningAdd(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	nama := formStr(r, "nama_rekening")
	nomor := formStr(r, "nomor_rekening")
	bank := formStr(r, "bank")
	if strings.TrimSpace(nama) == "" && strings.TrimSpace(nomor) == "" && strings.TrimSpace(bank) == "" {
		s.setFlash(w, "danger", "Isi minimal nama rekening, nomor rekening, atau bank.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
		return
	}
	rid, err := s.Store.AddPihakKetigaRekening(r.Context(), id, nama, nomor, bank)
	if err != nil {
		s.setFlash(w, "danger", "Gagal menambah rekening. Pastikan rekening belum terdaftar.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
		return
	}
	s.audit(r, nil, "create", "pihak_ketiga_rekening", &rid, map[string]any{"pihak_ketiga_id": id})
	s.setFlash(w, "success", "Rekening berhasil ditambahkan.")
	s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
}

func (s *Server) handlePihakKetigaRekeningEdit(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	rid, err := pathID(r, "rid")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	nama := formStr(r, "nama_rekening")
	nomor := formStr(r, "nomor_rekening")
	bank := formStr(r, "bank")
	if nama == "" && nomor == "" && bank == "" {
		s.setFlash(w, "danger", "Isi minimal nama rekening, nomor rekening, atau bank.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
		return
	}
	if err := s.Store.UpdatePihakKetigaRekening(r.Context(), rid, id, nama, nomor, bank); err != nil {
		s.setFlash(w, "danger", "Gagal memperbarui rekening.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
		return
	}
	s.audit(r, nil, "update", "pihak_ketiga_rekening", &rid, map[string]any{"pihak_ketiga_id": id})
	s.setFlash(w, "success", "Rekening berhasil diperbarui.")
	s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
}

func (s *Server) handlePihakKetigaRekeningDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	rid, err := pathID(r, "rid")
	if err != nil {
		s.fail(w, r, err, "/pihak-ketiga")
		return
	}
	if err := s.Store.DeletePihakKetigaRekening(r.Context(), rid, id); err != nil {
		s.setFlash(w, "danger", "Gagal menghapus rekening.")
		s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
		return
	}
	s.audit(r, nil, "delete", "pihak_ketiga_rekening", &rid, map[string]any{"pihak_ketiga_id": id})
	s.setFlash(w, "success", "Rekening berhasil dihapus.")
	s.redirect(w, r, "/pihak-ketiga/"+i64s(id))
}
