package server

import (
	"encoding/json"
	"net/http"
	"time"

	"ebku/internal/export"
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

// rekapPenggunaanViewRow adalah satu baris tabel Rekap Penggunaan Dana di
// halaman web. Dist sejajar dengan kolom distribusi yang dipilih user.
type rekapPenggunaanViewRow struct {
	Tanggal time.Time
	Bukti   string
	Uraian  string
	Bruto   int64
	Dist    []int64
	PPN     int64
	PPh21   int64
	PPh22   int64
	PPh23   int64
}

func (s *Server) handleRekapPenggunaanDana(w http.ResponseWriter, r *http.Request) {
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
	data, err := s.Store.ListRekapPenggunaan(r.Context(), id)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	opts := rekapPenggunaanOptsFromQuery(r.URL.Query())
	dist, headerRows, _ := s.rekapDistColsData(data, opts)

	rows := make([]rekapPenggunaanViewRow, 0, len(data.Rows))
	distTotals := make([]int64, len(dist))
	var tBruto, tPPN, tPPh21, tPPh22, tPPh23 int64
	for _, rw := range data.Rows {
		vr := rekapPenggunaanViewRow{
			Tanggal: rw.Tanggal, Bukti: rw.NomorBukti, Uraian: rw.Uraian, Bruto: rw.Bruto,
			PPN: rw.NilaiPPN, PPh21: rw.PPh21, PPh22: rw.PPh22, PPh23: rw.PPh23,
		}
		vr.Dist = make([]int64, len(dist))
		for j, c := range dist {
			vr.Dist[j] = rw.Values[c.Level+"\x00"+c.Name]
			distTotals[j] += vr.Dist[j]
		}
		rows = append(rows, vr)
		tBruto += rw.Bruto
		tPPN += rw.NilaiPPN
		tPPh21 += rw.PPh21
		tPPh22 += rw.PPh22
		tPPh23 += rw.PPh23
	}
	distNames := make([]string, len(dist))
	for i, c := range dist {
		distNames[i] = c.Name
	}
	numCount := len(dist)
	if numCount == 0 {
		numCount = 1
	}
	colCount := 4 + numCount
	if opts.Pajak {
		colCount += 4
	}
	ft, fm := s.getFlash(w, r)
	view := struct {
		baseView
		Rows         []rekapPenggunaanViewRow
		DistCols     []string
		DistTotals   []int64
		HasDist      bool
		HeaderRows   [][]export.HeaderCell
		Total        int64
		TotalPPN     int64
		TotalPPh21   int64
		TotalPPh22   int64
		TotalPPh23   int64
		SelKegiatan  bool
		SelSubKeg    bool
		SelAktivitas bool
		SelKomponen  bool
		SelPajak     bool
		NumCols      int
		MergeCols    int
		ColCount     int
	}{
		baseView: baseView{Title: "Rekap Penggunaan Dana", Active: "rekap-penggunaan-dana", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Rows: rows, DistCols: distNames, DistTotals: distTotals, HasDist: len(dist) > 0,
		HeaderRows: headerRows,
		Total:      tBruto, TotalPPN: tPPN, TotalPPh21: tPPh21, TotalPPh22: tPPh22, TotalPPh23: tPPh23,
		SelKegiatan: opts.Kegiatan, SelSubKeg: opts.SubKeg, SelAktivitas: opts.Aktivitas,
		SelKomponen: opts.Komponen, SelPajak: opts.Pajak,
		NumCols: numCount, MergeCols: 4, ColCount: colCount,
	}
	s.render(w, r, "rekap_penggunaan_dana.html", view)
}
