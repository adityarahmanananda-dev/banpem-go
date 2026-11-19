package store

import "testing"

func TestValidTaxJenis(t *testing.T) {
	valid := []string{"PPN", "PPh 21", "PPh 22", "PPh 23", "PPh 4 Ayat 2"}
	for _, j := range valid {
		if !ValidTaxJenis(j) {
			t.Errorf("ValidTaxJenis(%q) = false, want true", j)
		}
	}
	for _, j := range []string{"", "PPh", "PPh 4", "PPh 25", "pajak"} {
		if ValidTaxJenis(j) {
			t.Errorf("ValidTaxJenis(%q) = true, want false", j)
		}
	}
}

func TestNomorBuktiSetor(t *testing.T) {
	cases := []struct {
		ntbn, ntpn, want string
	}{
		{"", "8DF495BNDM2SG676", "NTPN\n=8DF495BNDM2SG676"},
		{"NTB001", "8DF495BNDM2SG676", "NTPN\n=8DF495BNDM2SG676"},
		{"NTB001", "", "NTB\n=NTB001"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := nomorBuktiSetor(c.ntbn, c.ntpn); got != c.want {
			t.Errorf("nomorBuktiSetor(%q,%q) = %q, want %q", c.ntbn, c.ntpn, got, c.want)
		}
	}
}
