package money

import "testing"

func TestMulDivHalfEven(t *testing.T) {
	cases := []struct {
		x, p, q, want int64
	}{
		{100000000, 100, 111, 90090090},
		{90090090, 11, 12, 82582582},
		{82582582, 12, 100, 9909910},
		{90090090, 15, 1000, 1351351},
		{90090090, 2, 100, 1801802},
		{1, 1, 2, 0}, // tie 0.5 -> even 0
		{3, 1, 2, 2}, // tie 1.5 -> even 2
		{5, 1, 2, 2}, // tie 2.5 -> even 2
		{7, 1, 2, 4}, // tie 3.5 -> even 4
		{1, 1, 4, 0}, // 0.25 -> 0
		{3, 1, 4, 1}, // 0.75 -> 1
		{5, 1, 4, 1}, // 1.25 -> 1
		{-100000000, 100, 111, -90090090},
	}
	for _, c := range cases {
		got := MulDiv(c.x, c.p, c.q)
		if got != c.want {
			t.Errorf("MulDiv(%d,%d,%d) = %d, want %d", c.x, c.p, c.q, got, c.want)
		}
	}
}

func TestRoundInt(t *testing.T) {
	cases := []struct {
		in, want int64
	}{
		{100, 1},
		{150, 2},
		{250, 2},
		{350, 4},
		{50, 0},
		{-150, -2},
		{-50, 0},
	}
	for _, c := range cases {
		if got := RoundInt(c.in); got != c.want {
			t.Errorf("RoundInt(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	if got := FormatReport(100000000); got != "1.000.000" {
		t.Errorf("FormatReport = %s", got)
	}
	if got := FormatReport(0); got != "0" {
		t.Errorf("FormatReport(0) = %s", got)
	}
	if got := FormatUI(100000000); got != "Rp.1.000.000" {
		t.Errorf("FormatUI = %s", got)
	}
	if got := FormatUI(0); got != "Rp.0" {
		t.Errorf("FormatUI(0) = %s", got)
	}
	if got := FormatIDR(88288288); got != "Rp 882.882,88" {
		t.Errorf("FormatIDR = %s", got)
	}
	if got := FormatIDR(100000000); got != "Rp 1.000.000,00" {
		t.Errorf("FormatIDR(1jt) = %s", got)
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1000000", 100000000},
		{"1.000.000", 100000000},
		{"1000000.50", 100000050},
		{"1.000.000,50", 100000050},
		{"1000000,5", 100000050},
		{"882882.88", 88288288},
		{"", 0},
		{"0", 0},
		{"2900", 290000},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
