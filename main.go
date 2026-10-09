package main

import (
	"embed"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(AppVersion)
		return
	}
	if (len(os.Args) == 4 || len(os.Args) == 5) && os.Args[1] == "--update-helper" {
		if err := runUpdateHelper(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if (len(os.Args) == 3 || len(os.Args) == 5 && os.Args[3] == "--profiles") && os.Args[1] == "--model-switcher-token" {
		var profileFile []string
		if len(os.Args) == 5 {
			profileFile = []string{os.Args[4]}
		}
		value, err := tokenForProfile(os.Args[2], profileFile...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(value)
		return
	}

	releaseSingleInstance, err := acquireSingleInstance()
	if err != nil {
		if errors.Is(err, errAlreadyRunning) {
			fmt.Fprintln(os.Stderr, "Model Switcher 已经在运行")
			return
		}
		fmt.Fprintln(os.Stderr, "无法获取 Model Switcher 单实例锁：", err)
		return
	}
	defer releaseSingleInstance()

	app := NewApp()
	err = wails.Run(&options.App{
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
