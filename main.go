package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/reeschan/atelier-desk/internal/appconfig"
	"github.com/reeschan/atelier-desk/internal/desk"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfgPath, err := appconfig.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	svc := desk.NewService(cfgPath)
	app := NewApp(svc)

	err = wails.Run(&options.App{
		Title:  "Atelier Desk",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			svc,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
