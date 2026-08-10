package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	arsipinConfigDirectory = "Scanify"
	arsipinConfigFilename  = "arsipin.json"
	arsipinUploadFilename  = "Scanify-upload.pdf"
	arsipinResponseLimit   = 1 << 20
)

type arsipinConfigFile struct {
	UploadURL         string `json:"uploadUrl"`
	WorkspacePassword string `json:"workspacePassword"`
}

type ArsipinService struct {
	configPath string
	client     *http.Client
	mu         sync.Mutex
}

type arsipinAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Archive struct {
			ID string `json:"id"`
		} `json:"archive"`
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	} `json:"data"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func NewArsipinService() (*ArsipinService, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("direktori konfigurasi pengguna tidak dapat ditentukan: %w", err)
	}
	return newArsipinService(filepath.Join(configDir, arsipinConfigDirectory, arsipinConfigFilename), nil), nil
}

func newArsipinService(configPath string, client *http.Client) *ArsipinService {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	return &ArsipinService{configPath: configPath, client: client}
}

func (s *ArsipinService) Config() (ArsipinConfigDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := s.readConfigLocked()
	if err != nil {
		return ArsipinConfigDTO{}, err
	}
	return arsipinConfigDTO(config), nil
}

func (s *ArsipinService) SaveConfig(uploadURL, password string) (ArsipinConfigDTO, error) {
	normalizedURL, err := validateArsipinURL(uploadURL)
	if err != nil {
		return ArsipinConfigDTO{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.readConfigLocked()
	if err != nil {
		return ArsipinConfigDTO{}, err
	}
	password = strings.TrimSpace(password)
	if password == "" {
		if current.WorkspacePassword == "" {
			return ArsipinConfigDTO{}, errors.New("password workspace wajib diisi")
		}
		if normalizedURL != current.UploadURL {
			return ArsipinConfigDTO{}, errors.New("isi password baru jika URL upload diubah")
		}
		password = current.WorkspacePassword
	}

	config := arsipinConfigFile{UploadURL: normalizedURL, WorkspacePassword: password}
	if err := s.writeConfigLocked(config); err != nil {
		return ArsipinConfigDTO{}, err
	}
	return arsipinConfigDTO(config), nil
}

func (s *ArsipinService) configForUpload() (arsipinConfigFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := s.readConfigLocked()
	if err != nil {
		return arsipinConfigFile{}, err
	}
	if config.UploadURL == "" || config.WorkspacePassword == "" {
		return arsipinConfigFile{}, errors.New("konfigurasi Arsipin belum lengkap")
	}
	return config, nil
}

func (s *ArsipinService) Upload(ctx context.Context, uploadURL, password, filePath, filename string) (ArsipinUploadResultDTO, error) {
	if _, err := validateArsipinURL(uploadURL); err != nil {
		return ArsipinUploadResultDTO{}, err
	}
	if strings.TrimSpace(password) == "" {
		return ArsipinUploadResultDTO{}, errors.New("password workspace belum dikonfigurasi")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("PDF upload tidak dapat dibuka: %w", err)
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("PDF upload tidak dapat dibaca: %w", err)
	} else if info.Size() == 0 {
		return ArsipinUploadResultDTO{}, errors.New("PDF upload kosong")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, sanitizeUploadFilename(filename)))
	header.Set("Content-Type", "application/pdf")
	part, err := writer.CreatePart(header)
	if err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("field file tidak dapat dibuat: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("PDF upload tidak dapat disalin: %w", err)
	}
	if err := writer.Close(); err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("multipart upload tidak dapat ditutup: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("request Arsipin tidak valid: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Workspace-Password", password)

	response, err := s.client.Do(request)
	if err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("tidak dapat terhubung ke Arsipin: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, arsipinResponseLimit))
	if err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("response Arsipin tidak dapat dibaca: %w", err)
	}
	var parsed arsipinAPIResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return ArsipinUploadResultDTO{}, fmt.Errorf("response Arsipin tidak valid (HTTP %d)", response.StatusCode)
	}

	result := ArsipinUploadResultDTO{
		Success:    parsed.Success && response.StatusCode == http.StatusAccepted,
		Message:    parsed.Message,
		StatusCode: response.StatusCode,
		ArchiveID:  parsed.Data.Archive.ID,
		JobID:      parsed.Data.Job.ID,
	}
	if parsed.Error != nil {
		result.ErrorCode = parsed.Error.Code
	}
	if result.Message == "" {
		result.Message = http.StatusText(response.StatusCode)
	}
	return result, nil
}

func (s *ArsipinService) readConfigLocked() (arsipinConfigFile, error) {
	contents, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		return arsipinConfigFile{}, nil
	}
	if err != nil {
		return arsipinConfigFile{}, fmt.Errorf("konfigurasi Arsipin tidak dapat dibaca: %w", err)
	}
	var config arsipinConfigFile
	if err := json.Unmarshal(contents, &config); err != nil {
		return arsipinConfigFile{}, fmt.Errorf("file konfigurasi Arsipin tidak valid: %w", err)
	}
	config.UploadURL = strings.TrimSpace(config.UploadURL)
	return config, nil
}

func (s *ArsipinService) writeConfigLocked(config arsipinConfigFile) error {
	directory := filepath.Dir(s.configPath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("direktori konfigurasi Arsipin tidak dapat dibuat: %w", err)
	}

	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("konfigurasi Arsipin tidak dapat diserialisasi: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(directory, ".arsipin-*.tmp")
	if err != nil {
		return fmt.Errorf("file konfigurasi sementara tidak dapat dibuat: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("permission konfigurasi Arsipin tidak dapat diatur: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("konfigurasi Arsipin tidak dapat ditulis: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("file konfigurasi Arsipin tidak dapat ditutup: %w", err)
	}
	if err := replaceFile(temporaryPath, s.configPath); err != nil {
		return fmt.Errorf("konfigurasi Arsipin tidak dapat disimpan: %w", err)
	}
	return nil
}

func arsipinConfigDTO(config arsipinConfigFile) ArsipinConfigDTO {
	return ArsipinConfigDTO{
		UploadURL:          config.UploadURL,
		PasswordConfigured: config.WorkspacePassword != "",
	}
}

func validateArsipinURL(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(normalized)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("URL upload harus berupa endpoint HTTP atau HTTPS yang lengkap")
	}
	return normalized, nil
}

func sanitizeUploadFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return arsipinUploadFilename
	}
	return filename
}
