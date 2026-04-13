package app

import (
	"path/filepath"
	"testing"
)

func testSQLiteDSN(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}
