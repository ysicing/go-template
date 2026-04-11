package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

type fakeApplicationClientStore struct {
	created          *model.OAuthClient
	listClients      []model.OAuthClient
	listCount        int64
	getClient        *model.OAuthClient
	workspaceClients []model.OAuthClient
	workspaceClient  *model.OAuthClient
	deletedID        string
}

type fakeEventPublisher struct {
	inputs []PublishDomainEventInput
}

func (f *fakeEventPublisher) Publish(_ context.Context, input PublishDomainEventInput) error {
	f.inputs = append(f.inputs, input)
	return nil
}

func (f *fakeApplicationClientStore) Create(_ context.Context, client *model.OAuthClient) error {
	copyClient := *client
	copyClient.ID = "app-1"
	f.created = &copyClient
	client.ID = copyClient.ID
	return nil
}

func (f *fakeApplicationClientStore) ListByUserID(context.Context, string, int, int) ([]model.OAuthClient, int64, error) {
	return f.listClients, f.listCount, nil
}

func (f *fakeApplicationClientStore) ListAccessibleByUser(context.Context, string, []string, int, int) ([]model.OAuthClient, int64, error) {
	return f.listClients, int64(len(f.listClients)), nil
}

func (f *fakeApplicationClientStore) GetAccessibleByUser(context.Context, string, string, []string) (*model.OAuthClient, error) {
	if f.getClient == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copyClient := *f.getClient
	return &copyClient, nil
}

func (f *fakeApplicationClientStore) ListByWorkspace(context.Context, string, string, string, int, int) ([]model.OAuthClient, int64, error) {
	if f.workspaceClients != nil {
		return f.workspaceClients, int64(len(f.workspaceClients)), nil
	}
	return f.listClients, int64(len(f.listClients)), nil
}

func (f *fakeApplicationClientStore) GetByWorkspace(context.Context, string, string, string, string) (*model.OAuthClient, error) {
	if f.workspaceClient == nil {
		return nil, gorm.ErrRecordNotFound
	}
	copyClient := *f.workspaceClient
	return &copyClient, nil
}

func (f *fakeApplicationClientStore) GetByID(context.Context, string) (*model.OAuthClient, error) {
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

type fakeApplicationOrganizationStore struct {
	orgs        map[string]*model.Organization
	memberships map[string]*model.OrganizationMember
}

type fakeApplicationOrganizationPolicyStore struct {
	policies map[string]*model.OrganizationPolicy
}

func (f *fakeApplicationOrganizationStore) ListForUser(_ context.Context, userID string) ([]model.Organization, error) {
	orgs := make([]model.Organization, 0)
	for _, membership := range f.memberships {
		if membership.UserID != userID {
			continue
		}
		if org, ok := f.orgs[membership.OrganizationID]; ok {
			orgs = append(orgs, *org)
		}
	}
	return orgs, nil
}

func (f *fakeApplicationOrganizationStore) GetByID(_ context.Context, id string) (*model.Organization, error) {
	if org, ok := f.orgs[id]; ok {
		copyOrg := *org
		return &copyOrg, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeApplicationOrganizationStore) GetMembership(_ context.Context, organizationID, userID string) (*model.OrganizationMember, error) {
	key := strings.TrimSpace(organizationID) + ":" + strings.TrimSpace(userID)
	if member, ok := f.memberships[key]; ok {
		copyMember := *member
		return &copyMember, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeApplicationOrganizationPolicyStore) GetByOrganizationID(_ context.Context, organizationID string) (*model.OrganizationPolicy, error) {
	if f == nil || f.policies == nil {
		return &model.OrganizationPolicy{OrganizationID: organizationID}, nil
	}
	if row, ok := f.policies[organizationID]; ok {
		copyRow := *row
		return &copyRow, nil
	}
	return &model.OrganizationPolicy{OrganizationID: organizationID}, nil
}

func TestApplicationServiceCreateAssignsDefaults(t *testing.T) {
	store := &fakeApplicationClientStore{}
	events := &fakeEventPublisher{}
	svc := NewApplicationService(store, nil, events, nil)

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
	require.Equal(t, "app-1", app.ID)
	require.Equal(t, "personal", app.Workspace.Type)
	require.Len(t, events.inputs, 1)
	require.Equal(t, model.EventApplicationCreated, events.inputs[0].EventType)
	require.Equal(t, "app-1", events.inputs[0].ResourceID)
}

func TestApplicationServiceCreateEnforcesPerUserLimit(t *testing.T) {
	store := &fakeApplicationClientStore{listCount: maxApplicationsPerUser}
	svc := NewApplicationService(store, nil, nil, nil)

	_, _, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:  "user-1",
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.ErrorIs(t, err, ErrApplicationLimitReached)
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
	svc := NewApplicationService(store, nil, nil, nil)

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

func TestApplicationServiceDeleteByIDForOwnerDelegatesToStore(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:   model.Base{ID: "app-9"},
		UserID: "user-7",
	}}
	svc := NewApplicationService(store, nil, nil, nil)

	err := svc.DeleteByIDForOwner(context.Background(), "app-9", "user-7")

	require.NoError(t, err)
	require.Equal(t, "app-9", store.deletedID)
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
	svc := NewApplicationService(store, nil, nil, nil)

	app, secret, err := svc.RotateSecretByIDForOwner(context.Background(), "app-1", "user-1")

	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Equal(t, "app-1", app.ID)
	require.True(t, store.getClient.CheckSecret(secret))
	require.False(t, store.getClient.CheckSecret("old-secret"))
}

func TestApplicationServiceRotateSecretByIDForWorkspaceAllowedApplicationsRejectsNonAllowedApp(t *testing.T) {
	client := &model.OAuthClient{
		Base:           model.Base{ID: "app-2"},
		Name:           "Workspace Portal",
		ClientID:       "client-2",
		RedirectURIs:   "https://portal.example.com/callback",
		OrganizationID: "org-1",
	}
	require.NoError(t, client.SetSecret("old-secret"))

	store := &fakeApplicationClientStore{workspaceClient: client}
	svc := NewApplicationService(store, nil, nil, nil)

	_, _, err := svc.RotateSecretByIDForWorkspaceAllowedApplications(
		context.Background(),
		"app-2",
		model.WorkspaceTypeOrganization,
		"",
		"org-1",
		[]string{"other-app"},
	)

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceGetByIDForOwnerReturnsStoreError(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, nil)

	_, err := svc.GetByIDForOwner(context.Background(), "missing", "user-1")

	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestApplicationServiceCreateAllowsOrganizationAdmins(t *testing.T) {
	store := &fakeApplicationClientStore{}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{
			"org-1:user-1": {OrganizationID: "org-1", UserID: "user-1", Role: model.OrganizationRoleAdmin},
		},
	}
	svc := NewApplicationService(store, orgs, nil, nil)

	app, _, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:    "user-1",
		Name:           "Portal",
		RedirectURIs:   "https://portal.example.com/callback",
		OrganizationID: "org-1",
	})

	require.NoError(t, err)
	require.Equal(t, "org-1", store.created.OrganizationID)
	require.Equal(t, "organization", app.Workspace.Type)
	require.Equal(t, "Acme", app.Workspace.OrganizationName)
	require.Equal(t, model.OrganizationRoleAdmin, app.Workspace.Role)
}

func TestApplicationServiceCreateRejectsOrganizationMembersWithoutWriteRole(t *testing.T) {
	store := &fakeApplicationClientStore{}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{
			"org-1:user-1": {OrganizationID: "org-1", UserID: "user-1", Role: model.OrganizationRoleMember},
		},
	}
	svc := NewApplicationService(store, orgs, nil, nil)

	_, _, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:    "user-1",
		Name:           "Portal",
		RedirectURIs:   "https://portal.example.com/callback",
		OrganizationID: "org-1",
	})

	require.ErrorIs(t, err, ErrApplicationWorkspaceForbidden)
}

func TestApplicationServiceCreateRejectsWhenWorkspaceQuotaExceeded(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, &fakeWorkspaceQuotaChecker{
		checkApplicationErr: ErrWorkspaceQuotaExceeded,
	})

	_, _, err := svc.Create(context.Background(), CreateApplicationInput{
		OwnerUserID:  "user-1",
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.ErrorIs(t, err, ErrWorkspaceQuotaExceeded)
}

func TestApplicationServiceListByWorkspaceReturnsOrganizationSummaryWithoutRole(t *testing.T) {
	store := &fakeApplicationClientStore{
		workspaceClients: []model.OAuthClient{
			{
				Base:           model.Base{ID: "app-1"},
				Name:           "Org Portal",
				ClientID:       "org-portal",
				RedirectURIs:   "https://org.example.com/callback",
				UserID:         "user-1",
				OrganizationID: "org-1",
			},
		},
	}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{},
	}
	svc := NewApplicationService(store, orgs, nil, nil)

	apps, total, err := svc.ListByWorkspace(context.Background(), model.WorkspaceTypeOrganization, "", "org-1", 1, 20)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, apps, 1)
	require.Equal(t, "organization", apps[0].Workspace.Type)
	require.Equal(t, "Acme", apps[0].Workspace.OrganizationName)
	require.Equal(t, "acme", apps[0].Workspace.OrganizationSlug)
	require.Empty(t, apps[0].Workspace.Role)
}

func TestApplicationServiceGetByIDForWorkspaceRejectsOutsideWorkspace(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, nil)

	_, err := svc.GetByIDForWorkspace(context.Background(), "missing", model.WorkspaceTypePersonal, "user-1", "")

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceCreateForWorkspaceCreatesPersonalApp(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, nil)

	app, _, err := svc.CreateForWorkspace(context.Background(), model.WorkspaceTypePersonal, "user-1", "", CreateApplicationInput{
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.NoError(t, err)
	require.Equal(t, "user-1", store.created.UserID)
	require.Empty(t, store.created.OrganizationID)
	require.Equal(t, model.WorkspaceTypePersonal, app.Workspace.Type)
}

func TestApplicationServiceCreateForWorkspaceCreatesOrganizationApp(t *testing.T) {
	store := &fakeApplicationClientStore{}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{},
	}
	svc := NewApplicationService(store, orgs, nil, nil)

	app, _, err := svc.CreateForWorkspace(context.Background(), model.WorkspaceTypeOrganization, "", "org-1", CreateApplicationInput{
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.NoError(t, err)
	require.Equal(t, "org-1", store.created.OrganizationID)
	require.Empty(t, store.created.UserID)
	require.Equal(t, model.WorkspaceTypeOrganization, app.Workspace.Type)
	require.Equal(t, "Acme", app.Workspace.OrganizationName)
}

func TestApplicationServiceCreateForWorkspaceForcesRequireConsentWhenOrganizationPolicyEnabled(t *testing.T) {
	store := &fakeApplicationClientStore{}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{},
	}
	policies := &fakeApplicationOrganizationPolicyStore{
		policies: map[string]*model.OrganizationPolicy{
			"org-1": {OrganizationID: "org-1", EnforceRequireConsent: true},
		},
	}
	svc := NewApplicationService(store, orgs, nil, nil)
	svc.SetOrganizationPolicies(policies)

	app, _, err := svc.CreateForWorkspace(context.Background(), model.WorkspaceTypeOrganization, "", "org-1", CreateApplicationInput{
		Name:           "Portal",
		RedirectURIs:   "https://portal.example.com/callback",
		RequireConsent: false,
	})

	require.NoError(t, err)
	require.True(t, store.created.RequireConsent)
	require.True(t, app.RequireConsent)
}

func TestApplicationServiceCreateForWorkspaceRejectsWhenWorkspaceQuotaExceeded(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, &fakeWorkspaceQuotaChecker{
		checkApplicationErr: ErrWorkspaceQuotaExceeded,
	})

	_, _, err := svc.CreateForWorkspace(context.Background(), model.WorkspaceTypePersonal, "user-1", "", CreateApplicationInput{
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.ErrorIs(t, err, ErrWorkspaceQuotaExceeded)
}

func TestApplicationServiceUpdateByIDForWorkspaceRejectsMismatchedWorkspace(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:           model.Base{ID: "app-1"},
		Name:           "Portal",
		ClientID:       "client-1",
		RedirectURIs:   "https://portal.example.com/callback",
		UserID:         "user-1",
		OrganizationID: "org-1",
	}}
	svc := NewApplicationService(store, nil, nil, nil)
	name := "Updated"

	_, err := svc.UpdateByIDForWorkspace(context.Background(), "app-1", model.WorkspaceTypeOrganization, "", "org-2", UpdateApplicationInput{
		Name: &name,
	})

	require.ErrorIs(t, err, ErrApplicationWorkspaceForbidden)
}

func TestApplicationServiceUpdateByIDForWorkspaceForcesRequireConsentWhenOrganizationPolicyEnabled(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:           model.Base{ID: "app-1"},
		Name:           "Portal",
		ClientID:       "client-1",
		RedirectURIs:   "https://portal.example.com/callback",
		OrganizationID: "org-1",
		RequireConsent: true,
	}}
	policies := &fakeApplicationOrganizationPolicyStore{
		policies: map[string]*model.OrganizationPolicy{
			"org-1": {OrganizationID: "org-1", EnforceRequireConsent: true},
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)
	svc.SetOrganizationPolicies(policies)
	value := false

	app, err := svc.UpdateByIDForWorkspace(context.Background(), "app-1", model.WorkspaceTypeOrganization, "", "org-1", UpdateApplicationInput{
		RequireConsent: &value,
	})

	require.NoError(t, err)
	require.True(t, store.getClient.RequireConsent)
	require.True(t, app.RequireConsent)
}

func TestApplicationServiceUpdateByIDForOwnerForcesRequireConsentWhenOrganizationPolicyEnabled(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:           model.Base{ID: "app-1"},
		Name:           "Portal",
		ClientID:       "client-1",
		RedirectURIs:   "https://portal.example.com/callback",
		OrganizationID: "org-1",
		RequireConsent: true,
	}}
	orgs := &fakeApplicationOrganizationStore{
		orgs: map[string]*model.Organization{
			"org-1": {Base: model.Base{ID: "org-1"}, Name: "Acme", Slug: "acme"},
		},
		memberships: map[string]*model.OrganizationMember{
			"org-1:user-1": {OrganizationID: "org-1", UserID: "user-1", Role: model.OrganizationRoleOwner},
		},
	}
	policies := &fakeApplicationOrganizationPolicyStore{
		policies: map[string]*model.OrganizationPolicy{
			"org-1": {OrganizationID: "org-1", EnforceRequireConsent: true},
		},
	}
	svc := NewApplicationService(store, orgs, nil, nil)
	svc.SetOrganizationPolicies(policies)
	value := false

	app, err := svc.UpdateByIDForOwner(context.Background(), UpdateApplicationInput{
		ID:             "app-1",
		OwnerUserID:    "user-1",
		RequireConsent: &value,
	})

	require.NoError(t, err)
	require.True(t, store.getClient.RequireConsent)
	require.True(t, app.RequireConsent)
}

func TestApplicationServiceDeleteByIDForWorkspaceDeletesInsideWorkspace(t *testing.T) {
	store := &fakeApplicationClientStore{getClient: &model.OAuthClient{
		Base:         model.Base{ID: "app-9"},
		Name:         "Portal",
		ClientID:     "client-9",
		RedirectURIs: "https://portal.example.com/callback",
		UserID:       "user-7",
	}}
	events := &fakeEventPublisher{}
	svc := NewApplicationService(store, nil, events, nil)

	err := svc.DeleteByIDForWorkspace(context.Background(), "app-9", model.WorkspaceTypePersonal, "user-7", "")

	require.NoError(t, err)
	require.Equal(t, "app-9", store.deletedID)
	require.Len(t, events.inputs, 1)
	require.Equal(t, model.EventApplicationDeleted, events.inputs[0].EventType)
}

func TestApplicationServiceAllowlistListByWorkspaceReturnsOnlyAllowedApps(t *testing.T) {
	store := &fakeApplicationClientStore{
		workspaceClients: []model.OAuthClient{
			{Base: model.Base{ID: "app-1"}, Name: "Portal A", ClientID: "client-a", RedirectURIs: "https://a.example.com/callback", UserID: "user-1"},
			{Base: model.Base{ID: "app-2"}, Name: "Portal B", ClientID: "client-b", RedirectURIs: "https://b.example.com/callback", UserID: "user-1"},
		},
		workspaceClient: &model.OAuthClient{
			Base:         model.Base{ID: "app-2"},
			Name:         "Portal B",
			ClientID:     "client-b",
			RedirectURIs: "https://b.example.com/callback",
			UserID:       "user-1",
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)

	apps, total, err := svc.ListByWorkspaceAllowedApplications(context.Background(), model.WorkspaceTypePersonal, "user-1", "", []string{"app-2"}, 1, 20)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, apps, 1)
	require.Equal(t, "app-2", apps[0].ID)
}

func TestApplicationServiceAllowlistGetByIDForWorkspaceRejectsOtherID(t *testing.T) {
	store := &fakeApplicationClientStore{
		workspaceClient: &model.OAuthClient{
			Base:         model.Base{ID: "app-2"},
			Name:         "Portal B",
			ClientID:     "client-b",
			RedirectURIs: "https://b.example.com/callback",
			UserID:       "user-1",
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)

	_, err := svc.GetByIDForWorkspaceAllowedApplications(context.Background(), "app-1", model.WorkspaceTypePersonal, "user-1", "", []string{"app-2"})

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceAllowlistUpdateByIDForWorkspaceRejectsOtherID(t *testing.T) {
	store := &fakeApplicationClientStore{
		getClient: &model.OAuthClient{
			Base:         model.Base{ID: "app-2"},
			Name:         "Portal B",
			ClientID:     "client-b",
			RedirectURIs: "https://b.example.com/callback",
			UserID:       "user-1",
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)
	name := "Updated"

	_, err := svc.UpdateByIDForWorkspaceAllowedApplications(context.Background(), "app-1", model.WorkspaceTypePersonal, "user-1", "", []string{"app-2"}, UpdateApplicationInput{
		Name: &name,
	})

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceAllowlistDeleteByIDForWorkspaceRejectsOtherID(t *testing.T) {
	store := &fakeApplicationClientStore{
		getClient: &model.OAuthClient{
			Base:         model.Base{ID: "app-2"},
			Name:         "Portal B",
			ClientID:     "client-b",
			RedirectURIs: "https://b.example.com/callback",
			UserID:       "user-1",
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)

	err := svc.DeleteByIDForWorkspaceAllowedApplications(context.Background(), "app-1", model.WorkspaceTypePersonal, "user-1", "", []string{"app-2"})

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestApplicationServiceAllowlistCreateForWorkspaceRejectsCreate(t *testing.T) {
	store := &fakeApplicationClientStore{}
	svc := NewApplicationService(store, nil, nil, nil)

	_, _, err := svc.CreateForWorkspaceAllowedApplications(context.Background(), model.WorkspaceTypePersonal, "user-1", "", []string{"app-2"}, CreateApplicationInput{
		Name:         "Portal",
		RedirectURIs: "https://portal.example.com/callback",
	})

	require.ErrorIs(t, err, ErrBoundApplicationCreateForbidden)
}

func TestApplicationServiceAllowlistListByWorkspaceFallsBackWhenUnbound(t *testing.T) {
	store := &fakeApplicationClientStore{
		workspaceClients: []model.OAuthClient{
			{Base: model.Base{ID: "app-1"}, Name: "Portal A", ClientID: "client-a", RedirectURIs: "https://a.example.com/callback", UserID: "user-1"},
			{Base: model.Base{ID: "app-2"}, Name: "Portal B", ClientID: "client-b", RedirectURIs: "https://b.example.com/callback", UserID: "user-1"},
		},
	}
	svc := NewApplicationService(store, nil, nil, nil)

	apps, total, err := svc.ListByWorkspaceAllowedApplications(context.Background(), model.WorkspaceTypePersonal, "user-1", "", nil, 1, 20)

	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, apps, 2)
}
