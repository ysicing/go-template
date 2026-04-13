package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

const maxApplicationsPerUser = 10

var ErrApplicationLimitReached = errors.New("maximum number of apps reached")

type applicationClientStore interface {
	Create(ctx context.Context, client *model.OAuthClient) error
	ListByUserID(ctx context.Context, userID string, page, pageSize int) ([]model.OAuthClient, int64, error)
	GetByIDAndUserID(ctx context.Context, id, userID string) (*model.OAuthClient, error)
	Update(ctx context.Context, client *model.OAuthClient) error
	Delete(ctx context.Context, id string) error
}

type Application struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ClientID       string `json:"client_id"`
	RedirectURIs   string `json:"redirect_uris"`
	GrantTypes     string `json:"grant_types"`
	Scopes         string `json:"scopes"`
	RequireConsent bool   `json:"require_consent"`
}

type CreateApplicationInput struct {
	OwnerUserID    string
	Name           string
	RedirectURIs   string
	GrantTypes     string
	Scopes         string
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
}

type ApplicationService struct {
	clients applicationClientStore
}

func NewApplicationService(clients applicationClientStore) *ApplicationService {
	return &ApplicationService{clients: clients}
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

	client := &model.OAuthClient{
		Name:           strings.TrimSpace(input.Name),
		ClientID:       uuid.NewString(),
		RedirectURIs:   strings.TrimSpace(input.RedirectURIs),
		GrantTypes:     defaultIfEmpty(input.GrantTypes, "authorization_code"),
		Scopes:         defaultIfEmpty(input.Scopes, "openid profile email"),
		RequireConsent: input.RequireConsent,
		UserID:         strings.TrimSpace(input.OwnerUserID),
	}
	if err := client.SetSecret(secret); err != nil {
		return nil, "", err
	}
	if err := s.clients.Create(ctx, client); err != nil {
		return nil, "", err
	}

	return toApplication(client), secret, nil
}

func (s *ApplicationService) ListByOwner(ctx context.Context, ownerUserID string, page, pageSize int) ([]Application, int64, error) {
	clients, total, err := s.clients.ListByUserID(ctx, ownerUserID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	apps := make([]Application, 0, len(clients))
	for index := range clients {
		if !isOwnedPersonalClient(&clients[index], ownerUserID) {
			continue
		}
		apps = append(apps, *toApplication(&clients[index]))
	}
	if int64(len(apps)) < total {
		total = int64(len(apps))
	}
	return apps, total, nil
}

func (s *ApplicationService) GetByIDForOwner(ctx context.Context, id, ownerUserID string) (*Application, error) {
	client, err := s.loadOwnedClient(ctx, id, ownerUserID)
	if err != nil {
		return nil, err
	}
	return toApplication(client), nil
}

func (s *ApplicationService) RotateSecretByIDForOwner(ctx context.Context, id, ownerUserID string) (*Application, string, error) {
	client, err := s.loadOwnedClient(ctx, id, ownerUserID)
	if err != nil {
		return nil, "", err
	}
	secret, err := s.rotateClientSecret(ctx, client)
	if err != nil {
		return nil, "", err
	}
	return toApplication(client), secret, nil
}

func (s *ApplicationService) UpdateByIDForOwner(ctx context.Context, input UpdateApplicationInput) (*Application, error) {
	client, err := s.loadOwnedClient(ctx, input.ID, input.OwnerUserID)
	if err != nil {
		return nil, err
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

	if err := s.clients.Update(ctx, client); err != nil {
		return nil, err
	}
	return toApplication(client), nil
}

func (s *ApplicationService) DeleteByIDForOwner(ctx context.Context, id, ownerUserID string) error {
	if _, err := s.loadOwnedClient(ctx, id, ownerUserID); err != nil {
		return err
	}
	return s.clients.Delete(ctx, id)
}

func (s *ApplicationService) loadOwnedClient(ctx context.Context, id, ownerUserID string) (*model.OAuthClient, error) {
	client, err := s.clients.GetByIDAndUserID(ctx, id, ownerUserID)
	if err != nil {
		return nil, err
	}
	if !isOwnedPersonalClient(client, ownerUserID) {
		return nil, gorm.ErrRecordNotFound
	}
	return client, nil
}

func isOwnedPersonalClient(client *model.OAuthClient, ownerUserID string) bool {
	if client == nil {
		return false
	}
	if strings.TrimSpace(client.OrganizationID) != "" {
		return false
	}
	return strings.TrimSpace(client.UserID) == strings.TrimSpace(ownerUserID)
}

func toApplication(client *model.OAuthClient) *Application {
	if client == nil {
		return nil
	}
	return &Application{
		ID:             client.ID,
		Name:           client.Name,
		ClientID:       client.ClientID,
		RedirectURIs:   client.RedirectURIs,
		GrantTypes:     client.GrantTypes,
		Scopes:         client.Scopes,
		RequireConsent: client.RequireConsent,
	}
}

func defaultIfEmpty(raw, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

func generateClientSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate client secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
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
