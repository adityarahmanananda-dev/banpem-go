package export

import "embed"

//go:embed fonts/*
var fontsFS embed.FS

// pdfFont mengembalikan isi file font DejaVu yang diminta ("" untuk regular,
// "B" untuk bold). nil bila font tidak tersedia.
func pdfFont(style string) []byte {
	name := "fonts/DejaVuSans.ttf"
	if style == "B" {
		name = "fonts/DejaVuSans-Bold.ttf"
	}
	b, err := fontsFS.ReadFile(name)
	if err != nil {
		return nil
	}
	return b
}
