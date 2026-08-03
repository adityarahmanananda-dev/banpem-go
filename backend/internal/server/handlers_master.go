package server

import (
	"encoding/json"
	"net/http"

	"ebku/internal/money"
	"ebku/internal/store"
)

func (s *Server) handleKegiatan(w http.ResponseWriter, r *http.Request) {
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
	tree, err := s.Store.ListKegiatanTree(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	var totalPagu int64
	for _, k := range tree {
		totalPagu += k.Pagu
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Tree      []store.KegiatanTree
		TotalPagu int64
	}{
		baseView: baseView{Title: "Data Kegiatan", Active: "master", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Tree: tree, TotalPagu: totalPagu,
	}
	s.render(w, r, "kegiatan.html", data)
}

func (s *Server) handleKegiatanTambah(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	nama := formStr(r, "nama")
	if nama != "" {
		if _, err := s.Store.CreateKegiatan(r.Context(), id, nama); err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.setFlash(w, "success", "Kegiatan berhasil ditambahkan.")
	}
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleKegiatanHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	kid, err := pathID(r, "kid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteKegiatan(r.Context(), kid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.setFlash(w, "success", "Kegiatan berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleSubKegiatanTambah(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	kegiatanID := formInt(r, "kegiatan_id")
	nama := formStr(r, "nama")
	if kegiatanID > 0 && nama != "" {
		if _, err := s.Store.CreateSubKegiatan(r.Context(), kegiatanID, nama); err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.setFlash(w, "success", "Sub kegiatan berhasil ditambahkan.")
	}
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleSubKegiatanHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	sid, err := pathID(r, "sid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteSubKegiatan(r.Context(), sid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.setFlash(w, "success", "Sub kegiatan berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleKomponenTambah(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	subID := formInt(r, "sub_kegiatan_id")
	nama := formStr(r, "nama")
	pagu, perr := money.Parse(formStr(r, "pagu"))
	if perr != nil || pagu < 0 {
		pagu = 0
	}
	if subID > 0 && nama != "" {
		if _, err := s.Store.CreateKomponen(r.Context(), subID, nama, pagu); err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.setFlash(w, "success", "Komponen berhasil ditambahkan.")
	}
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleKomponenEditGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	cid, err := pathID(r, "cid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ko, err := s.Store.GetKomponen(r.Context(), cid)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		K store.Komponen
	}{
		baseView: baseView{Title: "Edit Komponen", Active: "master", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		K: ko,
	}
	s.render(w, r, "komponen_edit.html", data)
}

func (s *Server) handleKomponenEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	cid, err := pathID(r, "cid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	nama := formStr(r, "nama")
	pagu, perr := money.Parse(formStr(r, "pagu"))
	if perr != nil || pagu < 0 {
		pagu = 0
	}
	if err := s.Store.UpdateKomponen(r.Context(), cid, nama, pagu); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.setFlash(w, "success", "Komponen berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleKomponenHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	cid, err := pathID(r, "cid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteKomponen(r.Context(), cid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.setFlash(w, "success", "Komponen berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

// handleAPISubKegiatan: GET /api/bantuan/{id}/sub-kegiatan?kegiatan_id=X
func (s *Server) handleAPISubKegiatan(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	kid := formInt(r, "kegiatan_id")
	rows, err := s.Store.ListSubKegiatanBantuan(r.Context(), id, kid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

// handleAPIKomponen: GET /api/bantuan/{id}/komponen?sub_kegiatan_id=X
func (s *Server) handleAPIKomponen(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	skid := formInt(r, "sub_kegiatan_id")
	rows, err := s.Store.ListKomponenBantuan(r.Context(), id, skid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}
