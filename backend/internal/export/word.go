package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

const twipsPerMM = 56.6929

const (
	wordMarginTwips = 567 // 1 cm
	wordHeaderFill  = "4472C4"
	wordTotalFill   = "D9E2F3"
)

// WordBytes menghasilkan file .docx dari model laporan. landscape menentukan
// orientasi A4 (landscape = lebar 277 mm, portrait = 190 mm).
func WordBytes(rep Report, landscape bool) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := wordZipFiles(rep, landscape)
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

func wordZipFiles(rep Report, landscape bool) map[string]string {
	doc := wordDocumentXML(rep, landscape)
	return map[string]string{
		"[Content_Types].xml":          wordContentTypes(),
		"_rels/.rels":                  wordRootRels(),
		"word/document.xml":            doc,
		"word/styles.xml":              wordStyles(),
		"word/_rels/document.xml.rels": wordDocRels(),
		"docProps/core.xml":            wordCoreProps(rep.Title),
		"docProps/app.xml":             wordAppProps(),
	}
}

func wordContentTypes() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`
}

func wordRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`
}

func wordDocRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
}

func wordStyles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults>
<w:rPrDefault><w:rPr>
<w:rFonts w:ascii="Arimo" w:hAnsi="Arimo" w:eastAsia="Arimo" w:cs="Arimo"/>
<w:sz w:val="20"/><w:szCs w:val="20"/>
</w:rPr></w:rPrDefault>
<w:pPrDefault><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault>
</w:docDefaults>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
</w:styles>`
}

func wordCoreProps(title string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<dc:title>%s</dc:title>
<dc:creator>Banpem-GO</dc:creator>
<cp:lastModifiedBy>Banpem-GO</cp:lastModifiedBy>
</cp:coreProperties>`, escXML(title))
}

func wordAppProps() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
<Application>Banpem-GO</Application>
</Properties>`
}

func escXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// wordDocumentXML menyusun isi word/document.xml.
func wordDocumentXML(rep Report, landscape bool) string {
	pageW := 11906
	pageH := 16838
	usableMM := 190.0
	orient := ""
	if landscape {
		pageW, pageH = 16838, 11906
		usableMM = 277.0
		orient = ` w:orient="landscape"`
	}

	widths := wordColWidths(rep.Cols, usableMM)
	twips := make([]int, len(widths))
	total := 0
	for i, w := range widths {
		twips[i] = int(w * twipsPerMM)
		total += twips[i]
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")
	b.WriteString("<w:body>")

	// Judul & nama bantuan.
	b.WriteString(wp(strings.ToUpper(rep.Title), "center", true, 28, 120, true))
	b.WriteString(wp(rep.Subtitle, "center", true, 24, 120, true))
	b.WriteString(wp("", "", false, 0, 0, false))

	// Tabel.
	b.WriteString("<w:tbl><w:tblPr>")
	b.WriteString(fmt.Sprintf(`<w:tblW w:w="%d" w:type="dxa"/>`, total))
	b.WriteString(`<w:tblLayout w:type="fixed"/>`)
	b.WriteString(`<w:tblBorders>`)
	for _, e := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		b.WriteString(fmt.Sprintf(`<w:%s w:val="single" w:sz="4" w:space="0" w:color="000000"/>`, e))
	}
	b.WriteString(`</w:tblBorders></w:tblPr>`)
	b.WriteString("<w:tblGrid>")
	for _, t := range twips {
		b.WriteString(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, t))
	}
	b.WriteString("</w:tblGrid>")

	// Baris-baris header: berjenjang (HeaderRows) atau grup + kolom (ColGroups).
	if len(rep.HeaderRows) > 0 {
		b.WriteString(wordHeaderRows(rep, twips))
	} else if len(rep.ColGroups) > 0 {
		cover := map[int]ColGroup{}
		for _, g := range rep.ColGroups {
			cover[g.Start] = g
		}
		b.WriteString("<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>")
		i := 0
		for i < len(rep.Cols) {
			if g, ok := cover[i]; ok {
				span := g.Span
				tw := 0
				for j := i; j < i+span && j < len(rep.Cols); j++ {
					tw += twips[j]
				}
				b.WriteString(wcell(g.Header, cellOpts{
					wTwips: tw, align: "center", bold: true,
					fill: wordHeaderFill, color: "FFFFFF", gridSpan: span,
				}))
				i += span
			} else {
				b.WriteString(wcell(rep.Cols[i].Header, cellOpts{
					wTwips: twips[i], align: "center", bold: true,
					fill: wordHeaderFill, color: "FFFFFF", vMerge: "restart",
				}))
				i++
			}
		}
		b.WriteString("</w:tr>")

		// Header kolom (berulang).
		b.WriteString("<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>")
		for i, c := range rep.Cols {
			if _, ok := groupAt(rep.ColGroups, i); !ok {
				// lanjutan gabung vertikal dari baris grup.
				b.WriteString(wcell("", cellOpts{wTwips: twips[i], vMerge: "continue"}))
				continue
			}
			b.WriteString(wcell(c.Header, cellOpts{
				wTwips: twips[i], align: "center", bold: true,
				fill: wordHeaderFill, color: "FFFFFF",
			}))
		}
		b.WriteString("</w:tr>")
	} else {
		// Header kolom (berulang), tanpa grup.
		b.WriteString("<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>")
		for i, c := range rep.Cols {
			b.WriteString(wcell(c.Header, cellOpts{
				wTwips: twips[i], align: "center", bold: true,
				fill: wordHeaderFill, color: "FFFFFF",
			}))
		}
		b.WriteString("</w:tr>")
	}

	// Baris data.
	for i, row := range rep.Rows {
		b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
		bold := i < len(rep.RowBold) && rep.RowBold[i]
		noBorder := i < len(rep.RowNoBorder) && rep.RowNoBorder[i]
		for j, cell := range row {
			if rep.Cols[j].Money {
				if _, ok := cell.(int64); ok {
					b.WriteString(wmoney(CellText(cell), cellOpts{wTwips: twips[j], bold: bold, noBorder: noBorder}))
					continue
				}
			}
			align := "left"
			if rep.Cols[j].Num {
				align = "right"
			}
			if j == 0 && !rep.Cols[j].Num && !rep.Cols[j].Left {
				align = "center"
			}
			if rich, ok := cell.(Rich); ok {
				b.WriteString(wcellRich(rich, cellOpts{wTwips: twips[j], align: align, bold: bold, noBorder: noBorder}))
				continue
			}
			b.WriteString(wcell(CellText(cell), cellOpts{wTwips: twips[j], align: align, bold: bold, noBorder: noBorder}))
		}
		b.WriteString("</w:tr>")
	}

	// Baris TOTAL.
	if rep.TotalRow != nil {
		b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
		m := rep.TotalMerge
		if m < 0 {
			m = 0
		}
		if m > len(rep.TotalRow) {
			m = len(rep.TotalRow)
		}
		if m > 0 {
			merged := 0
			for i := 0; i < m; i++ {
				merged += twips[i]
			}
			label := ""
			for i := 0; i < m; i++ {
				if s := CellText(rep.TotalRow[i]); s != "" {
					label = s
					break
				}
			}
			b.WriteString(wcell(label, cellOpts{
				wTwips: merged, align: "center", bold: true,
				fill: wordTotalFill, gridSpan: m, keepNext: true,
			}))
		}
		for i := m; i < len(rep.TotalRow); i++ {
			if rep.Cols[i].Money {
				if _, ok := rep.TotalRow[i].(int64); ok {
					b.WriteString(wmoney(CellText(rep.TotalRow[i]), cellOpts{
						wTwips: twips[i], bold: true, fill: wordTotalFill, keepNext: true,
					}))
					continue
				}
			}
			align := "left"
			if rep.Cols[i].Num {
				align = "right"
			}
			if i == 0 && !rep.Cols[i].Num {
				align = "center"
			}
			b.WriteString(wcell(CellText(rep.TotalRow[i]), cellOpts{
				wTwips: twips[i], align: align, bold: true,
				fill: wordTotalFill, keepNext: true,
			}))
		}
		b.WriteString("</w:tr>")
	}

	b.WriteString("</w:tbl>")

	// Satu paragraf kosong pemisah: memastikan blok tanda tangan tidak menyatu
	// dengan tabel (jadi border hanya sampai baris TOTAL) sekaligus memberi
	// jarak 1 baris kosong sebelum baris tanggal cetak.
	b.WriteString(wp("", "", false, 0, 0, false))

	// Blok tanda tangan dibuat dalam tabel terpisah TANPA border agar kolom
	// tanda tangan tidak ikut bergaris.
	b.WriteString(wordSignatureTable(rep.Sig, twips))

	// Penutup halaman.
	b.WriteString(`<w:sectPr>`)
	b.WriteString(fmt.Sprintf(`<w:pgSz w:w="%d" w:h="%d"%s/>`, pageW, pageH, orient))
	b.WriteString(fmt.Sprintf(`<w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="720" w:footer="720" w:gutter="0"/>`,
		wordMarginTwips, wordMarginTwips, wordMarginTwips, wordMarginTwips))
	b.WriteString(`<w:cols w:space="708"/><w:docGrid w:linePitch="360"/>`)
	b.WriteString(`</w:sectPr>`)

	b.WriteString("</w:body></w:document>")
	return b.String()
}

func wordColWidths(cols []Col, usableMM float64) []float64 {
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
	if total > usableMM {
		over := total - usableMM
		if flx > 0 && over <= flx {
			f := (flx - over) / flx
			for i, c := range cols {
				if c.Flex {
					ws[i] *= f
				}
			}
		} else {
			f := usableMM / total
			for i := range ws {
				ws[i] *= f
			}
		}
	}
	return ws
}

// wordHeaderRows menulis baris-baris header berjenjang ke XML Word. Sel
// Rowspan=0 menjadi vMerge continue (lanjutan sel dari baris di atas).
func wordHeaderRows(rep Report, twips []int) string {
	var b strings.Builder
	for _, row := range rep.HeaderRows {
		b.WriteString("<w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr>")
		col := 0
		for _, cell := range row {
			cs := cell.Colspan
			if cs < 1 {
				cs = 1
			}
			tw := 0
			for j := col; j < col+cs && j < len(twips); j++ {
				tw += twips[j]
			}
			switch {
			case cell.Rowspan < 0:
				b.WriteString(wcell("", cellOpts{wTwips: tw, gridSpan: cs, vMerge: "continue"}))
			case cell.Rowspan > 1:
				b.WriteString(wcell(cell.Text, cellOpts{
					wTwips: tw, align: "center", bold: true,
					fill: wordHeaderFill, color: "FFFFFF", gridSpan: cs, keepNext: true, vMerge: "restart",
				}))
			default:
				b.WriteString(wcell(cell.Text, cellOpts{
					wTwips: tw, align: "center", bold: true,
					fill: wordHeaderFill, color: "FFFFFF", gridSpan: cs, keepNext: true,
				}))
			}
			col += cs
		}
		b.WriteString("</w:tr>")
	}
	return b.String()
}

// wordSignatureTable menyusun blok tanda tangan sebagai tabel terpisah tanpa
// border (lebar mengikuti kolom tabel laporan).
func wordSignatureTable(sig SigData, twips []int) string {
	n := len(twips)
	total := 0
	for _, t := range twips {
		total += t
	}
	k := 1
	acc := 0
	for i, t := range twips {
		acc += t
		k = i + 1
		if acc >= total/2 {
			break
		}
	}
	if k >= n {
		k = n - 1
	}
	if k < 1 {
		k = 1
	}
	leftTw := 0
	for i := 0; i < k; i++ {
		leftTw += twips[i]
	}

	var b strings.Builder
	b.WriteString("<w:tbl><w:tblPr>")
	b.WriteString(fmt.Sprintf(`<w:tblW w:w="%d" w:type="dxa"/>`, total))
	b.WriteString(`<w:tblLayout w:type="fixed"/>`)
	b.WriteString(`<w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="0" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar>`)
	b.WriteString("</w:tblPr>")
	b.WriteString("<w:tblGrid>")
	for _, t := range twips {
		b.WriteString(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, t))
	}
	b.WriteString("</w:tblGrid>")

	// Tanggal cetak (kanan, di atas kolom Bendahara).
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(sig.DateText, cellOpts{
		wTwips: total, align: "right", gridSpan: n, keepNext: true,
	}))
	b.WriteString("</w:tr>")
	// Kepala | Bendahara (langsung di bawah baris tanggal, tanpa spasi).
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(sig.HeadLeft, cellOpts{wTwips: leftTw, align: "left", gridSpan: k, keepNext: true}))
	b.WriteString(wcell(sig.HeadRight, cellOpts{wTwips: total - leftTw, align: "right", gridSpan: n - k, keepNext: true}))
	b.WriteString("</w:tr>")
	// Baris kosong x1.
	b.WriteString(`<w:tr><w:trPr><w:cantSplit/><w:trHeight w:val="340" w:hRule="exact"/></w:trPr>`)
	b.WriteString(wcell("", cellOpts{wTwips: total, gridSpan: n, keepNext: true}))
	b.WriteString("</w:tr>")
	// Nama (tebal) | Nama (tebal).
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(sig.NameLeft, cellOpts{wTwips: leftTw, align: "left", gridSpan: k, bold: true, keepNext: true}))
	b.WriteString(wcell(sig.NameRight, cellOpts{wTwips: total - leftTw, align: "right", gridSpan: n - k, bold: true, keepNext: true}))
	b.WriteString("</w:tr>")
	// NIP | NIP.
	b.WriteString("<w:tr><w:trPr><w:cantSplit/></w:trPr>")
	b.WriteString(wcell(sig.NipLeft, cellOpts{wTwips: leftTw, align: "left", gridSpan: k}))
	b.WriteString(wcell(sig.NipRight, cellOpts{wTwips: total - leftTw, align: "right", gridSpan: n - k}))
	b.WriteString("</w:tr>")

	b.WriteString("</w:tbl>")
	return b.String()
}

type cellOpts struct {
	wTwips   int
	align    string
	bold     bool
	fill     string
	color    string
	gridSpan int
	keepNext bool
	noBorder bool
	vMerge   string // "" | "restart" | "continue"
	sz       int    // ukuran font dalam half-points; 0 = 18 (9pt)
}

func wcell(text string, o cellOpts) string {
	var b strings.Builder
	b.WriteString("<w:tc>")
	b.WriteString("<w:tcPr>")
	b.WriteString(fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, o.wTwips))
	if o.gridSpan > 1 {
		b.WriteString(fmt.Sprintf(`<w:gridSpan w:val="%d"/>`, o.gridSpan))
	}
	if o.vMerge != "" {
		b.WriteString(fmt.Sprintf(`<w:vMerge w:val="%s"/>`, o.vMerge))
	}
	if o.noBorder {
		b.WriteString(`<w:tcBorders><w:top w:val="nil"/><w:bottom w:val="nil"/></w:tcBorders>`)
	}
	if o.fill != "" {
		b.WriteString(fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, o.fill))
	}
	b.WriteString("<w:vAlign w:val=\"center\"/>")
	b.WriteString("</w:tcPr>")

	b.WriteString("<w:p><w:pPr>")
	if o.keepNext {
		b.WriteString("<w:keepNext/>")
	}
	if o.align != "" {
		b.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, o.align))
	}
	b.WriteString("</w:pPr>")

	if text != "" {
		b.WriteString("<w:r><w:rPr>")
		if o.bold {
			b.WriteString("<w:b/>")
		}
		if o.color != "" {
			b.WriteString(fmt.Sprintf(`<w:color w:val="%s"/>`, o.color))
		}
		sz := o.sz
		if sz == 0 {
			sz = 18
		}
		b.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, sz, sz))
		b.WriteString("</w:rPr>")
		b.WriteString(`<w:t xml:space="preserve">`)
		for i, line := range strings.Split(text, "\n") {
			if i > 0 {
				b.WriteString("<w:br/>")
			}
			b.WriteString(escXML(line))
		}
		b.WriteString("</w:t></w:r>")
	}
	b.WriteString("</w:p></w:tc>")
	return b.String()
}

// wcellRich menyusun cell dengan segmen teks berbeda gaya (label tebal, nilai
// normal), dengan baris baru antar segmen.
func wcellRich(rich Rich, o cellOpts) string {
	var b strings.Builder
	b.WriteString("<w:tc>")
	b.WriteString("<w:tcPr>")
	b.WriteString(fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, o.wTwips))
	if o.noBorder {
		b.WriteString(`<w:tcBorders><w:top w:val="nil"/><w:bottom w:val="nil"/></w:tcBorders>`)
	}
	b.WriteString("<w:vAlign w:val=\"center\"/>")
	b.WriteString("</w:tcPr>")
	b.WriteString("<w:p><w:pPr>")
	if o.keepNext {
		b.WriteString("<w:keepNext/>")
	}
	if o.align != "" {
		b.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, o.align))
	}
	b.WriteString("</w:pPr>")

	for _, seg := range rich.Segments {
		parts := strings.Split(seg.Text, "\n")
		for i, part := range parts {
			if i > 0 {
				b.WriteString("<w:r><w:br/></w:r>")
			}
			if part == "" {
				continue
			}
			b.WriteString("<w:r><w:rPr>")
			if o.bold || seg.Bold {
				b.WriteString("<w:b/>")
			}
			b.WriteString("<w:sz w:val=\"18\"/><w:szCs w:val=\"18\"/>")
			b.WriteString("</w:rPr>")
			b.WriteString(`<w:t xml:space="preserve">` + escXML(part) + `</w:t></w:r>`)
		}
	}
	b.WriteString("</w:p></w:tc>")
	return b.String()
}

// wmoney menyusun cell uang gaya pembukuan: "Rp." menempel di kiri cell dan
// angka menempel di kanan cell lewat tab stop rata kanan selebar cell. Nilai 0
// ditampilkan "Rp." di kiri dan "-" di kanan (konvensi pembukuan).
func wmoney(amount string, o cellOpts) string {
	if amount == "0" {
		amount = "-"
	}
	var b strings.Builder
	b.WriteString("<w:tc>")
	b.WriteString("<w:tcPr>")
	b.WriteString(fmt.Sprintf(`<w:tcW w:w="%d" w:type="dxa"/>`, o.wTwips))
	if o.noBorder {
		b.WriteString(`<w:tcBorders><w:top w:val="nil"/><w:bottom w:val="nil"/></w:tcBorders>`)
	}
	if o.fill != "" {
		b.WriteString(fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, o.fill))
	}
	b.WriteString("<w:vAlign w:val=\"center\"/>")
	b.WriteString("</w:tcPr>")

	b.WriteString("<w:p><w:pPr>")
	if o.keepNext {
		b.WriteString("<w:keepNext/>")
	}
	b.WriteString(fmt.Sprintf(`<w:tabs><w:tab w:val="right" w:pos="%d"/></w:tabs>`, o.wTwips))
	b.WriteString("</w:pPr>")

	run := func(t string) {
		b.WriteString("<w:r><w:rPr>")
		if o.bold {
			b.WriteString("<w:b/>")
		}
		b.WriteString("<w:sz w:val=\"18\"/><w:szCs w:val=\"18\"/>")
		b.WriteString("</w:rPr>")
		b.WriteString(`<w:t xml:space="preserve">` + escXML(t) + `</w:t></w:r>`)
	}
	run("Rp.")
	b.WriteString("<w:r><w:tab/></w:r>")
	run(amount)

	b.WriteString("</w:p></w:tc>")
	return b.String()
}

// wp menyusun paragraf bebas (judul, dll).
func wp(text, jc string, bold bool, sz, after int, keepNext bool) string {
	var b strings.Builder
	b.WriteString("<w:p><w:pPr>")
	if keepNext {
		b.WriteString("<w:keepNext/>")
	}
	if jc != "" {
		b.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, jc))
	}
	if after > 0 {
		b.WriteString(fmt.Sprintf(`<w:spacing w:after="%d"/>`, after))
	}
	b.WriteString("</w:pPr>")
	if text != "" {
		b.WriteString("<w:r><w:rPr>")
		if bold {
			b.WriteString("<w:b/>")
		}
		if sz > 0 {
			b.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, sz, sz))
		}
		b.WriteString("</w:rPr>")
		b.WriteString(`<w:t xml:space="preserve">` + escXML(text) + `</w:t></w:r>`)
	}
	b.WriteString("</w:p>")
	return b.String()
}
