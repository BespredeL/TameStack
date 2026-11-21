// Copyright (C) 2026 Aleksandr Kireev <hello@bespredel.name>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var AppVersion = "1.13.0"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("TameStack failed to start: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "TameStack — Linux Infrastructure Management",
		Width:     1366,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1}, // Slate-900 dark theme
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("TameStack Fatal Error:", err.Error())
	}
}
