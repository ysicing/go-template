package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

func setupOrganizationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

type fakeWorkspaceQuotaChecker struct {
	checkApplicationErr error
	checkWebhookErr     error
	checkMemberErr      error
}

func (f *fakeWorkspaceQuotaChecker) CheckApplicationCreate(context.Context, string, string) error {
	return f.checkApplicationErr
}

func (f *fakeWorkspaceQuotaChecker) CheckWebhookCreate(context.Context, string, string, string) error {
	return f.checkWebhookErr
}

func (f *fakeWorkspaceQuotaChecker) CheckOrganizationMemberAdd(context.Context, string, string) error {
	return f.checkMemberErr
}
