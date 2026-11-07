package server

import (
	"net/http"

	"ebku/internal/money"
	"ebku/internal/store"
)

// ---- jasa giro ----

func jasaGiroURL(id int64, ret string) string {
	if ret == "rekap" {
		return "/bantuan/" + i64s(id) + "/rekap-pajak"
	}
	return "/bantuan/" + i64s(id) + "/jasa-giro"
}

func jasaGiroTambahURL(id int64, ret string) string {
	u := "/bantuan/" + i64s(id) + "/jasa-giro/tambah"
	if ret == "rekap" {
		u += "?return=rekap"
	}
	return u
}

func (s *Server) handleJasaGiro(w http.ResponseWriter, r *http.Request) {
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
	list, err := s.Store.ListJasaGiro(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		JasaGiro []store.JasaGiro
		Total    int64
	}{
		baseView: baseView{Title: "Jasa Giro", Active: "keuangan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		JasaGiro: list, Total: s.Store.TotalJasaGiro(r.Context(), id),
	}
	s.render(w, r, "jasa_giro.html", data)
}

func (s *Server) handleJasaGiroTambahGet(w http.ResponseWriter, r *http.Request) {
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
		Tanggal string
		Return  string
	}{
		baseView: baseView{Title: "Tambah Jasa Giro", Active: "keuangan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Tanggal: store.TodayString(),
		Return:  r.URL.Query().Get("return"),
	}
	s.render(w, r, "jasa_giro_form.html", data)
}

func (s *Server) handleJasaGiroTambahPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ret := r.URL.Query().Get("return")
	nominal, err := money.ParsePositive(formStr(r, "nominal"))
	if err != nil {
		s.setFlash(w, "danger", "Nominal harus lebih dari 0.")
		s.redirect(w, r, jasaGiroTambahURL(id, ret))
		return
	}
	tgl, err := store.ParseDateStrict(formStr(r, "tanggal"))
	if err != nil {
		s.fail(w, r, err, jasaGiroTambahURL(id, ret))
		return
	}
	uraian := formStr(r, "uraian")
	if uraian == "" {
		uraian = "Jasa Giro"
	}
	if err := s.Store.CreateJasaGiro(r.Context(), id, tgl, nominal, uraian); err != nil {
		s.fail(w, r, err, jasaGiroTambahURL(id, ret))
		return
	}
	s.audit(r, &id, "create", "jasa_giro", nil, map[string]any{"nominal": nominal})
	s.setFlash(w, "success", "Jasa giro berhasil disimpan.")
	s.redirect(w, r, jasaGiroURL(id, ret))
}

func (s *Server) handleJasaGiroHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	jid, err := pathID(r, "jid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ret := r.URL.Query().Get("return")
	if err := s.Store.DeleteJasaGiro(r.Context(), id, jid); err != nil {
		s.fail(w, r, err, jasaGiroURL(id, ret))
		return
	}
	s.audit(r, &id, "delete", "jasa_giro", &jid, nil)
	s.setFlash(w, "success", "Jasa giro berhasil dihapus.")
	s.redirect(w, r, jasaGiroURL(id, ret))
}

// ---- pencairan ----

func (s *Server) handlePencairan(w http.ResponseWriter, r *http.Request) {
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
	list, err := s.Store.ListPencairan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	total := s.Store.TotalTurun(r.Context(), id)
	nominal := int64(0)
	if b.Nominal != nil {
		nominal = *b.Nominal
	}
	var persen int64
	if nominal > 0 {
		persen = money.MulDiv(total, 100, nominal)
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Pencairan []store.Pencairan
		Total     int64
		Nominal   int64
		Persen    int64
	}{
		baseView: baseView{Title: "Pencairan / Saldo Awal", Active: "keuangan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Pencairan: list, Total: total, Nominal: nominal, Persen: persen,
	}
	s.render(w, r, "pencairan.html", data)
}

func (s *Server) handlePencairanTambahGet(w http.ResponseWriter, r *http.Request) {
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
		P       store.Pencairan
		Tanggal string
	}{
		baseView: baseView{Title: "Tambah Pencairan", Active: "keuangan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		P:       store.Pencairan{},
		Tanggal: store.TodayString(),
	}
	s.render(w, r, "pencairan_form.html", data)
}

func parsePencairanForm(r *http.Request) (store.Pencairan, error) {
	nominal, err := money.ParsePositive(formStr(r, "nominal"))
	if err != nil {
		return store.Pencairan{}, err
	}
	t, err := store.ParseDateStrict(formStr(r, "tanggal"))
	if err != nil {
		return store.Pencairan{}, err
	}
	return store.Pencairan{
		Tanggal:    &t,
		Nominal:    nominal,
		Keterangan: formStr(r, "keterangan"),
	}, nil
}

func (s *Server) handlePencairanTambahPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	p, err := parsePencairanForm(r)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/pencairan/tambah")
		return
	}
	if err := s.Store.CreatePencairan(r.Context(), id, p); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/pencairan/tambah")
		return
	}
	s.audit(r, &id, "create", "pencairan", nil, map[string]any{"nominal": p.Nominal})
	s.setFlash(w, "success", "Pencairan berhasil ditambahkan.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/pencairan")
}

func (s *Server) handlePencairanEditGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	pid, err := pathID(r, "pid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	b, err := s.Store.GetBantuan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	p, err := s.Store.GetPencairan(r.Context(), id, pid)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		P       store.Pencairan
		Tanggal string
	}{
		baseView: baseView{Title: "Edit Pencairan", Active: "keuangan", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		P: p,
	}
	if p.Tanggal != nil {
		data.Tanggal = p.Tanggal.Format("2006-01-02")
	}
	s.render(w, r, "pencairan_form.html", data)
}

func (s *Server) handlePencairanEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	pid, err := pathID(r, "pid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	p, err := parsePencairanForm(r)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/pencairan/edit/"+i64s(pid))
		return
	}
	p.ID = pid
	p.BantuanID = id
	if err := s.Store.UpdatePencairan(r.Context(), id, p); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/pencairan/edit/"+i64s(pid))
		return
	}
	s.audit(r, &id, "update", "pencairan", &pid, map[string]any{"nominal": p.Nominal})
	s.setFlash(w, "success", "Pencairan berhasil diperbarui.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/pencairan")
}

func (s *Server) handlePencairanHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	pid, err := pathID(r, "pid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeletePencairan(r.Context(), id, pid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/pencairan")
		return
	}
	s.audit(r, &id, "delete", "pencairan", &pid, nil)
	s.setFlash(w, "success", "Pencairan berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/pencairan")
}
