package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) == 4 && os.Args[1] == "--update-helper" {
		if err := runUpdateHelper(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--model-switcher-token" {
		value, err := tokenForProfile(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(value)
		return
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "Model Switcher",
		Width:            defaultWindowWidth,
		Height:           defaultWindowHeight,
		MinWidth:         minWindowWidth,
		MinHeight:        minWindowHeight,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.startup,
		StartHidden:      true,
		OnDomReady:       app.restoreWindow,
		OnBeforeClose:    app.beforeClose,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
