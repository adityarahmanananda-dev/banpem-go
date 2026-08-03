// Package tax mengimplementasikan mesin pajak hitung_pajak (bagian 2 spesifikasi).
// Semua perhitungan memakai integer sen dan pembulatan HALF-EVEN bertahap.
package tax

import "ebku/internal/money"

// Result adalah keluaran hitung_pajak. Semua nilai dalam sen.
type Result struct {
	DPP          int64
	DPPNilaiLain int64
	PPN          int64
	PPH          int64
	JenisPPH     *string
	Netto        int64
}

func sp(s string) *string { return &s }

// Hitung menerima bruto (sen), jenisMenu 1-5, flagPpn 1/2, flagPph 1/2/3,
// kategori *int (1-4) dan mengembalikan hasil persis sesuai spesifikasi.
func Hitung(bruto int64, jenisMenu, flagPpn, flagPph int, kategori *int) Result {
	var r Result
	switch jenisMenu {
	case 1:
		if flagPpn == 1 {
			r.DPP = money.MulDiv(bruto, 100, 111)
			r.DPPNilaiLain = money.MulDiv(r.DPP, 11, 12)
			r.PPN = money.Pct(r.DPPNilaiLain, 12)
		} else {
			r.DPP = bruto
		}
		switch flagPph {
		case 1:
			r.PPH = money.MulDiv(r.DPP, 15, 1000)
			r.JenisPPH = sp("PPh 22")
		case 2:
			r.PPH = money.Pct(r.DPP, 2)
			r.JenisPPH = sp("PPh 23")
		}
	case 2:
		if kategori != nil {
			switch *kategori {
			case 2:
				r.PPH = money.Pct(bruto, 5)
				r.JenisPPH = sp("PPh 21 Gol II")
			case 3:
				r.PPH = money.Pct(bruto, 15)
				r.JenisPPH = sp("PPh 21 Gol IV")
			case 4:
				r.PPH = money.MulDiv(money.Pct(bruto, 5), 5, 10)
				r.JenisPPH = sp("PPh 21 Non PNS")
			}
		}
	case 3:
		r.PPH = money.Pct(bruto, 5)
		r.JenisPPH = sp("PPh 21")
	}
	r.Netto = bruto - r.PPN - r.PPH
	return r
}
