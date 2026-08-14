package main

import (
	"context"
	"embed"
	"log"
	"strings"

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
	// A panic on the main goroutine would otherwise terminate the process
	// without any visible trace (GUI app, no console). Record it into the
	// rolling diagnostic log so the crash can be investigated afterwards.
	defer func() {
		if recovered := recover(); recovered != nil {
			service.LogPanic(recovered)
		}
	}()
	err = wails.Run(&options.App{
		Title:       "LogMaster采集端",
		Width:       1440,
		Height:      900,
		MinWidth:    1024,
		MinHeight:   680,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   service.startup,
		OnBeforeClose: func(ctx context.Context) bool {
			warnings := service.CloseWarnings()
			message := "确认关闭程序？"
			if len(warnings) > 0 {
				message += "\n\n- " + strings.Join(warnings, "\n- ")
			} else {
				message += "关闭后将释放本地服务资源。"
			}
			answer, dialogErr := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Type:          runtime.QuestionDialog,
				Title:         "确认关闭",
				Message:       message,
				DefaultButton: "No",
				CancelButton:  "No",
			})
			if dialogErr != nil || answer != "Yes" {
				return true
			}
			service.shutdown()
			return false
		},
		OnShutdown: func(_ context.Context) { service.shutdown() },
		Bind:       []interface{}{service},
		Windows:    &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false, DisableWindowIcon: false},
	})
	if err != nil {
		log.Fatal(err)
	}
}
