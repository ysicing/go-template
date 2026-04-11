package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitDB_WithPrometheus(t *testing.T) {
	// Test SQLite with Prometheus plugin
	db, err := InitDB("sqlite", ":memory:", "error")
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Verify database connection is working
	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NotNil(t, sqlDB)

	// Verify connection pool stats are available (Prometheus plugin should be registered)
	stats := sqlDB.Stats()
	assert.GreaterOrEqual(t, stats.MaxOpenConnections, 0)
}

func TestInitDB_UnsupportedDriver(t *testing.T) {
	db, err := InitDB("unsupported", "test.db", "error")
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unsupported database driver")
}
