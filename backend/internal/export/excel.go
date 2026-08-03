package export

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

const (
	headerFill = "4472C4"
	totalFill  = "D9E2F3"
)

func colName(n int) string {
	name, _ := excelize.ColumnNumberToName(n)
	return name
}

func cellAt(col, row int) string {
	return fmt.Sprintf("%s%d", colName(col), row)
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

	n := len(r.Cols)

	titleStyle, err := mustStyle(f, &excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
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

	// Judul (baris 1) dan nama bantuan (baris 2).
	if err := f.MergeCell(sheet, cellAt(1, 1), cellAt(n, 1)); err != nil {
		return nil, err
	}
	if err := f.SetCellValue(sheet, cellAt(1, 1), r.Title); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, cellAt(1, 1), cellAt(n, 1), titleStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(sheet, 1, 24); err != nil {
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
	if err := f.SetRowHeight(sheet, 2, 24); err != nil {
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
	for _, dataRow := range r.Rows {
		for i, cell := range dataRow {
			addr := cellAt(i+1, row)
			col := r.Cols[i]
			if col.Num {
				if n, ok := cell.(int64); ok {
					if err := f.SetCellValue(sheet, addr, float64(n)/100.0); err != nil {
						return nil, err
					}
					if err := f.SetCellStyle(sheet, addr, addr, numStyle); err != nil {
						return nil, err
					}
					continue
				}
			}
			if err := f.SetCellValue(sheet, addr, CellText(cell)); err != nil {
				return nil, err
			}
			switch {
			case col.Num:
				if err := f.SetCellStyle(sheet, addr, addr, numStyle); err != nil {
					return nil, err
				}
			case col.Wrap:
				if err := f.SetCellStyle(sheet, addr, addr, wrapStyle); err != nil {
					return nil, err
				}
			case i == 0:
				if err := f.SetCellStyle(sheet, addr, addr, centerStyle); err != nil {
					return nil, err
				}
			default:
				if err := f.SetCellStyle(sheet, addr, addr, textStyle); err != nil {
					return nil, err
				}
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
					if err := f.SetCellStyle(sheet, addr, addr, totalNumStyle); err != nil {
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
