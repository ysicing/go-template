package handler

import (
	"context"
	"strings"

	"github.com/ysicing/go-template/store"

	"github.com/zitadel/oidc/v3/pkg/op"
)

type oidcConsentStorage interface {
	AuthRequestRequiresConsent(ctx context.Context, id string) bool
	AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error)
}

func shouldPromptOIDCConsent(ctx context.Context, storage oidcConsentStorage, clients *store.OAuthClientStore, grants *store.OAuthConsentGrantStore, authRequestID, userID string) (bool, error) {
	if storage.AuthRequestRequiresConsent(ctx, authRequestID) {
		return true, nil
	}
	authReq, err := storage.AuthRequestByID(ctx, authRequestID)
	if err != nil {
		return false, err
	}
	client, err := clients.GetByClientID(ctx, authReq.GetClientID())
	if err != nil {
		return false, err
	}
	if !client.RequireConsent {
		return false, nil
	}
	if grants == nil {
		return true, nil
	}
	granted, err := grants.HasGrantedScopes(ctx, userID, client.ClientID, authReq.GetScopes())
	if err != nil {
		return false, err
	}
	return !granted, nil
}

func oidcConsentContextRequired(ctx context.Context, storage oidcConsentStorage, clients *store.OAuthClientStore, authRequestID string) (bool, error) {
	if storage.AuthRequestRequiresConsent(ctx, authRequestID) {
		return true, nil
	}
	authReq, err := storage.AuthRequestByID(ctx, authRequestID)
	if err != nil {
		return false, err
	}
	client, err := clients.GetByClientID(ctx, authReq.GetClientID())
	if err != nil {
		return false, err
	}
	return client.RequireConsent, nil
}

func oidcConsentPending(ctx context.Context, storage oidcConsentStorage, clients *store.OAuthClientStore, authRequestID string) (bool, error) {
	authReq, err := storage.AuthRequestByID(ctx, authRequestID)
	if err != nil {
		return false, err
	}
	if authReq.Done() || strings.TrimSpace(authReq.GetSubject()) == "" {
		return false, nil
	}
	required, err := oidcConsentContextRequired(ctx, storage, clients, authRequestID)
	if err != nil {
		return false, err
	}
	return required, nil
}
