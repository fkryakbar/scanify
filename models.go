package main

type ScannerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ScanRequest struct {
	ScannerID string `json:"scannerId"`
	Mode      string `json:"mode"`
	DPI       int    `json:"dpi"`
}

type PageDTO struct {
	ID               string `json:"id"`
	ThumbnailDataURL string `json:"thumbnailDataURL"`
	Selected         bool   `json:"selected"`
	SelectionOrder   int    `json:"selectionOrder"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
}

type SessionDTO struct {
	Pages         []PageDTO `json:"pages"`
	SelectedCount int       `json:"selectedCount"`
	Status        string    `json:"status"`
}

type ScanResultDTO struct {
	Session  SessionDTO `json:"session"`
	Warnings []string   `json:"warnings"`
}

type ExportResultDTO struct {
	Cancelled bool     `json:"cancelled"`
	Format    string   `json:"format"`
	Paths     []string `json:"paths"`
}

const (
	modeColor      = "color"
	modeGrayscale  = "grayscale"
	modeBlackWhite = "blackwhite"
	exportJPG      = "jpg"
	exportPDF      = "pdf"
)
