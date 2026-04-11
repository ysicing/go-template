package handler

import (
	"context"

	"github.com/ysicing/go-template/internal/service"
)

type testWorkspaceQuotaChecker struct {
	checkApplicationErr error
	checkWebhookErr     error
	checkMemberErr      error
}

func (f *testWorkspaceQuotaChecker) CheckApplicationCreate(context.Context, string, string) error {
	return f.checkApplicationErr
}

func (f *testWorkspaceQuotaChecker) CheckWebhookCreate(context.Context, string, string, string) error {
	return f.checkWebhookErr
}

func (f *testWorkspaceQuotaChecker) CheckOrganizationMemberAdd(context.Context, string, string) error {
	return f.checkMemberErr
}

var _ service.WorkspaceQuotaChecker = (*testWorkspaceQuotaChecker)(nil)
