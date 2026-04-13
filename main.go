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

	if err := logger.Init(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		File: logger.FileConfig{
			Enabled:    cfg.Log.File.Enabled,
			Path:       cfg.Log.File.Path,
			MaxSizeMB:  cfg.Log.File.MaxSizeMB,
			MaxBackups: cfg.Log.File.MaxBackups,
			MaxAgeDays: cfg.Log.File.MaxAgeDays,
			Compress:   cfg.Log.File.Compress,
		},
	}, nil); err != nil {
		panic("init logger: " + err.Error())
	}
	log := &logger.L

	app.Run(ctx, cfg, webDistFS, app.BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}, log)
}
