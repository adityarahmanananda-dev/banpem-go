package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

const (
	pdfW      = 297.0
	pdfH      = 210.0
	pdfLeft   = 10.0
	pdfTop    = 15.0
	pdfBot    = 15.0
	pdfUsable = pdfW - pdfLeft - 10.0 // 277

	headerH  = 10.0
	bodySize = 9.0
	cellPadX = 1.5
	cellPadY = 1.5
)

func lineH(size float64) float64 { return size * 0.45 }

type pdfRender struct {
	p   *fpdf.Fpdf
	utf bool
}

// PDF menghasilkan file .pdf A4 landscape dari model laporan.
func PDF(rep Report) ([]byte, error) {
	r, err := newPDF()
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

func newPDF() (*pdfRender, error) {
	p := fpdf.New("L", "mm", "A4", "")
	p.SetMargins(pdfLeft, pdfTop, 10.0)
	p.SetAutoPageBreak(true, pdfBot)
	r := &pdfRender{p: p}
	if err := r.addUTF8Fonts(); err == nil {
		r.utf = true
	}
	return r, nil
}

func (r *pdfRender) addUTF8Fonts() error {
	reg := pdfFont("")
	bold := pdfFont("B")
	if reg == nil || bold == nil {
		return fmt.Errorf("font dejavu tidak tersedia")
	}
	r.p.AddUTF8FontFromBytes("DejaVu", "", reg)
	r.p.AddUTF8FontFromBytes("DejaVu", "B", bold)
	return nil
}

func (r *pdfRender) font(style string, size float64) {
	if r.utf {
		r.p.SetFont("DejaVu", style, size)
	} else {
		r.p.SetFont("Helvetica", style, size)
	}
}

func (r *pdfRender) txt(s string) string {
	if r.utf {
		return s
	}
	return toCp1252(s)
}

func (r *pdfRender) render(rep Report) error {
	p := r.p
	p.AddPage()
	p.SetTextColor(0, 0, 0)

	r.font("B", 14)
	p.CellFormat(0, 9, r.txt(strings.ToUpper(rep.Title)), "", 2, "C", false, 0, "")
	r.font("B", 12)
	p.CellFormat(0, 8, r.txt(rep.Subtitle), "", 2, "C", false, 0, "")
	p.Ln(3)

	widths := scaleWidths(rep.Cols, pdfUsable)

	r.drawHeader(widths, rep)
	for _, row := range rep.Rows {
		rh := r.rowHeight(row, rep.Cols, widths)
		if p.GetY()+rh > pdfH-pdfBot {
			p.AddPage()
			r.drawHeader(widths, rep)
		}
		r.drawRow(row, rep.Cols, widths, rh)
	}
	if rep.TotalRow != nil {
		rh := r.rowHeight(rep.TotalRow, rep.Cols, widths)
		if p.GetY()+rh > pdfH-pdfBot {
			p.AddPage()
			r.drawHeader(widths, rep)
		}
		r.drawTotal(rep.TotalRow, rep.Cols, widths, rh, rep.TotalMerge)
	}
	r.drawSig(rep.Sig, widths)
	return nil
}

func scaleWidths(cols []Col, usable float64) []float64 {
	ws := make([]float64, len(cols))
	fixed := 0.0
	flx := 0.0
	for i, c := range cols {
		ws[i] = c.Width
		if c.Flex {
			flx += c.Width
		} else {
			fixed += c.Width
		}
	}
	total := fixed + flx
	if total > usable {
		over := total - usable
		if flx > 0 && over <= flx {
			f := (flx - over) / flx
			for i, c := range cols {
				if c.Flex {
					ws[i] *= f
				}
			}
		} else {
			f := usable / total
			for i := range ws {
				ws[i] *= f
			}
		}
	}
	return ws
}

func (r *pdfRender) drawHeader(ws []float64, rep Report) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	p.SetFillColor(0x44, 0x72, 0xC4)
	p.SetTextColor(255, 255, 255)
	for i, c := range rep.Cols {
		usable := ws[i] - 2*cellPadX
		lines, size := r.cellLines(c.Header, usable, 9, "B", true)
		if len(lines) == 0 {
			lines = [][]byte{[]byte("")}
		}
		r.font("B", size)
		lh := lineH(size)
		p.Rect(x, y, ws[i], headerH, "DF")
		ly := y + (headerH-lh*float64(len(lines)))/2
		for _, ln := range lines {
			p.SetXY(x+cellPadX, ly)
			p.CellFormat(usable, lh, string(ln), "", 0, "C", false, 0, "")
			ly += lh
		}
		x += ws[i]
	}
	p.SetTextColor(0, 0, 0)
	p.SetY(y + headerH)
}

func (r *pdfRender) cellAlign(col Col, i int) string {
	switch {
	case col.Num:
		return "R"
	case i == 0:
		return "C"
	default:
		return "L"
	}
}

// cellLines memecah teks menjadi baris-baris yang muat di usable. Jika wrap
// true, teks boleh membungkus (No. Bukti, Uraian, NTB/NTPN). Jika false, teks
// dipaksa satu baris dengan mengecilkan font (kolom tetap seperti tanggal,
// debit, kredit, saldo). Mengembalikan baris dan ukuran font terpakai.
func (r *pdfRender) cellLines(text string, usable, size float64, style string, wrap bool) ([][]byte, float64) {
	r.font(style, size)
	if text == "" {
		return nil, size
	}
	lines := r.p.SplitLines([]byte(r.txt(text)), usable)
	if len(lines) > 1 && !strings.Contains(text, "\n") {
		if !wrap {
			for size > 5 && len(lines) > 1 {
				size -= 0.25
				r.font(style, size)
				lines = r.p.SplitLines([]byte(r.txt(text)), usable)
			}
		} else {
			for size > 6 && len(lines) > 5 {
				size -= 0.25
				r.font(style, size)
				lines = r.p.SplitLines([]byte(r.txt(text)), usable)
			}
		}
	}
	return lines, size
}

func (r *pdfRender) rowHeight(row []any, cols []Col, ws []float64) float64 {
	max := 0.0
	for i, cell := range row {
		usable := ws[i] - 2*cellPadX
		lines, size := r.cellLines(CellText(cell), usable, bodySize, "", cols[i].Wrap)
		if len(lines) < 1 {
			continue
		}
		h := float64(len(lines))*lineH(size) + 2*cellPadY
		if h > max {
			max = h
		}
	}
	if max == 0 {
		max = lineH(bodySize) + 2*cellPadY
	}
	return max
}

func (r *pdfRender) drawCell(cell any, col Col, i int, x, y, w, rh float64) {
	text := CellText(cell)
	if text == "" {
		return
	}
	usable := w - 2*cellPadX
	align := r.cellAlign(col, i)
	lines, size := r.cellLines(text, usable, bodySize, "", col.Wrap)
	r.font("", size)
	lh := lineH(size)
	ly := y + cellPadY
	for _, ln := range lines {
		p := r.p
		p.SetXY(x+cellPadX, ly)
		p.CellFormat(usable, lh, string(ln), "", 0, align, false, 0, "")
		ly += lh
	}
}

func (r *pdfRender) drawRow(row []any, cols []Col, ws []float64, rh float64) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	for i, cell := range row {
		p.Rect(x, y, ws[i], rh, "D")
		r.drawCell(cell, cols[i], i, x, y, ws[i], rh)
		x += ws[i]
	}
	p.SetY(y + rh)
}

func (r *pdfRender) drawTotal(row []any, cols []Col, ws []float64, rh float64, merge int) {
	p := r.p
	y := p.GetY()
	x := pdfLeft
	p.SetFillColor(0xD9, 0xE2, 0xF3)
	i := 0
	if merge > 0 && len(row) >= merge {
		mw := 0.0
		for j := 0; j < merge; j++ {
			mw += ws[j]
		}
		p.Rect(x, y, mw, rh, "DF")
		label := ""
		for j := 0; j < merge; j++ {
			if s := CellText(row[j]); s != "" {
				label = s
				break
			}
		}
		if label != "" {
			usable := mw - 2*cellPadX
			lines, size := r.cellLines(label, usable, bodySize, "B", true)
			r.font("B", size)
			lh := lineH(size)
			ly := y + (rh-lh*float64(len(lines)))/2
			for _, ln := range lines {
				p.SetXY(x+cellPadX, ly)
				p.CellFormat(usable, lh, string(ln), "", 0, "C", false, 0, "")
				ly += lh
			}
		}
		x += mw
		i = merge
	}
	for ; i < len(row); i++ {
		cell := row[i]
		p.Rect(x, y, ws[i], rh, "DF")
		text := CellText(cell)
		if text != "" {
			usable := ws[i] - 2*cellPadX
			align := r.cellAlign(cols[i], i)
			lines, size := r.cellLines(text, usable, bodySize, "B", cols[i].Wrap)
			r.font("B", size)
			lh := lineH(size)
			ly := y + cellPadY
			for _, ln := range lines {
				p.SetXY(x+cellPadX, ly)
				p.CellFormat(usable, lh, string(ln), "", 0, align, false, 0, "")
				ly += lh
			}
		}
		x += ws[i]
	}
	p.SetY(y + rh)
	p.SetTextColor(0, 0, 0)
}

func (r *pdfRender) drawSig(sig SigData, ws []float64) {
	p := r.p
	total := 0.0
	for _, w := range ws {
		total += w
	}
	half := total / 2
	rightX := pdfLeft + half
	r.font("", 10)
	p.Ln(4)
	p.SetX(rightX)
	p.CellFormat(half, 7, r.txt(sig.DateText), "", 2, "R", false, 0, "")
	p.Ln(4)
	r.font("", 10)
	p.SetX(pdfLeft)
	p.CellFormat(half, 7, r.txt(sig.HeadLeft), "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 7, r.txt(sig.HeadRight), "", 2, "R", false, 0, "")
	p.Ln(16)
	r.font("B", 10)
	p.SetX(pdfLeft)
	p.CellFormat(half, 7, r.txt(sig.NameLeft), "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 7, r.txt(sig.NameRight), "", 2, "R", false, 0, "")
	r.font("", 10)
	p.SetX(pdfLeft)
	p.CellFormat(half, 7, r.txt(sig.NipLeft), "", 0, "L", false, 0, "")
	p.SetX(rightX)
	p.CellFormat(half, 7, r.txt(sig.NipRight), "", 2, "R", false, 0, "")
}

var cp1252High = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85, '†': 0x86,
	'‡': 0x87, 'ˆ': 0x88, '‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C,
	'Ž': 0x8E, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95,
	'–': 0x96, '—': 0x97, '˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B,
	'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

func toCp1252(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7E, r >= 0xA0 && r <= 0xFF:
			b.WriteRune(r)
		default:
			if c, ok := cp1252High[r]; ok {
				b.WriteByte(c)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}
