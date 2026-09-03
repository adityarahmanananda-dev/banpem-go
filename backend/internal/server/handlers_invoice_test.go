package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestInvoiceBupotPPH memastikan nomor bupot PPh diambil dari field menu yang
// aktif (menu 2 = _ns untuk Honor Narasumber, menu 3 = _pst untuk Honor
// Peserta, selain itu _pph) agar nilai bupot PPh 21 tidak tertimpa field menu
// lain yang tersembunyi dan kosong.
func TestInvoiceBupotPPH(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		expected string
	}{
		{"menu1 -> nomor_bupot_pph", "jenis_menu=1&nomor_bupot_pph=M1&nomor_bupot_pph_ns=NS&nomor_bupot_pph_pst=PST", "M1"},
		{"menu2 -> nomor_bupot_pph_ns", "jenis_menu=2&nomor_bupot_pph=&nomor_bupot_pph_ns=NS-21&nomor_bupot_pph_pst=", "NS-21"},
		{"menu3 -> nomor_bupot_pph_pst", "jenis_menu=3&nomor_bupot_pph=&nomor_bupot_pph_ns=&nomor_bupot_pph_pst=PST-21", "PST-21"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(c.body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if got := invoiceBupotPPH(r); got != c.expected {
				t.Fatalf("invoiceBupotPPH() = %q, want %q", got, c.expected)
			}
		})
	}
}

// TestExtractNoBukti memastikan nomor NTB/NTPN terbaca dari nomor bukti format
// dua baris ("NTPN" lalu "=nilai") maupun format satu baris lama.
func TestExtractNoBukti(t *testing.T) {
	cases := []struct {
		nb, prefix, want string
	}{
		{"NTPN\n=8DF495BNDM2SG676", "NTPN", "8DF495BNDM2SG676"},
		{"NTPN=8DF495BNDM2SG676", "NTPN", "8DF495BNDM2SG676"},
		{"NTB\n=ABC123", "NTB", "ABC123"},
		{"Penyetoran PPN\nNTPN\n=8DF495BNDM2SG676", "NTPN", "8DF495BNDM2SG676"},
	}
	for _, c := range cases {
		if got := extractNoBukti(c.nb, c.prefix); got != c.want {
			t.Errorf("extractNoBukti(%q,%q) = %q, want %q", c.nb, c.prefix, got, c.want)
		}
	}
}