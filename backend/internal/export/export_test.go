package export

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func sampleReport() Report {
	cols := []Col{
		{Header: "No", Width: 12, ExWidth: 8},
		{Header: "Tanggal", Width: 22, ExWidth: 14},
		{Header: "No. Bukti", Width: 30, ExWidth: 16, Wrap: true},
		{Header: "Uraian", Width: 100, ExWidth: 45, Wrap: true},
		{Header: "Debet (Rp)", Width: 35, ExWidth: 18, Num: true},
		{Header: "Kredit (Rp)", Width: 35, ExWidth: 18, Num: true},
		{Header: "Saldo (Rp)", Width: 35, ExWidth: 18, Num: true},
	}
	rows := [][]any{
		{1, "05-02-2026", "B.1", "Pencairan Bantuan Biaya Operasional Sekolah Tahap 1 (PPN 12% + PPh 23)", int64(5000000000), int64(0), int64(5000000000)},
		{2, "06-02-2026", "T.1", "Pengadaan ATK - PT Sumber Jaya Mandiri", int64(0), int64(88288288), int64(4911711712)},
	}
	total := []any{"TOTAL", "", "", "", int64(5000000000), int64(88288288), int64(4911711712)}
	sig := SigData{
		DateText:  "Jakarta, 31 Juli 2026",
		HeadLeft:  "Kepala SMK Negeri 26 Jakarta",
		HeadRight: "Bendahara SMK Negeri 26 Jakarta",
		NameLeft:  "Dr. Ahmad Santoso, M.Pd.",
		NameRight: "Dewi Lestari, S.Pd.",
		NipLeft:   "NIP 197012102000031001",
		NipRight:  "NIP 198205152009022004",
	}
	return Report{
		Title:    "Buku Kas Umum",
		Subtitle: "Bantuan Biaya Operasional Sekolah Tahun 2026",
		Cols:     cols,
		Rows:     rows,
		TotalRow: total,
		Sig:      sig,
	}
}

func TestFmtAmount(t *testing.T) {
	cases := map[int64]string{
		0:           "0",
		5000000000:  "50.000.000",
		88288288:    "882.883",
		-1:          "0",
		12345678901: "123.456.789",
		100000001:   "1.000.000",
		41600000:    "416.000",
	}
	for in, want := range cases {
		if got := FmtAmount(in); got != want {
			t.Errorf("FmtAmount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestExcel(t *testing.T) {
	b, err := ExcelBytes(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 1000 {
		t.Fatal("excel too small")
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) != 1 {
		t.Fatalf("sheets = %v, want 1", sheets)
	}
	if sheets[0] != "Laporan" {
		t.Fatalf("sheet name = %q, want Laporan", sheets[0])
	}
}

func TestPDF(t *testing.T) {
	b, err := PDF(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 1000 {
		t.Fatal("pdf too small")
	}
}

func TestWordLandscapePortrait(t *testing.T) {
	for _, landscape := range []bool{true, false} {
		b, err := WordBytes(sampleReport(), landscape)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) < 1000 {
			t.Fatal("word too small")
		}
	}
}

func TestWordRejectLargeWidths(t *testing.T) {
	rep := sampleReport()
	// Simulasikan laporan Rekap Belanja: 10 kolom, lebar total 277 mm.
	rep.Cols = []Col{
		{Header: "No", Width: 8},
		{Header: "Kegiatan", Width: 29},
		{Header: "Sub Kegiatan", Width: 29},
		{Header: "Komponen", Width: 29},
		{Header: "Penyedia / Bank / No.Rek / NPWP", Width: 64, Wrap: true},
		{Header: "Bruto", Width: 21, Num: true},
		{Header: "PPN", Width: 19, Num: true},
		{Header: "PPh", Width: 19, Num: true},
		{Header: "Netto", Width: 21, Num: true},
		{Header: "Uraian", Width: 38, Wrap: true},
	}
	rep.Rows = [][]any{
		{1, "Kegiatan", "Sub", "Komponen", "CV Maju\nBank BRI\nNo.Rek 12345\nNPWP 012345", int64(100000000), int64(10909091), int64(2000000), int64(88181818), "Pembelian ATK"},
	}
	for _, landscape := range []bool{true, false} {
		b, err := WordBytes(rep, landscape)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) < 1000 {
			t.Fatal("word too small")
		}
	}
}
