package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	headerFill = "4472C4"
	totalFill  = "D9E2F3"
)

// moneyNumFmt adalah format akuntansi Excel: "Rp." menempel di kiri cell dan
// angka menempel di kanan cell (simbol * mengisi ruang di antaranya).
const moneyNumFmt = `_("Rp."* #,##0_);_("Rp."* (#,##0)_);_("Rp."* "-"_)`

func colName(n int) string {
	name, _ := excelize.ColumnNumberToName(n)
	return name
}

func cellAt(col, row int) string {
	return fmt.Sprintf("%s%d", colName(col), row)
}

// titleRowHeight memperkirakan tinggi baris judul (dalam point) agar teks
// panjang yang terbungkus (wrap) tetap tampil utuh. width adalah total lebar
// kolom dalam satuan karakter (satuan lebar kolom Excel).
func titleRowHeight(text string, width float64) float64 {
	perLine := int(width)
	if perLine < 8 {
		perLine = 8
	}
	lines := 1
	for _, seg := range strings.Split(text, "\n") {
		if len(seg) == 0 {
			continue
		}
		n := (len(seg) + perLine - 1) / perLine
		if n > lines {
			lines = n
		}
	}
	h := float64(lines)*20 + 4
	if h < 24 {
		h = 24
	}
	return h
}

func mustStyle(f *excelize.File, s *excelize.Style) (int, error) {
	return f.NewStyle(s)
}

// ExcelBytes menghasilkan file .xlsx dari model laporan.
func ExcelBytes(r Report) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Laporan"
	if _, err := f.NewSheet(sheet); err != nil {
		return nil, err
	}
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}
	idx, _ := f.GetSheetIndex(sheet)
	f.SetActiveSheet(idx)

	if err := f.SetDefaultFont("Arimo"); err != nil {
		return nil, err
	}

	n := len(r.Cols)

	titleStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return nil, err
	}
	headerStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{headerFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	numStyle, err := mustStyle(f, &excelize.Style{
		NumFmt:    3, // #,##0 (tanpa desimal)
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	textStyle, err := mustStyle(f, &excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	centerStyle, err := mustStyle(f, &excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	wrapStyle, err := mustStyle(f, &excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", WrapText: true},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	totalStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{totalFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	totalNumStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		NumFmt:    3, // #,##0 (tanpa desimal)
		Fill:      excelize.Fill{Type: "pattern", Color: []string{totalFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	subtotalStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{totalFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	subtotalNumStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true},
		NumFmt:    3, // #,##0 (tanpa desimal)
		Fill:      excelize.Fill{Type: "pattern", Color: []string{totalFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	moneyFmt := moneyNumFmt
	moneyStyle, err := mustStyle(f, &excelize.Style{
		CustomNumFmt: &moneyFmt,
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	moneyBoldStyle, err := mustStyle(f, &excelize.Style{
		Font:         &excelize.Font{Bold: true},
		CustomNumFmt: &moneyFmt,
		Fill:         excelize.Fill{Type: "pattern", Color: []string{totalFill}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       []excelize.Border{{Type: "thin", Color: "000000", Style: 1}},
	})
	if err != nil {
		return nil, err
	}
	rightStyle, err := mustStyle(f, &excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	boldStyle, err := mustStyle(f, &excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	// Style untuk baris tanpa garis atas/bawah (baris komponen RAB): hanya
	// garis kiri & kanan agar tabel tetap menyambung.
	vBorder := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
	}
	textNoBorderStyle, err := mustStyle(f, &excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    vBorder,
	})
	if err != nil {
		return nil, err
	}
	numNoBorderStyle, err := mustStyle(f, &excelize.Style{
		NumFmt:    3, // #,##0 (tanpa desimal)
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    vBorder,
	})
	if err != nil {
		return nil, err
	}
	moneyNoBorderStyle, err := mustStyle(f, &excelize.Style{
		CustomNumFmt: &moneyFmt,
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       vBorder,
	})
	if err != nil {
		return nil, err
	}

	// Judul (baris 1) dan nama bantuan (baris 2). Tinggi baris dihitung agar
	// teks judul yang panjang ikut membungkus dan tidak terpotong.
	totalEx := 0.0
	for _, c := range r.Cols {
		w := c.ExWidth
		if w <= 0 {
			w = c.Width
		}
		totalEx += w
	}
	if err := f.MergeCell(sheet, cellAt(1, 1), cellAt(n, 1)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, 1), r.Title); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(1, 1), cellAt(n, 1), titleStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(sheet, 1, titleRowHeight(r.Title, totalEx)); err != nil {
		return nil, err
	}
	if err := f.MergeCell(sheet, cellAt(1, 2), cellAt(n, 2)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, 2), r.Subtitle); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(1, 2), cellAt(n, 2), titleStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(sheet, 2, titleRowHeight(r.Subtitle, totalEx)); err != nil {
		return nil, err
	}

	// Header (baris 4).
	for i, c := range r.Cols {
		if err := f.SetCellValue(sheet, cellAt(i+1, 4), c.Header); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheet, cellAt(i+1, 4), cellAt(i+1, 4), headerStyle); err != nil {
			return nil, err
		}
	}
	if err := f.SetRowHeight(sheet, 4, 22); err != nil {
		return nil, err
	}

	// Lebar kolom.
	for i, c := range r.Cols {
		w := c.ExWidth
		if w <= 0 {
			w = c.Width
		}
		if err := f.SetColWidth(sheet, colName(i+1), colName(i+1), w); err != nil {
			return nil, err
		}
	}

	// Data mulai baris 5.
	row := 5
	for ri, dataRow := range r.Rows {
		bold := ri < len(r.RowBold) && r.RowBold[ri]
		noBorder := ri < len(r.RowNoBorder) && r.RowNoBorder[ri]
		for i, cell := range dataRow {
			addr := cellAt(i+1, row)
			col := r.Cols[i]
			if col.Num {
				if n, ok := cell.(int64); ok {
					if err := f.SetCellValue(sheet, addr, float64(n)/100.0); err != nil {
						return nil, err
					}
					st := numStyle
					if bold {
						st = subtotalNumStyle
					}
					if col.Money {
						st = moneyStyle
						if bold {
							st = moneyBoldStyle
						}
					}
					if noBorder {
						if col.Money {
							st = moneyNoBorderStyle
						} else {
							st = numNoBorderStyle
						}
					}
					if err := f.SetCellStyle(sheet, addr, addr, st); err != nil {
						return nil, err
					}
					continue
				}
			}
			if err := f.SetCellValue(sheet, addr, CellText(cell)); err != nil {
				return nil, err
			}
			st := textStyle
			switch {
			case col.Num:
				st = numStyle
			case col.Wrap:
				st = wrapStyle
			case i == 0 && !col.Left:
				st = centerStyle
			}
			if bold {
				if col.Num {
					st = subtotalNumStyle
				} else {
					st = subtotalStyle
				}
			}
			if noBorder {
				st = textNoBorderStyle
			}
			if err := f.SetCellStyle(sheet, addr, addr, st); err != nil {
				return nil, err
			}
		}
		row++
	}

	// Baris TOTAL.
	if r.TotalRow != nil {
		m := r.TotalMerge
		if m < 0 {
			m = 0
		}
		if m > len(r.TotalRow) {
			m = len(r.TotalRow)
		}
		if m > 0 {
			label := ""
			for i := 0; i < m; i++ {
				if s := CellText(r.TotalRow[i]); s != "" {
					label = s
					break
				}
			}
			if err := f.MergeCell(sheet, cellAt(1, row), cellAt(m, row)); err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cellAt(1, row), label); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, cellAt(1, row), cellAt(m, row), totalStyle); err != nil {
				return nil, err
			}
		}
		for i := m; i < len(r.TotalRow); i++ {
			cell := r.TotalRow[i]
			addr := cellAt(i+1, row)
			if r.Cols[i].Num {
				if n, ok := cell.(int64); ok {
					if err := f.SetCellValue(sheet, addr, float64(n)/100.0); err != nil {
						return nil, err
					}
					st := totalNumStyle
					if r.Cols[i].Money {
						st = moneyBoldStyle
					}
					if err := f.SetCellStyle(sheet, addr, addr, st); err != nil {
						return nil, err
					}
					continue
				}
			}
			if err := f.SetCellValue(sheet, addr, CellText(cell)); err != nil {
				return nil, err
			}
			if err := f.SetCellStyle(sheet, addr, addr, totalStyle); err != nil {
				return nil, err
			}
		}
		row++
	}

	// Blok tanda tangan.
	row++ // 1 baris kosong
	mid := n/2 + 1

	// Tanggal cetak (kanan atas).
	if err := f.MergeCell(sheet, cellAt(1, row), cellAt(n, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, row), r.Sig.DateText); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(1, row), cellAt(n, row), rightStyle); err != nil {
		return nil, err
	}
	row++

	// Kepala | Bendahara
	if err := f.MergeCell(sheet, cellAt(1, row), cellAt(mid-1, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, row), r.Sig.HeadLeft); err != nil {
		return nil, err
	}
	if err := f.MergeCell(sheet, cellAt(mid, row), cellAt(n, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(mid, row), r.Sig.HeadRight); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(mid, row), cellAt(n, row), rightStyle); err != nil {
		return nil, err
	}
	row += 3 // 2 baris kosong

	// Nama (tebal)
	if err := f.MergeCell(sheet, cellAt(1, row), cellAt(mid-1, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, row), r.Sig.NameLeft); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(1, row), cellAt(mid-1, row), boldStyle); err != nil {
		return nil, err
	}
	if err := f.MergeCell(sheet, cellAt(mid, row), cellAt(n, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(mid, row), r.Sig.NameRight); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(mid, row), cellAt(n, row), rightStyle); err != nil {
		return nil, err
	}
	row++

	// NIP
	if err := f.MergeCell(sheet, cellAt(1, row), cellAt(mid-1, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, row), r.Sig.NipLeft); err != nil {
		return nil, err
	}
	if err := f.MergeCell(sheet, cellAt(mid, row), cellAt(n, row)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(mid, row), r.Sig.NipRight); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(mid, row), cellAt(n, row), rightStyle); err != nil {
		return nil, err
	}

	// Freeze baris 4 & print title 1:4.
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      4,
		TopLeftCell: cellAt(1, 5),
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, err
	}
	_ = f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Titles",
		RefersTo: fmt.Sprintf("%s!$1:$4", sheet),
		Scope:    sheet,
	})
	orientation := "landscape"
	a4 := 9 // ST_SheetPaperType: 9 = A4
	if err := f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Size:        &a4,
		Orientation: &orientation,
	}); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
