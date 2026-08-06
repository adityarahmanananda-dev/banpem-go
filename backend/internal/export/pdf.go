package export

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

// Layout dokumen resmi: margin ~18-20 mm, monokrom, font serif. Orientasi
// A4 portrait (default) atau landscape (untuk laporan berkolom banyak).
const (
	pdfLeft   = 18.0
	pdfRight  = 18.0
	pdfTop    = 20.0
	pdfBot    = 18.0

	titleSize = 14.0
	subSize   = 12.0
	headSize  = 10.5
	bodySize  = 10.0
	footSize  = 11.0
	minRowH   = 7.0  // ~20pt
	cellPadX  = 1.8  // ~5pt
	cellPadY  = 0.9  // ~2.5pt
)

func lineH(size float64) float64 { return size * 0.40 }

type pdfRender struct {
	p      *fpdf.Fpdf
	utf    bool
	usable float64 // lebar area cetak
	pageH  float64 // tinggi halaman
}

// PDF menghasilkan file .pdf A4 dari model laporan dengan gaya dokumen resmi:
// monokrom, font serif, tanpa shading, border tipis, header tabel diulang tiap
// halaman, footer tanda tangan hanya di halaman terakhir.
func PDF(rep Report) ([]byte, error) {
	r, err := newPDF(rep.Landscape)
	if err != nil {
		return nil, err
	}
	if err := r.render(rep); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := r.p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newPDF(landscape bool) (*pdfRender, error) {
	pageW, pageH, orient := 210.0, 297.0, "P"
	if landscape {
		pageW, pageH, orient = 297.0, 210.0, "L"
	}
	p := fpdf.New(orient, "mm", "A4", "")
	p.SetMargins(pdfLeft, pdfTop, pdfRight)
	p.SetAutoPageBreak(true, pdfBot)
	r := &pdfRender{p: p, usable: pageW - pdfLeft - pdfRight, pageH: pageH}
	if err := r.addFonts(); err != nil {
		return nil, err
	}
	r.utf = true
	return r, nil
}

func (r *pdfRender) addFonts() error {
	reg := pdfFont("")
	bold := pdfFont("B")
	if reg == nil || bold == nil {
		return fmt.Errorf("font serif tidak tersedia")
	}
	r.p.AddUTF8FontFromBytes("Serif", "", reg)
	r.p.AddUTF8FontFromBytes("Serif", "B", bold)
	return nil
}

func (r *pdfRender) font(style string, size float64) {
	r.p.SetFont("Serif", style, size)
}

func (r *pdfRender) render(rep Report) error {
	p := r.p
	p.AddPage()
	p.SetTextColor(0, 0, 0)

	// Judul (hanya halaman pertama): baris 1 = judul (bold), baris 2 = nama hibah.
	r.font("B", titleSize)
	p.CellFormat(0, lineH(titleSize), strings.ToUpper(rep.Title), "", 2, "C", false, 0, "")
	// Sub judul boleh membungkus bila namanya panjang agar tidak terpotong.
	r.font("", subSize)
	subLines := r.wrapText(rep.Subtitle, r.usable-2*pdfLeft)
	for _, ln := range subLines {
		p.CellFormat(0, lineH(subSize), ln, "", 2, "C", false, 0, "")
	}
	p.Ln(3)

	widths := r.computeWidths(rep, r.usable)

	r.drawHeader(widths, rep)
	for i, row := range rep.Rows {
		bold := i < len(rep.RowBold) && rep.RowBold[i]
		noBorder := i < len(rep.RowNoBorder) && rep.RowNoBorder[i]
		rh := r.rowHeight(row, rep.Cols, widths, bold)
		if p.GetY()+rh > r.pageH-pdfBot {
			p.AddPage()
			r.drawHeader(widths, rep)
		}
		r.drawRow(row, rep.Cols, widths, rh, bold, noBorder)
	}
	if rep.TotalRow != nil {
		rh := r.rowHeight(rep.TotalRow, rep.Cols, widths, true)
		if p.GetY()+rh > r.pageH-pdfBot {
			p.AddPage()
			r.drawHeader(widths, rep)
		}
		r.drawTotal(rep.TotalRow, rep.Cols, widths, rh, rep.TotalMerge)
	}
	r.drawSig(rep.Sig, widths)
	return nil
}

// computeWidths menghitung lebar kolom: kolom tetap (bukan Flex) diset cukup
// lebar agar isinya (tanggal & nilai rupiah) muat SATU BARIS tanpa wrapping,
// sedangkan sisa lebar halaman dibagikan ke kolom Flex (Uraian & No Bukti)
// yang boleh membungkus.
func (r *pdfRender) computeWidths(rep Report, usable float64) []float64 {
	n := len(rep.Cols)
	ws := make([]float64, n)
	fixed := 0.0
	for i, col := range rep.Cols {
		ws[i] = col.Width // deklarasi sebagai minimum
		r.font("B", headSize)
		if w := r.p.GetStringWidth(col.Header) + 2*cellPadX; w > ws[i] {
			ws[i] = w
		}
		r.font("", bodySize)
		for ri, row := range rep.Rows {
			if i >= len(row) || row[i] == nil {
				continue
			}
			style := ""
			if ri < len(rep.RowBold) && rep.RowBold[ri] {
				style = "B"
			}
			r.font(style, bodySize)
			w := r.p.GetStringWidth(r.cellText(row[i], col)) + 2*cellPadX
			if col.Money {
				w += r.p.GetStringWidth("Rp.") + 2
			}
			if w > ws[i] {
				ws[i] = w
			}
		}
		r.font("", bodySize)
		if rep.TotalRow != nil && i < len(rep.TotalRow) && rep.TotalRow[i] != nil {
			r.font("B", bodySize)
			w := r.p.GetStringWidth(r.cellText(rep.TotalRow[i], col)) + 2*cellPadX
			if col.Money {
				w += r.p.GetStringWidth("Rp.") + 2
			}
			if w > ws[i] {
				ws[i] = w
			}
			r.font("", bodySize)
		}
		if !col.Flex {
			fixed += ws[i]
		}
	}
	flexDeclared := 0.0
	for _, col := range rep.Cols {
		if col.Flex {
			flexDeclared += col.Width
		}
	}
	remain := usable - fixed
	if remain <= 0 {
		// tetap butuh lebih; skala semua kolom proporsional.
		total := fixed
		for _, w := range ws {
			total += w
		}
		f := usable / total
		for i := range ws {
			ws[i] *= f
		}
		return ws
	}
	if flexDeclared > 0 {
		for i, col := range rep.Cols {
			if col.Flex {
				ws[i] = remain * (col.Width / flexDeclared)
			}
		}
	} else {
		total := 0.0
		for _, w := range ws {
			total += w
		}
		f := usable / total
		for i := range ws {
			ws[i] *= f
		}
	}
	return ws
}

func (r *pdfRender) drawHeader(ws []float64, rep Report) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	r.font("B", headSize)
	lh := lineH(headSize)
	maxLines := 1
	for i, c := range rep.Cols {
		usable := ws[i] - 2*cellPadX
		lines := r.wrapText(c.Header, usable)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	hh := float64(maxLines)*lh + 2*cellPadY
	if hh < minRowH {
		hh = minRowH
	}
	for i, c := range rep.Cols {
		p.Rect(x, y, ws[i], hh, "D")
		usable := ws[i] - 2*cellPadX
		lines := r.wrapText(c.Header, usable)
		if len(lines) == 0 {
			lines = []string{""}
		}
		ly := y + (hh-lh*float64(len(lines)))/2
		for _, ln := range lines {
			p.SetXY(x+cellPadX, ly)
			p.CellFormat(usable, lh, ln, "", 0, "C", false, 0, "")
			ly += lh
		}
		x += ws[i]
	}
	p.SetY(y + hh)
}

func (r *pdfRender) cellAlign(col Col, i int) string {
	switch {
	case col.Num:
		return "R"
	case col.Left:
		return "L"
	case col.Center || i == 0:
		return "C"
	default:
		return "L"
	}
}

// wrapText memecah teks menjadi baris-baris yang muat di usable: word wrapping
// biasa + break-word untuk string panjang tanpa spasi. TIDAK mengecilkan font,
// TIDAK ellipsis. Indentasi (spasi di awal baris) dipertahankan pada setiap
// baris hasil wrap agar hierarki berjenjang tetap terlihat.
func (r *pdfRender) wrapText(text string, usable float64) []string {
	if text == "" {
		return nil
	}
	if usable <= 1 {
		// Kolom terlalu sempit: kembalikan per baris tanpa break-word.
		return strings.Split(text, "\n")
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		indent := ""
		rest := para
		for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
			indent += string(rest[0])
			rest = rest[1:]
		}
		if rest == "" {
			out = append(out, indent)
			continue
		}
		words := strings.Fields(rest)
		line := indent
		for _, w := range words {
			// Break kata panjang (tanpa spasi) per karakter.
			for r.p.GetStringWidth(w) > usable {
				cut := len(w)
				for cut > 0 && r.p.GetStringWidth(w[:cut]) > usable {
					cut--
				}
				if cut == 0 {
					cut = 1
				}
				if line != indent {
					out = append(out, line)
				}
				out = append(out, indent+w[:cut])
				line = indent
				w = w[cut:]
			}
			test := w
			if line != indent {
				test = line + " " + w
			} else {
				test = indent + w
			}
			if r.p.GetStringWidth(test) > usable && line != indent {
				out = append(out, line)
				line = indent + w
			} else {
				line = test
			}
		}
		out = append(out, line)
	}
	return out
}

// pdfAmount memformat sen menjadi "20,850,000" (komma ribu, tanpa desimal,
// tanpa Rp). Nilai 0 ditampilkan kosong.
func pdfAmount(c int64) string {
	if c == 0 {
		return ""
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
	s := thousandsComma(q)
	if neg && q > 0 {
		return "-" + s
	}
	return s
}

func thousandsComma(n int64) string {
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

func (r *pdfRender) cellText(cell any, col Col) string {
	if col.Num {
		switch v := cell.(type) {
		case int64:
			return pdfAmount(v)
		case int:
			return pdfAmount(int64(v))
		}
	}
	return CellText(cell)
}

func (r *pdfRender) rowHeight(row []any, cols []Col, ws []float64, bold bool) float64 {
	max := minRowH
	style := ""
	if bold {
		style = "B"
	}
	r.font(style, bodySize)
	lh := lineH(bodySize)
	for i, cell := range row {
		if cell == nil {
			continue
		}
		col := cols[i]
		if col.Money {
			if _, ok := cell.(int64); ok {
				continue // cell uang selalu satu baris
			}
		}
		usable := ws[i] - 2*cellPadX
		lines := r.wrapText(r.cellText(cell, col), usable)
		if len(lines) == 0 {
			continue
		}
		h := float64(len(lines))*lh + 2*cellPadY
		if h > max {
			max = h
		}
	}
	return max
}

func (r *pdfRender) drawRow(row []any, cols []Col, ws []float64, rh float64, bold bool, noBorder bool) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	for i, cell := range row {
		if noBorder {
			// hanya garis vertikal kiri & kanan agar tabel tetap menyambung
			// (tanpa garis atas/bawah antar baris komponen).
			p.Line(x, y, x, y+rh)
			if i == len(row)-1 {
				p.Line(x+ws[i], y, x+ws[i], y+rh)
			}
		} else {
			p.Rect(x, y, ws[i], rh, "D")
		}
		r.drawCell(cell, cols[i], i, x, y, ws[i], rh, bold)
		x += ws[i]
	}
	p.SetY(y + rh)
}

func (r *pdfRender) drawCell(cell any, col Col, i int, x, y, w, rh float64, bold bool) {
	if cell == nil {
		return
	}
	if col.Money {
		if n, ok := cell.(int64); ok {
			r.drawMoney(n, bold, x, y, w, rh)
			return
		}
	}
	text := r.cellText(cell, col)
	if text == "" {
		return
	}
	usable := w - 2*cellPadX
	align := r.cellAlign(col, i)
	style := ""
	if bold {
		style = "B"
	}
	r.font(style, bodySize)
	lh := lineH(bodySize)
	ly := y + cellPadY
	for _, ln := range r.wrapText(text, usable) {
		p := r.p
		p.SetXY(x+cellPadX, ly)
		p.CellFormat(usable, lh, ln, "", 0, align, false, 0, "")
		ly += lh
	}
}

// drawMoney menggambar cell uang gaya pembukuan: "Rp." di kiri dan angka
// menempel di kanan cell. Bila kolom terlalu sempit untuk keduanya, angka
// ditulis menyatu dengan "Rp." rata kiri agar tidak terpotong/tumpang tindih.
func (r *pdfRender) drawMoney(n int64, bold bool, x, y, w, rh float64) {
	style := ""
	if bold {
		style = "B"
	}
	r.font(style, bodySize)
	lh := lineH(bodySize)
	ly := y + (rh-lh)/2
	amount := pdfAmount(n)
	usable := w - 2*cellPadX
	rpWidth := r.p.GetStringWidth("Rp.") + 2
	aw := r.p.GetStringWidth(amount)
	if rpWidth+aw > usable {
		r.p.SetXY(x+cellPadX, ly)
		r.p.CellFormat(usable, lh, "Rp. "+amount, "", 0, "L", false, 0, "")
		return
	}
	r.p.SetXY(x+cellPadX, ly)
	r.p.CellFormat(rpWidth, lh, "Rp.", "", 0, "L", false, 0, "")
	r.p.SetXY(x+w-cellPadX-aw, ly)
	r.p.CellFormat(aw, lh, amount, "", 0, "L", false, 0, "")
}

func (r *pdfRender) drawTotal(row []any, cols []Col, ws []float64, rh float64, merge int) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	r.font("B", bodySize)
	i := 0
	if merge > 0 && len(row) >= merge {
		mw := 0.0
		for j := 0; j < merge; j++ {
			mw += ws[j]
		}
		p.Rect(x, y, mw, rh, "D")
		label := ""
		for j := 0; j < merge; j++ {
			if s := CellText(row[j]); s != "" {
				label = s
				break
			}
		}
		if label != "" {
			usable := mw - 2*cellPadX
			lh := lineH(bodySize)
			ly := y + (rh-lh)/2
			p.SetXY(x+cellPadX, ly)
			p.CellFormat(usable, lh, label, "", 0, "C", false, 0, "")
		}
		x += mw
		i = merge
	}
	for ; i < len(row); i++ {
		cell := row[i]
		p.Rect(x, y, ws[i], rh, "D")
		if cols[i].Money {
			if n, ok := cell.(int64); ok {
				r.drawMoney(n, true, x, y, ws[i], rh)
				x += ws[i]
				continue
			}
		}
		text := r.cellText(cell, cols[i])
		if text != "" {
			usable := ws[i] - 2*cellPadX
			align := r.cellAlign(cols[i], i)
			lh := lineH(bodySize)
			ly := y + (rh-lh)/2
			p.SetXY(x+cellPadX, ly)
			p.CellFormat(usable, lh, text, "", 0, align, false, 0, "")
		}
		x += ws[i]
	}
	p.SetY(y + rh)
}

func (r *pdfRender) drawSig(sig SigData, ws []float64) {
	p := r.p
	total := 0.0
	for _, w := range ws {
		total += w
	}
	half := total / 2
	rightX := pdfLeft + half
	// Seluruh blok tanda tangan (gap + tanggal + kepala + spasi + nama + NIP)
	// harus muat di satu halaman; jika tidak, pindah ke halaman berikutnya.
	topGap := lineH(footSize)
	sigH := topGap + 6 + 6 + 18 + 6 + 6
	if p.GetY()+sigH > r.pageH-pdfBot {
		p.AddPage()
	}
	r.font("", footSize)
	p.Ln(topGap)
	p.SetX(rightX)
	p.CellFormat(half, 6, sig.DateText, "", 2, "R", false, 0, "")
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, sig.HeadLeft, "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 6, sig.HeadRight, "", 2, "R", false, 0, "")
	p.Ln(18)
	r.font("B", footSize)
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, sig.NameLeft, "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 6, sig.NameRight, "", 2, "R", false, 0, "")
	r.font("", footSize)
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, sig.NipLeft, "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 6, sig.NipRight, "", 2, "R", false, 0, "")
}

