package server

import (
	"context"
	"fmt"
	"net/url"
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
	InvID   int64
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

// buildRekapRows menyusun satu baris per invoice (urut input/sort_order)
// beserta NTB/NTPN per jenis pajak.
func (s *Server) buildRekapRows(ctx context.Context, bantuanID int64) ([]rekapRow, error) {
	invoices, err := s.Store.ListInvoices(ctx, bantuanID, "sort")
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
			InvID:   inv.ID,
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
		{Header: "Nominal (Rp)", Width: 25, ExWidth: 18, Num: true},
		{Header: "PPN", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 21", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 22", Width: 22, ExWidth: 16, Num: true},
		{Header: "PPh 23", Width: 22, ExWidth: 16, Num: true},
		{Header: "Penyetoran", Width: 26, ExWidth: 18, Num: true},
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

// buildRekapBelanja membuat laporan Rekap Belanja. Jika filtTanggal tidak nil,
// hanya transaksi pada tanggal tersebut yang disertakan (untuk serah ke bank).
// Kolom angka yang tampil mengikuti pilihan colOpts.
func (s *Server) buildRekapBelanja(ctx context.Context, b *store.Bantuan, tglCetak time.Time, filtTanggal *time.Time, colOpts rekapBelanjaColOpts) (export.Report, error) {
	rows, err := s.Store.ListBelanja(ctx, b.ID)
	if err != nil {
		return export.Report{}, err
	}
	if filtTanggal != nil {
		rows = filterBelanjaTanggal(rows, *filtTanggal)
	}
	cols := []export.Col{
		{Header: "No", Width: 8, ExWidth: 6},
		{Header: "Tanggal", Width: 17, ExWidth: 13, Center: true},
	}
	if colOpts.Kegiatan {
		cols = append(cols, export.Col{Header: "Kegiatan / Sub Kegiatan / Aktivitas", Width: 46, ExWidth: 30, Wrap: true, Flex: true, Left: true})
	}
	cols = append(cols,
		export.Col{Header: "Uraian Tagihan", Width: 40, ExWidth: 26, Wrap: true, Flex: true},
		export.Col{Header: "Penyedia / Bank / No.Rek / NPWP", Width: 64, ExWidth: 34, Wrap: true, Flex: true, Top: true},
	)
	numCol := func(header string) export.Col {
		return export.Col{Header: header, Width: 21, ExWidth: 16, Num: true}
	}
	if colOpts.Bruto {
		cols = append(cols, numCol("Bruto"))
	}
	if colOpts.Potongan {
		cols = append(cols, numCol("Potongan Pajak"))
	}
	if colOpts.SetelahPajak {
		cols = append(cols, numCol("Setelah Potong Pajak"))
	}
	if colOpts.BiayaAdmin {
		cols = append(cols, numCol("Biaya Admin"))
	}
	if colOpts.Ditransfer {
		cols = append(cols, numCol("Ditransfer"))
	}
	totalMerge := 4
	if colOpts.Kegiatan {
		totalMerge = 5
	}
	rep := export.Report{
		Title:      "REKAP BELANJA",
		Subtitle:   b.Nama,
		Cols:       cols,
		TotalMerge: totalMerge,
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
	var tBruto, tPotongan, tSetelah, tAdmin, tDitransfer int64
	for i, r := range rows {
		row := []any{i + 1, r.Tanggal.Format("02-01-2006")}
		if colOpts.Kegiatan {
			row = append(row, kegiatanRich(r))
		}
		row = append(row, r.Uraian, penyedia(r))
		if colOpts.Bruto {
			row = append(row, r.Bruto)
		}
		if colOpts.Potongan {
			row = append(row, r.PPN+r.PPH)
		}
		if colOpts.SetelahPajak {
			row = append(row, r.Netto)
		}
		if colOpts.BiayaAdmin {
			row = append(row, r.BiayaAdmin)
		}
		if colOpts.Ditransfer {
			row = append(row, r.Netto-r.BiayaAdmin)
		}
		rep.Rows = append(rep.Rows, row)
		tBruto += r.Bruto
		tPotongan += r.PPN + r.PPH
		tSetelah += r.Netto
		tAdmin += r.BiayaAdmin
		tDitransfer += r.Netto - r.BiayaAdmin
	}
	totalRow := []any{"TOTAL", ""}
	if colOpts.Kegiatan {
		totalRow = append(totalRow, "")
	}
	totalRow = append(totalRow, "", "")
	if colOpts.Bruto {
		totalRow = append(totalRow, tBruto)
	}
	if colOpts.Potongan {
		totalRow = append(totalRow, tPotongan)
	}
	if colOpts.SetelahPajak {
		totalRow = append(totalRow, tSetelah)
	}
	if colOpts.BiayaAdmin {
		totalRow = append(totalRow, tAdmin)
	}
	if colOpts.Ditransfer {
		totalRow = append(totalRow, tDitransfer)
	}
	rep.TotalRow = totalRow
	return rep, nil
}

// kegiatanRich menggabungkan Kegiatan, Sub Kegiatan, dan Aktivitas menjadi cell
// kaya: label (KEGIATAN:/SUB KEGIATAN:/AKTIVITAS:) dicetak tebal, nilainya normal,
// dengan satu baris kosong di antara ketiganya.
func kegiatanRich(r store.BelanjaRow) export.Rich {
	return export.Rich{Segments: []export.RichSeg{
		{Text: "KEGIATAN: ", Bold: true},
		{Text: r.Kegiatan + "\n\n"},
		{Text: "SUB KEGIATAN: ", Bold: true},
		{Text: r.SubKeg + "\n\n"},
		{Text: "AKTIVITAS: ", Bold: true},
		{Text: r.Aktivitas},
	}}
}

// kegiatanText menggabungkan Kegiatan, Sub Kegiatan, dan Aktivitas menjadi satu
// teks berbaris untuk kolom gabungan Rekap Belanja.
func kegiatanText(r store.BelanjaRow) string {
	return strings.Join([]string{
		"KEGIATAN: " + r.Kegiatan,
		"",
		"SUB KEGIATAN: " + r.SubKeg,
		"",
		"AKTIVITAS: " + r.Aktivitas,
	}, "\n")
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

// rekapPenggunaanOpts berisi pilihan kolom tambahan Rekap Penggunaan Dana.
type rekapPenggunaanOpts struct {
	Kegiatan  bool
	SubKeg    bool
	Aktivitas bool
	Komponen  bool
	Pajak     bool
}

// rekapPenggunaanOptsFromQuery membaca pilihan kolom dari query string
// (nilai "1" = tampil). Dipakai halaman web & export.
func rekapPenggunaanOptsFromQuery(q url.Values) rekapPenggunaanOpts {
	return rekapPenggunaanOpts{
		Kegiatan:  q.Get("kegiatan") == "1",
		SubKeg:    q.Get("sub_kegiatan") == "1",
		Aktivitas: q.Get("aktivitas") == "1",
		Komponen:  q.Get("komponen") == "1",
		Pajak:     q.Get("pajak") == "1",
	}
}

// rekapBelanjaColOpts berisi pilihan kolom Rekap Belanja. Kolom deskriptif
// (No, Tanggal, Penyedia, Uraian) selalu tampil; Kegiatan & kolom angka dapat
// dipilih lewat query string.
type rekapBelanjaColOpts struct {
	Kegiatan     bool // Kegiatan / Sub Kegiatan / Aktivitas
	Bruto        bool // Bruto
	Potongan     bool // Nilai Potongan Pajak (PPN+PPh)
	SetelahPajak bool // Nilai Setelah Potong Pajak (Bruto - PPN - PPh)
	BiayaAdmin   bool // Biaya Admin
	Ditransfer   bool // Biaya Ditransfer ke Pihak Ketiga (Bruto - PPN - PPh - Biaya Admin)
}

// Count menghitung banyak kolom angka yang dipilih.
func (o rekapBelanjaColOpts) Count() int {
	n := 0
	for _, b := range []bool{o.Bruto, o.Potongan, o.SetelahPajak, o.BiayaAdmin, o.Ditransfer} {
		if b {
			n++
		}
	}
	return n
}

// rekapBelanjaColOptsFromQuery membaca pilihan kolom Rekap Belanja dari query
// string. Tanpa parameter sama sekali -> semua kolom tampil (default). Bila ada
// parameter, kolom yang tidak disertakan ("1" = tampil) dianggap disembunyikan.
func rekapBelanjaColOptsFromQuery(q url.Values) rekapBelanjaColOpts {
	o := rekapBelanjaColOpts{
		Kegiatan: true, Bruto: true, Potongan: true,
		SetelahPajak: true, BiayaAdmin: true, Ditransfer: true,
	}
	for _, c := range []string{"kegiatan", "bruto", "potongan", "setelah_pajak", "biaya_admin", "ditransfer"} {
		if _, ok := q[c]; ok {
			o.Kegiatan = q.Get("kegiatan") == "1"
			o.Bruto = q.Get("bruto") == "1"
			o.Potongan = q.Get("potongan") == "1"
			o.SetelahPajak = q.Get("setelah_pajak") == "1"
			o.BiayaAdmin = q.Get("biaya_admin") == "1"
			o.Ditransfer = q.Get("ditransfer") == "1"
			break
		}
	}
	return o
}

// rekapPenggunaanDistCols mengembalikan kolom angka distribusi untuk level
// yang dipilih: satu kolom per nilai kegiatan/sub kegiatan/aktivitas/komponen
// di anggaran (urutan RAB).
func rekapPenggunaanDistCols(data store.RekapPenggunaanData, opts rekapPenggunaanOpts) []store.RekapPenggunaanCol {
	var out []store.RekapPenggunaanCol
	for _, c := range data.Cols {
		sel := (c.Level == "kegiatan" && opts.Kegiatan) ||
			(c.Level == "sub" && opts.SubKeg) ||
			(c.Level == "aktivitas" && opts.Aktivitas) ||
			(c.Level == "komponen" && opts.Komponen)
		if sel {
			out = append(out, c)
		}
	}
	return out
}

// rekapSelLevels mengembalikan indeks level yang dipilih, terurut naik sesuai
// hierarki RAB: 0=kegiatan, 1=sub, 2=aktivitas, 3=komponen.
func rekapSelLevels(opts rekapPenggunaanOpts) []int {
	var out []int
	if opts.Kegiatan {
		out = append(out, 0)
	}
	if opts.SubKeg {
		out = append(out, 1)
	}
	if opts.Aktivitas {
		out = append(out, 2)
	}
	if opts.Komponen {
		out = append(out, 3)
	}
	return out
}

// rekapLevelNames nama level sesuai indeks pada hierarki RAB.
var rekapLevelNames = []string{"kegiatan", "sub", "aktivitas", "komponen"}

// rekapHeaderNode adalah simpul pohon header distribusi: satu nilai kegiatan/
// sub kegiatan/aktivitas/komponen beserta anak-anaknya pada level lebih dalam
// yang dipilih.
type rekapHeaderNode struct {
	Level    string
	Name     string
	Children []rekapHeaderNode
}

// rabItem mewakili satu nilai hierarki RAB (kegiatan -> sub -> aktivitas ->
// komponen).
type rabItem struct {
	name     string
	subItems []rabItem
}

// buildRABItems mengubah pohon RAB menjadi rabItem.
func buildRABItems(tree []store.KegiatanTree) []rabItem {
	var out []rabItem
	for _, k := range tree {
		var subs []rabItem
		for _, sk := range k.Subs {
			var acts []rabItem
			for _, a := range sk.Aktivitass {
				var kos []rabItem
				for _, ko := range a.Komponens {
					kos = append(kos, rabItem{name: ko.Nama})
				}
				acts = append(acts, rabItem{name: a.Nama, subItems: kos})
			}
			subs = append(subs, rabItem{name: sk.Nama, subItems: acts})
		}
		out = append(out, rabItem{name: k.Nama, subItems: subs})
	}
	return out
}

// buildRekapHeaderTree membangun pohon header dari rabItem, hanya memuat
// level yang dipilih (selIdx). Level tak dipilih dilompati (anak-nya naik ke
// level atas). Daun pohon = nilai pada level terdalam yang dipilih.
func buildRekapHeaderTree(items []rabItem, level int, selIdx []int) []rekapHeaderNode {
	var out []rekapHeaderNode
	for _, it := range items {
		if containsInt(selIdx, level) {
			n := rekapHeaderNode{Level: rekapLevelNames[level], Name: it.name}
			if level < 3 {
				n.Children = buildRekapHeaderTree(it.subItems, level+1, selIdx)
			}
			out = append(out, n)
		} else if level < 3 {
			out = append(out, buildRekapHeaderTree(it.subItems, level+1, selIdx)...)
		}
	}
	return out
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// rekapHeaderLeaves mengumpulkan daun pohon header (urutan DFS = urutan kolom).
func rekapHeaderLeaves(nodes []rekapHeaderNode) []rekapHeaderNode {
	var out []rekapHeaderNode
	var walk func([]rekapHeaderNode)
	walk = func(ns []rekapHeaderNode) {
		for _, n := range ns {
			if len(n.Children) == 0 {
				out = append(out, n)
			} else {
				walk(n.Children)
			}
		}
	}
	walk(nodes)
	return out
}

// rekapNodesAtDepth meratakan simpul pohon pada kedalaman tertentu.
func rekapNodesAtDepth(nodes []rekapHeaderNode, depth int) []rekapHeaderNode {
	if depth == 0 {
		return nodes
	}
	var out []rekapHeaderNode
	for _, n := range nodes {
		out = append(out, rekapNodesAtDepth(n.Children, depth-1)...)
	}
	return out
}

// rekapCountLeaves menghitung jumlah daun di bawah satu simpul.
func rekapCountLeaves(n *rekapHeaderNode) int {
	if len(n.Children) == 0 {
		return 1
	}
	c := 0
	for i := range n.Children {
		c += rekapCountLeaves(&n.Children[i])
	}
	return c
}

// buildRekapHeaderRows menyusun header berjenjang Rekap Penggunaan Dana.
// Baris 0 memuat grup utama (No, Kuitansi, Keperluan, "Nominal (Rp)", Pajak),
// baris-baris berikut memuat level distribusi sesuai selCount, baris terakhir
// = daun (kolom angka). Setiap baris menutupi seluruh kolom; sel Rowspan=0
// adalah lanjutan dari baris di atas.
func buildRekapHeaderRows(selCount int, tree []rekapHeaderNode, hasPajak bool) [][]export.HeaderCell {
	depth := 2
	if selCount > 0 {
		depth = selCount + 1
	}
	leaves := rekapHeaderLeaves(tree)
	nomSpan := len(leaves)
	if selCount == 0 {
		nomSpan = 1
	}
	total := 4 + nomSpan
	if hasPajak {
		total += 4
	}
	cont := func() export.HeaderCell { return export.HeaderCell{Colspan: 1, Rowspan: -1} }

	rows := make([][]export.HeaderCell, depth)
	for ri := 0; ri < depth; ri++ {
		row := make([]export.HeaderCell, 0, total)

		// kolom No
		if ri == 0 {
			row = append(row, export.HeaderCell{Text: "No", Colspan: 1, Rowspan: depth})
		} else {
			row = append(row, cont())
		}

		// kolom 1-2: Kuitansi -> No. Bukti / Tanggal
		if ri == 0 {
			row = append(row, export.HeaderCell{Text: "Kuitansi", Colspan: 2, Rowspan: 1})
		} else if ri == 1 {
			row = append(row,
				export.HeaderCell{Text: "No. Bukti Dokumen", Colspan: 1, Rowspan: depth - 1},
				export.HeaderCell{Text: "Tanggal Bukti Dokumen", Colspan: 1, Rowspan: depth - 1},
			)
		} else {
			row = append(row, cont(), cont())
		}

		// kolom Keperluan Pembayaran
		if ri == 0 {
			row = append(row, export.HeaderCell{Text: "Keperluan Pembayaran", Colspan: 1, Rowspan: depth})
		} else {
			row = append(row, cont())
		}

		// kolom nominal: "Nominal (Rp.)" tunggal atau pohon distribusi
		if selCount == 0 {
			if ri == 0 {
				row = append(row, export.HeaderCell{Text: "Nominal (Rp.)", Colspan: 1, Rowspan: depth})
			} else {
				row = append(row, cont())
			}
		} else if ri == 0 {
			row = append(row, export.HeaderCell{Text: "Nominal (Rp)", Colspan: nomSpan, Rowspan: 1})
		} else {
			li := ri - 1
			if li < selCount {
				for _, n := range rekapNodesAtDepth(tree, li) {
					if li == selCount-1 {
						row = append(row, export.HeaderCell{Text: n.Name, Colspan: 1, Rowspan: 1})
					} else {
						row = append(row, export.HeaderCell{Text: n.Name, Colspan: rekapCountLeaves(&n), Rowspan: 1})
					}
				}
			} else {
				for i := 0; i < nomSpan; i++ {
					row = append(row, cont())
				}
			}
		}

		// kolom pajak
		if hasPajak {
			if ri == 0 {
				row = append(row, export.HeaderCell{Text: "Pajak Yang Dipungut dan Disetorkan (Rp)", Colspan: 4, Rowspan: 1})
			} else if ri == 1 {
				row = append(row,
					export.HeaderCell{Text: "PPN", Colspan: 1, Rowspan: depth - 1},
					export.HeaderCell{Text: "PPh 21", Colspan: 1, Rowspan: depth - 1},
					export.HeaderCell{Text: "PPh 22", Colspan: 1, Rowspan: depth - 1},
					export.HeaderCell{Text: "PPh 23", Colspan: 1, Rowspan: depth - 1},
				)
			} else {
				row = append(row, cont(), cont(), cont(), cont())
			}
		}
		rows[ri] = row
	}
	return rows
}

// rekapDistColsData menentukan kolom distribusi (daun) dan konfigurasi header
// untuk Rekap Penggunaan Dana. Bila memilih 2+ level, header memakai HeaderRows
// berjenjang; selain itu memakai ColGroups.
func (s *Server) rekapDistColsData(data store.RekapPenggunaanData, opts rekapPenggunaanOpts) (cols []store.RekapPenggunaanCol, headerRows [][]export.HeaderCell, colGroups []export.ColGroup) {
	sel := rekapSelLevels(opts)
	switch {
	case len(sel) >= 2:
		tree := buildRekapHeaderTree(buildRABItems(data.Tree), 0, sel)
		leaves := rekapHeaderLeaves(tree)
		for _, leaf := range leaves {
			cols = append(cols, store.RekapPenggunaanCol{Level: leaf.Level, Name: leaf.Name})
		}
		headerRows = buildRekapHeaderRows(len(sel), tree, opts.Pajak)
	case len(sel) == 1:
		cols = rekapPenggunaanDistCols(data, opts)
		colGroups = []export.ColGroup{
			{Header: "KUITANSI", Start: 1, Span: 2},
			{Header: "NILAI NOMINAL (Rp)", Start: 4, Span: len(cols)},
		}
	default:
		colGroups = []export.ColGroup{{Header: "KUITANSI", Start: 1, Span: 2}}
	}
	return
}

// buildRekapPenggunaanDana membuat laporan Rekapitulasi Penggunaan Dana:
// per tagihan ditampilkan No. Bukti, tanggal, keperluan, lalu kolom angka.
// Bila ada level distribusi dipilih (kegiatan/sub kegiatan/aktivitas/komponen),
// dibuat kolom per nilai anggaran yang diisi nominal tagihan (kolom Nominal
// tunggal diganti). Memilih 2+ level menghasilkan header berjenjang (mis.
// Nominal (Rp) -> Kegiatan -> Aktivitas). Kolom pajak (PPN, PPh 21/22/23)
// tampil bila dipilih. Baris TOTAL menjumlahkan tiap kolom angka.
func (s *Server) buildRekapPenggunaanDana(ctx context.Context, b *store.Bantuan, tglCetak time.Time, opts rekapPenggunaanOpts) (export.Report, error) {
	data, err := s.Store.ListRekapPenggunaan(ctx, b.ID)
	if err != nil {
		return export.Report{}, err
	}
	dist, headerRows, colGroups := s.rekapDistColsData(data, opts)

	cols := []export.Col{
		{Header: "No", Width: 8.7, ExWidth: 6, Center: true},
		{Header: "No. Bukti Dokumen", Width: 32, ExWidth: 15, Wrap: true, MaxWidth: 32},
		{Header: "Tanggal Bukti Dokumen", Width: 26, ExWidth: 14, Center: true, MaxWidth: 26},
		{Header: "Keperluan Pembayaran", Width: 90, ExWidth: 55, Wrap: true, Flex: true, Left: true},
	}
	for _, c := range dist {
		cols = append(cols, export.Col{Header: c.Name, Width: 30, ExWidth: 14, Wrap: true, Num: true, Money: true, Flex: true})
	}
	if len(dist) == 0 {
		cols = append(cols, export.Col{Header: "Nominal (Rp.)", Width: 26, ExWidth: 18, Num: true, Money: true, MaxWidth: 26})
	}
	if opts.Pajak {
		cols = append(cols,
			export.Col{Header: "PPN", Width: 20, ExWidth: 14, MaxWidth: 20, Num: true, Money: true},
			export.Col{Header: "PPh 21", Width: 20, ExWidth: 14, MaxWidth: 20, Num: true, Money: true},
			export.Col{Header: "PPh 22", Width: 20, ExWidth: 14, MaxWidth: 20, Num: true, Money: true},
			export.Col{Header: "PPh 23", Width: 20, ExWidth: 14, MaxWidth: 20, Num: true, Money: true},
		)
		if len(headerRows) == 0 {
			colGroups = append(colGroups, export.ColGroup{
				Header: "PAJAK YANG DIPUNGUT DAN DISETORKAN (Rp)", Start: 4 + len(dist), Span: 4,
			})
		}
	}
	rep := export.Report{
		Title:      "REKAPITULASI PENGGUNAAN DANA",
		Subtitle:   b.Nama,
		Cols:       cols,
		ColGroups:  colGroups,
		HeaderRows: headerRows,
		TotalMerge: 4,
		Sig:        sigData(b, tglCetak),
	}
	if len(dist) > 0 || opts.Pajak {
		rep.Landscape = true
	}

	var tPPN, tPPh21, tPPh22, tPPh23 int64
	totals := make([]int64, len(dist))
	for i, r := range data.Rows {
		row := []any{i + 1, r.NomorBukti, tanggalID(r.Tanggal), r.Uraian}
		if len(dist) > 0 {
			for j, c := range dist {
				v := r.Values[c.Level+"\x00"+c.Name]
				row = append(row, v)
				totals[j] += v
			}
		} else {
			row = append(row, r.Bruto)
		}
		if opts.Pajak {
			row = append(row, r.NilaiPPN, r.PPh21, r.PPh22, r.PPh23)
			tPPN += r.NilaiPPN
			tPPh21 += r.PPh21
			tPPh22 += r.PPh22
			tPPh23 += r.PPh23
		}
		rep.Rows = append(rep.Rows, row)
	}

	total := make([]any, len(cols))
	total[0] = "TOTAL"
	if len(dist) > 0 {
		for j, v := range totals {
			total[4+j] = v
		}
	} else {
		var tb int64
		for _, r := range data.Rows {
			tb += r.Bruto
		}
		total[4] = tb
	}
	if opts.Pajak {
		taxIdx := 4 + len(dist)
		if taxIdx == 4 {
			taxIdx = 5
		}
		total[taxIdx] = tPPN
		total[taxIdx+1] = tPPh21
		total[taxIdx+2] = tPPh22
		total[taxIdx+3] = tPPh23
	}
	rep.TotalRow = total
	return rep, nil
}

// reportFilename menghasilkan nama file export sesuai spesifikasi.
func reportFilename(kind, nama string, t time.Time) string {
	nama = strings.ReplaceAll(nama, " ", "_")
	return fmt.Sprintf("%s_%s_%s", kind, nama, t.Format("20060102"))
}
