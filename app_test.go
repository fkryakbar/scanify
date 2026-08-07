package main

import "testing"

func TestValidateScanRequest(t *testing.T) {
	valid := ScanRequest{ScannerID: "scanner-1", Mode: modeColor, DPI: 300}
	if err := validateScanRequest(valid); err != nil {
		t.Fatalf("permintaan valid ditolak: %v", err)
	}

	invalid := []ScanRequest{
		{Mode: modeColor, DPI: 300},
		{ScannerID: "scanner-1", Mode: "sepia", DPI: 300},
		{ScannerID: "scanner-1", Mode: modeColor, DPI: 1200},
	}
	for _, request := range invalid {
		if err := validateScanRequest(request); err == nil {
			t.Fatalf("permintaan tidak valid diterima: %#v", request)
		}
	}
}

func TestNewIDIsNonEmptyAndDistinct(t *testing.T) {
	first, second := newID(), newID()
	if first == "" || second == "" || first == second {
		t.Fatalf("ID tidak valid: %q, %q", first, second)
	}
}

func TestApplicationTitleIncludesReleaseVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "dev"
	if title := applicationTitle(); title != "Scanify" {
		t.Fatalf("judul development = %q", title)
	}
	Version = "2.3.4"
	if title := applicationTitle(); title != "Scanify 2.3.4" {
		t.Fatalf("judul release = %q", title)
	}
}
