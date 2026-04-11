package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

const maxApplicationsPerUser = 10

var (
	ErrApplicationLimitReached         = errors.New("maximum number of apps reached")
	ErrApplicationWorkspaceForbidden   = errors.New("application workspace forbidden")
	ErrBoundApplicationCreateForbidden = errors.New("bound application create forbidden")
)

type applicationClientStore interface {
	Create(ctx context.Context, client *model.OAuthClient) error
	ListByUserID(ctx context.Context, userID string, page, pageSize int) ([]model.OAuthClient, int64, error)
	ListAccessibleByUser(ctx context.Context, userID string, organizationIDs []string, page, pageSize int) ([]model.OAuthClient, int64, error)
	GetAccessibleByUser(ctx context.Context, id, userID string, organizationIDs []string) (*model.OAuthClient, error)
	ListByWorkspace(ctx context.Context, workspaceType, ownerUserID, organizationID string, page, pageSize int) ([]model.OAuthClient, int64, error)
	GetByWorkspace(ctx context.Context, id, workspaceType, ownerUserID, organizationID string) (*model.OAuthClient, error)
	GetByID(ctx context.Context, id string) (*model.OAuthClient, error)
	Update(ctx context.Context, client *model.OAuthClient) error
	Delete(ctx context.Context, id string) error
}

type applicationOrganizationStore interface {
	ListForUser(ctx context.Context, userID string) ([]model.Organization, error)
	GetByID(ctx context.Context, id string) (*model.Organization, error)
	GetMembership(ctx context.Context, organizationID, userID string) (*model.OrganizationMember, error)
}

type applicationOrganizationPolicyStore interface {
	GetByOrganizationID(ctx context.Context, organizationID string) (*model.OrganizationPolicy, error)
}

type WorkspaceSummary struct {
	Type             string `json:"type"`
	OrganizationID   string `json:"organization_id,omitempty"`
	OrganizationName string `json:"organization_name,omitempty"`
	OrganizationSlug string `json:"organization_slug,omitempty"`
	Role             string `json:"role,omitempty"`
}

type Application struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	ClientID       string           `json:"client_id"`
	RedirectURIs   string           `json:"redirect_uris"`
	GrantTypes     string           `json:"grant_types"`
	Scopes         string           `json:"scopes"`
	RequireConsent bool             `json:"require_consent"`
	OrganizationID string           `json:"organization_id,omitempty"`
	Workspace      WorkspaceSummary `json:"workspace"`
}

type CreateApplicationInput struct {
	OwnerUserID    string
	Name           string
	RedirectURIs   string
	GrantTypes     string
	Scopes         string
	OrganizationID string
	RequireConsent bool
}

type UpdateApplicationInput struct {
	ID             string
	OwnerUserID    string
	Name           *string
	RedirectURIs   *string
	GrantTypes     *string
	Scopes         *string
	RequireConsent *bool
	OrganizationID *string
}

type ApplicationService struct {
	clients       applicationClientStore
	organizations applicationOrganizationStore
	policies      applicationOrganizationPolicyStore
	events        EventPublisher
	quotas        WorkspaceQuotaChecker
}

func NewApplicationService(clients applicationClientStore, organizations applicationOrganizationStore, events EventPublisher, quotas WorkspaceQuotaChecker) *ApplicationService {
	return &ApplicationService{clients: clients, organizations: organizations, events: events, quotas: quotas}
}

func (s *ApplicationService) SetOrganizationPolicies(policies applicationOrganizationPolicyStore) {
	s.policies = policies
}

func (s *ApplicationService) Create(ctx context.Context, input CreateApplicationInput) (*Application, string, error) {
	_, count, err := s.clients.ListByUserID(ctx, input.OwnerUserID, 1, 1)
	if err != nil {
		return nil, "", err
	}
	if count >= maxApplicationsPerUser {
		return nil, "", ErrApplicationLimitReached
	}

	secret, err := generateClientSecret()
	if err != nil {
		return nil, "", err
	}

	workspace, err := s.resolveWriteWorkspace(ctx, input.OwnerUserID, input.OrganizationID)
	if err != nil {
		return nil, "", err
	}
	if s.quotas != nil {
		if err := s.quotas.CheckApplicationCreate(ctx, input.OwnerUserID, workspace.OrganizationID); err != nil {
			return nil, "", err
		}
	}

	client := &model.OAuthClient{
		Name:           strings.TrimSpace(input.Name),
		ClientID:       uuid.NewString(),
		RedirectURIs:   strings.TrimSpace(input.RedirectURIs),
		GrantTypes:     defaultIfEmpty(input.GrantTypes, "authorization_code"),
		Scopes:         defaultIfEmpty(input.Scopes, "openid profile email"),
		RequireConsent: input.RequireConsent,
		UserID:         input.OwnerUserID,
		OrganizationID: workspace.OrganizationID,
	}
	if err := s.enforceOrganizationPolicy(ctx, client); err != nil {
		return nil, "", err
	}
	if err := client.SetSecret(secret); err != nil {
		return nil, "", err
	}
	if err := s.clients.Create(ctx, client); err != nil {
		return nil, "", err
	}

	app, err := s.toApplication(ctx, input.OwnerUserID, client)
	if err != nil {
		return nil, "", err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationCreated, input.OwnerUserID, app)
	return app, secret, nil
}

func (s *ApplicationService) CreateForWorkspace(ctx context.Context, workspaceType, ownerUserID, organizationID string, input CreateApplicationInput) (*Application, string, error) {
	workspaceType, ownerUserID, organizationID, err := normalizeApplicationWorkspace(workspaceType, ownerUserID, organizationID)
	if err != nil {
		return nil, "", err
	}

	if input.OrganizationID != "" {
		if workspaceType == model.WorkspaceTypePersonal || strings.TrimSpace(input.OrganizationID) != organizationID {
			return nil, "", ErrApplicationWorkspaceForbidden
		}
	}

	secret, err := generateClientSecret()
	if err != nil {
		return nil, "", err
	}
	if s.quotas != nil {
		if err := s.quotas.CheckApplicationCreate(ctx, ownerUserID, organizationID); err != nil {
			return nil, "", err
		}
	}

	client := &model.OAuthClient{
		Name:           strings.TrimSpace(input.Name),
		ClientID:       uuid.NewString(),
		RedirectURIs:   strings.TrimSpace(input.RedirectURIs),
		GrantTypes:     defaultIfEmpty(input.GrantTypes, "authorization_code"),
		Scopes:         defaultIfEmpty(input.Scopes, "openid profile email"),
		RequireConsent: input.RequireConsent,
		UserID:         ownerUserID,
		OrganizationID: organizationID,
	}
	if workspaceType == model.WorkspaceTypeOrganization {
		client.UserID = ""
	}
	if err := s.enforceOrganizationPolicy(ctx, client); err != nil {
		return nil, "", err
	}
	if err := client.SetSecret(secret); err != nil {
		return nil, "", err
	}
	if err := s.clients.Create(ctx, client); err != nil {
		return nil, "", err
	}

	app, err := s.toApplicationForWorkspace(ctx, client)
	if err != nil {
		return nil, "", err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationCreated, ownerUserID, app)
	return app, secret, nil
}

func (s *ApplicationService) CreateForWorkspaceBoundToApplication(ctx context.Context, workspaceType, ownerUserID, organizationID, boundApplicationID string, input CreateApplicationInput) (*Application, string, error) {
	return s.CreateForWorkspaceAllowedApplications(ctx, workspaceType, ownerUserID, organizationID, singleAllowedApplicationID(boundApplicationID), input)
}

func (s *ApplicationService) CreateForWorkspaceAllowedApplications(ctx context.Context, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string, input CreateApplicationInput) (*Application, string, error) {
	if len(normalizeAllowedApplicationIDs(allowedApplicationIDs)) > 0 {
		return nil, "", ErrBoundApplicationCreateForbidden
	}
	return s.CreateForWorkspace(ctx, workspaceType, ownerUserID, organizationID, input)
}

func (s *ApplicationService) ListByOwner(ctx context.Context, ownerUserID string, page, pageSize int) ([]Application, int64, error) {
	orgIDs, err := s.accessibleOrganizationIDs(ctx, ownerUserID)
	if err != nil {
		return nil, 0, err
	}
	clients, total, err := s.clients.ListAccessibleByUser(ctx, ownerUserID, orgIDs, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	applications := make([]Application, 0, len(clients))
	for i := range clients {
		app, err := s.toApplication(ctx, ownerUserID, &clients[i])
		if err != nil {
			return nil, 0, err
		}
		applications = append(applications, *app)
	}
	return applications, total, nil
}

func (s *ApplicationService) GetByIDForOwner(ctx context.Context, id, ownerUserID string) (*Application, error) {
	orgIDs, err := s.accessibleOrganizationIDs(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.GetAccessibleByUser(ctx, id, ownerUserID, orgIDs)
	if err != nil {
		return nil, err
	}
	return s.toApplication(ctx, ownerUserID, client)
}

func (s *ApplicationService) ListByWorkspace(ctx context.Context, workspaceType, ownerUserID, organizationID string, page, pageSize int) ([]Application, int64, error) {
	clients, total, err := s.clients.ListByWorkspace(ctx, workspaceType, ownerUserID, organizationID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	applications := make([]Application, 0, len(clients))
	for i := range clients {
		app, err := s.toApplicationForWorkspace(ctx, &clients[i])
		if err != nil {
			return nil, 0, err
		}
		applications = append(applications, *app)
	}
	return applications, total, nil
}

func (s *ApplicationService) ListByWorkspaceBoundToApplication(ctx context.Context, workspaceType, ownerUserID, organizationID, boundApplicationID string, page, pageSize int) ([]Application, int64, error) {
	return s.ListByWorkspaceAllowedApplications(ctx, workspaceType, ownerUserID, organizationID, singleAllowedApplicationID(boundApplicationID), page, pageSize)
}

func (s *ApplicationService) ListByWorkspaceAllowedApplications(ctx context.Context, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string, page, pageSize int) ([]Application, int64, error) {
	allowedApplicationIDs = normalizeAllowedApplicationIDs(allowedApplicationIDs)
	if len(allowedApplicationIDs) == 0 {
		return s.ListByWorkspace(ctx, workspaceType, ownerUserID, organizationID, page, pageSize)
	}
	applications := make([]Application, 0, len(allowedApplicationIDs))
	for _, allowedApplicationID := range allowedApplicationIDs {
		application, err := s.GetByIDForWorkspace(ctx, allowedApplicationID, workspaceType, ownerUserID, organizationID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrApplicationWorkspaceForbidden) {
				continue
			}
			return nil, 0, err
		}
		applications = append(applications, *application)
	}
	total := int64(len(applications))
	start, end := paginateApplicationSlice(page, pageSize, len(applications))
	return applications[start:end], total, nil
}

func (s *ApplicationService) GetByIDForWorkspace(ctx context.Context, id, workspaceType, ownerUserID, organizationID string) (*Application, error) {
	client, err := s.clients.GetByWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
	if err != nil {
		return nil, err
	}
	return s.toApplicationForWorkspace(ctx, client)
}

func (s *ApplicationService) GetByIDForWorkspaceBoundToApplication(ctx context.Context, id, workspaceType, ownerUserID, organizationID, boundApplicationID string) (*Application, error) {
	return s.GetByIDForWorkspaceAllowedApplications(ctx, id, workspaceType, ownerUserID, organizationID, singleAllowedApplicationID(boundApplicationID))
}

func (s *ApplicationService) GetByIDForWorkspaceAllowedApplications(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string) (*Application, error) {
	allowedApplicationIDs = normalizeAllowedApplicationIDs(allowedApplicationIDs)
	if len(allowedApplicationIDs) == 0 {
		return s.GetByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
	}
	if !applicationIDAllowed(id, allowedApplicationIDs) {
		return nil, gorm.ErrRecordNotFound
	}
	return s.GetByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
}

func (s *ApplicationService) LoadWritableClientByIDForOwner(ctx context.Context, id, ownerUserID string) (*model.OAuthClient, error) {
	orgIDs, err := s.accessibleOrganizationIDs(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.GetAccessibleByUser(ctx, id, ownerUserID, orgIDs)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCanWriteClient(ctx, ownerUserID, client); err != nil {
		return nil, err
	}
	return client, nil
}

func (s *ApplicationService) LoadWritableClientByIDForWorkspaceAllowedApplications(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string) (*model.OAuthClient, error) {
	allowedApplicationIDs = normalizeAllowedApplicationIDs(allowedApplicationIDs)
	if len(allowedApplicationIDs) > 0 && !applicationIDAllowed(id, allowedApplicationIDs) {
		return nil, gorm.ErrRecordNotFound
	}
	return s.requireClientInWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
}

func (s *ApplicationService) RotateSecretByIDForOwner(ctx context.Context, id, ownerUserID string) (*Application, string, error) {
	client, err := s.LoadWritableClientByIDForOwner(ctx, id, ownerUserID)
	if err != nil {
		return nil, "", err
	}
	secret, err := s.rotateClientSecret(ctx, client)
	if err != nil {
		return nil, "", err
	}
	app, err := s.toApplication(ctx, ownerUserID, client)
	if err != nil {
		return nil, "", err
	}
	return app, secret, nil
}

func (s *ApplicationService) RotateSecretByIDForWorkspaceAllowedApplications(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string) (*Application, string, error) {
	client, err := s.LoadWritableClientByIDForWorkspaceAllowedApplications(ctx, id, workspaceType, ownerUserID, organizationID, allowedApplicationIDs)
	if err != nil {
		return nil, "", err
	}
	secret, err := s.rotateClientSecret(ctx, client)
	if err != nil {
		return nil, "", err
	}
	app, err := s.toApplicationForWorkspace(ctx, client)
	if err != nil {
		return nil, "", err
	}
	return app, secret, nil
}

func (s *ApplicationService) UpdateByIDForOwner(ctx context.Context, input UpdateApplicationInput) (*Application, error) {
	orgIDs, err := s.accessibleOrganizationIDs(ctx, input.OwnerUserID)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.GetAccessibleByUser(ctx, input.ID, input.OwnerUserID, orgIDs)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCanWriteClient(ctx, input.OwnerUserID, client); err != nil {
		return nil, err
	}

	if input.OrganizationID != nil {
		workspace, err := s.resolveWriteWorkspace(ctx, input.OwnerUserID, *input.OrganizationID)
		if err != nil {
			return nil, err
		}
		client.OrganizationID = workspace.OrganizationID
	}

	if input.Name != nil {
		client.Name = strings.TrimSpace(*input.Name)
	}
	if input.RedirectURIs != nil {
		client.RedirectURIs = strings.TrimSpace(*input.RedirectURIs)
	}
	if input.GrantTypes != nil {
		client.GrantTypes = defaultIfEmpty(*input.GrantTypes, "authorization_code")
	}
	if input.Scopes != nil {
		client.Scopes = defaultIfEmpty(*input.Scopes, "openid profile email")
	}
	if input.RequireConsent != nil {
		client.RequireConsent = *input.RequireConsent
	}
	if err := s.enforceOrganizationPolicy(ctx, client); err != nil {
		return nil, err
	}

	if err := s.clients.Update(ctx, client); err != nil {
		return nil, err
	}
	app, err := s.toApplication(ctx, input.OwnerUserID, client)
	if err != nil {
		return nil, err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationUpdated, input.OwnerUserID, app)
	return app, nil
}

func (s *ApplicationService) UpdateByIDForWorkspace(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, input UpdateApplicationInput) (*Application, error) {
	client, err := s.requireClientInWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
	if err != nil {
		return nil, err
	}

	if input.OrganizationID != nil {
		nextOrgID := strings.TrimSpace(*input.OrganizationID)
		if client.OrganizationID != nextOrgID {
			return nil, ErrApplicationWorkspaceForbidden
		}
	}
	if input.Name != nil {
		client.Name = strings.TrimSpace(*input.Name)
	}
	if input.RedirectURIs != nil {
		client.RedirectURIs = strings.TrimSpace(*input.RedirectURIs)
	}
	if input.GrantTypes != nil {
		client.GrantTypes = defaultIfEmpty(*input.GrantTypes, "authorization_code")
	}
	if input.Scopes != nil {
		client.Scopes = defaultIfEmpty(*input.Scopes, "openid profile email")
	}
	if input.RequireConsent != nil {
		client.RequireConsent = *input.RequireConsent
	}
	if err := s.enforceOrganizationPolicy(ctx, client); err != nil {
		return nil, err
	}

	if err := s.clients.Update(ctx, client); err != nil {
		return nil, err
	}
	app, err := s.toApplicationForWorkspace(ctx, client)
	if err != nil {
		return nil, err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationUpdated, ownerUserID, app)
	return app, nil
}

func (s *ApplicationService) UpdateByIDForWorkspaceBoundToApplication(ctx context.Context, id, workspaceType, ownerUserID, organizationID, boundApplicationID string, input UpdateApplicationInput) (*Application, error) {
	return s.UpdateByIDForWorkspaceAllowedApplications(ctx, id, workspaceType, ownerUserID, organizationID, singleAllowedApplicationID(boundApplicationID), input)
}

func (s *ApplicationService) UpdateByIDForWorkspaceAllowedApplications(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string, input UpdateApplicationInput) (*Application, error) {
	allowedApplicationIDs = normalizeAllowedApplicationIDs(allowedApplicationIDs)
	if len(allowedApplicationIDs) == 0 {
		return s.UpdateByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID, input)
	}
	if !applicationIDAllowed(id, allowedApplicationIDs) {
		return nil, gorm.ErrRecordNotFound
	}
	return s.UpdateByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID, input)
}

func (s *ApplicationService) DeleteByIDForOwner(ctx context.Context, id, ownerUserID string) error {
	orgIDs, err := s.accessibleOrganizationIDs(ctx, ownerUserID)
	if err != nil {
		return err
	}
	client, err := s.clients.GetAccessibleByUser(ctx, id, ownerUserID, orgIDs)
	if err != nil {
		return err
	}
	if err := s.ensureCanWriteClient(ctx, ownerUserID, client); err != nil {
		return err
	}
	app, err := s.toApplication(ctx, ownerUserID, client)
	if err != nil {
		return err
	}
	if err := s.clients.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationDeleted, ownerUserID, app)
	return nil
}

func (s *ApplicationService) DeleteByIDForWorkspace(ctx context.Context, id, workspaceType, ownerUserID, organizationID string) error {
	client, err := s.requireClientInWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
	if err != nil {
		return err
	}
	app, err := s.toApplicationForWorkspace(ctx, client)
	if err != nil {
		return err
	}
	if err := s.clients.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.publishApplicationEvent(ctx, model.EventApplicationDeleted, ownerUserID, app)
	return nil
}

func (s *ApplicationService) DeleteByIDForWorkspaceBoundToApplication(ctx context.Context, id, workspaceType, ownerUserID, organizationID, boundApplicationID string) error {
	return s.DeleteByIDForWorkspaceAllowedApplications(ctx, id, workspaceType, ownerUserID, organizationID, singleAllowedApplicationID(boundApplicationID))
}

func (s *ApplicationService) DeleteByIDForWorkspaceAllowedApplications(ctx context.Context, id, workspaceType, ownerUserID, organizationID string, allowedApplicationIDs []string) error {
	allowedApplicationIDs = normalizeAllowedApplicationIDs(allowedApplicationIDs)
	if len(allowedApplicationIDs) == 0 {
		return s.DeleteByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
	}
	if !applicationIDAllowed(id, allowedApplicationIDs) {
		return gorm.ErrRecordNotFound
	}
	return s.DeleteByIDForWorkspace(ctx, id, workspaceType, ownerUserID, organizationID)
}

func (s *ApplicationService) toApplication(ctx context.Context, actorUserID string, client *model.OAuthClient) (*Application, error) {
	app := &Application{
		ID:             client.ID,
		Name:           client.Name,
		ClientID:       client.ClientID,
		RedirectURIs:   client.RedirectURIs,
		GrantTypes:     client.GrantTypes,
		Scopes:         client.Scopes,
		RequireConsent: client.RequireConsent,
		OrganizationID: client.OrganizationID,
	}
	workspace, err := s.workspaceSummary(ctx, actorUserID, client)
	if err != nil {
		return nil, err
	}
	app.Workspace = workspace
	return app, nil
}

func (s *ApplicationService) enforceOrganizationPolicy(ctx context.Context, client *model.OAuthClient) error {
	if s.policies == nil || client == nil || strings.TrimSpace(client.OrganizationID) == "" {
		return nil
	}
	policy, err := s.policies.GetByOrganizationID(ctx, client.OrganizationID)
	if err != nil {
		return err
	}
	if policy != nil && policy.EnforceRequireConsent {
		client.RequireConsent = true
	}
	return nil
}

func (s *ApplicationService) toApplicationForWorkspace(ctx context.Context, client *model.OAuthClient) (*Application, error) {
	app := &Application{
		ID:             client.ID,
		Name:           client.Name,
		ClientID:       client.ClientID,
		RedirectURIs:   client.RedirectURIs,
		GrantTypes:     client.GrantTypes,
		Scopes:         client.Scopes,
		RequireConsent: client.RequireConsent,
		OrganizationID: client.OrganizationID,
	}
	workspace, err := s.workspaceSummary(ctx, "", client)
	if err != nil {
		return nil, err
	}
	app.Workspace = workspace
	return app, nil
}

func defaultIfEmpty(raw, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

func normalizeApplicationWorkspace(workspaceType, ownerUserID, organizationID string) (string, string, string, error) {
	workspaceType = strings.TrimSpace(workspaceType)
	ownerUserID = strings.TrimSpace(ownerUserID)
	organizationID = strings.TrimSpace(organizationID)
	switch workspaceType {
	case "", model.WorkspaceTypePersonal:
		if ownerUserID == "" {
			return "", "", "", ErrApplicationWorkspaceForbidden
		}
		return model.WorkspaceTypePersonal, ownerUserID, "", nil
	case model.WorkspaceTypeOrganization:
		if organizationID == "" {
			return "", "", "", ErrApplicationWorkspaceForbidden
		}
		return model.WorkspaceTypeOrganization, ownerUserID, organizationID, nil
	default:
		return "", "", "", ErrApplicationWorkspaceForbidden
	}
}

func generateClientSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate client secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func normalizeAllowedApplicationIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, id)
	}
	return items
}

func singleAllowedApplicationID(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return []string{id}
}

func applicationIDAllowed(id string, allowedApplicationIDs []string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	return slices.Contains(allowedApplicationIDs, id)
}

func paginateApplicationSlice(page, pageSize, total int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func (s *ApplicationService) accessibleOrganizationIDs(ctx context.Context, userID string) ([]string, error) {
	if s.organizations == nil {
		return nil, nil
	}
	orgs, err := s.organizations.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(orgs))
	for _, org := range orgs {
		ids = append(ids, org.ID)
	}
	slices.Sort(ids)
	return ids, nil
}

func (s *ApplicationService) workspaceSummary(ctx context.Context, actorUserID string, client *model.OAuthClient) (WorkspaceSummary, error) {
	if client.OrganizationID == "" {
		return WorkspaceSummary{Type: model.WorkspaceTypePersonal}, nil
	}
	if s.organizations == nil {
		return WorkspaceSummary{Type: model.WorkspaceTypeOrganization, OrganizationID: client.OrganizationID}, nil
	}
	org, err := s.organizations.GetByID(ctx, client.OrganizationID)
	if err != nil {
		return WorkspaceSummary{}, err
	}
	workspace := WorkspaceSummary{
		Type:             model.WorkspaceTypeOrganization,
		OrganizationID:   org.ID,
		OrganizationName: org.Name,
		OrganizationSlug: org.Slug,
	}
	if actorUserID == "" {
		return workspace, nil
	}
	membership, err := s.organizations.GetMembership(ctx, client.OrganizationID, actorUserID)
	if err != nil {
		return WorkspaceSummary{}, err
	}
	workspace.Role = membership.Role
	return workspace, nil
}

func (s *ApplicationService) ensureCanWriteClient(ctx context.Context, actorUserID string, client *model.OAuthClient) error {
	if client.OrganizationID == "" {
		if client.UserID != actorUserID {
			return ErrApplicationWorkspaceForbidden
		}
		return nil
	}
	if s.organizations == nil {
		return ErrApplicationWorkspaceForbidden
	}
	membership, err := s.organizations.GetMembership(ctx, client.OrganizationID, actorUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationWorkspaceForbidden
		}
		return err
	}
	if membership.Role != model.OrganizationRoleOwner && membership.Role != model.OrganizationRoleAdmin {
		return ErrApplicationWorkspaceForbidden
	}
	return nil
}

func (s *ApplicationService) requireClientInWorkspace(ctx context.Context, id, workspaceType, ownerUserID, organizationID string) (*model.OAuthClient, error) {
	workspaceType, ownerUserID, organizationID, err := normalizeApplicationWorkspace(workspaceType, ownerUserID, organizationID)
	if err != nil {
		return nil, err
	}
	client, err := s.clients.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if workspaceType == model.WorkspaceTypePersonal {
		if client.OrganizationID != "" || client.UserID != ownerUserID {
			return nil, ErrApplicationWorkspaceForbidden
		}
		return client, nil
	}
	if client.OrganizationID != organizationID {
		return nil, ErrApplicationWorkspaceForbidden
	}
	return client, nil
}

func (s *ApplicationService) rotateClientSecret(ctx context.Context, client *model.OAuthClient) (string, error) {
	secret, err := generateClientSecret()
	if err != nil {
		return "", err
	}
	if err := client.SetSecret(secret); err != nil {
		return "", err
	}
	if err := s.clients.Update(ctx, client); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *ApplicationService) resolveWriteWorkspace(ctx context.Context, actorUserID, organizationID string) (WorkspaceSummary, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return WorkspaceSummary{Type: "personal"}, nil
	}
	if s.organizations == nil {
		return WorkspaceSummary{}, ErrApplicationWorkspaceForbidden
	}
	org, err := s.organizations.GetByID(ctx, organizationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WorkspaceSummary{}, ErrApplicationWorkspaceForbidden
		}
		return WorkspaceSummary{}, err
	}
	membership, err := s.organizations.GetMembership(ctx, organizationID, actorUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WorkspaceSummary{}, ErrApplicationWorkspaceForbidden
		}
		return WorkspaceSummary{}, err
	}
	if membership.Role != model.OrganizationRoleOwner && membership.Role != model.OrganizationRoleAdmin {
		return WorkspaceSummary{}, ErrApplicationWorkspaceForbidden
	}
	return WorkspaceSummary{
		Type:             "organization",
		OrganizationID:   org.ID,
		OrganizationName: org.Name,
		OrganizationSlug: org.Slug,
		Role:             membership.Role,
	}, nil
}

func (s *ApplicationService) publishApplicationEvent(ctx context.Context, eventType, actorUserID string, app *Application) error {
	if s.events == nil || app == nil {
		return nil
	}
	return s.events.Publish(ctx, PublishDomainEventInput{
		EventType:      eventType,
		WorkspaceType:  app.Workspace.Type,
		OrganizationID: app.OrganizationID,
		ActorUserID:    actorUserID,
		ResourceType:   "application",
		ResourceID:     app.ID,
		Payload: map[string]any{
			"type": eventType,
			"workspace": map[string]any{
				"type":              app.Workspace.Type,
				"organization_id":   app.Workspace.OrganizationID,
				"organization_slug": app.Workspace.OrganizationSlug,
			},
			"resource": map[string]any{
				"type": "application",
				"id":   app.ID,
				"name": app.Name,
			},
		},
	})
}
