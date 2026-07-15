package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	resumeService := NewApp()
	options := application.Options{
		Name:        "Resume Studio",
		Description: "结构化 LaTeX 简历工作台",
		Services: []application.Service{
			application.NewService(resumeService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	}
	if os.Getenv("RESUME_STUDIO_DEBUG") == "1" {
		options.Windows.AdditionalBrowserArgs = []string{"--remote-debugging-port=9222"}
	}
	app := application.New(options)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Resume Studio",
		Width:            1440,
		Height:           900,
		MinWidth:         1040,
		MinHeight:        680,
		InitialPosition:  application.WindowCentered,
		BackgroundColour: application.NewRGB(246, 244, 239),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
