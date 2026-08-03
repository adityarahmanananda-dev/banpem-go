package server

import (
	"encoding/json"
	"html/template"
	"strings"
	"time"

	"ebku/internal/money"
	"ebku/internal/store"
)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"fmtMoney":     money.FormatUI,
		"fmtMoneyNoRp": money.FormatReport,
		"dateID": func(t time.Time) string {
			return t.Format("02-01-2006")
		},
		"dateTimeID": func(t time.Time) string {
			return t.Format("02-01-2006 15:04")
		},
		"jenisPajakLabel": func(s string) string {
			if s == "" {
				return "-"
			}
			return s
		},
		"jenisMenuLabel": func(i int) string {
			if n, ok := menuNames[i]; ok && n != "" {
				return n
			}
			return "Barang/Jasa"
		},
		"adminFee": store.AdminFee,
		"join":     strings.Join,
		"add": func(a, b int64) int64 { return a + b },
		"sub": func(a, b int64) int64 { return a - b },
		"inc": func(i int) int { return i + 1 },
		"kegColor": func(i int) string {
			c := []string{"primary", "success", "danger", "info", "secondary", "dark"}[i%6]
			if c == "warning" {
				return "bg-warning text-dark"
			}
			return "bg-" + c + " text-white"
		},
		"subColor": func(i int) string {
			c := []string{"primary", "success", "warning", "danger", "info", "secondary"}[i%6]
			return "bg-" + c + "-subtle"
		},
		"jsonDump": func(v any) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return template.JS(b)
		},
		"intp": func(v *int) int {
			if v == nil {
				return 0
			}
			return *v
		},
		"int64p": func(v *int64) int64 {
			if v == nil {
				return 0
			}
			return *v
		},
		"splitp": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"formatNPWP": formatNPWP,
	}
}

// formatNPWP menampilkan NPWP 16 digit dalam kelompok 4 digit berjarak spasi
// (tanpa titik atau strip), mis. "1234567890123456" -> "1234 5678 9012 3456".
func formatNPWP(s string) string {
	var d strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	ds := d.String()
	if ds == "" {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(ds); i++ {
		if i > 0 && i%4 == 0 {
			out.WriteByte(' ')
		}
		out.WriteByte(ds[i])
	}
	return out.String()
}

// bantuanSummary berisi angka ringkas untuk sidebar & kartu bantuan.
type bantuanSummary struct {
	Bantuan          *store.Bantuan
	SaldoAwal        int64
	TotalTurun       int64
	TotalDebit       int64
	TotalKredit      int64
	SaldoBKU         int64
	SaldoBank        int64
	InvoiceCount     int64
	TotalBelanja     int64
	TotalPajak       int64
	JasaGiro         int64
	KegiatanCount    int64
	SubKegiatanCount int64
	KomponenCount    int64
}

// baseView adalah data umum setiap halaman.
type baseView struct {
	Title     string
	Active    string
	FlashType string
	FlashMsg  string
	Bantuan   *store.Bantuan
	Summary   *bantuanSummary
	Q         map[string]string
}
