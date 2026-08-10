package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArsipinConfigPersistsAndKeepsExistingPassword(t *testing.T) {
	service := newArsipinService(filepath.Join(t.TempDir(), "Scanify", "arsipin.json"), nil)
	url := "https://arsipin.example/api/v1/public-api/workspaces/workspace-1/archives/upload"

	config, err := service.SaveConfig(url, "workspace-secret")
	if err != nil {
		t.Fatal(err)
	}
	if config.UploadURL != url || !config.PasswordConfigured {
		t.Fatalf("konfigurasi awal tidak sesuai: %#v", config)
	}

	contents, err := os.ReadFile(service.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "workspace-secret") {
		t.Fatal("password tidak tersimpan di file konfigurasi")
	}

	saved, err := service.SaveConfig(url, "")
	if err != nil {
		t.Fatal(err)
	}
	if !saved.PasswordConfigured {
		t.Fatal("password lama tidak dipertahankan")
	}

	if _, err := service.SaveConfig(url+"/changed", ""); err == nil {
		t.Fatal("perubahan URL tanpa password baru seharusnya ditolak")
	}

	loaded, err := service.Config()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != config {
		t.Fatalf("konfigurasi yang dimuat berbeda: %#v != %#v", loaded, config)
	}
}

func TestArsipinUploadSendsMultipartPDFAndParsesAcceptedResponse(t *testing.T) {
	const password = "workspace-secret"
	const pdfContents = "%PDF-test-content"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, ingin POST", request.Method)
		}
		if request.Header.Get("X-Workspace-Password") != password {
			t.Errorf("password header = %q", request.Header.Get("X-Workspace-Password"))
		}
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Errorf("content type bukan multipart: %q", request.Header.Get("Content-Type"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart tidak dapat diparse: %v", err)
			return
		}
		fileHeaders := request.MultipartForm.File["file"]
		if len(fileHeaders) != 1 {
			t.Errorf("jumlah field file = %d, ingin 1", len(fileHeaders))
			return
		}
		file, err := fileHeaders[0].Open()
		if err != nil {
			t.Errorf("file multipart tidak dapat dibuka: %v", err)
			return
		}
		defer file.Close()
		contents, err := io.ReadAll(file)
		if err != nil || string(contents) != pdfContents {
			t.Errorf("isi file = %q, ingin %q (err=%v)", contents, pdfContents, err)
		}
		if fileHeaders[0].Filename != "scan.pdf" {
			t.Errorf("nama file = %q, ingin scan.pdf", fileHeaders[0].Filename)
		}

		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusAccepted)
		_, _ = response.Write([]byte(`{"success":true,"message":"Dokumen masuk antrean pemrosesan.","data":{"archive":{"id":"01AR"},"job":{"id":"01JB"}}}`))
	}))
	defer server.Close()

	root := t.TempDir()
	pdfPath := filepath.Join(root, "source.pdf")
	if err := os.WriteFile(pdfPath, []byte(pdfContents), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newArsipinService(filepath.Join(root, "config.json"), server.Client())

	result, err := service.Upload(context.Background(), server.URL+"/upload", password, pdfPath, "scan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.StatusCode != http.StatusAccepted || result.ArchiveID != "01AR" || result.JobID != "01JB" {
		t.Fatalf("hasil upload tidak sesuai: %#v", result)
	}
}

func TestArsipinUploadParsesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"success":false,"message":"Password API publik tidak sesuai.","error":{"code":"PUBLIC_API_UNAUTHORIZED","fields":null}}`))
	}))
	defer server.Close()

	root := t.TempDir()
	pdfPath := filepath.Join(root, "source.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-test-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := newArsipinService(filepath.Join(root, "config.json"), server.Client())

	result, err := service.Upload(context.Background(), server.URL, "wrong-password", pdfPath, "scan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.StatusCode != http.StatusUnauthorized || result.ErrorCode != "PUBLIC_API_UNAUTHORIZED" {
		t.Fatalf("hasil error tidak sesuai: %#v", result)
	}
}

func TestValidateArsipinURL(t *testing.T) {
	valid, err := validateArsipinURL("  https://arsipin.example/upload  ")
	if err != nil || valid != "https://arsipin.example/upload" {
		t.Fatalf("URL valid ditolak atau tidak dinormalisasi: %q, %v", valid, err)
	}

	for _, invalid := range []string{"", "arsipin.example/upload", "ftp://arsipin.example/upload", "https://"} {
		if _, err := validateArsipinURL(invalid); err == nil {
			t.Fatalf("URL tidak valid diterima: %q", invalid)
		}
	}
}

func TestArsipinConfigJSONShape(t *testing.T) {
	config := arsipinConfigFile{UploadURL: "https://example.test/upload", WorkspacePassword: "secret"}
	contents, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["uploadUrl"] == "" || decoded["workspacePassword"] != "secret" {
		t.Fatalf("bentuk JSON konfigurasi tidak sesuai: %s", contents)
	}
}
