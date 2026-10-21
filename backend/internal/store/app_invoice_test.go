package store

import "testing"

func TestAdminFee(t *testing.T) {
	cases := []struct {
		name         string
		bankBantuan  string
		bankPenerima string
		jenis        string
		want         int64
	}{
		{"bank kosong berarti tunai", "BRI", "", "kliring", 0},
		{"bank bantuan kosong", "", "BNI", "kliring", 0},
		{"bank sama tanpa biaya", "BRI", "bri", "rtgs", 0},
		{"kliring", "BRI", "BNI", "kliring", AdminFeeKliring},
		{"rtgs", "BRI", "BNI", "rtgs", AdminFeeRTGS},
		{"jenis kosong default kliring", "BRI", "BNI", "", AdminFeeKliring},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AdminFee(c.bankBantuan, c.bankPenerima, c.jenis); got != c.want {
				t.Fatalf("AdminFee(%q, %q, %q) = %d, want %d",
					c.bankBantuan, c.bankPenerima, c.jenis, got, c.want)
			}
		})
	}
}
