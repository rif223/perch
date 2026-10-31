package main

import (
	"embed"

	"perch/internal/player"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Before any window exists, so the mpv window can join Perch's taskbar group.
	_ = player.SetProcessAppID()

	app := NewApp()

	err := wails.Run(&options.App{
		Title: "Perch",
		// Tall enough for all controls without scrolling; fits a 1440x912
		// work area. Smaller screens scroll the content.
		Width:     500,
		Height:    860,
		MinWidth:  460,
		MinHeight: 720,

		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		BackgroundColour: &options.RGBA{R: 28, G: 28, B: 32, A: 255},

		// A second launch focuses the existing window instead of starting
		// another app (and another mpv).
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "perch-6b1f0c2e",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.onSecondInstance() },
		},

		// Dropped files arrive as paths via the runtime's OnFileDrop.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},

		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       []interface{}{app},

		Windows: &windows.Options{
			Theme: windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:           windows.RGB(28, 28, 32),
				DarkModeTitleBarInactive:   windows.RGB(28, 28, 32),
				DarkModeTitleText:          windows.RGB(235, 235, 240),
				DarkModeTitleTextInactive:  windows.RGB(140, 140, 150),
				DarkModeBorder:             windows.RGB(38, 110, 76),
				DarkModeBorderInactive:     windows.RGB(45, 45, 50),
				LightModeTitleBar:          windows.RGB(28, 28, 32),
				LightModeTitleBarInactive:  windows.RGB(28, 28, 32),
				LightModeTitleText:         windows.RGB(235, 235, 240),
				LightModeTitleTextInactive: windows.RGB(140, 140, 150),
				LightModeBorder:            windows.RGB(38, 110, 76),
				LightModeBorderInactive:    windows.RGB(45, 45, 50),
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
