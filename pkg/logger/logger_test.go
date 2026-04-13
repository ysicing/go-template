package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesToStdoutAndFileWhenEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "app.log")
	var stdout bytes.Buffer

	err := Init(Config{
		Level:  "info",
		Format: "json",
		File: FileConfig{
			Enabled: true,
			Path:    logPath,
		},
	}, &stdout)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	L.Info().Str("component", "test").Msg("hello")

	if got := stdout.String(); !strings.Contains(got, `"message":"hello"`) {
		t.Fatalf("expected stdout log output, got %q", got)
	}

	fileContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(fileContent); !strings.Contains(got, `"message":"hello"`) {
		t.Fatalf("expected file log output, got %q", got)
	}
}

func TestInitRequiresFilePathWhenFileLoggingEnabled(t *testing.T) {
	var stdout bytes.Buffer

	err := Init(Config{
		Level:  "info",
		Format: "json",
		File: FileConfig{
			Enabled: true,
		},
	}, &stdout)
	if err == nil {
		t.Fatal("expected error when file logging enabled without path")
	}
}
