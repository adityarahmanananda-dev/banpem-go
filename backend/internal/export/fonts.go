package export

import "embed"

//go:embed fonts/*
//go:embed Arimo/static/*
var fontsFS embed.FS

// pdfFont mengembalikan isi file font Arimo (reguler / bold). Arimo adalah
// font sans-serif (mirip Arial). nil bila font tidak tersedia.
func pdfFont(style string) []byte {
	name := "Arimo/static/Arimo-Regular.ttf"
	if style == "B" {
		name = "Arimo/static/Arimo-Bold.ttf"
	}
	b, err := fontsFS.ReadFile(name)
	if err != nil {
		return nil
	}
	return b
}
