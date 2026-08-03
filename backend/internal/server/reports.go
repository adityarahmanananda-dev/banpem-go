package server

import (
	"context"
	"fmt"
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
		{Header: "No", Width: 12, ExWidth: 8},
		{Header: "Tanggal", Width: 22, ExWidth: 14},
		{Header: "No. Bukti", Width: 30, ExWidth: 16, Wrap: true, Flex: true},
		{Header: "Uraian", Width: 100, ExWidth: 45, Wrap: true, Flex: true},
		{Header: "Debet (Rp)", Width: 35, ExWidth: 18, Num: true},
		{Header: "Kredit (Rp)", Width: 35, ExWidth: 18, Num: true},
		{Header: "Saldo", Width: 35, ExWidth: 18, Num: true},
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
			kredit = base
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
		rep.Rows = append(rep.Rows, []any{
			e.Nomor, tanggal, e.NomorBukti, e.Uraian, e.Debit, kredit, saldo,
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
		{Header: "No", Width: 8, ExWidth: 6},
		{Header: "Tanggal", Width: 20, ExWidth: 14},
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

// buildDaftarTagihan membuat laporan Daftar Tagihan.
func (s *Server) buildDaftarTagihan(ctx context.Context, b *store.Bantuan, tglCetak time.Time) (export.Report, error) {
	invoices, err := s.Store.ListInvoices(ctx, b.ID, "sort")
	if err != nil {
		return export.Report{}, err
	}
	cols := []export.Col{
		{Header: "No", Width: 8, ExWidth: 6},
		{Header: "Tanggal", Width: 17, ExWidth: 14},
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
		Title:    "DAFTAR TAGIHAN",
		Subtitle: sekolahNama(b),
		Cols:     cols,
		Sig:      sigData(b, tglCetak),
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

// reportFilename menghasilkan nama file export sesuai spesifikasi.
func reportFilename(kind, nama string, t time.Time) string {
	nama = strings.ReplaceAll(nama, " ", "_")
	return fmt.Sprintf("%s_%s_%s", kind, nama, t.Format("20060102"))
}
