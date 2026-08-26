package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"ebku/internal/export"
	"ebku/internal/money"
	"ebku/internal/store"
)

// hariID nama hari dalam bahasa Indonesia (indeks sesuai time.Weekday+1).
var hariID = []string{"", "Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

// bapKasData menyusun data Berita Acara Pemeriksaan Kas dari bantuan,
// tanggal pemeriksaan, dan uang tunai (kertas & logam) yang diinput user.
// Saldo & pajak dihitung sampai tanggal pemeriksaan (tanggal tanda tangan BAP).
func (s *Server) bapKasData(ctx context.Context, b *store.Bantuan, tgl time.Time, uangKertas, uangLogam int64) export.BAPKasData {
	sekolah := sekolahNama(b)
	pajakBelum := s.Store.PajakBelumSetorAsOf(ctx, b.ID, tgl)
	saldoBank := s.Store.SaldoAsOf(ctx, b.ID, "bank", tgl)
	saldoBuku := s.Store.SaldoAsOf(ctx, b.ID, "bku", tgl)
	total := uangKertas + uangLogam + pajakBelum + saldoBank
	return export.BAPKasData{
		DateText: fmt.Sprintf("Pada hari ini %s tanggal %d bulan %s tahun %d yang bertandatangan di bawah ini",
			hariID[int(tgl.Weekday())+1], tgl.Day(), bulanID[int(tgl.Month())], tgl.Year()),
		ExaminerName:   b.KepalaSekolah,
		ExaminerTitle:  "Kepala " + sekolah,
		ExaminerNIP:    b.NIPKepalaSekolah,
		BendaharaName:  b.Bendahara,
		BendaharaTitle: "Bendahara " + sekolah,
		BendaharaNIP:   b.NIPBendahara,
		UangKertas:     uangKertas,
		UangLogam:      uangLogam,
		PajakBelum:     pajakBelum,
		SaldoBank:      saldoBank,
		Total:          total,
		SaldoBuku:      saldoBuku,
		Perbedaan:      total - saldoBuku,
	}
}

// bapKasParams membaca tanggal & uang tunai dari query string.
func bapKasParams(r *http.Request) (time.Time, int64, int64) {
	tgl := store.ParseDate(r.URL.Query().Get("tanggal"))
	if tgl.IsZero() {
		tgl = time.Now()
	}
	uangKertas, _ := money.Parse(r.URL.Query().Get("uang_kertas"))
	uangLogam, _ := money.Parse(r.URL.Query().Get("uang_logam"))
	return tgl, uangKertas, uangLogam
}

func (s *Server) handleBAPKas(w http.ResponseWriter, r *http.Request) {
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
	tgl, uangKertas, uangLogam := bapKasParams(r)
	d := s.bapKasData(r.Context(), &b, tgl, uangKertas, uangLogam)
	view := struct {
		baseView
		Data       export.BAPKasData
		Tanggal    string
		UangKertas string
		UangLogam  string
	}{
		baseView: baseView{Title: "BA Pemeriksaan Kas", Active: "bap-kas",
			Bantuan: &b, Summary: s.summary(r.Context(), id, &b), Q: map[string]string{}},
		Data: d, Tanggal: tgl.Format("2006-01-02"),
		UangKertas: money.FormatReport(uangKertas), UangLogam: money.FormatReport(uangLogam),
	}
	s.render(w, r, "bap_kas.html", view)
}

func (s *Server) handleBAPKasExportPDF(w http.ResponseWriter, r *http.Request) {
	s.bapKasExport(w, r, "pdf")
}

func (s *Server) handleBAPKasExportWord(w http.ResponseWriter, r *http.Request) {
	s.bapKasExport(w, r, "word")
}

func (s *Server) bapKasExport(w http.ResponseWriter, r *http.Request, kind string) {
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
	tgl, uangKertas, uangLogam := bapKasParams(r)
	d := s.bapKasData(r.Context(), &b, tgl, uangKertas, uangLogam)
	fname := reportFilename("BAPKas", b.Nama, tgl)
	switch kind {
	case "pdf":
		data, err := export.BAPPdf(d)
		if err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bap-kas")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+fname+`.pdf"`)
		w.Write(data)
	case "word":
		data, err := export.BAPWordBytes(d)
		if err != nil {
			s.fail(w, r, err, "/bantuan/"+i64s(id)+"/bap-kas")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		w.Header().Set("Content-Disposition", `attachment; filename="`+fname+`.docx"`)
		w.Write(data)
	}
}
