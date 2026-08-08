package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseAPI = "https://api.github.com/repos/fkryakbar/scanify/releases/latest"
	maxReleaseJSON   = 2 << 20
	maxChecksumFile  = 1 << 20
	maxUpdateSize    = 512 << 20
)

var strictVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Body    string         `json:"body"`
	Assets  []releaseAsset `json:"assets"`
}

type UpdateService struct {
	currentVersion    string
	latestReleaseURL  string
	client            *http.Client
	downloadDir       func() (string, error)
	currentExecutable func() (string, error)
	scheduleRestart   func(oldPath, updatePath string) error
}

func NewUpdateService(currentVersion string) *UpdateService {
	return &UpdateService{
		currentVersion:   currentVersion,
		latestReleaseURL: latestReleaseAPI,
		client: &http.Client{
			Timeout: 30 * time.Minute,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("terlalu banyak pengalihan saat mengunduh")
				}
				if request.URL.Scheme != "https" {
					return errors.New("pengalihan unduhan harus menggunakan HTTPS")
				}
				return nil
			},
		},
		downloadDir:       defaultExecutableDirectory,
		currentExecutable: currentExecutablePath,
		scheduleRestart:   scheduleWindowsUpdate,
	}
}

func (u *UpdateService) Check(ctx context.Context) (UpdateInfoDTO, error) {
	current := strings.TrimSpace(u.currentVersion)
	result := UpdateInfoDTO{CurrentVersion: displayVersion(current)}
	if current == "" || strings.EqualFold(current, "dev") {
		return result, nil
	}

	checkContext, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	release, err := u.latestRelease(checkContext)
	if err != nil {
		return UpdateInfoDTO{}, err
	}
	comparison, err := compareVersions(release.TagName, current)
	if err != nil {
		return UpdateInfoDTO{}, fmt.Errorf("versi rilis GitHub tidak valid: %w", err)
	}
	result.Available = comparison > 0
	result.LatestVersion = displayVersion(release.TagName)
	result.ReleaseName = strings.TrimSpace(release.Name)
	result.ReleaseNotes = truncateRunes(strings.TrimSpace(release.Body), 2000)
	return result, nil
}

func (u *UpdateService) Download(ctx context.Context, requestedVersion string) (UpdateDownloadDTO, error) {
	requestedVersion = displayVersion(requestedVersion)
	if _, err := parseVersion(requestedVersion); err != nil {
		return UpdateDownloadDTO{}, errors.New("versi pembaruan tidak valid")
	}

	release, err := u.latestRelease(ctx)
	if err != nil {
		return UpdateDownloadDTO{}, err
	}
	latestVersion := displayVersion(release.TagName)
	if latestVersion != requestedVersion {
		return UpdateDownloadDTO{}, fmt.Errorf("rilis terbaru berubah menjadi %s; periksa pembaruan kembali", latestVersion)
	}
	if comparison, compareErr := compareVersions(release.TagName, u.currentVersion); compareErr != nil || comparison <= 0 {
		return UpdateDownloadDTO{}, errors.New("versi tersebut bukan pembaruan untuk aplikasi ini")
	}

	executableName := "Scanify-" + release.TagName + "-windows-amd64.exe"
	executable, ok := findReleaseAsset(release.Assets, executableName)
	if !ok {
		return UpdateDownloadDTO{}, fmt.Errorf("asset %s tidak ditemukan pada GitHub Release", executableName)
	}
	if executable.Size <= 0 || executable.Size > maxUpdateSize {
		return UpdateDownloadDTO{}, errors.New("ukuran asset pembaruan tidak valid")
	}
	expectedHash, err := u.expectedHash(ctx, release, executable)
	if err != nil {
		return UpdateDownloadDTO{}, err
	}

	oldPath, err := u.currentExecutable()
	if err != nil {
		return UpdateDownloadDTO{}, fmt.Errorf("lokasi binary saat ini tidak dapat ditemukan: %w", err)
	}
	directory, err := u.downloadDir()
	if err != nil {
		return UpdateDownloadDTO{}, fmt.Errorf("folder binary tidak dapat ditemukan: %w", err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return UpdateDownloadDTO{}, fmt.Errorf("folder binary tidak dapat dibuat: %w", err)
	}
	destination := temporaryUpdatePath(directory, filepath.Base(oldPath))
	if err := u.downloadVerified(ctx, executable, expectedHash, destination); err != nil {
		return UpdateDownloadDTO{}, err
	}
	return UpdateDownloadDTO{Version: latestVersion, Path: destination}, nil
}

func (u *UpdateService) ScheduleRestart(updatePath string) (string, error) {
	if u.scheduleRestart == nil {
		return "", errors.New("mekanisme restart pembaruan belum siap")
	}
	oldPath, err := u.currentExecutable()
	if err != nil {
		return "", fmt.Errorf("lokasi binary saat ini tidak dapat ditemukan: %w", err)
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(oldPath)), filepath.Clean(filepath.Dir(updatePath))) {
		return "", errors.New("pembaruan harus berada di folder binary yang sama")
	}
	if err := u.scheduleRestart(oldPath, updatePath); err != nil {
		return "", err
	}
	return oldPath, nil
}

func (u *UpdateService) latestRelease(ctx context.Context) (githubRelease, error) {
	response, err := u.get(ctx, u.latestReleaseURL)
	if err != nil {
		return githubRelease{}, fmt.Errorf("GitHub Release tidak dapat diperiksa: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub Release merespons dengan status %d", response.StatusCode)
	}
	var release githubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReleaseJSON))
	if err := decoder.Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("respons GitHub Release tidak valid: %w", err)
	}
	if _, err := parseVersion(release.TagName); err != nil {
		return githubRelease{}, fmt.Errorf("tag rilis %q tidak valid", release.TagName)
	}
	return release, nil
}

func (u *UpdateService) expectedHash(ctx context.Context, release githubRelease, executable releaseAsset) (string, error) {
	if digest := strings.TrimSpace(executable.Digest); strings.HasPrefix(digest, "sha256:") {
		hash := strings.TrimPrefix(digest, "sha256:")
		if validSHA256(hash) {
			return strings.ToLower(hash), nil
		}
	}
	checksumAsset, ok := findReleaseAsset(release.Assets, "checksums.txt")
	if !ok {
		return "", errors.New("checksum SHA-256 pembaruan tidak tersedia")
	}
	response, err := u.get(ctx, checksumAsset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("checksum pembaruan tidak dapat diunduh: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum pembaruan merespons dengan status %d", response.StatusCode)
	}

	scanner := bufio.NewScanner(io.LimitReader(response.Body, maxChecksumFile))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == executable.Name && validSHA256(fields[0]) {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("checksum pembaruan tidak dapat dibaca: %w", err)
	}
	return "", fmt.Errorf("checksum untuk %s tidak ditemukan", executable.Name)
}

func (u *UpdateService) downloadVerified(ctx context.Context, asset releaseAsset, expectedHash, destination string) error {
	response, err := u.get(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("pembaruan tidak dapat diunduh: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unduhan pembaruan merespons dengan status %d", response.StatusCode)
	}

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".scanify-update-*.tmp")
	if err != nil {
		return fmt.Errorf("file sementara pembaruan tidak dapat dibuat: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hasher), io.LimitReader(response.Body, maxUpdateSize+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("unduhan pembaruan terputus: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("file pembaruan tidak dapat disimpan: %w", closeErr)
	}
	if written > maxUpdateSize || written != asset.Size {
		return errors.New("ukuran pembaruan tidak sesuai metadata GitHub")
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return errors.New("verifikasi SHA-256 pembaruan gagal; file dibatalkan")
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("pembaruan tidak dapat dipindahkan ke folder binary: %w", err)
	}
	return nil
}

func (u *UpdateService) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "Scanify-Update-Checker")
	return u.client.Do(request)
}

func findReleaseAsset(assets []releaseAsset, name string) (releaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	return filepath.Abs(path)
}

func defaultExecutableDirectory() (string, error) {
	path, err := currentExecutablePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func temporaryUpdatePath(directory, executableName string) string {
	base := strings.TrimSuffix(executableName, filepath.Ext(executableName))
	return filepath.Join(directory, fmt.Sprintf(".%s-update-%d.exe", base, time.Now().UnixNano()))
}

func scheduleWindowsUpdate(oldPath, updatePath string) error {
	if filepath.Ext(oldPath) == "" || filepath.Ext(updatePath) == "" {
		return errors.New("path binary pembaruan tidak valid")
	}
	scriptFile, err := os.CreateTemp(filepath.Dir(oldPath), ".scanify-updater-*.ps1")
	if err != nil {
		return fmt.Errorf("skrip updater tidak dapat dibuat: %w", err)
	}
	scriptPath := scriptFile.Name()
	defer scriptFile.Close()
	if _, err := scriptFile.WriteString(selfUpdateScript); err != nil {
		return fmt.Errorf("skrip updater tidak dapat ditulis: %w", err)
	}
	if err := scriptFile.Close(); err != nil {
		return fmt.Errorf("skrip updater tidak dapat disimpan: %w", err)
	}

	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", scriptPath, oldPath, updatePath, strconv.Itoa(os.Getpid()), scriptPath)
	if err := command.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("proses updater tidak dapat dimulai: %w", err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("proses updater tidak dapat dilepas: %w", err)
	}
	return nil
}

const selfUpdateScript = `param(
    [Parameter(Mandatory=$true)][string]$OldPath,
    [Parameter(Mandatory=$true)][string]$UpdatePath,
    [Parameter(Mandatory=$true)][int]$ProcessId,
    [Parameter(Mandatory=$true)][string]$ScriptPath
)

$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    try {
        Get-Process -Id $ProcessId -ErrorAction Stop | Out-Null
        Start-Sleep -Milliseconds 200
    } catch {
        break
    }
}

$backupPath = "$OldPath.scanify-backup-$ProcessId"
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    try {
        if (Test-Path -LiteralPath $backupPath) {
            Remove-Item -LiteralPath $backupPath -Force -ErrorAction Stop
        }
        Move-Item -LiteralPath $OldPath -Destination $backupPath -Force -ErrorAction Stop
        Move-Item -LiteralPath $UpdatePath -Destination $OldPath -Force -ErrorAction Stop
        Start-Process -FilePath $OldPath -ErrorAction Stop
        Remove-Item -LiteralPath $backupPath -Force -ErrorAction Stop
        break
    } catch {
        if ((Test-Path -LiteralPath $backupPath) -and (Test-Path -LiteralPath $OldPath)) {
            Remove-Item -LiteralPath $OldPath -Force -ErrorAction SilentlyContinue
        }
        if (Test-Path -LiteralPath $backupPath) {
            Move-Item -LiteralPath $backupPath -Destination $OldPath -Force -ErrorAction SilentlyContinue
        }
        Start-Sleep -Milliseconds 500
    }
}

Remove-Item -LiteralPath $ScriptPath -Force -ErrorAction SilentlyContinue
`

func compareVersions(left, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	for index := range leftParts {
		if leftParts[index] > rightParts[index] {
			return 1, nil
		}
		if leftParts[index] < rightParts[index] {
			return -1, nil
		}
	}
	return 0, nil
}

func parseVersion(version string) ([3]uint64, error) {
	match := strictVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return [3]uint64{}, fmt.Errorf("format versi %q harus MAJOR.MINOR.PATCH", version)
	}
	var result [3]uint64
	for index := range result {
		value, err := strconv.ParseUint(match[index+1], 10, 64)
		if err != nil {
			return [3]uint64{}, err
		}
		result[index] = value
	}
	return result, nil
}

func displayVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum]) + "…"
}
