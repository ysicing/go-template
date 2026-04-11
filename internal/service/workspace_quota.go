package service

import (
	"context"
	"errors"
)

var ErrWorkspaceQuotaExceeded = errors.New("workspace quota exceeded")

type WorkspaceQuotaChecker interface {
	CheckApplicationCreate(ctx context.Context, ownerUserID, organizationID string) error
	CheckWebhookCreate(ctx context.Context, actorUserID, workspaceType, organizationID string) error
	CheckOrganizationMemberAdd(ctx context.Context, actorUserID, organizationID string) error
}
