package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

func authRequestFromDB(ar *model.AuthRequest, loginURL string) *oidcAuthRequest {
	req := &oidcAuthRequest{
		id:           ar.ID,
		creationDate: ar.CreatedAt,
		clientID:     ar.ClientID,
		redirectURI:  ar.RedirectURI,
		scopes:       splitTrimmed(ar.Scopes),
		prompt:       splitTrimmed(ar.Prompt),
		responseType: oidc.ResponseType(ar.ResponseType),
		responseMode: oidc.ResponseMode(ar.ResponseMode),
		nonce:        ar.Nonce,
		state:        ar.State,
		userID:       ar.UserID,
		done:         ar.Done,
		authTime:     ar.AuthTime,
		loginURL:     loginURL,
	}
	if ar.CodeChallenge != "" {
		req.codeChallenge = &oidc.CodeChallenge{
			Challenge: ar.CodeChallenge,
			Method:    oidc.CodeChallengeMethod(ar.CodeChallengeMethod),
		}
	}
	return req
}

func (s *OIDCStorage) GetAuthRequestClientID(ctx context.Context, id string) string {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return ""
	}
	return ar.ClientID
}

func (s *OIDCStorage) CompleteAuthRequest(ctx context.Context, id, userID string) error {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return err
	}
	ar.UserID = userID
	ar.Done = true
	ar.AuthTime = time.Now()
	return s.storeAuthRequest(ctx, ar)
}

func (s *OIDCStorage) AssignAuthRequestUser(ctx context.Context, id, userID string) error {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return err
	}
	ar.UserID = userID
	ar.Done = false
	ar.AuthTime = time.Now()
	return s.storeAuthRequest(ctx, ar)
}

func (s *OIDCStorage) AuthRequestRequiresConsent(ctx context.Context, id string) bool {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return false
	}
	for _, prompt := range splitTrimmed(ar.Prompt) {
		if prompt == oidc.PromptConsent {
			return true
		}
	}
	return false
}

func (s *OIDCStorage) CreateAuthRequest(ctx context.Context, req *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	ar := model.AuthRequest{
		Base:         model.Base{ID: uuid.NewString(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ClientID:     req.ClientID,
		RedirectURI:  req.RedirectURI,
		Scopes:       strings.Join(req.Scopes, ","),
		Prompt:       strings.Join(req.Prompt, ","),
		State:        req.State,
		Nonce:        req.Nonce,
		ResponseType: string(req.ResponseType),
		ResponseMode: string(req.ResponseMode),
		ExpiresAt:    time.Now().Add(s.authRequestTTL),
	}
	if req.CodeChallenge != "" {
		ar.CodeChallenge = req.CodeChallenge
		ar.CodeChallengeMethod = string(req.CodeChallengeMethod)
	}
	if err := s.storeAuthRequest(ctx, &ar); err != nil {
		return nil, fmt.Errorf("create auth request: %w", err)
	}
	return authRequestFromDB(&ar, s.loginURL), nil
}

func (s *OIDCStorage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return nil, errors.New("auth request not found")
	}
	return authRequestFromDB(ar, s.loginURL), nil
}

func (s *OIDCStorage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	if s.cache == nil {
		return nil, errors.New("auth code not found")
	}
	id, err := s.cache.Get(ctx, oidcAuthRequestCodeKey(code))
	if err != nil {
		return nil, errors.New("auth code not found")
	}
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return nil, errors.New("auth code not found")
	}
	return authRequestFromDB(ar, s.loginURL), nil
}

func (s *OIDCStorage) SaveAuthCode(ctx context.Context, id, code string) error {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return errors.New("auth request not found")
	}
	if ar.Code != "" && s.cache != nil {
		_, _ = s.cache.DelIfValue(ctx, oidcAuthRequestCodeKey(ar.Code), id)
	}
	ar.Code = code
	ar.UpdatedAt = time.Now()
	if err := s.storeAuthRequest(ctx, ar); err != nil {
		return err
	}
	return s.cache.Set(ctx, oidcAuthRequestCodeKey(code), id, time.Until(ar.ExpiresAt))
}

func (s *OIDCStorage) DeleteAuthRequest(ctx context.Context, id string) error {
	ar, err := s.loadAuthRequest(ctx, id)
	if err != nil {
		return nil
	}
	if s.cache != nil {
		_, _ = s.cache.DelIfValue(ctx, oidcAuthRequestKey(id), string(mustMarshalAuthRequest(ar)))
		if ar.Code != "" {
			_, _ = s.cache.DelIfValue(ctx, oidcAuthRequestCodeKey(ar.Code), id)
		}
	}
	return nil
}

func oidcAuthRequestKey(id string) string {
	return "oidc:auth_request:" + id
}

func oidcAuthRequestCodeKey(code string) string {
	return "oidc:auth_request:code:" + code
}

func (s *OIDCStorage) loadAuthRequest(ctx context.Context, id string) (*model.AuthRequest, error) {
	if s.cache == nil {
		return nil, errors.New("auth request not found")
	}
	raw, err := s.cache.Get(ctx, oidcAuthRequestKey(id))
	if err != nil {
		return nil, err
	}
	var ar model.AuthRequest
	if err := json.Unmarshal([]byte(raw), &ar); err != nil {
		return nil, err
	}
	if !ar.ExpiresAt.IsZero() && time.Now().After(ar.ExpiresAt) {
		_ = s.DeleteAuthRequest(ctx, id)
		return nil, errors.New("auth request not found")
	}
	return &ar, nil
}

func (s *OIDCStorage) storeAuthRequest(ctx context.Context, ar *model.AuthRequest) error {
	if s.cache == nil {
		return errors.New("auth request cache unavailable")
	}
	if ar == nil {
		return errors.New("auth request is nil")
	}
	if ar.UpdatedAt.IsZero() {
		ar.UpdatedAt = time.Now()
	}
	raw, err := json.Marshal(ar)
	if err != nil {
		return err
	}
	ttl := time.Until(ar.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	return s.cache.Set(ctx, oidcAuthRequestKey(ar.ID), string(raw), ttl)
}

func mustMarshalAuthRequest(ar *model.AuthRequest) []byte {
	raw, _ := json.Marshal(ar)
	return raw
}
