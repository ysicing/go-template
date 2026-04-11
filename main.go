package main

import (
	"context"
	"embed"

	"github.com/ysicing/go-template/internal/app"
	"github.com/ysicing/go-template/pkg/logger"
)

//go:embed all:web/dist
var webDistFS embed.FS

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := app.LoadConfig()
	if err != nil {
		panic("load config: " + err.Error())
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	log := &logger.L

	app.Run(ctx, cfg, webDistFS, app.BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}, log)
}
