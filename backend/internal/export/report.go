// Package export merender laporan (Excel, PDF, Word) sesuai layout spesifikasi.
package export

import (
	"strconv"
	"strings"
)

// Col mendeskripsikan satu kolom laporan.
type Col struct {
	Header string
	// Width mm (untuk PDF & Word).
	Width float64
	// ExWidth lebar kolom Excel (satuan karakter).
	ExWidth float64
	// Num menandai kolom angka (rata kanan, format angka, ikut TOTAL).
	Num bool
	// Wrap menandai kolom teks yang boleh membungkus (No. Bukti, Uraian, NTB/NTPN).
	Wrap bool
	// Flex menandai kolom yang lebar-nya fleksibel: ketika total lebar kolom
	// melebihi halaman, hanya kolom Flex yang dikurangi (kolom tetap tidak berubah).
	Flex bool
	// Center menandai kolom identitas (No, Tanggal) yang dirata-tengahkan.
	Center bool
	// Top menandai perataan vertikal atas (untuk Excel NTB/NTPN).
	Top bool
	// Left memaksa perataan kiri pada kolom pertama (untuk hierarki berjenjang).
	Left bool
	// Money menandai kolom uang gaya pembukuan: "Rp." di kiri cell dan angka
	// menempel di kanan cell (format akuntansi Excel).
	Money bool
	// MaxWidth membatasi lebar kolom (mm, untuk PDF) agar isi panjang membungkus
	// alih-alih kolom melebar; 0 = tanpa batas.
	MaxWidth float64
}

// ColGroup mendeskripsikan grup header di atas kolom (mis. "KUITANSI" yang
// merangkum kolom No. Bukti Dokumen & Tanggal Bukti Dokumen).
type ColGroup struct {
	Header string
	Start  int // indeks kolom pertama (0-based) yang dicakup grup
	Span   int // jumlah kolom yang dicakup grup
}

// SigData adalah blok tanda tangan laporan.
type SigData struct {
	DateText  string
	HeadLeft  string
	HeadRight string
	NameLeft  string
	NameRight string
	NipLeft   string
	NipRight  string
}

// Report adalah model laporan generik. Cell berupa string (teks) atau int64
// (jumlah uang dalam sen).
type Report struct {
	Title    string
	Subtitle string
	Cols     []Col
	Rows     [][]any
	TotalRow []any // nil jika tanpa TOTAL
	// ColGroups menambahkan baris header berkelompok di atas kolom (mis.
	// "KUITANSI" merangkum beberapa kolom). Bila kosong, tidak ada baris grup.
	ColGroups []ColGroup
	// RowBold menandai baris data yang harus dicetak tebal (baris subtotal
	// pada pivot berjenjang). Panjangnya menyusul baris Rows.
	RowBold []bool
	// RowNoBorder menandai baris data yang dicetak TANPA border (mis. baris
	// komponen pada RAB). Panjangnya menyusul baris Rows.
	RowNoBorder []bool
	// TotalMerge jumlah kolom awal pada baris TOTAL yang digabung menjadi satu
	// cell label (mis. "TOTAL") yang dirapikan di tengah.
	TotalMerge int
	// Landscape true = A4 landscape (untuk laporan berkolom banyak seperti
	// Rekap Pajak & Rekap Belanja); false = A4 portrait.
	Landscape bool
	Sig       SigData
}

// FmtAmount mengubah sen menjadi string laporan "1.000.000" (ribuan titik,
// TANPA desimal), dibulatkan half-even ke rupiah utuh, tanpa "Rp".
func FmtAmount(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	q, r := c/100, c%100
	switch {
	case r > 50:
		q++
	case r == 50:
		if q&1 == 1 {
			q++
		}
	}
	out := formatThousands(q)
	if neg && q > 0 {
		return "-" + out
	}
	return out
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func formatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		return "-" + out
	}
	return out
}

// CellText mengubah isi cell menjadi teks untuk PDF/Word.
func CellText(v any) string {
	switch t := v.(type) {
	case int64:
		return FmtAmount(t)
	case int:
		return strconv.Itoa(t)
	case string:
		return t
	case nil:
		return ""
	}
	return ""
}

// AmountValue mengembalikan nilai uang cell (0 bila bukan angka).
func AmountValue(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	}
	return 0
}
