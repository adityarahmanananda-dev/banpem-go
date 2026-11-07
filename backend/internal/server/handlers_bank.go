package server

import (
	"encoding/json"
	"log"
	"net/http"

	"ebku/internal/store"
)

func (s *Server) handleBank(w http.ResponseWriter, r *http.Request) {
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
	vi := s.versiInfo(r.Context(), id, r.URL.Query().Get("versi"))
	rows, err := store.ListLedger(r.Context(), s.Store.Pool, id, "bank")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	views := applyVersion(rows, vi.Base)
	from, to := dateRangeFromQuery(r)
	if from != nil || to != nil {
		filtered := make([]ledgerView, 0, len(views))
		for _, v := range views {
			if datePtrInRange(v.Tanggal, from, to) {
				filtered = append(filtered, v)
			}
		}
		views = filtered
	}
	var totDebit, totKredit int64
	for _, v := range views {
		totDebit += v.Debit
		totKredit += v.KreditDisp
	}
	var saldoAkhir int64
	if len(views) > 0 {
		saldoAkhir = views[len(views)-1].SaldoDisp
	}
	page, perPage := pageParams(r)
	pagination := buildPagination(r, page, perPage, len(views))
	views = paginateSlice(views, pagination.Page, perPage)
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Rows       []ledgerView
		TotDebit   int64
		TotKredit  int64
		SaldoAkhir int64
		Versi      versiInfo
		Pagination Pagination
		Dari       string
		Sampai     string
	}{
		baseView: baseView{Title: "Buku Kas Bank", Active: "bank", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Rows: views, TotDebit: totDebit, TotKredit: totKredit, SaldoAkhir: saldoAkhir, Versi: vi,
		Pagination: pagination, Dari: r.URL.Query().Get("dari"), Sampai: r.URL.Query().Get("sampai"),
	}
	s.render(w, r, "bank.html", data)
}

func (s *Server) handleBankReorder(w http.ResponseWriter, r *http.Request) {
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
	if err := s.Store.ReorderLedger(r.Context(), id, "bank", body.Order); err != nil {
		log.Printf("reorder bank: %v", err)
		http.Error(w, "gagal menyimpan urutan", http.StatusInternalServerError)
		return
	}
	s.audit(r, &id, "reorder", "ledger_bank", nil, map[string]any{"count": len(body.Order)})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleBankHapus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	eid, err := pathID(r, "eid")
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	if err := s.Store.DeleteBankLedger(r.Context(), id, eid); err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bank")
		return
	}
	s.audit(r, &id, "delete", "ledger_bank", &eid, nil)
	s.setFlash(w, "success", "Entri bank berhasil dihapus.")
	s.redirect(w, r, "/bantuan/"+i64s(id)+"/bank")
}
