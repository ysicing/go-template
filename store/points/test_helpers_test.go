package points

import (
	"testing"

	"github.com/ysicing/go-template/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPointsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}
