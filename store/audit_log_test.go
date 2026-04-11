package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ysicing/go-template/model"
)

func TestAuditLogStoreAppLoginStatsIncludesMachineActivity(t *testing.T) {
	db := setupUserStoreTestDB(t)
	ctx := context.Background()
	logs := NewAuditLogStore(db)

	client := &model.OAuthClient{
		Name:         "Machine Portal",
		ClientID:     "machine-portal",
		ClientSecret: "hash",
		RedirectURIs: "https://example.com/callback",
		UserID:       "user-1",
	}
	require.NoError(t, db.WithContext(ctx).Create(client).Error)

	user := &model.User{
		Username:   "alice",
		Email:      "alice@example.com",
		Provider:   "local",
		ProviderID: "alice",
		InviteCode: "INV-alice",
	}
	require.NoError(t, db.WithContext(ctx).Create(user).Error)

	loginAt := time.Date(2026, 4, 6, 8, 0, 0, 0, time.UTC)
	issueAt := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	revokeAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)

	require.NoError(t, db.WithContext(ctx).Create(&model.AuditLog{
		UserID:   user.ID,
		Action:   model.AuditLogin,
		ClientID: client.ClientID,
		Status:   "success",
		Base:     model.Base{CreatedAt: loginAt},
	}).Error)
	require.NoError(t, db.WithContext(ctx).Create(&model.AuditLog{
		Action:     model.AuditOAuthClientTokenIssue,
		Resource:   "oauth_client",
		ResourceID: "token-1",
		ClientID:   client.ClientID,
		Status:     "success",
		Base:       model.Base{CreatedAt: issueAt},
	}).Error)
	require.NoError(t, db.WithContext(ctx).Create(&model.AuditLog{
		Action:     model.AuditOAuthTokenRevoke,
		Resource:   "oauth_token",
		ResourceID: "token-1",
		ClientID:   client.ClientID,
		Status:     "success",
		Base:       model.Base{CreatedAt: revokeAt},
	}).Error)

	stats, err := logs.AppLoginStats(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, int64(1), stats[0].LoginCount)
	require.Equal(t, int64(1), stats[0].UserCount)
	require.Equal(t, int64(1), stats[0].MachineTokenIssueCount)
	require.Equal(t, int64(1), stats[0].MachineTokenRevokeCount)
	require.NotNil(t, stats[0].LastMachineTokenIssuedAt)
	require.NotNil(t, stats[0].LastMachineTokenRevokedAt)
	require.Equal(t, "2026-04-06T09:00:00Z", *stats[0].LastMachineTokenIssuedAt)
	require.Equal(t, "2026-04-06T10:00:00Z", *stats[0].LastMachineTokenRevokedAt)
}

func TestFormatAppStatTimestampUsesRFC3339UTC(t *testing.T) {
	at := time.Date(2026, 4, 6, 9, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	formatted := formatAppStatTimestamp(&at)
	require.NotNil(t, formatted)
	require.Equal(t, "2026-04-06T01:30:00Z", *formatted)
	require.Nil(t, formatAppStatTimestamp(nil))
}
