package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"v2.1.0", "2.0.9", 1},
		{"2.0.0", "v2.0.0", 0},
		{"1.12.0", "2.0.0", -1},
	}
	for _, test := range cases {
		got, err := compareVersions(test.left, test.right)
		if err != nil || got != test.want {
			t.Fatalf("compareVersions(%q, %q) = %d, %v; ingin %d", test.left, test.right, got, err, test.want)
		}
	}
	if _, err := compareVersions("latest", "2.0.0"); err == nil {
		t.Fatal("versi non-SemVer diterima")
	}
}

func TestUpdateCheckSkipsDevelopmentBuild(t *testing.T) {
	service := NewUpdateService("dev")
	service.latestReleaseURL = "http://alamat-yang-tidak-boleh-dipanggil.invalid"
	info, err := service.Check(context.Background())
	if err != nil || info.Available {
		t.Fatalf("development build seharusnya melewati cek update: %#v, %v", info, err)
	}
}

func TestUpdateCheckFindsNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if userAgent := request.Header.Get("User-Agent"); userAgent != "Scanify-Update-Checker" {
			t.Errorf("User-Agent = %q", userAgent)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"tag_name":"v2.1.0","name":"Scanify 2.1.0","body":"Perbaikan penting.","assets":[]}`)
	}))
	defer server.Close()

	service := NewUpdateService("2.0.0")
	service.latestReleaseURL = server.URL
	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.CurrentVersion != "2.0.0" || info.LatestVersion != "2.1.0" {
		t.Fatalf("hasil cek update tidak sesuai: %#v", info)
	}
}

func TestDownloadUpdateVerifiesChecksum(t *testing.T) {
	payload := []byte("portable Scanify executable")
	hashBytes := sha256.Sum256(payload)
	expectedHash := hex.EncodeToString(hashBytes[:])
	downloadDirectory := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/release":
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(writer, `{"tag_name":"v2.1.0","name":"Scanify 2.1.0","body":"","assets":[{"name":"Scanify-v2.1.0-windows-amd64.exe","browser_download_url":%q,"size":%d},{"name":"checksums.txt","browser_download_url":%q,"size":100}]}`,
				server.URL+"/scanify.exe", len(payload), server.URL+"/checksums.txt")
		case "/checksums.txt":
			fmt.Fprintf(writer, "%s  Scanify-v2.1.0-windows-amd64.exe\n", expectedHash)
		case "/scanify.exe":
			writer.Write(payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewUpdateService("2.0.0")
	service.latestReleaseURL = server.URL + "/release"
	service.downloadDir = func() (string, error) { return downloadDirectory, nil }
	result, err := service.Download(context.Background(), "2.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) || filepath.Dir(result.Path) != downloadDirectory {
		t.Fatalf("file update tidak sesuai: %q di %q", got, result.Path)
	}
}

func TestDownloadUpdateRejectsInvalidChecksum(t *testing.T) {
	payload := []byte("damaged executable")
	downloadDirectory := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/release" {
			fmt.Fprintf(writer, `{"tag_name":"v2.1.0","assets":[{"name":"Scanify-v2.1.0-windows-amd64.exe","browser_download_url":%q,"size":%d,"digest":"sha256:%s"}]}`,
				server.URL+"/scanify.exe", len(payload), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
			return
		}
		writer.Write(payload)
	}))
	defer server.Close()

	service := NewUpdateService("2.0.0")
	service.latestReleaseURL = server.URL + "/release"
	service.downloadDir = func() (string, error) { return downloadDirectory, nil }
	if _, err := service.Download(context.Background(), "2.1.0"); err == nil {
		t.Fatal("unduhan dengan checksum salah diterima")
	}
	entries, err := os.ReadDir(downloadDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("file gagal seharusnya dibersihkan, ditemukan %d file", len(entries))
	}
}
