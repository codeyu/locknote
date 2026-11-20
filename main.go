// NaishoMemo — 基于 LockNote（https://github.com/JackyZhang8/locknote，MIT 许可）修改。
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "NaishoMemo",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 251, B: 246, A: 1},
		// 启动时先隐藏窗口，等前端把首屏（启动进度页）绘制完成后再显示，
		// 避免 WebView 尚未渲染时先露出一块白屏。
		StartHidden:   true,
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
		// 只允许运行一个实例：程序已打开时再双击 .naisho 文件，交给已运行的窗口处理
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "app.naishomemo.single-instance",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			OnFileOpen:           app.onFileOpen,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},
		EnableDefaultContextMenu: false,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
