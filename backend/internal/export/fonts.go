package export

import "embed"

//go:embed fonts/*
var fontsFS embed.FS

// pdfFont mengembalikan isi file font serif (Liberation Serif, setara Times
// New Roman) yang diminta ("" untuk regular, "B" untuk bold). nil bila font
// tidak tersedia.
func pdfFont(style string) []byte {
	name := "fonts/LiberationSerif.ttf"
	if style == "B" {
		name = "fonts/LiberationSerif-Bold.ttf"
	}
	b, err := fontsFS.ReadFile(name)
	if err != nil {
		return nil
	}
	return b
}
