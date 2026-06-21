package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	webMode := flag.Bool("web", false, "以Web服务器模式运行")
	port := flag.Int("port", 2018, "Web服务器端口")
	user := flag.String("user", "", "Basic认证用户名")
	pass := flag.String("pass", "", "Basic认证密码")
	flag.Parse()

	app := NewApp()

	if *webMode {
		app.startup(nil)
		defer app.shutdown(nil)

		fmt.Fprintf(os.Stderr, "股票分析系统 Web 模式启动于 :%d\n", *port)
		server := NewWebServer(app, *port, *user, *pass)
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "服务器启动失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	err := wails.Run(&options.App{
		Title:     "智能股票分析系统",
		Width:     1200,
		Height:    760,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
