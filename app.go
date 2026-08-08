package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx       context.Context
	scanner   ScannerBackend
	session   *Session
	exporter  Exporter
	updater   *UpdateService
	initErr   error
	operation sync.Mutex
	statusMu  sync.RWMutex
	status    string
}

func NewApp() *App {
	session, err := NewSession()
	app := &App{
		scanner:  NewWIAWorker(),
		session:  session,
		exporter: Exporter{},
		updater:  NewUpdateService(Version),
		initErr:  err,
		status:   "Mencari scanner WIA...",
	}
	if err != nil {
		app.status = "Folder sementara tidak dapat dibuat."
	}
	return app
}

func (a *App) CheckForUpdate() (UpdateInfoDTO, error) {
	if a.updater == nil {
		return UpdateInfoDTO{}, errors.New("layanan pembaruan belum siap")
	}
	return a.updater.Check(a.context())
}

func (a *App) DownloadUpdate(version string) (UpdateDownloadDTO, error) {
	if a.updater == nil {
		return UpdateDownloadDTO{}, errors.New("layanan pembaruan belum siap")
	}
	result, err := a.updater.Download(a.context(), version)
	if err != nil {
		return UpdateDownloadDTO{}, err
	}
	finalPath, err := a.updater.ScheduleRestart(result.Path)
	if err != nil {
		return UpdateDownloadDTO{}, err
	}
	result.Path = finalPath
	// Beri waktu pada Wails mengirim respons ke frontend sebelum proses lama ditutup.
	go func(ctx context.Context) {
		time.Sleep(500 * time.Millisecond)
		runtime.Quit(ctx)
	}(a.context())
	return result, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(context.Context) {
	if a.scanner != nil {
		a.scanner.Close()
	}
	if a.session != nil {
		a.session.Cleanup()
	}
}

func (a *App) GetSession() SessionDTO {
	if a.session == nil {
		return SessionDTO{Pages: []PageDTO{}, Status: a.getStatus()}
	}
	return a.session.Snapshot(a.getStatus())
}

func (a *App) ListScanners() ([]ScannerDTO, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if !a.operation.TryLock() {
		return nil, errors.New("operasi lain sedang berjalan")
	}
	defer a.operation.Unlock()

	a.setStatus("Mencari scanner WIA...")
	scanners, err := a.scanner.List(a.context())
	if err != nil {
		a.setStatus("Scanner WIA tidak dapat dibaca.")
		return nil, fmt.Errorf("gagal membaca scanner: %w", err)
	}
	if len(scanners) == 0 {
		a.setStatus("Tidak ada scanner WIA yang terdeteksi.")
	} else {
		a.setStatus(fmt.Sprintf("%d scanner siap digunakan.", len(scanners)))
	}
	return scanners, nil
}

func (a *App) Scan(request ScanRequest) (ScanResultDTO, error) {
	if err := a.ready(); err != nil {
		return ScanResultDTO{}, err
	}
	if err := validateScanRequest(request); err != nil {
		return ScanResultDTO{}, err
	}
	if !a.operation.TryLock() {
		return ScanResultDTO{}, errors.New("operasi lain sedang berjalan")
	}
	defer a.operation.Unlock()

	a.setStatus("Memindai dokumen...")
	path, format, warnings, err := a.scanner.Scan(a.context(), request, a.session.RootDir())
	if err != nil {
		a.setStatus("Pemindaian gagal.")
		return ScanResultDTO{}, fmt.Errorf("pemindaian gagal: %w", err)
	}
	snapshot, err := a.session.AddPage(path, format)
	if err != nil {
		_ = os.Remove(path)
		a.setStatus("Hasil scan tidak dapat diproses.")
		return ScanResultDTO{}, fmt.Errorf("gagal membuat pratinjau: %w", err)
	}
	if len(warnings) > 0 {
		a.setStatus("Scan selesai dengan peringatan driver.")
		snapshot.Status = a.getStatus()
	} else {
		a.setStatus(fmt.Sprintf("Scan selesai. Total %d halaman.", len(snapshot.Pages)))
		snapshot.Status = a.getStatus()
	}
	return ScanResultDTO{Session: snapshot, Warnings: warnings}, nil
}

func (a *App) SetPageSelected(pageID string, selected bool) (SessionDTO, error) {
	if err := a.ready(); err != nil {
		return SessionDTO{}, err
	}
	if !a.operation.TryLock() {
		return SessionDTO{}, errors.New("tunggu hingga operasi selesai")
	}
	defer a.operation.Unlock()
	snapshot, err := a.session.SetSelected(pageID, selected)
	if err != nil {
		return SessionDTO{}, err
	}
	a.setStatus(fmt.Sprintf("%d halaman dipilih.", snapshot.SelectedCount))
	snapshot.Status = a.getStatus()
	return snapshot, nil
}

func (a *App) DeletePage(pageID string) (SessionDTO, error) {
	if err := a.ready(); err != nil {
		return SessionDTO{}, err
	}
	if !a.operation.TryLock() {
		return SessionDTO{}, errors.New("tunggu hingga operasi selesai")
	}
	defer a.operation.Unlock()
	snapshot, err := a.session.Delete(pageID)
	if err != nil {
		return SessionDTO{}, err
	}
	a.setStatus("Halaman dihapus.")
	snapshot.Status = a.getStatus()
	return snapshot, nil
}

func (a *App) ExportSelected(format string) (ExportResultDTO, error) {
	if err := a.ready(); err != nil {
		return ExportResultDTO{}, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != exportJPG && format != exportPDF {
		return ExportResultDTO{}, errors.New("format ekspor harus JPG atau PDF")
	}
	if !a.operation.TryLock() {
		return ExportResultDTO{}, errors.New("operasi lain sedang berjalan")
	}
	defer a.operation.Unlock()

	pages, err := a.session.SelectedPages()
	if err != nil {
		return ExportResultDTO{}, err
	}
	if format == exportJPG {
		directory, err := runtime.OpenDirectoryDialog(a.context(), runtime.OpenDialogOptions{
			Title:                "Pilih folder penyimpanan JPG",
			CanCreateDirectories: true,
		})
		if err != nil {
			return ExportResultDTO{}, fmt.Errorf("dialog folder gagal dibuka: %w", err)
		}
		if directory == "" {
			return ExportResultDTO{Cancelled: true, Format: format, Paths: []string{}}, nil
		}
		a.setStatus("Mengekspor halaman sebagai JPG...")
		paths, err := a.exporter.ExportJPG(pages, directory)
		if err != nil {
			a.setStatus("Ekspor JPG gagal.")
			return ExportResultDTO{}, err
		}
		a.setStatus(fmt.Sprintf("%d file JPG berhasil disimpan.", len(paths)))
		return ExportResultDTO{Format: format, Paths: paths}, nil
	}

	defaultName := "Scan_" + time.Now().Format("20060102_150405") + ".pdf"
	destination, err := runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{
		Title:                "Simpan PDF hasil scan",
		DefaultFilename:      defaultName,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{{
			DisplayName: "Dokumen PDF (*.pdf)",
			Pattern:     "*.pdf",
		}},
	})
	if err != nil {
		return ExportResultDTO{}, fmt.Errorf("dialog simpan gagal dibuka: %w", err)
	}
	if destination == "" {
		return ExportResultDTO{Cancelled: true, Format: format, Paths: []string{}}, nil
	}
	if !strings.EqualFold(filepath.Ext(destination), ".pdf") {
		destination += ".pdf"
	}
	a.setStatus("Menyusun PDF...")
	if err := a.exporter.ExportPDF(pages, destination, a.session.RootDir()); err != nil {
		a.setStatus("Ekspor PDF gagal.")
		return ExportResultDTO{}, err
	}
	a.setStatus("PDF berhasil disimpan.")
	return ExportResultDTO{Format: format, Paths: []string{destination}}, nil
}

func validateScanRequest(request ScanRequest) error {
	if strings.TrimSpace(request.ScannerID) == "" {
		return errors.New("pilih scanner terlebih dahulu")
	}
	if request.Mode != modeColor && request.Mode != modeGrayscale && request.Mode != modeBlackWhite {
		return errors.New("mode warna tidak valid")
	}
	if request.DPI != 150 && request.DPI != 300 && request.DPI != 600 {
		return errors.New("resolusi harus 150, 300, atau 600 DPI")
	}
	return nil
}

func (a *App) ready() error {
	if a.initErr != nil {
		return fmt.Errorf("aplikasi gagal diinisialisasi: %w", a.initErr)
	}
	if a.session == nil || a.scanner == nil {
		return errors.New("layanan aplikasi belum siap")
	}
	return nil
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) setStatus(status string) {
	a.statusMu.Lock()
	a.status = status
	a.statusMu.Unlock()
}

func (a *App) getStatus() string {
	a.statusMu.RLock()
	defer a.statusMu.RUnlock()
	return a.status
}
