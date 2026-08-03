package server

import (
	"encoding/json"
	"net/http"

	"ebku/internal/store"
)

func (s *Server) handleRekapPajak(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.buildRekapRows(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	setors, err := s.Store.ListSetorEntries(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	jasas, err := s.Store.ListJasaGiro(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows            []rekapRow
		Setors          []store.Ledger
		JasaGiro        []store.JasaGiro
		TotalPemungutan int64
		TotalPenyetoran int64
		TotalJasaGiro   int64
	}{
		baseView: baseView{Title: "Rekap Pajak", Active: "rekap-pajak", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Rows: rows, Setors: setors, JasaGiro: jasas,
		TotalPemungutan: s.Store.TotalPemungutan(r.Context(), id),
		TotalPenyetoran: s.Store.TotalSetorPajak(r.Context(), id),
		TotalJasaGiro:   s.Store.TotalJasaGiro(r.Context(), id),
	}
	s.render(w, r, "rekap_pajak.html", data)
}

func (s *Server) handleRekapBelanja(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.Store.ListBelanja(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	var tBruto, tPPN, tPPh, tAdmin int64
	for _, rw := range rows {
		tBruto += rw.Bruto
		tPPN += rw.PPN
		tPPh += rw.PPH
		tAdmin += rw.BiayaAdmin
	}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows          []store.BelanjaRow
		TotBruto      int64
		TotPPN        int64
		TotPPh        int64
		TotBiayaAdmin int64
	}{
		baseView: baseView{Title: "Rekap Belanja", Active: "rekap-belanja", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Rows: rows, TotBruto: tBruto, TotPPN: tPPN, TotPPh: tPPh, TotBiayaAdmin: tAdmin,
	}
	s.render(w, r, "rekap_belanja.html", data)
}

func (s *Server) handleRekapBelanjaReorder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var body struct {
		Order []int64 `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	seen := map[int64]bool{}
	var ids []int64
	for _, iid := range body.Order {
		if iid > 0 && !seen[iid] {
			seen[iid] = true
			ids = append(ids, iid)
		}
	}
	if err := s.Store.SetInvoiceOrder(r.Context(), id, ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
