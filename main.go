package main

import (
	"embed"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"pgevidence/internal/config"
)

// Window sizing: defaults and floor. The persisted size is the OS window size
// (via Wails WindowGetSize), so save/restore round-trips without shrinking.
const (
	defaultWinW = 1200
	defaultWinH = 820
	minWinW     = 900
	minWinH     = 600
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed VERSION
var versionRaw string

// AppVersion is the application version, sourced from the repo-root VERSION file.
var AppVersion = strings.TrimSpace(versionRaw)

// AppName is the single source of truth for the product's display name (window
// title + UI header via EnvInfo). Packaging names live in wails.json.
const AppName = "PgEvidence"

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Restore the last window size (floored at the minimum); fall back to default.
	width, height := defaultWinW, defaultWinH
	if cfg, err := config.Load(); err == nil {
		if cfg.WindowWidth >= minWinW {
			width = cfg.WindowWidth
		}
		if cfg.WindowHeight >= minWinH {
			height = cfg.WindowHeight
		}
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     AppName,
		Width:     width,
		Height:    height,
		MinWidth:  minWinW,
		MinHeight: minWinH,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
