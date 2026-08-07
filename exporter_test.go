package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 90, 140))
	for y := 0; y < 140; y++ {
		for x := 0; x < 90; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y), B: 80, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExporterWritesRealJPGAndValidPDF(t *testing.T) {
	root := t.TempDir()
	jpegPath := filepath.Join(root, "first.jpg")
	pngPath := filepath.Join(root, "second.png")
	writeTestJPEG(t, jpegPath, 100, 150, color.RGBA{R: 20, G: 140, B: 70, A: 255})
	writeTestPNG(t, pngPath)
	pages := []pageRecord{
		{PageDTO: PageDTO{ID: "first", SelectionOrder: 1}, SourcePath: jpegPath, Format: "jpeg"},
		{PageDTO: PageDTO{ID: "second", SelectionOrder: 2}, SourcePath: pngPath, Format: "png"},
	}

	exporter := Exporter{}
	jpgPaths, err := exporter.ExportJPG(pages, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(jpgPaths) != 2 {
		t.Fatalf("jumlah JPG = %d, ingin 2", len(jpgPaths))
	}
	for _, path := range jpgPaths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(contents) < 2 || contents[0] != 0xff || contents[1] != 0xd8 {
			t.Fatalf("%s bukan berkas JPEG", path)
		}
	}

	secondExport, err := exporter.ExportJPG(pages[:1], root)
	if err != nil {
		t.Fatal(err)
	}
	if secondExport[0] == jpgPaths[0] {
		t.Fatal("ekspor JPG menimpa file yang sudah ada")
	}

	pdfPath := filepath.Join(root, "result.pdf")
	if err := exporter.ExportPDF(pages, pdfPath, root); err != nil {
		t.Fatal(err)
	}
	pageCount, err := api.PageCountFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if pageCount != 2 {
		t.Fatalf("jumlah halaman PDF = %d, ingin 2", pageCount)
	}
}
