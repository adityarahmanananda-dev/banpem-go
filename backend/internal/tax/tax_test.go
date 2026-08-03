package tax

import (
	"testing"

	"ebku/internal/money"
)

func strPtrEq(a *string, b string) bool {
	if a == nil {
		return b == ""
	}
	return *a == b
}

func TestVectors(t *testing.T) {
	type tc struct {
		bruto                       int64
		menu, flagPPN, flagPPH, kat int
		hasKat                      bool
		dpp, dppnl, ppn, pph, netto int64
		jenis                       string
	}
	k := func(v int) *int { return &v }
	cases := []tc{
		{100000000, 1, 1, 2, 0, false, 90090090, 82582582, 9909910, 1801802, 88288288, "PPh 23"},
		{1000000000, 2, 2, 3, 3, true, 0, 0, 0, 150000000, 850000000, "PPh 21 Gol IV"},
		{1000000000, 2, 2, 3, 4, true, 0, 0, 0, 25000000, 975000000, "PPh 21 Non PNS"},
		{100000000, 3, 2, 3, 0, false, 0, 0, 0, 5000000, 95000000, "PPh 21"},
		{50000000, 4, 2, 3, 0, false, 0, 0, 0, 0, 50000000, ""},
	}
	for i, c := range cases {
		var kp *int
		if c.hasKat {
			kp = k(c.kat)
		}
		got := Hitung(c.bruto, c.menu, c.flagPPN, c.flagPPH, kp)
		if got.DPP != c.dpp || got.DPPNilaiLain != c.dppnl || got.PPN != c.ppn ||
			got.PPH != c.pph || got.Netto != c.netto || !strPtrEq(got.JenisPPH, c.jenis) {
			t.Errorf("vector %d: got dpp=%d dppnl=%d ppn=%d pph=%d netto=%d jenis=%v, want dpp=%d dppnl=%d ppn=%d pph=%d netto=%d jenis=%s",
				i, got.DPP, got.DPPNilaiLain, got.PPN, got.PPH, got.Netto, got.JenisPPH,
				c.dpp, c.dppnl, c.ppn, c.pph, c.netto, c.jenis)
		}
		if got.JenisPPH != nil && got.JenisPPH == nil {
		}
	}
}

func TestVectorDisplay(t *testing.T) {
	got := Hitung(100000000, 1, 1, 2, nil)
	if money.FormatIDR(got.Netto) != "Rp 882.882,88" {
		t.Errorf("netto display = %s", money.FormatIDR(got.Netto))
	}
}
