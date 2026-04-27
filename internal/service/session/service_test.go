package sessionservice

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"

	"github.com/stretchr/testify/require"
)

type fakeRefreshTokenStore struct {
	created *model.APIRefreshToken
}

func (f *fakeRefreshTokenStore) Create(_ context.Context, rt *model.APIRefreshToken) error {
	copyToken := *rt
	f.created = &copyToken
	return nil
}

func TestSessionServiceIssueBrowserSessionPersistsMetadata(t *testing.T) {
	refreshStore := &fakeRefreshTokenStore{}
	svc := NewSessionService(refreshStore, TokenConfig{
		Secret:     "test-secret",
		Issuer:     "id",
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
	})

	session, err := svc.IssueBrowserSession(context.Background(), SessionRequest{
		User:      &model.User{Base: model.Base{ID: "user-1"}, Provider: "local"},
		IP:        "127.0.0.1",
		UserAgent: "unit-test",
	})

	require.NoError(t, err)
	require.NotEmpty(t, session.AccessToken)
	require.NotEmpty(t, session.RefreshToken)
	require.Equal(t, "127.0.0.1", refreshStore.created.IP)
	require.Equal(t, "unit-test", refreshStore.created.UserAgent)
	require.NotEmpty(t, refreshStore.created.Family)
}

func TestSessionServiceRotateBrowserSessionPreservesFamily(t *testing.T) {
	refreshStore := &fakeRefreshTokenStore{}
	svc := NewSessionService(refreshStore, TokenConfig{
		Secret:     "test-secret",
		Issuer:     "id",
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
	})

	session, err := svc.RotateBrowserSession(context.Background(), SessionRequest{
		User:      &model.User{Base: model.Base{ID: "user-1"}, Provider: "local"},
		IP:        "127.0.0.1",
		UserAgent: "unit-test",
		Family:    "family-1",
	})

	require.NoError(t, err)
	require.Equal(t, "family-1", session.Family)
	require.Equal(t, "family-1", refreshStore.created.Family)
}
