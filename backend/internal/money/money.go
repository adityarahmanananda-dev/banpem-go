// Package money menangani seluruh perhitungan uang sebagai integer sen (cent).
// Semua pembulatan memakai ROUND HALF-EVEN (banker's rounding), bertahap di
// setiap langkah perantara.
package money

import (
	"math/big"
	"strconv"
	"strings"
)

// MulDiv menghitung x*p/q dengan pembulatan half-even ke bilangan bulat.
// Memakai big.Int agar tidak pernah melimpah.
func MulDiv(x, p, q int64) int64 {
	if q == 0 || x == 0 || p == 0 {
		return 0
	}
	neg := (x < 0) != (p < 0)
	if q < 0 {
		q = -q
	}
	ax := new(big.Int).SetInt64(x)
	ap := new(big.Int).SetInt64(p)
	if ax.Sign() < 0 {
		ax.Neg(ax)
	}
	if ap.Sign() < 0 {
		ap.Neg(ap)
	}
	num := new(big.Int).Mul(ax, ap)
	qb := big.NewInt(q)
	quot, rem := new(big.Int).QuoRem(num, qb, new(big.Int))
	twice := new(big.Int).Mul(rem, big.NewInt(2))
	switch twice.Cmp(qb) {
	case 1:
		quot.Add(quot, big.NewInt(1))
	case 0:
		if quot.Bit(0) == 1 {
			quot.Add(quot, big.NewInt(1))
		}
	}
	if neg {
		quot.Neg(quot)
	}
	return quot.Int64()
}

// Pct menghitung x*p/100 dengan pembulatan half-even.
func Pct(x, p int64) int64 { return MulDiv(x, p, 100) }

// RoundInt membulatkan sen ke rupiah utuh (half-even).
func RoundInt(c int64) int64 {
	if c == 0 {
		return 0
	}
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
	if neg {
		q = -q
	}
	return q
}

func thousands(n int64) string {
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

// FormatReport menghasilkan "1.000.000" (tanpa Rp, ribuan titik, tanpa desimal).
func FormatReport(c int64) string {
	return thousands(RoundInt(c))
}

// FormatUI menghasilkan "Rp.1.000.000" (tanpa desimal, prefix Rp., tanpa spasi).
// Untuk nilai 0 -> "Rp.0".
func FormatUI(c int64) string {
	v := thousands(RoundInt(c))
	if strings.HasPrefix(v, "-") {
		return "-Rp." + v[1:]
	}
	return "Rp." + v
}

// FormatIDR menghasilkan "Rp 1.000.000,00" (desimal 2 digit, ribuan titik,
// koma desimal). Dipakai di uraian/notifikasi.
func FormatIDR(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole, frac := c/100, c%100
	fs := strconv.FormatInt(frac, 10)
	if len(fs) < 2 {
		fs = "0" + fs
	}
	s := thousands(whole) + "," + fs
	if neg {
		return "-Rp " + s
	}
	return "Rp " + s
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Parse mengubah input user menjadi sen. Menerima "1000000", "1.000.000",
// "1000000.50", "1.000.000,50" dan "-1.000.000".
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	comma := strings.LastIndexByte(s, ',')
	dot := strings.LastIndexByte(s, '.')
	decSep := -1
	switch {
	case comma >= 0 && dot >= 0:
		if comma > dot {
			decSep = comma
		} else {
			decSep = dot
		}
	case comma >= 0:
		decSep = comma
	case dot >= 0:
		rest := s[dot+1:]
		if strings.Count(s, ".") == 1 && isAllDigits(rest) && len(rest) > 0 && len(rest) <= 2 {
			decSep = dot
		}
	}
	whole := s
	frac := ""
	if decSep >= 0 {
		whole = s[:decSep]
		frac = s[decSep+1:]
	}
	digits := ""
	for _, r := range whole {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}
	if digits == "" {
		digits = "0"
	}
	iv, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, err
	}
	fdig := ""
	for _, r := range frac {
		if r >= '0' && r <= '9' {
			fdig += string(r)
		}
	}
	if len(fdig) > 2 {
		fdig = fdig[:2]
	}
	for len(fdig) < 2 {
		fdig += "0"
	}
	var fv int64
	if fdig != "" {
		fv, err = strconv.ParseInt(fdig, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	total := iv*100 + fv
	if neg {
		total = -total
	}
	return total, nil
}
