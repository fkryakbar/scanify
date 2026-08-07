package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is replaced from the Git tag by the release workflow.
var Version = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            applicationTitle(),
		Width:            1100,
		Height:           720,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        false,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 11, G: 15, B: 14, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Windows: &windows.Options{
			Theme:                windows.Dark,
			IsZoomControlEnabled: false,
			DisablePinchZoom:     true,
		},
		Bind: []interface{}{app},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func applicationTitle() string {
	if Version == "" || Version == "dev" {
		return "Scanify"
	}
	return "Scanify " + Version
}
