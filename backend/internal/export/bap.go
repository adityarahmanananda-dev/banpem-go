package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// bapSize adalah ukuran font (pt) seluruh isi Berita Acara Pemeriksaan Kas.
const bapSize = 12.0

// BAPKasData adalah data Berita Acara Pemeriksaan Kas.
type BAPKasData struct {
	DateText       string
	ExaminerName   string
	ExaminerTitle  string
	ExaminerNIP    string
	BendaharaName  string
	BendaharaTitle string
	BendaharaNIP   string
	UangKertas     int64
	UangLogam      int64
	PajakBelum     int64
	SaldoBank      int64
	Total          int64
	SaldoBuku      int64
	Perbedaan      int64
}

// bapMoneyText memformat nilai sen menjadi teks laporan (0 -> "-").
func bapMoneyText(c int64) string {
	if c == 0 {
		return "-"
	}
	return FmtAmount(c)
}

// ---------------------------------------------------------------------------
// PDF

// BAPPdf menghasilkan file .pdf A4 portrait Berita Acara Pemeriksaan Kas.
func BAPPdf(d BAPKasData) ([]byte, error) {
	r, err := newPDF(false)
	if err != nil {
		return nil, err
	}
	// A4, margin 20 mm sesuai template.
	r.p.SetMargins(20, 20, 20)
	r.usable = 210.0 - 40.0
	if err := r.renderBAP(d); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := r.p.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// bapDateLines memecah kalimat tanggal menjadi dua baris: "bertandatangan di
// bawah" di baris 1 dan "ini" di baris 2 (persis template).
func bapDateLines(t string) (string, string) {
	if i := strings.LastIndex(t, " ini"); i > 0 {
		return t[:i], t[i+1:]
	}
	return t, ""
}

func (r *pdfRender) renderBAP(d BAPKasData) error {
	p := r.p
	p.AddPage()
	p.SetTextColor(0, 0, 0)
	usable := r.usable
	// Posisi FIX terhadap kertas (A4, margin 20 mm) persis template:
	// label identitas ~26,4mm; nilai identitas ~67,6mm (tab 2700 twips);
	// deskripsi uang ~32,7mm (indent 720 twips); nilai uang ~140mm (tab 6800).
	labelX := 20.0 + 6.4
	valueX := 20.0 + 47.6
	descX := 20.0 + 12.7
	moneyX := 20.0 + 120.0

	r.font("B", bapSize)
	p.CellFormat(0, lineH(bapSize), "BERITA ACARA PEMERIKSAAN KAS", "", 2, "C", false, 0, "")
	p.Ln(6)

	r.font("", bapSize)
	l1, l2 := bapDateLines(d.DateText)
	p.CellFormat(0, lineH(bapSize), l1, "", 2, "L", false, 0, "")
	if l2 != "" {
		p.CellFormat(0, lineH(bapSize), l2, "", 2, "L", false, 0, "")
	}
	p.Ln(2)

	identity := func(label, value string) {
		r.font("", bapSize)
		lh := lineH(bapSize)
		p.SetXY(labelX, p.GetY())
		p.CellFormat(valueX-labelX, lh, label, "", 0, "L", false, 0, "")
		p.SetXY(valueX, p.GetY())
		p.CellFormat(usable-valueX, lh, ": "+value, "", 2, "L", false, 0, "")
	}
	identity("Nama", d.ExaminerName)
	identity("Jabatan", d.ExaminerTitle)
	p.Ln(2)
	p.CellFormat(0, lineH(bapSize), "Selaku "+d.ExaminerTitle+" sebagai atasan langsung Bendahara", "", 2, "L", false, 0, "")
	p.CellFormat(0, lineH(bapSize), "telah melakukan pemeriksaan setempat kepada", "", 2, "L", false, 0, "")
	p.Ln(2)
	identity("Nama", d.BendaharaName)
	identity("Jabatan", d.BendaharaTitle)
	p.Ln(2)
	p.CellFormat(0, lineH(bapSize), "Berdasarkan hasil pemeriksaan bukti-bukti yang berada dalam pengawasan itu,", "", 2, "L", false, 0, "")
	p.CellFormat(0, lineH(bapSize), "kami menemukan kenyataan sebagai berikut:", "", 2, "L", false, 0, "")
	p.Ln(2)

	moneyLine := func(num, desc string, amount int64, plus bool) {
		r.font("", bapSize)
		lh := lineH(bapSize)
		y := p.GetY()
		text := desc
		if num != "" {
			text = num + ". " + desc
		}
		p.SetXY(descX, y)
		p.CellFormat(moneyX-descX, lh, text, "", 0, "L", false, 0, "")
		// Nilai uang "Rp [jumlah]" rata kiri di kolom uang (tab 6800 twips).
		amt := bapMoneyText(amount)
		val := "Rp " + amt
		if plus {
			val += " +"
		}
		p.SetXY(moneyX, y)
		p.CellFormat(usable-moneyX, lh, val, "", 0, "L", false, 0, "")
		// Garis bawah hanya di bawah nilai uang Saldo pada Bank.
		if plus {
			vw := r.p.GetStringWidth(val)
			p.Line(moneyX, y+lh-0.4, moneyX+vw, y+lh-0.4)
		}
		p.SetY(y + lh)
	}
	moneyLine("1", "Uang kertas lembaran sejumlah", d.UangKertas, false)
	moneyLine("2", "Uang logam sejumlah", d.UangLogam, false)
	moneyLine("3", "Pajak yang belum disetor sejumlah", d.PajakBelum, false)
	moneyLine("4", "Saldo pada Bank sejumlah", d.SaldoBank, true)
	moneyLine("5", "Total", d.Total, false)
	moneyLine("6", "Saldo uang menurut buku kas umum", d.SaldoBuku, false)
	p.Ln(1)
	moneyLine("", "Perbedaan antara KAS dan BUKU sejumlah", d.Perbedaan, false)

	r.drawBAPSig(d.ExaminerTitle, d.ExaminerName, "NIP "+d.ExaminerNIP,
		d.BendaharaTitle, d.BendaharaName, "NIP "+d.BendaharaNIP)
	return nil
}

// drawBAPSig menggambar blok tanda tangan BAP (Kepala | Bendahara).
func (r *pdfRender) drawBAPSig(leftTitle, leftName, leftNip, rightTitle, rightName, rightNip string) {
	p := r.p
	sigH := 6.0 + 18 + 6 + 6
	if p.GetY()+sigH > r.pageH-pdfBot {
		p.AddPage()
	}
	half := r.usable / 2
	mid := pdfLeft + half
	r.font("", bapSize)
	p.Ln(10)
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, leftTitle, "", 0, "L", false, 0, "")
	p.SetX(mid)
	p.CellFormat(half, 6, rightTitle, "", 2, "R", false, 0, "")
	p.Ln(18)
	r.font("B", bapSize)
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, leftName, "", 0, "L", false, 0, "")
	p.SetX(mid)
	p.CellFormat(half, 6, rightName, "", 2, "R", false, 0, "")
	r.font("", bapSize)
	p.SetX(pdfLeft)
	p.CellFormat(half, 6, leftNip, "", 0, "L", false, 0, "")
	p.SetX(mid)
	p.CellFormat(half, 6, rightNip, "", 2, "R", false, 0, "")
}

// ---------------------------------------------------------------------------
// Word

// BAPWordBytes menghasilkan file .docx Berita Acara Pemeriksaan Kas.
func BAPWordBytes(d BAPKasData) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml":          wordContentTypes(),
		"_rels/.rels":                  wordRootRels(),
		"word/document.xml":            bapWordDocumentXML(d),
		"word/styles.xml":              wordStyles(),
		"word/_rels/document.xml.rels": wordDocRels(),
		"docProps/core.xml":            wordCoreProps("Berita Acara Pemeriksaan Kas"),
		"docProps/app.xml":             wordAppProps(),
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// bapWordDocumentXML menyusun isi word/document.xml BAP Kas.
func bapWordDocumentXML(d BAPKasData) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")
	b.WriteString("<w:body>")

	// Judul.
	b.WriteString(wp("BERITA ACARA PEMERIKSAAN KAS", "center", true, 24, 240, true))
	b.WriteString(wp("", "", false, 0, 0, false))
	// Paragraf tanggal (dua baris: "bertandatangan di bawah" / "ini").
	l1, l2 := bapDateLines(d.DateText)
	b.WriteString(bapP2(l1, l2, "left"))
	b.WriteString(wp("", "", false, 0, 0, false))

	// Identitas pemeriksa (atasan langsung = Kepala).
	b.WriteString(bapIdentity("Nama", d.ExaminerName))
	b.WriteString(bapIdentity("Jabatan", d.ExaminerTitle))
	b.WriteString(wp("", "", false, 0, 0, false))
	b.WriteString(bapP2("Selaku "+d.ExaminerTitle+" sebagai atasan langsung Bendahara", "telah melakukan pemeriksaan setempat kepada", "left"))
	b.WriteString(wp("", "", false, 0, 0, false))
	b.WriteString(bapIdentity("Nama", d.BendaharaName))
	b.WriteString(bapIdentity("Jabatan", d.BendaharaTitle))
	b.WriteString(wp("", "", false, 0, 0, false))
	b.WriteString(bapP2("Berdasarkan hasil pemeriksaan bukti-bukti yang berada dalam pengawasan itu,", "kami menemukan kenyataan sebagai berikut:", "left"))
	b.WriteString(wp("", "", false, 0, 0, false))

	// Butir temuan.
	val := func(c int64) string {
		t := bapMoneyText(c)
		return "Rp " + t
	}
	b.WriteString(bapMoneyLine("1", "Uang kertas lembaran sejumlah", val(d.UangKertas), false))
	b.WriteString(bapMoneyLine("2", "Uang logam sejumlah", val(d.UangLogam), false))
	b.WriteString(bapMoneyLine("3", "Pajak yang belum disetor sejumlah", val(d.PajakBelum), false))
	b.WriteString(bapMoneyLine("4", "Saldo pada Bank sejumlah", val(d.SaldoBank)+" +", true))
	b.WriteString(bapMoneyLine("5", "Total", val(d.Total), false))
	b.WriteString(bapMoneyLine("6", "Saldo uang menurut buku kas umum", val(d.SaldoBuku), false))
	b.WriteString(wp("", "", false, 0, 0, false))
	b.WriteString(bapMoneyLine("", "Perbedaan antara KAS dan BUKU sejumlah", val(d.Perbedaan), false))

	b.WriteString(bapSignatureTable(d))
	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1134" w:right="1134" w:bottom="1134" w:left="1134" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:space="708"/><w:docGrid w:linePitch="360"/></w:sectPr>`)
	b.WriteString("</w:body></w:document>")
	return b.String()
}

// bapP menyusun paragraf 12pt dengan alignment dan tab stop opsional.
// tabs berisi urutan "<w:tab .../>" yang diletakkan sebelum jc.
func bapP(text, tabs, jc, indent string) string {
	var b strings.Builder
	b.WriteString("<w:p><w:pPr>")
	if tabs != "" {
		b.WriteString(tabs)
	}
	if jc != "" {
		b.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, jc))
	}
	if indent != "" {
		b.WriteString(indent)
	}
	b.WriteString("</w:pPr>")
	if text != "" {
		b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(text)))
	}
	b.WriteString("</w:p>")
	return b.String()
}

// bapP2 menyusun paragraf 12pt dua baris yang dipisah <w:br/> (baris tetap,
// tidak ikut membungkus).
func bapP2(l1, l2, jc string) string {
	var b strings.Builder
	b.WriteString("<w:p><w:pPr>")
	if jc != "" {
		b.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, jc))
	}
	b.WriteString("</w:pPr>")
	b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(l1)))
	if l2 != "" {
		b.WriteString("<w:r><w:br/></w:r>")
		b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(l2)))
	}
	b.WriteString("</w:p>")
	return b.String()
}

// bapIdentity menyusun baris "Label [tab] : nilai".
func bapIdentity(label, value string) string {
	var b strings.Builder
	b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Normal"/><w:tabs><w:tab w:val="clear" w:pos="709"/><w:tab w:val="left" w:pos="2700" w:leader="none"/></w:tabs><w:spacing w:before="0" w:after="0"/><w:ind w:left="360"/></w:pPr>`)
	b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(label)))
	b.WriteString("<w:r><w:tab/></w:r>")
	b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">: %s</w:t></w:r>`, escXML(value)))
	b.WriteString("</w:p>")
	return b.String()
}

// bapMoneyLine menyusun baris uang persis template: deskripsi (indent hanging
// 720/360) lalu tab ke posisi uang (6800 twips) dan nilai "Rp ...". Bila
// underline true (Saldo pada Bank), nilai digarisbawahi.
func bapMoneyLine(num, desc, amount string, underline bool) string {
	var b strings.Builder
	indent := `<w:ind w:hanging="360" w:left="720"/>`
	b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Normal"/><w:tabs><w:tab w:val="clear" w:pos="709"/><w:tab w:val="left" w:pos="6800" w:leader="none"/></w:tabs><w:spacing w:before="0" w:after="0"/>`)
	b.WriteString(indent)
	b.WriteString(`</w:pPr>`)
	text := desc
	if num != "" {
		text = num + ". " + desc
	}
	b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(text)))
	b.WriteString("<w:r><w:tab/></w:r>")
	if underline {
		b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:u w:val="single"/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(amount)))
	} else {
		b.WriteString(fmt.Sprintf(`<w:r><w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r>`, escXML(amount)))
	}
	b.WriteString("</w:p>")
	return b.String()
}

// bapSignatureTable menyusun blok tanda tangan BAP sebagai tabel tanpa border.
func bapSignatureTable(d BAPKasData) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="9228" w:type="dxa"/><w:tblLayout w:type="fixed"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="0" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr>`)
	b.WriteString(`<w:tblGrid><w:gridCol w:w="4614"/><w:gridCol w:w="4614"/></w:tblGrid>`)
	// Baris kosong pemisah.
	b.WriteString(`<w:tr><w:trPr><w:cantSplit/><w:trHeight w:val="360" w:hRule="exact"/></w:trPr>`)
	b.WriteString(wcell("", cellOpts{wTwips: 9228, gridSpan: 2}))
	b.WriteString("</w:tr>")
	// Kepala | Bendahara.
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(d.ExaminerTitle, cellOpts{wTwips: 4614, align: "left", keepNext: true, sz: 24}))
	b.WriteString(wcell(d.BendaharaTitle, cellOpts{wTwips: 4614, align: "right", keepNext: true, sz: 24}))
	b.WriteString("</w:tr>")
	// Spasi tanda tangan.
	b.WriteString(`<w:tr><w:trPr><w:cantSplit/><w:trHeight w:val="1020" w:hRule="exact"/></w:trPr>`)
	b.WriteString(wcell("", cellOpts{wTwips: 4614, keepNext: true}))
	b.WriteString(wcell("", cellOpts{wTwips: 4614, keepNext: true}))
	b.WriteString("</w:tr>")
	// Nama (tebal).
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(d.ExaminerName, cellOpts{wTwips: 4614, align: "left", bold: true, keepNext: true, sz: 24}))
	b.WriteString(wcell(d.BendaharaName, cellOpts{wTwips: 4614, align: "right", bold: true, keepNext: true, sz: 24}))
	b.WriteString("</w:tr>")
	// NIP.
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell("NIP "+d.ExaminerNIP, cellOpts{wTwips: 4614, align: "left", sz: 24}))
	b.WriteString(wcell("NIP "+d.BendaharaNIP, cellOpts{wTwips: 4614, align: "right", sz: 24}))
	b.WriteString("</w:tr>")
	b.WriteString("</w:tbl>")
	return b.String()
}
