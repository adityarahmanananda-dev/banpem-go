package server

import (
	"net/http"

	"ebku/internal/store"
)

var pivotLevelDefs = []struct {
	Key  string
	Name string
}{
	{"kegiatan", "Kegiatan"},
	{"sub", "Sub Kegiatan"},
	{"aktivitas", "Aktivitas"},
	{"komponen", "Komponen"},
}

// pivotGroups membaca pilihan level dari query (?group_kegiatan=1&group_sub=1).
// kosong => default semua level.
func pivotGroups(r *http.Request) []string {
	var out []string
	for _, def := range pivotLevelDefs {
		if r.URL.Query().Get("group_"+def.Key) == "1" {
			out = append(out, def.Key)
		}
	}
	return out
}

func pivotName(row store.PivotRow, key string) string {
	switch key {
	case "kegiatan":
		return row.Kegiatan
	case "sub":
		return row.SubKeg
	case "aktivitas":
		return row.Aktivitas
	default:
		return row.Komponen
	}
}

type pivotDisplayRow struct {
	Names []string
	Bruto int64
	PPN   int64
	PPH   int64
	Netto int64
}

func (s *Server) handleRekapRealisasi(w http.ResponseWriter, r *http.Request) {
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
	groups := pivotGroups(r)
	if len(groups) == 0 {
		groups = []string{"kegiatan", "sub", "aktivitas", "komponen"}
	}
	rows, total, err := s.Store.ListRealisasiPivot(r.Context(), id, groups)
	if err != nil {
		s.fail(w, r, err, "/bantuan/"+i64s(id)+"/rekap-realisasi")
		return
	}
	var headers []string
	for _, g := range groups {
		for _, def := range pivotLevelDefs {
			if def.Key == g {
				headers = append(headers, def.Name)
			}
		}
	}
	sel := map[string]bool{}
	for _, g := range groups {
		sel[g] = true
	}
	var disp []pivotDisplayRow
	for _, row := range rows {
		dr := pivotDisplayRow{Bruto: row.Bruto, PPN: row.PPN, PPH: row.PPH, Netto: row.Netto}
		for _, g := range groups {
			dr.Names = append(dr.Names, pivotName(row, g))
		}
		disp = append(disp, dr)
	}
	totalDisp := pivotDisplayRow{Bruto: total.Bruto, PPN: total.PPN, PPH: total.PPH, Netto: total.Netto}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		Headers []string
		Rows    []pivotDisplayRow
		Total   pivotDisplayRow
		Sel     map[string]bool
	}{
		baseView: baseView{Title: "Rekap Realisasi", Active: "rekap-realisasi", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Headers: headers, Rows: disp, Total: totalDisp, Sel: sel,
	}
	s.render(w, r, "rekap_realisasi.html", data)
}

func (s *Server) rekapRealisasiExport(w http.ResponseWriter, r *http.Request, kind string) {
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
	tgl, _, _ := exportParams(r)
	groups := pivotGroups(r)
	if len(groups) == 0 {
		groups = []string{"kegiatan", "sub", "aktivitas", "komponen"}
	}
	rep, err := s.buildRekapRealisasi(r.Context(), &b, groups, tgl)
	if err != nil {
		s.fail(w, r, err, "/")
		return
	}
	fname := reportFilename("RekapRealisasi", b.Nama, tgl)
	switch kind {
	case "excel":
		s.serveExcel(w, rep, fname)
	case "word":
		s.serveWord(w, rep, fname, r.URL.Query().Get("orientasi") == "portrait")
	}
}

func (s *Server) handleRekapRealisasiExportExcel(w http.ResponseWriter, r *http.Request) {
	s.rekapRealisasiExport(w, r, "excel")
}
func (s *Server) handleRekapRealisasiExportWord(w http.ResponseWriter, r *http.Request) {
	s.rekapRealisasiExport(w, r, "word")
}
