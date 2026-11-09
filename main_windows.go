package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"log"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := newApp()
	err := wails.Run(&options.App{
		Title: "ZAPRET", Width: 600, Height: 420, MinWidth: 600, MinHeight: 420, Frameless: true,
		BackgroundColour: &options.RGBA{R: 12, G: 13, B: 22, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets}, OnStartup: app.startup,
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
