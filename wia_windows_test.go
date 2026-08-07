//go:build windows

package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWIAHardwareEnumeration(t *testing.T) {
	if os.Getenv("SCANIFY_HARDWARE_TEST") != "1" {
		t.Skip("set SCANIFY_HARDWARE_TEST=1 untuk menguji scanner fisik")
	}
	worker := NewWIAWorker()
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	scanners, err := worker.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(scanners) == 0 {
		t.Fatal("tidak ada scanner WIA yang terdeteksi")
	}
	for _, scanner := range scanners {
		if scanner.ID == "" || scanner.Name == "" {
			t.Fatalf("data scanner tidak lengkap: %#v", scanner)
		}
		t.Logf("scanner terdeteksi: %s (%s)", scanner.Name, scanner.ID)
	}
}

func TestWIAHardwareScan(t *testing.T) {
	if os.Getenv("SCANIFY_SCAN_HARDWARE_TEST") != "1" {
		t.Skip("set SCANIFY_SCAN_HARDWARE_TEST=1 untuk menjalankan satu pemindaian fisik")
	}
	session, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Cleanup()
	worker := NewWIAWorker()
	defer worker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	scanners, err := worker.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(scanners) == 0 {
		t.Fatal("tidak ada scanner WIA yang terdeteksi")
	}
	path, format, warnings, err := worker.Scan(ctx, ScanRequest{
		ScannerID: scanners[0].ID,
		Mode:      modeColor,
		DPI:       300,
	}, session.RootDir())
	if err != nil {
		t.Fatal(err)
	}
	image, decodedFormat, err := decodeImage(path)
	if err != nil {
		t.Fatalf("hasil scan tidak dapat didekode: %v", err)
	}
	if format != "jpeg" || decodedFormat != "jpeg" {
		t.Fatalf("format hasil scan = %q/%q, ingin jpeg/jpeg", format, decodedFormat)
	}
	if image.Bounds().Dx() < 1 || image.Bounds().Dy() < 1 {
		t.Fatalf("dimensi hasil scan tidak valid: %v", image.Bounds())
	}
	t.Logf("scan fisik berhasil: %dx%d px; peringatan=%v", image.Bounds().Dx(), image.Bounds().Dy(), warnings)
}
