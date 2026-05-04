package store

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"

	"github.com/stretchr/testify/require"
)

func TestAuditLogStoreListLoginAllPagedDoesNotRequireOAuthClients(t *testing.T) {
	db := setupUserStoreTestDB(t)
	ctx := context.Background()
	logs := NewAuditLogStore(db)

	user := &model.User{
		Username:   "alice",
		Email:      "alice@example.com",
		Provider:   "local",
		ProviderID: "alice",
		InviteCode: "INV-alice",
	}
	require.NoError(t, db.WithContext(ctx).Create(user).Error)

	at := time.Date(2026, 4, 6, 8, 0, 0, 0, time.UTC)
	require.NoError(t, db.WithContext(ctx).Create(&model.AuditLog{
		UserID:   user.ID,
		Action:   model.AuditLogin,
		ClientID: "legacy-client",
		Detail:   "local",
		Status:   "success",
		Base:     model.Base{CreatedAt: at},
	}).Error)

	rows, total, err := logs.ListLoginAllPaged(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, user.ID, rows[0].UserID)
	require.Equal(t, user.Username, rows[0].Username)
	require.Equal(t, "legacy-client", rows[0].ClientID)
	require.Empty(t, rows[0].AppName)
}
