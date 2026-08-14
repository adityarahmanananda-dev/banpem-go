package server

import (
	"net/http"
	"strconv"
	"time"

	"ebku/internal/money"
	"ebku/internal/store"
)

func (s *Server) handleBantuanMenu(w http.ResponseWriter, r *http.Request) {
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
	jenis := r.PathValue("jenis")
	if jenis != "transaksi" && jenis != "laporan" {
		jenis = "informasi"
	}
	data := struct {
		baseView
		Jenis string
	}{
		baseView: baseView{Title: "Menu " + b.Nama, Active: "bantuan-menu", Bantuan: &b,
			Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Jenis: jenis,
	}
	s.render(w, r, "menu.html", data)
}

func bantuanFromForm(r *http.Request) store.Bantuan {
	nominal, _ := money.Parse(formStr(r, "nominal"))
	var nominalPtr *int64
	if nominal > 0 {
		nominalPtr = &nominal
	}
	return store.Bantuan{
		Nama:             formStr(r, "nama"),
		NamaSekolah:      formStr(r, "nama_sekolah"),
		NamaRekening:     formStr(r, "bank_penerima"),
		NomorRekening:    formStr(r, "no_rekening"),
		Bank:             formStr(r, "bank_bantuan"),
		NPWP:             formStr(r, "npwp"),
		KepalaSekolah:    formStr(r, "kepala_sekolah"),
		NIPKepalaSekolah: formStr(r, "nip_kepala"),
		Bendahara:        formStr(r, "bendahara"),
		NIPBendahara:     formStr(r, "nip_bendahara"),
		Nominal:          nominalPtr,
	}
}

func (s *Server) handleBantuanTambahGet(w http.ResponseWriter, r *http.Request) {
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		B store.Bantuan
	}{
		baseView: baseView{Title: "Tambah Bantuan", Active: "bantuan", FlashType: ft, FlashMsg: fm, Q: map[string]string{}},
	}
	s.render(w, r, "bantuan_form.html", data)
}

func (s *Server) handleBantuanTambahPost(w http.ResponseWriter, r *http.Request) {
	b := bantuanFromForm(r)
	if b.Nama == "" {
		s.setFlash(w, "danger", "Nama bantuan wajib diisi.")
		s.redirect(w, r, "/bantuan/tambah")
		return
	}
	id, err := s.Store.CreateBantuan(r.Context(), b)
	if err != nil {
		s.fail(w, r, err, "/bantuan/tambah")
		return
	}
	s.setFlash(w, "success", "Bantuan berhasil ditambahkan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func (s *Server) handleBantuanEditGet(w http.ResponseWriter, r *http.Request) {
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
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		B store.Bantuan
	}{
		baseView: baseView{Title: "Edit Bantuan", Active: "bantuan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Q: map[string]string{}},
		B: b,
	}
	s.render(w, r, "bantuan_form.html", data)
}

func (s *Server) handleBantuanEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b := bantuanFromForm(r)
	b.ID = id
	if err := s.Store.UpdateBantuan(r.Context(), b); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/edit")
		return
	}
	s.setFlash(w, "success", "Bantuan berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func (s *Server) handleSaldoAwalGet(w http.ResponseWriter, r *http.Request) {
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
	sa, _ := s.Store.GetSaldoAwal(r.Context(), id)
	var saldo int64
	var tanggal string
	if sa != nil {
		saldo = sa.Saldo
		if sa.Tanggal != nil {
			tanggal = sa.Tanggal.Format("2006-01-02")
		}
	}
	if tanggal == "" {
		tanggal = time.Now().Format("2006-01-02")
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Saldo   int64
		Tanggal string
	}{
		baseView: baseView{Title: "Saldo Awal", Active: "saldo", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Saldo:   saldo,
		Tanggal: tanggal,
	}
	s.render(w, r, "saldo_awal.html", data)
}

func (s *Server) handleSaldoAwalPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	saldo, _ := money.Parse(formStr(r, "saldo_awal"))
	tgl := store.ParseDate(formStr(r, "tanggal"))
	if err := s.Store.SetSaldoAwal(r.Context(), id, saldo, tgl); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/saldo-awal")
		return
	}
	s.setFlash(w, "success", "Saldo awal berhasil disimpan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bku")
}

func i64s(v int64) string {
	return strconv.FormatInt(v, 10)
}
