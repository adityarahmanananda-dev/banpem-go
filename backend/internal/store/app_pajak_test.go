package store

import "testing"

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