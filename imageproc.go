package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
)

func decodeImage(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		signature := make([]byte, 16)
		read, _ := f.ReadAt(signature, 0)
		return nil, "", fmt.Errorf("format gambar tidak didukung (signature %s): %w", hex.EncodeToString(signature[:read]), err)
	}
	return img, format, nil
}

func detectImageFormat(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, format, err := image.DecodeConfig(f)
	if err != nil {
		signature := make([]byte, 16)
		read, _ := f.ReadAt(signature, 0)
		return "", fmt.Errorf("hasil driver bukan gambar yang valid (signature %s): %w", hex.EncodeToString(signature[:read]), err)
	}
	return format, nil
}

func createThumbnail(path string) (string, int, int, error) {
	img, _, err := decodeImage(path)
	if err != nil {
		return "", 0, 0, err
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	thumbWidth, thumbHeight := fitSize(width, height, 240, 240)
	thumb := image.NewRGBA(image.Rect(0, 0, thumbWidth, thumbHeight))
	draw.CatmullRom.Scale(thumb, thumb.Bounds(), img, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 82}); err != nil {
		return "", 0, 0, err
	}

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), width, height, nil
}

func fitSize(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= 0 || height <= 0 {
		return 1, 1
	}
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	ratioW := float64(maxWidth) / float64(width)
	ratioH := float64(maxHeight) / float64(height)
	ratio := ratioW
	if ratioH < ratio {
		ratio = ratioH
	}
	return max(1, int(float64(width)*ratio)), max(1, int(float64(height)*ratio))
}

func reencodeJPEG(path string, quality int) error {
	img, _, err := decodeImage(path)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".scanify-jpeg-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: quality}); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpName, path)
}

func normalizeToPNG(sourcePath, directory string) (string, error) {
	img, _, err := decodeImage(sourcePath)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp(directory, "scanify-pdf-*.png")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := png.Encode(f, img); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
