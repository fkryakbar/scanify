//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	wiaDeviceTypeScanner = 1
	wiaCurrentIntent     = 6146
	wiaHorizontalDPI     = 6147
	wiaVerticalDPI       = 6148
	wiaFormatJPEG        = "{B96B3CAE-0728-11D3-9D7B-0000F81EF32E}"
	wiaFormatTIFF        = "{B96B3CB1-0728-11D3-9D7B-0000F81EF32E}"
)

type ScannerBackend interface {
	List(context.Context) ([]ScannerDTO, error)
	Scan(context.Context, ScanRequest, string) (string, string, []string, error)
	Close()
}

type wiaJob struct {
	run    func() (any, error)
	result chan wiaJobResult
	stop   bool
}

type wiaJobResult struct {
	value any
	err   error
}

type WIAWorker struct {
	jobs      chan wiaJob
	closed    chan struct{}
	closeOnce sync.Once
	initErr   error
}

func NewWIAWorker() *WIAWorker {
	worker := &WIAWorker{
		jobs:   make(chan wiaJob),
		closed: make(chan struct{}),
	}
	ready := make(chan error, 1)
	go worker.loop(ready)
	worker.initErr = <-ready
	return worker
}

func (w *WIAWorker) loop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.closed)

	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	ready <- err
	if err == nil {
		defer ole.CoUninitialize()
	}

	for job := range w.jobs {
		if job.stop {
			return
		}
		if err != nil {
			job.result <- wiaJobResult{err: err}
			continue
		}
		value, jobErr := job.run()
		job.result <- wiaJobResult{value: value, err: jobErr}
	}
}

func (w *WIAWorker) execute(ctx context.Context, run func() (any, error)) (any, error) {
	if w.initErr != nil {
		return nil, fmt.Errorf("WIA tidak dapat diinisialisasi: %w", w.initErr)
	}
	job := wiaJob{run: run, result: make(chan wiaJobResult, 1)}
	select {
	case w.jobs <- job:
	case <-w.closed:
		return nil, errors.New("layanan scanner sudah dihentikan")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case result := <-job.result:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *WIAWorker) Close() {
	w.closeOnce.Do(func() {
		select {
		case w.jobs <- wiaJob{stop: true}:
			<-w.closed
		case <-w.closed:
		}
	})
}

func (w *WIAWorker) List(ctx context.Context) ([]ScannerDTO, error) {
	value, err := w.execute(ctx, func() (any, error) {
		return listWIAScanners()
	})
	if err != nil {
		return nil, err
	}
	return value.([]ScannerDTO), nil
}

func (w *WIAWorker) Scan(ctx context.Context, request ScanRequest, directory string) (string, string, []string, error) {
	value, err := w.execute(ctx, func() (any, error) {
		return scanWIA(request, directory)
	})
	if err != nil {
		return "", "", nil, err
	}
	result := value.(wiaScanResult)
	return result.path, result.format, result.warnings, nil
}

type wiaScanResult struct {
	path     string
	format   string
	warnings []string
}

func createWIAManager() (*ole.IDispatch, func(), error) {
	unknown, err := oleutil.CreateObject("WIA.DeviceManager")
	if err != nil {
		return nil, nil, err
	}
	dispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	unknown.Release()
	if err != nil {
		return nil, nil, err
	}
	return dispatch, func() { dispatch.Release() }, nil
}

func listWIAScanners() ([]ScannerDTO, error) {
	manager, release, err := createWIAManager()
	if err != nil {
		return nil, fmt.Errorf("WIA DeviceManager tidak tersedia: %w", err)
	}
	defer release()

	infosVariant, err := oleutil.GetProperty(manager, "DeviceInfos")
	if err != nil {
		return nil, err
	}
	defer infosVariant.Clear()
	infos := infosVariant.ToIDispatch()
	count, err := dispatchInt(infos, "Count")
	if err != nil {
		return nil, err
	}

	scanners := make([]ScannerDTO, 0)
	for index := 1; index <= count; index++ {
		infoVariant, err := oleutil.GetProperty(infos, "Item", index)
		if err != nil {
			continue
		}
		info := infoVariant.ToIDispatch()
		deviceType, typeErr := dispatchInt(info, "Type")
		if typeErr == nil && deviceType == wiaDeviceTypeScanner {
			id, _ := dispatchString(info, "DeviceID")
			name, _ := wiaPropertyString(info, "Name", "Device Name")
			if id != "" {
				if name == "" {
					name = "Scanner WIA"
				}
				scanners = append(scanners, ScannerDTO{ID: id, Name: name})
			}
		}
		infoVariant.Clear()
	}
	return scanners, nil
}

func scanWIA(request ScanRequest, directory string) (wiaScanResult, error) {
	manager, release, err := createWIAManager()
	if err != nil {
		return wiaScanResult{}, err
	}
	defer release()

	deviceVariant, err := connectWIADevice(manager, request.ScannerID)
	if err != nil {
		return wiaScanResult{}, err
	}
	defer deviceVariant.Clear()
	device := deviceVariant.ToIDispatch()

	itemsVariant, err := oleutil.GetProperty(device, "Items")
	if err != nil {
		return wiaScanResult{}, fmt.Errorf("scanner tidak menyediakan sumber gambar: %w", err)
	}
	defer itemsVariant.Clear()
	items := itemsVariant.ToIDispatch()
	count, err := dispatchInt(items, "Count")
	if err != nil || count < 1 {
		return wiaScanResult{}, errors.New("scanner tidak memiliki flatbed yang dapat dipindai")
	}

	itemVariant, err := oleutil.GetProperty(items, "Item", 1)
	if err != nil {
		return wiaScanResult{}, err
	}
	defer itemVariant.Clear()
	item := itemVariant.ToIDispatch()

	warnings := make([]string, 0)
	intent := map[string]int{modeColor: 1, modeGrayscale: 2, modeBlackWhite: 4}[request.Mode]
	for _, setting := range []struct {
		id    int
		value int
		label string
	}{
		{wiaHorizontalDPI, request.DPI, "resolusi horizontal"},
		{wiaVerticalDPI, request.DPI, "resolusi vertikal"},
		{wiaCurrentIntent, intent, "mode warna"},
	} {
		if warning := setWIAProperty(item, setting.id, setting.value, setting.label); warning != "" {
			warnings = append(warnings, warning)
		}
	}

	formatID, extension, format := wiaFormatTIFF, ".tif", "tiff"
	if request.Mode == modeColor {
		formatID, extension, format = wiaFormatJPEG, ".jpg", "jpeg"
	}
	fileName := fmt.Sprintf("scan-%s-%s%s", time.Now().Format("20060102-150405"), newID()[:8], extension)
	path := filepath.Join(directory, fileName)

	imageVariant, err := oleutil.CallMethod(item, "Transfer", formatID)
	if err != nil {
		return wiaScanResult{}, fmt.Errorf("transfer gambar dari scanner gagal: %w", err)
	}
	defer imageVariant.Clear()
	imageFile := imageVariant.ToIDispatch()
	if err := writeWIAImageData(imageFile, path); err != nil {
		return wiaScanResult{}, fmt.Errorf("gagal menyimpan hasil scan: %w", err)
	}
	actualFormat, err := detectImageFormat(path)
	if err != nil {
		_ = os.Remove(path)
		return wiaScanResult{}, err
	}
	if format == "jpeg" {
		if err := reencodeJPEG(path, 75); err != nil {
			warnings = append(warnings, "Kompresi JPEG gagal; file asli tetap digunakan.")
			format = actualFormat
		}
	} else {
		format = actualFormat
	}
	return wiaScanResult{path: path, format: format, warnings: warnings}, nil
}

// writeWIAImageData mirrors the proven path used by the original application:
// copy WIA.ImageFile.FileData.BinaryData instead of relying on SaveFile, whose
// behaviour differs between older Canon WIA drivers.
func writeWIAImageData(imageFile *ole.IDispatch, path string) error {
	fileDataVariant, err := oleutil.GetProperty(imageFile, "FileData")
	if err != nil {
		return err
	}
	defer fileDataVariant.Clear()
	fileData := fileDataVariant.ToIDispatch()

	binaryVariant, err := oleutil.GetProperty(fileData, "BinaryData")
	if err != nil {
		return err
	}
	defer binaryVariant.Clear()
	array := binaryVariant.ToArray()
	if array == nil {
		return errors.New("driver WIA mengembalikan data gambar kosong")
	}
	contents := array.ToByteArray()
	if len(contents) == 0 {
		return errors.New("driver WIA mengembalikan file berukuran nol byte")
	}
	return os.WriteFile(path, contents, 0o600)
}

func connectWIADevice(manager *ole.IDispatch, deviceID string) (*ole.VARIANT, error) {
	infosVariant, err := oleutil.GetProperty(manager, "DeviceInfos")
	if err != nil {
		return nil, err
	}
	defer infosVariant.Clear()
	infos := infosVariant.ToIDispatch()
	count, err := dispatchInt(infos, "Count")
	if err != nil {
		return nil, err
	}
	for index := 1; index <= count; index++ {
		infoVariant, err := oleutil.GetProperty(infos, "Item", index)
		if err != nil {
			continue
		}
		info := infoVariant.ToIDispatch()
		id, _ := dispatchString(info, "DeviceID")
		if id == deviceID {
			deviceVariant, connectErr := oleutil.CallMethod(info, "Connect")
			infoVariant.Clear()
			if connectErr != nil {
				return nil, fmt.Errorf("scanner gagal dihubungkan: %w", connectErr)
			}
			return deviceVariant, nil
		}
		infoVariant.Clear()
	}
	return nil, errors.New("scanner tidak ditemukan atau telah dilepas")
}

func setWIAProperty(item *ole.IDispatch, propertyID, requested int, label string) string {
	propsVariant, err := oleutil.GetProperty(item, "Properties")
	if err != nil {
		return fmt.Sprintf("Driver tidak menyediakan %s.", label)
	}
	defer propsVariant.Clear()
	props := propsVariant.ToIDispatch()
	count, err := dispatchInt(props, "Count")
	if err != nil {
		return fmt.Sprintf("Driver tidak dapat membaca %s.", label)
	}
	for index := 1; index <= count; index++ {
		propertyVariant, err := oleutil.GetProperty(props, "Item", index)
		if err != nil {
			continue
		}
		property := propertyVariant.ToIDispatch()
		id, idErr := dispatchInt(property, "PropertyID")
		if idErr == nil && id == propertyID {
			result, setErr := oleutil.PutProperty(property, "Value", requested)
			if result != nil {
				result.Clear()
			}
			applied, readErr := dispatchInt(property, "Value")
			propertyVariant.Clear()
			if setErr != nil {
				return fmt.Sprintf("%s ditolak driver; pengaturan bawaan scanner digunakan.", capitalizeLabel(label))
			}
			if readErr == nil && applied != requested {
				return fmt.Sprintf("%s diterapkan sebagai %d oleh driver.", capitalizeLabel(label), applied)
			}
			return ""
		}
		propertyVariant.Clear()
	}
	return fmt.Sprintf("Driver tidak mendukung %s.", label)
}

func capitalizeLabel(label string) string {
	if label == "" {
		return label
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func dispatchInt(dispatch *ole.IDispatch, property string) (int, error) {
	variant, err := oleutil.GetProperty(dispatch, property)
	if err != nil {
		return 0, err
	}
	defer variant.Clear()
	return int(variant.Val), nil
}

func dispatchString(dispatch *ole.IDispatch, property string) (string, error) {
	variant, err := oleutil.GetProperty(dispatch, property)
	if err != nil {
		return "", err
	}
	defer variant.Clear()
	return strings.TrimSpace(variant.ToString()), nil
}

func wiaPropertyString(dispatch *ole.IDispatch, names ...string) (string, error) {
	propsVariant, err := oleutil.GetProperty(dispatch, "Properties")
	if err != nil {
		return "", err
	}
	defer propsVariant.Clear()
	props := propsVariant.ToIDispatch()
	count, err := dispatchInt(props, "Count")
	if err != nil {
		return "", err
	}
	for index := 1; index <= count; index++ {
		propertyVariant, err := oleutil.GetProperty(props, "Item", index)
		if err != nil {
			continue
		}
		property := propertyVariant.ToIDispatch()
		name, _ := dispatchString(property, "Name")
		for _, wanted := range names {
			if strings.EqualFold(name, wanted) {
				value, valueErr := dispatchString(property, "Value")
				propertyVariant.Clear()
				return value, valueErr
			}
		}
		propertyVariant.Clear()
	}
	return "", errors.New("properti WIA tidak ditemukan")
}
