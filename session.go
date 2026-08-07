package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type pageRecord struct {
	PageDTO
	SourcePath string
	Format     string
	ScanOrder  int
}

type Session struct {
	mu      sync.RWMutex
	rootDir string
	pages   []pageRecord
	next    int
}

func NewSession() (*Session, error) {
	base := filepath.Join(os.TempDir(), "Scanify")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, err
	}
	cleanupOldSessions(base, 24*time.Hour)

	root, err := os.MkdirTemp(base, "session-")
	if err != nil {
		return nil, err
	}
	return &Session{rootDir: root}, nil
}

func cleanupOldSessions(base string, maxAge time.Duration) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() || !startsWith(entry.Name(), "session-") {
			continue
		}
		info, err := entry.Info()
		if err == nil && now.Sub(info.ModTime()) > maxAge {
			_ = os.RemoveAll(filepath.Join(base, entry.Name()))
		}
	}
}

func startsWith(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}

func (s *Session) RootDir() string {
	return s.rootDir
}

func (s *Session) AddPage(path, format string) (SessionDTO, error) {
	thumbnail, width, height, err := createThumbnail(path)
	if err != nil {
		return SessionDTO{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	s.pages = append(s.pages, pageRecord{
		PageDTO: PageDTO{
			ID:               newID(),
			ThumbnailDataURL: thumbnail,
			Width:            width,
			Height:           height,
		},
		SourcePath: path,
		Format:     format,
		ScanOrder:  s.next,
	})
	return s.snapshotLocked("Halaman berhasil dipindai."), nil
}

func (s *Session) Snapshot(status string) SessionDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked(status)
}

func (s *Session) snapshotLocked(status string) SessionDTO {
	pages := make([]PageDTO, len(s.pages))
	selected := 0
	for i := range s.pages {
		pages[i] = s.pages[i].PageDTO
		if pages[i].Selected {
			selected++
		}
	}
	return SessionDTO{Pages: pages, SelectedCount: selected, Status: status}
}

func (s *Session) SetSelected(id string, selected bool) (SessionDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.indexByID(id)
	if index < 0 {
		return SessionDTO{}, errors.New("halaman tidak ditemukan")
	}
	page := &s.pages[index]
	if page.Selected == selected {
		return s.snapshotLocked("Pilihan halaman diperbarui."), nil
	}

	page.Selected = selected
	if selected {
		page.SelectionOrder = s.maxSelectionOrder() + 1
	} else {
		page.SelectionOrder = 0
		s.compactSelectionOrder()
	}
	return s.snapshotLocked("Pilihan halaman diperbarui."), nil
}

func (s *Session) Delete(id string) (SessionDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.indexByID(id)
	if index < 0 {
		return SessionDTO{}, errors.New("halaman tidak ditemukan")
	}
	path := s.pages[index].SourcePath
	s.pages = append(s.pages[:index], s.pages[index+1:]...)
	s.compactSelectionOrder()
	_ = os.Remove(path)
	return s.snapshotLocked("Halaman dihapus."), nil
}

func (s *Session) SelectedPages() ([]pageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	selected := make([]pageRecord, 0)
	for _, page := range s.pages {
		if page.Selected {
			selected = append(selected, page)
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("pilih minimal satu halaman terlebih dahulu")
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].SelectionOrder < selected[j].SelectionOrder
	})
	return selected, nil
}

func (s *Session) indexByID(id string) int {
	for i := range s.pages {
		if s.pages[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *Session) maxSelectionOrder() int {
	maximum := 0
	for _, page := range s.pages {
		if page.SelectionOrder > maximum {
			maximum = page.SelectionOrder
		}
	}
	return maximum
}

func (s *Session) compactSelectionOrder() {
	indices := make([]int, 0)
	for i := range s.pages {
		if s.pages[i].Selected {
			indices = append(indices, i)
		} else {
			s.pages[i].SelectionOrder = 0
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return s.pages[indices[i]].SelectionOrder < s.pages[indices[j]].SelectionOrder
	})
	for order, index := range indices {
		s.pages[index].SelectionOrder = order + 1
	}
}

func (s *Session) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = os.RemoveAll(s.rootDir)
	s.pages = nil
}

func newID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buffer)
}
