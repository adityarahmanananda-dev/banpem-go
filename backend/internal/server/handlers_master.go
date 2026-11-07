package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
		baseView: baseView{Title: "Rencana Anggaran Biaya", Active: "master", FlashType: ft, FlashMsg: fm,
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
		kid, err := s.Store.CreateKegiatan(r.Context(), id, nama)
		if err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.audit(r, &id, "create", "kegiatan", &kid, map[string]any{"nama": nama})
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
	if err := s.Store.DeleteKegiatan(r.Context(), id, kid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.audit(r, &id, "delete", "kegiatan", &kid, nil)
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
		skid, err := s.Store.CreateSubKegiatan(r.Context(), id, kegiatanID, nama)
		if err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.audit(r, &id, "create", "sub_kegiatan", &skid, map[string]any{"nama": nama})
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
	if err := s.Store.DeleteSubKegiatan(r.Context(), id, sid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.audit(r, &id, "delete", "sub_kegiatan", &sid, nil)
	s.setFlash(w, "success", "Sub kegiatan berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleAktivitasTambah(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	subID := formInt(r, "sub_kegiatan_id")
	nama := formStr(r, "nama")
	if subID > 0 && nama != "" {
		aid, err := s.Store.CreateAktivitas(r.Context(), id, subID, nama)
		if err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
			return
		}
		s.audit(r, &id, "create", "aktivitas", &aid, map[string]any{"nama": nama})
		s.setFlash(w, "success", "Aktivitas berhasil ditambahkan.")
	}
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleAktivitasHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	aid, err := pathID(r, "aid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteAktivitas(r.Context(), id, aid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.audit(r, &id, "delete", "aktivitas", &aid, nil)
	s.setFlash(w, "success", "Aktivitas berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/kegiatan")
}

func (s *Server) handleKomponenTambah(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	aktID := formInt(r, "aktivitas_id")
	_ = r.ParseForm()
	names := r.PostForm["nama"]
	pagus := r.PostForm["pagu"]
	var inputs []store.KomponenInput
	for i, nama := range names {
		nama = strings.TrimSpace(nama)
		if nama == "" {
			continue
		}
		pagu := int64(0)
		if i < len(pagus) && strings.TrimSpace(pagus[i]) != "" {
			p, perr := money.ParseNonNegative(pagus[i])
			if perr != nil {
				s.fail(w, r, perr, "/bantuan/"+i64s(id)+"/kegiatan")
				return
			}
			pagu = p
		}
		inputs = append(inputs, store.KomponenInput{Nama: nama, Pagu: pagu})
	}
	created, err := s.Store.CreateKomponens(r.Context(), id, aktID, inputs)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	if created > 0 {
		s.audit(r, &id, "create", "komponen", nil, map[string]any{"count": created})
		s.setFlash(w, "success", fmt.Sprintf("%d komponen berhasil ditambahkan.", created))
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
	ko, err := s.Store.GetKomponen(r.Context(), id, cid)
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
	pagu, perr := money.ParseNonNegative(formStr(r, "pagu"))
	if perr != nil {
		s.fail(w, r, perr, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	if err := s.Store.UpdateKomponen(r.Context(), id, cid, nama, pagu); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.audit(r, &id, "update", "komponen", &cid, map[string]any{"nama": nama, "pagu": pagu})
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
	if err := s.Store.DeleteKomponen(r.Context(), id, cid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/kegiatan")
		return
	}
	s.audit(r, &id, "delete", "komponen", &cid, nil)
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
		http.Error(w, "gagal memuat data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

// handleAPIAktivitas: GET /api/bantuan/{id}/aktivitas?sub_kegiatan_id=X
func (s *Server) handleAPIAktivitas(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	skid := formInt(r, "sub_kegiatan_id")
	rows, err := s.Store.ListAktivitasBantuan(r.Context(), id, skid)
	if err != nil {
		http.Error(w, "gagal memuat data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

// handleAPIKomponen: GET /api/bantuan/{id}/komponen?aktivitas_id=X
func (s *Server) handleAPIKomponen(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	aktID := formInt(r, "aktivitas_id")
	rows, err := s.Store.ListKomponenBantuan(r.Context(), id, aktID)
	if err != nil {
		http.Error(w, "gagal memuat data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}
