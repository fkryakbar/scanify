package main

import (
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/sys/windows"
)

type Exporter struct{}

func (Exporter) ExportJPG(pages []pageRecord, directory string) ([]string, error) {
	stamp := time.Now().Format("20060102_150405")
	paths := make([]string, 0, len(pages))

	for index, page := range pages {
		base := fmt.Sprintf("Scan_%s_%03d", stamp, index+1)
		destination, file, err := createUniqueFile(directory, base, ".jpg")
		if err != nil {
			removeFiles(paths)
			return nil, fmt.Errorf("gagal membuat file JPG: %w", err)
		}

		err = writeJPG(page, file)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(destination)
			removeFiles(paths)
			return nil, fmt.Errorf("gagal mengekspor halaman %d: %w", index+1, err)
		}
		paths = append(paths, destination)
	}
	return paths, nil
}

func writeJPG(page pageRecord, destination *os.File) error {
	if strings.EqualFold(page.Format, "jpeg") || strings.EqualFold(page.Format, "jpg") {
		source, err := os.Open(page.SourcePath)
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(destination, source)
		return err
	}

	img, _, err := decodeImage(page.SourcePath)
	if err != nil {
		return err
	}
	return jpeg.Encode(destination, img, &jpeg.Options{Quality: 90})
}

func (Exporter) ExportPDF(pages []pageRecord, destination, tempDirectory string) error {
	// A portable build must not create pdfcpu's optional configuration folder
	// under the current Windows profile.
	api.DisableConfigDir()

	inputs := make([]string, 0, len(pages))
	generated := make([]string, 0)
	for _, page := range pages {
		if strings.EqualFold(page.Format, "jpeg") || strings.EqualFold(page.Format, "jpg") {
			inputs = append(inputs, page.SourcePath)
			continue
		}
		normalized, err := normalizeToPNG(page.SourcePath, tempDirectory)
		if err != nil {
			removeFiles(generated)
			return fmt.Errorf("gagal menyiapkan gambar untuk PDF: %w", err)
		}
		generated = append(generated, normalized)
		inputs = append(inputs, normalized)
	}
	defer removeFiles(generated)

	stage, err := os.CreateTemp(filepath.Dir(destination), ".scanify-*.pdf")
	if err != nil {
		return err
	}
	stagePath := stage.Name()
	if err := stage.Close(); err != nil {
		os.Remove(stagePath)
		return err
	}
	if err := os.Remove(stagePath); err != nil {
		return err
	}
	defer os.Remove(stagePath)

	config, err := api.Import("formsize:A4, position:c, scalefactor:1.0", types.POINTS)
	if err != nil {
		return err
	}
	if err := api.ImportImagesFile(inputs, stagePath, config, nil); err != nil {
		return fmt.Errorf("gagal membuat PDF: %w", err)
	}
	if err := api.ValidateFile(stagePath, nil); err != nil {
		return fmt.Errorf("PDF hasil ekspor tidak valid: %w", err)
	}
	return replaceFile(stagePath, destination)
}

func createUniqueFile(directory, base, extension string) (string, *os.File, error) {
	for suffix := 0; suffix < 1000; suffix++ {
		name := base + extension
		if suffix > 0 {
			name = fmt.Sprintf("%s_%02d%s", base, suffix, extension)
		}
		path := filepath.Join(directory, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return path, file, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("terlalu banyak nama file yang sama")
}

func removeFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func replaceFile(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
