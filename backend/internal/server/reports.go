package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ebku/internal/export"
	"ebku/internal/store"
)

var menuNames = map[int]string{
	1: "Barang/Jasa",
	2: "Honor Narasumber",
	3: "Honor Peserta",
	4: "Transport Narasumber",
	5: "Uang Harian Narasumber",
}

var bulanID = []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// sekolahNama mengembalikan nama sekolah, fallback ke nama bantuan.
func sekolahNama(b *store.Bantuan) string {
	if b != nil && b.NamaSekolah != "" {
		return b.NamaSekolah
	}
	if b != nil {
		return b.Nama
	}
	return ""
}

// tanggalID panjang, contoh "31 Juli 2026".
func tanggalID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), bulanID[int(t.Month())], t.Year())
}

// sigData membangun blok tanda tangan dari bantuan.
func sigData(b *store.Bantuan, tanggalCetak time.Time) export.SigData {
	return export.SigData{
		DateText:  "Jakarta, " + tanggalID(tanggalCetak),
		HeadLeft:  "Kepala SMK Negeri 26 Jakarta",
		HeadRight: "Bendahara SMK Negeri 26 Jakarta",
		NameLeft:  b.KepalaSekolah,
		NameRight: b.Bendahara,
		NipLeft:   b.NIPKepalaSekolah,
		NipRight:  b.NIPBendahara,
	}
}

type ledgerReportRow struct {
	Nomor   int
	Tanggal string
	Bukti   string
	Uraian  string
	Debit   int64
	Kredit  int64
	Saldo   int64
}

// buildLedgerReport membuat laporan BKU/Bank. base adalah nilai baris
// 'saldo_awal' sesuai versi aktif (total hibah di rencana, pencairan pertama
// di real); saldo berjalan dihitung ulang dari base.
func (s *Server) buildLedgerReport(ctx context.Context, b *store.Bantuan, table string, base int64, tglCetak time.Time) (export.Report, error) {
	rows, err := store.ListLedger(ctx, s.Store.Pool, b.ID, table)
	if err != nil {
		return export.Report{}, err
	}
	title := "BUKU KAS UMUM"
	if table == "bank" {
		title = "BUKU KAS BANK"
	}
	cols := []export.Col{
		{Header: "No", Width: 8.7, ExWidth: 8, Center: true},
		{Header: "Tanggal", Width: 20.9, ExWidth: 14, Center: true},
		{Header: "No Bukti", Width: 26.1, ExWidth: 16, Wrap: true, Flex: true},
		{Header: "Uraian", Width: 71.3, ExWidth: 45, Wrap: true, Flex: true},
		{Header: "Debet (Rp)", Width: 15.7, ExWidth: 18, Num: true},
		{Header: "Kredit (Rp)", Width: 15.7, ExWidth: 18, Num: true},
		{Header: "Saldo (Rp)", Width: 15.7, ExWidth: 18, Num: true},
	}
	rep := export.Report{
		Title:      title,
		Subtitle:   b.Nama,
		Cols:       cols,
		TotalMerge: 4,
		Sig:        sigData(b, tglCetak),
	}
	var totDebit, totKredit int64
	running := base
	for _, e := range rows {
		kredit := e.Kredit
		saldo := e.Saldo
		if e.JenisTransaksi == "saldo_awal" {
			kredit = 0
			saldo = base
			running = base
		} else {
			running = running - e.Debit + e.Kredit
			saldo = running
		}
		tanggal := ""
		if e.Tanggal != nil {
			tanggal = e.Tanggal.Format("02-01-2006")
		}
		// Nilai 0 ditampilkan kosong (kolom tidak diisi), konsisten dengan web.
		var debitCell, kreditCell any = e.Debit, kredit
		if e.Debit == 0 {
			debitCell = nil
		}
		if kredit == 0 {
			kreditCell = nil
		}
		rep.Rows = append(rep.Rows, []any{
			e.Nomor, tanggal, e.NomorBukti, e.Uraian, debitCell, kreditCell, saldo,
		})
		totDebit += e.Debit
		totKredit += kredit
	}
	rep.TotalRow = []any{"TOTAL", "", "", "", totDebit, totKredit, ""}
	return rep, nil
}

// rekapRow adalah satu baris section 1 rekap pajak.
type rekapRow struct {
	Tanggal time.Time
	Uraian  string
	Bruto   int64
	PPN     int64
	PPh21   int64
	PPh22   int64
	PPh23   int64
	Total   int64
	NTB     string
	NTPN    string
}

type rekapSetorLine struct {
	Jenis string
	NTBN  string
	NTPN  string
}

// buildRekapRows menyusun satu baris per invoice (urut tanggal,id) beserta
// NTB/NTPN per jenis pajak.
func (s *Server) buildRekapRows(ctx context.Context, bantuanID int64) ([]rekapRow, error) {
	invoices, err := s.Store.ListInvoices(ctx, bantuanID, "")
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.Pool.Query(ctx, `SELECT rp.ntbn, rp.ntpn, rp.invoice_id, rp.jenis_pajak, COALESCE(l.uraian,'')
		FROM rekap_pajak rp
		LEFT JOIN trx_ledger l ON l.id=rp.setor_ledger_id
		WHERE rp.bantuan_id=$1 AND rp.invoice_id IS NOT NULL AND rp.setor_ledger_id IS NOT NULL
		ORDER BY rp.invoice_id, rp.id`, bantuanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := map[int64][]rekapSetorLine{}
	for rows.Next() {
		var ntbn, ntpn, jenis, uraian string
		var iid int64
		if err := rows.Scan(&ntbn, &ntpn, &iid, &jenis, &uraian); err != nil {
			return nil, err
		}
		if jenis == "" {
			jenis = store.InferJenisPajak(uraian)
		}
		lines[iid] = append(lines[iid], rekapSetorLine{Jenis: jenis, NTBN: ntbn, NTPN: ntpn})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []rekapRow
	for _, inv := range invoices {
		r := rekapRow{
			Tanggal: inv.Tanggal,
			Uraian:  inv.Uraian,
			Bruto:   inv.Bruto,
			PPN:     inv.NilaiPPN,
			Total:   inv.NilaiPPN + inv.NilaiPPH,
		}
		if inv.JenisPPH != nil {
			switch {
			case strings.Contains(*inv.JenisPPH, "21"):
				r.PPh21 = inv.NilaiPPH
			case strings.Contains(*inv.JenisPPH, "22"):
				r.PPh22 = inv.NilaiPPH
			case strings.Contains(*inv.JenisPPH, "23"):
				r.PPh23 = inv.NilaiPPH
			}
		}
		r.NTB = formatNTBNTPN(lines[inv.ID], true)
		r.NTPN = formatNTBNTPN(lines[inv.ID], false)
		out = append(out, r)
	}
	return out, nil
}

func formatNTBNTPN(lines []rekapSetorLine, ntb bool) string {
	var parts []string
	for i, l := range lines {
		prefix := "NTPN"
		val := l.NTPN
		if ntb {
			prefix = "NTB"
			val = l.NTBN
		}
		if val == "" {
			continue
		}
		parts = append(parts, prefix+" "+l.Jenis, "= "+val)
		if i < len(lines)-1 {
			parts = append(parts, "")
		}
	}
	return strings.Join(parts, "\n")
}

// buildRekapPajak membuat laporan Rekap Pajak.
func (s *Server) buildRekapPajak(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	rows, err := s.buildRekapRows(ctx, b.ID)
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "No", Width: 8, ExWidth: 6, Center: true},
		{Header: "Tanggal", Width: 20, ExWidth: 14, Center: true},
		{Header: "NTB", Width: 20, ExWidth: 18, Wrap: true, Flex: true, Top: true},
		{Header: "NTPN", Width: 20, ExWidth: 18, Wrap: true, Flex: true, Top: true},
		{Header: "Uraian", Width: 70, ExWidth: 45, Wrap: true, Flex: true},
		{Header: "Bruto", Width: 25, ExWidth: 18, Num: true},
		{Header: "PPN", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 21", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 22", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 23", Width: 22, ExWidth: 16, Num: true},
		{Header: "Total Pajak", Width: 26, ExWidth: 18, Num: true},
	}
	rep := export.Report{
		Title:      "REKAP PAJAK",
		Subtitle:   b.Nama,
		Cols:       cols,
		TotalMerge: 5,
		Landscape:  true,
		Sig:        sigData(b, tglCetak),
	}
	var tBruto, tPPN, t21, t22, t23, tTotal int64
	for i, r := range rows {
		rep.Rows = append(rep.Rows, []any{
			i + 1, r.Tanggal.Format("02-01-2006"), r.NTB, r.NTPN, r.Uraian,
			r.Bruto, r.PPN, r.PPh21, r.PPh22, r.PPh23, r.Total,
		})
		tBruto += r.Bruto
		tPPN += r.PPN
		t21 += r.PPh21
		t22 += r.PPh22
		t23 += r.PPh23
		tTotal += r.Total
	}
	rep.TotalRow = []any{"TOTAL", "", "", "", "", tBruto, tPPN, t21, t22, t23, tTotal}
	return rep, nil
}

// buildRekapBelanja membuat laporan Rekap Belanja.
func (s *Server) buildRekapBelanja(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	rows, err := s.Store.ListBelanja(ctx, b.ID)
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "No", Width: 8, ExWidth: 6},
		{Header: "Kegiatan", Width: 29, ExWidth: 20, Wrap: true, Flex: true},
		{Header: "Sub Kegiatan", Width: 29, ExWidth: 20, Wrap: true, Flex: true},
		{Header: "Uraian Tagihan", Width: 40, ExWidth: 26, Wrap: true, Flex: true},
		{Header: "Penyedia / Bank / No.Rek / NPWP", Width: 64, ExWidth: 34, Wrap: true, Flex: true, Top: true},
		{Header: "Bruto", Width: 21, ExWidth: 18, Num: true},
		{Header: "PPN", Width: 19, ExWidth: 16, Num: true},
		{Header: "PPh", Width: 19, ExWidth: 16, Num: true},
		{Header: "Biaya Admin", Width: 21, ExWidth: 16, Num: true},
	}
	rep := export.Report{
		Title:      "REKAP BELANJA",
		Subtitle:   b.Nama,
		Cols:       cols,
		TotalMerge: 5,
		Landscape:  true,
		Sig:        sigData(b, tglCetak),
	}
	penyedia := func(r store.BelanjaRow) string {
		return strings.Join([]string{
			r.Penyedia,
			r.Bank + " - " + r.NoRek,
			"NPWP: " + formatNPWP(r.NPWP),
		}, "\n")
	}
	var tBruto, tPPN, tPPh, tAdmin int64
	for i, r := range rows {
		rep.Rows = append(rep.Rows, []any{
			i + 1, r.Kegiatan, r.SubKeg, r.Uraian, penyedia(r),
			r.Bruto, r.PPN, r.PPH, r.BiayaAdmin,
		})
		tBruto += r.Bruto
		tPPN += r.PPN
		tPPh += r.PPH
		tAdmin += r.BiayaAdmin
	}
	rep.TotalRow = []any{"TOTAL", "", "", "", "", tBruto, tPPN, tPPh, tAdmin}
	return rep, nil
}

// buildRekapRealisasi membuat laporan Rekap Realisasi (pivot) sesuai level
// yang dipilih (groups). Gaya tampilan "Compact Form" ala Excel: seluruh level
// hierarki di satu kolom berindentasi, setiap grup ditutup baris subtotal tebal.
func (s *Server) buildRekapRealisasi(ctx context.Context, b *store.Bantuan, groups []string, tglCetak time.Time) (export.Report, error) {
	rows, total, err := s.Store.ListRealisasiPivot(ctx, b.ID, groups)
	if err != nil {
		return export.Report{}, err
	}
	var disp []pivotDisplayRow
	for _, row := range rows {
		dr := pivotDisplayRow{Pagu: row.Pagu, Realisasi: row.Realisasi, Sisa: row.Sisa}
		for _, g := range groups {
			dr.Names = append(dr.Names, pivotName(row, g))
		}
		disp = append(disp, dr)
	}
	compact := mergeSubtotalsToHeaders(styleRABCompact(buildPivotCompact(disp), groups))

	var labels []string
	for _, g := range groups {
		for _, def := range pivotLevelDefs {
			if def.Key == g {
				labels = append(labels, def.Name)
			}
		}
	}
	cols := []export.Col{
		{Header: strings.Join(labels, " / "), Width: 130, ExWidth: 46, Wrap: true, Flex: true, Left: true},
		{Header: "Pagu Anggaran", Width: 24, ExWidth: 18, Num: true, Money: true},
		{Header: "Nilai Realisasi", Width: 24, ExWidth: 18, Num: true, Money: true},
		{Header: "Sisa", Width: 24, ExWidth: 18, Num: true, Money: true},
	}
	rep := export.Report{
		Title:      "REKAP REALISASI",
		Subtitle:   b.Nama,
		Cols:       cols,
		Landscape:  true,
		TotalMerge: 1,
		Sig:        sigData(b, tglCetak),
	}
	for _, r := range compact {
		rep.Rows = append(rep.Rows, []any{
			indentPivotLabel(r.Label, r.Depth), r.Pagu, r.Realisasi, r.Sisa,
		})
		rep.RowBold = append(rep.RowBold, r.Bold)
	}
	rep.TotalRow = []any{"TOTAL", total.Pagu, total.Realisasi, total.Sisa}
	return rep, nil
}

// indentPivotLabel menjorokkan label hierarki ke kanan sesuai kedalaman level
// (dipakai pada export berjenjang: 8 spasi per level).
func indentPivotLabel(label string, depth int) string {
	if depth <= 0 || label == "" {
		return label
	}
	return strings.Repeat("        ", depth) + label
}

// buildDaftarTagihan membuat laporan Daftar Tagihan.
func (s *Server) buildDaftarTagihan(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	invoices, err := s.Store.ListInvoices(ctx, b.ID, "sort")
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "No", Width: 8, ExWidth: 6, Center: true},
		{Header: "Tanggal", Width: 17, ExWidth: 14, Center: true},
		{Header: "No. Bukti", Width: 27, ExWidth: 16, Wrap: true},
		{Header: "Uraian", Width: 56, ExWidth: 36, Wrap: true},
		{Header: "Nama Rekening / Bank / No.Rek", Width: 46, ExWidth: 30, Wrap: true, Top: true},
		{Header: "Bruto", Width: 22, ExWidth: 18, Num: true},
		{Header: "PPN", Width: 19, ExWidth: 16, Num: true},
		{Header: "PPh", Width: 19, ExWidth: 16, Num: true},
		{Header: "Admin", Width: 17, ExWidth: 14, Num: true},
		{Header: "Netto", Width: 22, ExWidth: 18, Num: true},
		{Header: "Jenis", Width: 24, ExWidth: 18, Wrap: true},
	}
	rep := export.Report{
		Title:     "DAFTAR TAGIHAN",
		Subtitle:  b.Nama,
		Cols:      cols,
		Landscape: true,
		Sig:       sigData(b, tglCetak),
	}
	for i, inv := range invoices {
		admin := store.AdminFee(b.Bank, inv.Bank)
		rek := strings.Join([]string{inv.NamaRekening, inv.Bank, inv.NomorRekening}, "\n")
		jenis := menuNames[inv.JenisMenu]
		if jenis == "" {
			jenis = "Barang/Jasa"
		}
		rep.Rows = append(rep.Rows, []any{
			i + 1, inv.Tanggal.Format("02-01-2006"), inv.NomorBukti, inv.Uraian, rek,
			inv.Bruto, inv.NilaiPPN, inv.NilaiPPH, admin, inv.NilaiNetto, jenis,
		})
	}
	return rep, nil
}

// buildRABReport membuat laporan Rencana Anggaran Biaya: hierarki
// Kegiatan->Sub Kegiatan->Aktivitas->Komponen dalam satu kolom berjenjang
// (indent 3 spasi per level) dengan penomoran I/A/1/a. Sub total ditulis
// pada baris kepala masing-masing level; baris Kegiatan/Sub Kegiatan/Aktivitas
// (beserta nilai subtotalnya) dicetak tebal, diakhiri TOTAL.
func (s *Server) buildRABReport(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	tree, err := s.Store.ListKegiatanTree(ctx, b.ID)
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "Uraian", Width: 150, ExWidth: 62, Wrap: true, Flex: true, Left: true},
		{Header: "Pagu Anggaran (Rp)", Width: 30, ExWidth: 20, Num: true, Money: true},
	}
	rep := export.Report{
		Title:      "RENCANA ANGGARAN BIAYA",
		Subtitle:   b.Nama,
		Cols:       cols,
		TotalMerge: 1,
		Sig:        sigData(b, tglCetak),
	}
	addRow := func(prefix, label string, bold, noBorder bool, pagu int64) {
		rep.Rows = append(rep.Rows, []any{prefix + label, pagu})
		rep.RowBold = append(rep.RowBold, bold)
		rep.RowNoBorder = append(rep.RowNoBorder, noBorder)
	}
	var totalPagu int64
	for ki, k := range tree {
		totalPagu += k.Pagu
		addRow("", romanNumeral(ki+1)+". "+strings.ToUpper(k.Nama), true, false, k.Pagu)
		for si, sk := range k.Subs {
			addRow("   ", letterUpper(si+1)+". "+strings.ToUpper(sk.Nama), true, false, sk.Pagu)
			for ai, a := range sk.Aktivitass {
				addRow("      ", strconv.Itoa(ai+1)+". "+a.Nama, true, false, a.Pagu)
				for ci, ko := range a.Komponens {
					addRow("         ", letterLower(ci+1)+". "+ko.Nama, false, true, ko.Pagu)
				}
			}
		}
	}
	rep.TotalRow = []any{"TOTAL", totalPagu}
	return rep, nil
}

// romanNumeral mengubah angka 1-based menjadi angka Romawi (I, II, III, ...).
func romanNumeral(n int) string {
	if n <= 0 {
		return ""
	}
	table := []struct {
		val int
		sym string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}
	var b strings.Builder
	for _, t := range table {
		for n >= t.val {
			b.WriteString(t.sym)
			n -= t.val
		}
	}
	return b.String()
}

// letterUpper mengubah angka 1-based menjadi huruf kapital (A, B, ..., Z).
// Di luar jangkauan A-Z dikembalikan sebagai angka biasa.
func letterUpper(n int) string {
	if n < 1 || n > 26 {
		return strconv.Itoa(n)
	}
	return string(rune('A' + n - 1))
}

// letterLower mengubah angka 1-based menjadi huruf kecil (a, b, ..., z).
func letterLower(n int) string {
	if n < 1 || n > 26 {
		return strconv.Itoa(n)
	}
	return string(rune('a' + n - 1))
}

// buildRekapPenggunaanDana membuat laporan Rekapitulasi Penggunaan Dana:
// per tagihan ditampilkan No. Bukti, tanggal bukti, keperluan (uraian) dan
// nominal, dengan baris TOTAL dan tanda tangan. Kolom No. Bukti & Tanggal
// berada di bawah grup header "Kuitansi".
func (s *Server) buildRekapPenggunaanDana(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	invoices, err := s.Store.ListInvoices(ctx, b.ID, "sort")
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "No", Width: 8.7, ExWidth: 6, Center: true},
		{Header: "No. Bukti Dokumen", Width: 32, ExWidth: 15, Wrap: true, MaxWidth: 32},
		{Header: "Tanggal Bukti Dokumen", Width: 26, ExWidth: 14, Center: true, MaxWidth: 26},
		{Header: "Keperluan Pembayaran", Width: 120, ExWidth: 55, Wrap: true, Flex: true, Left: true},
		{Header: "Nominal", Width: 26, ExWidth: 18, Num: true, Money: true},
	}
	rep := export.Report{
		Title:      "REKAPITULASI PENGGUNAAN DANA",
		Subtitle:   b.Nama,
		Cols:       cols,
		ColGroups:  []export.ColGroup{{Header: "KUITANSI", Start: 1, Span: 2}},
		TotalMerge: 4,
		Sig:        sigData(b, tglCetak),
	}
	var total int64
	for i, inv := range invoices {
		rep.Rows = append(rep.Rows, []any{
			i + 1, inv.NomorBukti, tanggalID(inv.Tanggal), inv.Uraian, inv.Bruto,
		})
		total += inv.Bruto
	}
	rep.TotalRow = []any{"TOTAL", "", "", "", total}
	return rep, nil
}

// reportFilename menghasilkan nama file export sesuai spesifikasi.
func reportFilename(kind, nama string, t time.Time) string {
	nama = strings.ReplaceAll(nama, " ", "_")
	return fmt.Sprintf("%s_%s_%s", kind, nama, t.Format("20060102"))
}
