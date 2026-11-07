package money

import "testing"

func TestParseStrict(t *testing.T) {
	valid := map[string]int64{
		"1000000":      100000000,
		"1.000.000":    100000000,
		"1.000.000,50": 100000050,
		"-1.000":       -100000,
	}
	for in, want := range valid {
		got, err := ParseStrict(in)
		if err != nil || got != want {
			t.Errorf("ParseStrict(%q)=(%d,%v), want %d", in, got, err, want)
		}
	}
	invalid := []string{"", "abc", "1.000rb", "12x", "1,2,3"}
	for _, in := range invalid {
		if _, err := ParseStrict(in); err == nil {
			t.Errorf("ParseStrict(%q) harus error", in)
		}
	}
}

func TestParseNonNegativeAndPositive(t *testing.T) {
	if _, err := ParseNonNegative("-1"); err == nil {
		t.Fatal("negatif harus ditolak")
	}
	if _, err := ParsePositive("0"); err == nil {
		t.Fatal("nol harus ditolak untuk ParsePositive")
	}
	if v, err := ParsePositive("1.000"); err != nil || v != 100000 {
		t.Fatalf("ParsePositive(1.000)=(%d,%v)", v, err)
	}
}
