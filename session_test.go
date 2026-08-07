package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/bmp"
)

func writeTestJPEG(t *testing.T, path string, width, height int, fill color.Color) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionCompactsSelectionOrderAfterDeselectAndDelete(t *testing.T) {
	session, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Cleanup()

	var ids []string
	var paths []string
	for i, fill := range []color.Color{color.RGBA{R: 220, A: 255}, color.RGBA{G: 220, A: 255}, color.RGBA{B: 220, A: 255}} {
		path := filepath.Join(session.RootDir(), "page-"+string(rune('1'+i))+".jpg")
		writeTestJPEG(t, path, 80+i, 120+i, fill)
		snapshot, addErr := session.AddPage(path, "jpeg")
		if addErr != nil {
			t.Fatal(addErr)
		}
		ids = append(ids, snapshot.Pages[len(snapshot.Pages)-1].ID)
		paths = append(paths, path)
	}

	for _, id := range []string{ids[2], ids[0], ids[1]} {
		if _, err := session.SetSelected(id, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := session.SetSelected(ids[0], false); err != nil {
		t.Fatal(err)
	}

	selected, err := session.SelectedPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != ids[2] || selected[0].SelectionOrder != 1 || selected[1].ID != ids[1] || selected[1].SelectionOrder != 2 {
		t.Fatalf("urutan setelah batal pilih salah: %#v", selected)
	}

	snapshot, err := session.Delete(ids[2])
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SelectedCount != 1 {
		t.Fatalf("selectedCount = %d, ingin 1", snapshot.SelectedCount)
	}
	if _, err := os.Stat(paths[2]); !os.IsNotExist(err) {
		t.Fatalf("file halaman yang dihapus masih ada: %v", err)
	}
	selected, err = session.SelectedPages()
	if err != nil {
		t.Fatal(err)
	}
	if selected[0].ID != ids[1] || selected[0].SelectionOrder != 1 {
		t.Fatalf("urutan setelah hapus tidak dipadatkan: %#v", selected)
	}
}

func TestFitSizePreservesBounds(t *testing.T) {
	width, height := fitSize(2400, 3600, 240, 240)
	if width != 160 || height != 240 {
		t.Fatalf("fitSize = %dx%d, ingin 160x240", width, height)
	}
}

func TestDecodeImageAcceptsBMPDriverFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "driver-output.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 48))
	if err := bmp.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	decoded, format, err := decodeImage(path)
	if err != nil {
		t.Fatal(err)
	}
	if format != "bmp" || decoded.Bounds().Dx() != 32 || decoded.Bounds().Dy() != 48 {
		t.Fatalf("hasil decode fallback salah: format=%q bounds=%v", format, decoded.Bounds())
	}
}
