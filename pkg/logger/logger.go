package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// L is the global zerolog logger instance.
var L zerolog.Logger

// Init initializes the global logger.
// format: "json" or "text" (console). level: "debug", "info", "warn", "error".
func Init(level, format string) {
	var w io.Writer
	if format == "text" {
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.DateTime}
	} else {
		w = os.Stdout
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	L = zerolog.New(w).With().Timestamp().Logger().Level(lvl)
	zerolog.DefaultContextLogger = &L
}
