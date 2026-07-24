package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	service, err := NewService()
	if err != nil {
		log.Fatal(err)
	}
	defer service.shutdown()
	err = wails.Run(&options.App{Title: "LogMaster 采集端", Width: 1440, Height: 900, MinWidth: 1024, MinHeight: 680, AssetServer: &assetserver.Options{Assets: assets}, OnStartup: service.startup, OnShutdown: func(_ context.Context) { service.shutdown() }, OnBeforeClose: func(ctx context.Context) bool { runtime.WindowHide(ctx); return true }, Bind: []interface{}{service}, Windows: &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false, DisableWindowIcon: false}})
	if err != nil {
		log.Fatal(err)
	}
}
