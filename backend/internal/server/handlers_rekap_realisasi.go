package server

import (
	"net/http"
	"strings"

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
	Names     []string
	Pagu      int64
	Realisasi int64
	Sisa      int64
}

// pivotCompactRow adalah satu baris tampilan pivot gaya "Compact Form" Excel:
// seluruh level hierarki dirapikan ke satu kolom berindentasi. Kind:
// "header" (nama grup, tanpa angka), "leaf" (baris data terkecil), atau
// "subtotal" (total per grup, tebal).
type pivotCompactRow struct {
	Label     string
	Depth     int
	Kind      string
	Pagu      int64
	Realisasi int64
	Sisa      int64
}

// buildPivotCompact mengubah baris hasil agregasi menjadi urutan baris
// berjenjang: nama grup muncul sekali di atas, anak-anaknya menjorok ke kanan,
// dan setiap grup ditutup baris subtotal. Baris paling dalam (leaf) membawa angka.
func buildPivotCompact(rows []pivotDisplayRow) []pivotCompactRow {
	n := 0
	if len(rows) > 0 {
		n = len(rows[0].Names)
	}
	if n == 0 {
		return nil
	}
	var out []pivotCompactRow
	prev := make([]string, n)
	sum := make([][3]int64, n)
	for _, r := range rows {
		lcp := 0
		for lcp < n && prev[lcp] == r.Names[lcp] {
			lcp++
		}
		for l := n - 1; l >= lcp; l-- {
			if prev[l] == "" {
				continue
			}
			if l < n-1 {
				out = append(out, pivotCompactRow{
					Label: prev[l], Depth: l, Kind: "subtotal",
					Pagu: sum[l][0], Realisasi: sum[l][1], Sisa: sum[l][2],
				})
			}
			prev[l] = ""
			sum[l] = [3]int64{}
		}
		for j := lcp; j < n; j++ {
			if j < n-1 {
				out = append(out, pivotCompactRow{Label: r.Names[j], Depth: j, Kind: "header"})
			}
			prev[j] = r.Names[j]
		}
		for l := 0; l < n; l++ {
			sum[l][0] += r.Pagu
			sum[l][1] += r.Realisasi
			sum[l][2] += r.Sisa
		}
		out = append(out, pivotCompactRow{
			Label: r.Names[n-1], Depth: n - 1, Kind: "leaf",
			Pagu: r.Pagu, Realisasi: r.Realisasi, Sisa: r.Sisa,
		})
	}
	for l := n - 1; l >= 0; l-- {
		if prev[l] == "" {
			continue
		}
		if l < n-1 {
			out = append(out, pivotCompactRow{
				Label: prev[l], Depth: l, Kind: "subtotal",
				Pagu: sum[l][0], Realisasi: sum[l][1], Sisa: sum[l][2],
			})
		}
		prev[l] = ""
	}
	return out
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
		dr := pivotDisplayRow{Pagu: row.Pagu, Realisasi: row.Realisasi, Sisa: row.Sisa}
		for _, g := range groups {
			dr.Names = append(dr.Names, pivotName(row, g))
		}
		disp = append(disp, dr)
	}
	totalDisp := pivotDisplayRow{Pagu: total.Pagu, Realisasi: total.Realisasi, Sisa: total.Sisa}
	ft, fm := s.getFlash(w, r)
	data := struct {
		baseView
		ColHeader string
		Compact   []pivotCompactRow
		Total     pivotDisplayRow
		Sel       map[string]bool
	}{
		baseView: baseView{Title: "Rekap Realisasi", Active: "rekap-realisasi", FlashType: ft, FlashMsg: fm,
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		ColHeader: strings.Join(headers, " / "),
		Compact:   buildPivotCompact(disp),
		Total:     totalDisp,
		Sel:       sel,
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
