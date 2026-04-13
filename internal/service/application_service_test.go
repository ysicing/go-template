package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

type fakeApplicationClientStore struct {
	created   *model.OAuthClient
	list      []model.OAuthClient
	listCount int64
	getClient *model.OAuthClient
	deletedID string
}

func (f *fakeApplicationClientStore) Create(_ context.Context, client *model.OAuthClient) error {
	copyClient := *client
	copyClient.ID = "app-1"
	f.created = &copyClient
	client.ID = copyClient.ID
	return nil
}

func (f *fakeApplicationClientStore) ListByUserID(context.Context, string, int, int) ([]model.OAuthClient, int64, error) {
	total := f.listCount
	if total == 0 {
		total = int64(len(f.list))
	}
	return f.list, total, nil
}

func (f *fakeApplicationClientStore) GetByIDAndUserID(context.Context, string, string) (*model.OAuthClient, error) {
	if f.getClient == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copyClient := *f.getClient
	return &copyClient, nil
}

func (f *fakeApplicationClientStore) Update(_ context.Context, client *model.OAuthClient) error {
	copyClient := *client
	f.getClient = &copyClient
	return nil
}

func (f *fakeApplicationClientStore) Delete(_ context.Context, id string) error {
	f.deletedID = id
	return nil
}

func TestApplicationServiceCreateAssignsDefaults(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store)

	app, secret, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:  "user-1",
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Equal(t, "Portal", app.Name)
	require.Equal(t, "authorization_code", app.GrantTypes)
	require.Equal(t, "openid profile email", app.Scopes)
	require.Equal(t, "user-1", store.created.UserID)
	require.Empty(t, store.created.OrganizationID)
	require.Equal(t, "app-1", app.ID)
}

func TestApplicationServiceCreateEnforcesPerUserLimit(t *testing.T) {
	store := &fakeApplicationClientStore{listCount: maxApplicationsPerUser}
	svc := NewApplicationService(store)

	_, _, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:  "user-1",
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.ErrorIs(t, err, ErrApplicationLimitReached)
}

func TestApplicationServiceListByOwnerSkipsOrganizationScopedClients(t *testing.T) {
	store := &fakeApplicationClientStore{
		list: []model.OAuthClient{
			{
				Base:         model.Base{ID: "app-1"},
				Name:         "Portal A",
				ClientID:     "client-a",
				RedirectURIs: "https://a.example.com/callback",
				UserID:       "user-1",
			},
			{
				Base:           model.Base{ID: "app-2"},
				Name:           "Portal B",
				ClientID:       "client-b",
				RedirectURIs:   "https://b.example.com/callback",
				UserID:         "user-1",
				OrganizationID: "org-1",
			},
		},
	}
	svc := NewApplicationService(store)

	apps, total, err := svc.ListByOwner(context.Background(), "user-1", 1, 20)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, apps, 1)
	require.Equal(t, "app-1", apps[0].ID)
}

func TestApplicationServiceGetByIDForOwnerRejectsOrganizationScopedClient(t *testing.T) {
	store := &fakeApplicationClientStore{
		getClient: &model.OAuthClient{
			Base:           model.Base{ID: "app-2"},
			Name:           "Portal B",
			ClientID:       "client-b",
			RedirectURIs:   "https://b.example.com/callback",
			UserID:         "user-1",
			OrganizationID: "org-1",
		},
	}
	svc := NewApplicationService(store)

	_, err := svc.GetByIDForOwner(context.Background(), "app-2", "user-1")

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceUpdateByIDForOwnerTrimsFields(t *testing.T) {
	name := " Updated App "
	redirects := " https://example.com/callback "
	grantTypes := ""
	scopes := ""
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:         model.Base{ID: "app-1"},
		Name:         "Old",
		ClientID:     "client-1",
		RedirectURIs: "https://old.example.com/callback",
		GrantTypes:   "authorization_code",
		Scopes:       "openid profile email",
		UserID:       "user-1",
	}}
	svc := NewApplicationService(store)

	app, err := svc.UpdateByIDForOwner(context.Background(), UpdateApplicationInput{
		ID:           "app-1",
		OwnerUserID:  "user-1",
		Name:         &name,
		RedirectURIs: &redirects,
		GrantTypes:   &grantTypes,
		Scopes:       &scopes,
	})

	require.NoError(t, err)
	require.Equal(t, "Updated App", app.Name)
	require.Equal(t, "https://example.com/callback", app.RedirectURIs)
	require.Equal(t, "authorization_code", app.GrantTypes)
	require.Equal(t, "openid profile email", app.Scopes)
}

func TestApplicationServiceDeleteByIDForOwnerRejectsOrganizationScopedClient(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:           model.Base{ID: "app-9"},
		UserID:         "user-7",
		OrganizationID: "org-1",
	}}
	svc := NewApplicationService(store)

	err := svc.DeleteByIDForOwner(context.Background(), "app-9", "user-7")

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Empty(t, store.deletedID)
}

func TestApplicationServiceRotateSecretByIDForOwnerReplacesStoredSecret(t *testing.T) {
	client := &model.OAuthClient{
		Base:         model.Base{ID: "app-1"},
		Name:         "Portal",
		ClientID:     "client-1",
		RedirectURIs: "https://portal.example.com/callback",
		UserID:       "user-1",
	}
	require.NoError(t, client.SetSecret("old-secret"))

	store := &fakeApplicationClientStore{getClient: client}
	svc := NewApplicationService(store)

	app, secret, err := svc.RotateSecretByIDForOwner(context.Background(), "app-1", "user-1")

	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Equal(t, "app-1", app.ID)
	require.True(t, store.getClient.CheckSecret(secret))
	require.False(t, store.getClient.CheckSecret("old-secret"))
}
