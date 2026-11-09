package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
	"golang.org/x/sys/windows"
	"log"
	"os"
	"path/filepath"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := newApp()
	cache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}
	profile := "user"
	if windows.GetCurrentProcessToken().IsElevated() {
		profile = "admin"
	}
	err = wails.Run(&options.App{
		Title: "ZAPRET", Width: 600, Height: 420, MinWidth: 600, MinHeight: 420, Frameless: true,
		BackgroundColour: &options.RGBA{R: 12, G: 13, B: 22, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets}, OnStartup: app.startup,
		OnDomReady: app.domReady,
		Windows:    &windowsoptions.Options{WebviewUserDataPath: filepath.Join(cache, "gui-zapret", "webview", profile)},
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
