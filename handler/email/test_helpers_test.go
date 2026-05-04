package emailhandler

import (
	"testing"

	httprequest "github.com/ysicing/go-template/internal/http/request"
	"github.com/ysicing/go-template/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
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

func trustAll(t *testing.T) func() {
	t.Helper()
	httprequest.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	return func() { httprequest.SetTrustedProxies(nil) }
}
