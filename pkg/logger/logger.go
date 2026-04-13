package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// L is the global zerolog logger instance.
var L zerolog.Logger

type FileConfig struct {
	Enabled    bool
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

type Config struct {
	Level  string
	Format string
	File   FileConfig
}

// Init initializes the global logger.
// format: "json" or "text" (console). level: "debug", "info", "warn", "error".
func Init(cfg Config, stdout io.Writer) error {
	w, err := buildWriter(cfg, stdout)
	if err != nil {
		return err
	}

	lvl, parseErr := zerolog.ParseLevel(cfg.Level)
	if parseErr != nil {
		lvl = zerolog.InfoLevel
	}

	L = zerolog.New(w).With().Timestamp().Logger().Level(lvl)
	zerolog.DefaultContextLogger = &L
	return nil
}

func buildWriter(cfg Config, stdout io.Writer) (io.Writer, error) {
	if stdout == nil {
		stdout = os.Stdout
	}

	stdoutWriter := stdout
	if cfg.Format == "text" {
		stdoutWriter = zerolog.ConsoleWriter{Out: stdout, TimeFormat: time.DateTime}
	}

	if !cfg.File.Enabled {
		return stdoutWriter, nil
	}
	if cfg.File.Path == "" {
		return nil, fmt.Errorf("log file path is required when file logging is enabled")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.File.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	maxSize := cfg.File.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100
	}
	maxBackups := cfg.File.MaxBackups
	if maxBackups < 0 {
		maxBackups = 0
	}
	maxAge := cfg.File.MaxAgeDays
	if maxAge < 0 {
		maxAge = 0
	}

	fileWriter := &lumberjack.Logger{
		Filename:   cfg.File.Path,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   cfg.File.Compress,
	}

	return io.MultiWriter(stdoutWriter, fileWriter), nil
}
