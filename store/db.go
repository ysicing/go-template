package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
)

func InitDB(driver, dsn, logLevel string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch driver {
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: NewTraceLogger(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Register Prometheus plugin for database metrics
	promConfig := prometheus.Config{
		DBName:          "xid",
		RefreshInterval: 15,
	}

	// Add MySQL-specific metrics only for MySQL databases
	if driver == "mysql" {
		promConfig.MetricsCollector = []prometheus.MetricsCollector{
			&prometheus.MySQL{
				VariableNames: []string{"Threads_running"},
			},
		}
	}

	if err := db.Use(prometheus.New(promConfig)); err != nil {
		return nil, fmt.Errorf("failed to register prometheus plugin: %w", err)
	}

	// Configure connection pool for non-SQLite drivers.
	if driver != "sqlite" {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.SetMaxOpenConns(25)
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetConnMaxLifetime(5 * time.Minute)
		}
	}

	return db, nil
}

// IsUniqueViolation checks if an error is a database unique constraint violation.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "23505") || strings.Contains(msg, "1062")
}
