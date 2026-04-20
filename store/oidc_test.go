package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

func newTestOIDCStorage(t *testing.T) *OIDCStorage {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	cache := NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })
	s, err := NewOIDCStorage(context.Background(), db, cache, NewUserStore(db), NewOAuthClientStore(db), "http://localhost:3206/login", "", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOIDCStorage_KeySetIncludesCurrentSigningKey(t *testing.T) {
	s := newTestOIDCStorage(t)

	keys, err := s.KeySet(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 signing key in key set, got %d", len(keys))
	}
	if keys[0].ID() != s.signingKey.id {
		t.Fatalf("expected key id %q, got %q", s.signingKey.id, keys[0].ID())
	}
}

func TestOIDCStorage_RotateSigningKeyKeepsPreviousKeyInKeySet(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()
	oldID := s.signingKey.id

	if err := s.db.WithContext(ctx).Model(&model.SigningKey{}).Where("id = ?", oldID).Update("created_at", time.Now().Add(-31*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.rotateSigningKey(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}

	if s.signingKey.id == oldID {
		t.Fatalf("expected signing key to rotate, still using %q", oldID)
	}

	keys, err := s.KeySet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected key set to include old and new keys, got %d", len(keys))
	}

	hasOld := false
	hasNew := false
	for _, k := range keys {
		if k.ID() == oldID {
			hasOld = true
		}
		if k.ID() == s.signingKey.id {
			hasNew = true
		}
	}
	if !hasOld || !hasNew {
		t.Fatalf("expected key set to contain both old and new keys (old=%v,new=%v)", hasOld, hasNew)
	}
}

func TestNewOIDCStorage(t *testing.T) {
	s := newTestOIDCStorage(t)
	if s.signingKey.privateKey == nil {
		t.Fatal("expected signing key to be generated")
	}
	if s.signingKey.id == "" {
		t.Fatal("expected signing key ID to be set")
	}
}

func TestCreateAndGetAuthRequest(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Nonce:        "test-nonce",
		State:        "test-state",
	}

	ar, err := s.CreateAuthRequest(ctx, req, "")
	if err != nil {
		t.Fatal(err)
	}
	if ar.GetID() == "" {
		t.Fatal("expected auth request ID")
	}
	if ar.GetClientID() != "test-client" {
		t.Errorf("expected client_id 'test-client', got %q", ar.GetClientID())
	}
	if ar.GetNonce() != "test-nonce" {
		t.Errorf("expected nonce 'test-nonce', got %q", ar.GetNonce())
	}
	if ar.Done() {
		t.Error("expected auth request to not be done")
	}

	// Retrieve by ID.
	got, err := s.AuthRequestByID(ctx, ar.GetID())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetID() != ar.GetID() {
		t.Errorf("expected ID %q, got %q", ar.GetID(), got.GetID())
	}
}

func TestCreateAuthRequestPersistsPromptConsent(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}

	ar, err := s.CreateAuthRequest(ctx, req, "")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := s.cache.Get(ctx, oidcAuthRequestKey(ar.GetID()))
	if err != nil {
		t.Fatalf("get cached auth request: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal cached auth request: %v", err)
	}

	gotPrompt, _ := payload["prompt"].(string)
	if gotPrompt != string(oidc.PromptConsent) {
		t.Fatalf("expected cached prompt %q, got %q", oidc.PromptConsent, gotPrompt)
	}
}

func TestAuthRequestByID_NotFound(t *testing.T) {
	s := newTestOIDCStorage(t)
	_, err := s.AuthRequestByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent auth request")
	}
}

func TestSaveAuthCodeAndRetrieve(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")

	if err := s.SaveAuthCode(ctx, ar.GetID(), "test-code-123"); err != nil {
		t.Fatal(err)
	}

	got, err := s.AuthRequestByCode(ctx, "test-code-123")
	if err != nil {
		t.Fatal(err)
	}
	if got.GetID() != ar.GetID() {
		t.Errorf("expected ID %q, got %q", ar.GetID(), got.GetID())
	}
}

func TestAuthRequestByCode_NotFound(t *testing.T) {
	s := newTestOIDCStorage(t)
	_, err := s.AuthRequestByCode(context.Background(), "nonexistent-code")
	if err == nil {
		t.Fatal("expected error for nonexistent code")
	}
}

func TestDeleteAuthRequest(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.SaveAuthCode(ctx, ar.GetID(), "code-to-delete")

	if err := s.DeleteAuthRequest(ctx, ar.GetID()); err != nil {
		t.Fatal(err)
	}

	_, err := s.AuthRequestByID(ctx, ar.GetID())
	if err == nil {
		t.Fatal("expected error after deletion")
	}

	_, err = s.AuthRequestByCode(ctx, "code-to-delete")
	if err == nil {
		t.Fatal("expected code to be cleaned up after deletion")
	}
}

func TestAuthRequestStoredOutsideDatabase(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
		Nonce:        "test-nonce",
		State:        "test-state",
	}

	ar, err := s.CreateAuthRequest(ctx, req, "")
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AuthRequest{}).Where("id = ?", ar.GetID()).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected auth request to avoid database persistence, got %d rows", count)
	}

	got, err := s.AuthRequestByID(ctx, ar.GetID())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetNonce() != "test-nonce" || got.GetState() != "test-state" {
		t.Fatalf("expected cached auth request fields to round-trip, got nonce=%q state=%q", got.GetNonce(), got.GetState())
	}
}

func TestCompleteAuthRequest(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")

	if err := s.CompleteAuthRequest(ctx, ar.GetID(), "user-123"); err != nil {
		t.Fatal(err)
	}

	got, _ := s.AuthRequestByID(ctx, ar.GetID())
	if !got.Done() {
		t.Error("expected auth request to be done")
	}
	if got.GetSubject() != "user-123" {
		t.Errorf("expected subject 'user-123', got %q", got.GetSubject())
	}
}

func TestCompleteAuthRequest_NotFound(t *testing.T) {
	s := newTestOIDCStorage(t)
	err := s.CompleteAuthRequest(context.Background(), "nonexistent", "user-123")
	if err == nil {
		t.Fatal("expected error for nonexistent auth request")
	}
}

func TestCreateAccessToken(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())

	tr, ok := ar.(op.TokenRequest)
	if !ok {
		t.Fatal("auth request does not implement op.TokenRequest")
	}
	tokenID, expiry, err := s.CreateAccessToken(ctx, tr)
	if err != nil {
		t.Fatal(err)
	}
	if tokenID == "" {
		t.Fatal("expected token ID")
	}
	if expiry.Before(time.Now()) {
		t.Error("expected expiry to be in the future")
	}
}

func TestOIDCStorageCreateAccessTokenSetsUserSubjectType(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())

	tokenID, _, err := s.CreateAccessToken(ctx, ar.(op.TokenRequest))
	if err != nil {
		t.Fatal(err)
	}

	if !s.db.Migrator().HasColumn(&model.Token{}, "subject_type") {
		t.Fatal("expected subject_type column on tokens table")
	}
	if !s.db.Migrator().HasColumn(&model.Token{}, "subject_id") {
		t.Fatal("expected subject_id column on tokens table")
	}

	var token struct {
		UserID      string
		SubjectType string
		SubjectID   string
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT user_id, subject_type, subject_id FROM tokens WHERE token_id = ?",
		tokenID,
	).Scan(&token).Error; err != nil {
		t.Fatalf("query persisted access token: %v", err)
	}
	if token.SubjectType != "user" {
		t.Fatalf("expected subject_type=user, got %q", token.SubjectType)
	}
	if token.SubjectID != "user-123" {
		t.Fatalf("expected subject_id=user-123, got %q", token.SubjectID)
	}
	if token.UserID != "user-123" {
		t.Fatalf("expected user_id=user-123 for compatibility, got %q", token.UserID)
	}
}

func TestCreateAccessAndRefreshTokens(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID()) // re-fetch to get updated userID

	tokenID, refreshToken, expiry, err := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")
	if err != nil {
		t.Fatal(err)
	}
	if tokenID == "" {
		t.Fatal("expected token ID")
	}
	if refreshToken == "" {
		t.Fatal("expected refresh token")
	}
	if expiry.Before(time.Now()) {
		t.Error("expected expiry to be in the future")
	}

	// Verify refresh token can be retrieved.
	rt, err := s.TokenRequestByRefreshToken(ctx, refreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if rt.GetSubject() != "user-123" {
		t.Errorf("expected subject 'user-123', got %q", rt.GetSubject())
	}
}

func TestOIDCStorageCreateAccessAndRefreshTokensSetUserSubjectType(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())

	tokenID, refreshToken, _, err := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")
	if err != nil {
		t.Fatal(err)
	}

	if !s.db.Migrator().HasColumn(&model.Token{}, "subject_type") {
		t.Fatal("expected subject_type column on tokens table")
	}
	if !s.db.Migrator().HasColumn(&model.Token{}, "subject_id") {
		t.Fatal("expected subject_id column on tokens table")
	}

	type storedToken struct {
		TokenType   string
		UserID      string
		SubjectType string
		SubjectID   string
	}
	var tokens []storedToken
	if err := s.db.WithContext(ctx).Raw(
		"SELECT token_type, user_id, subject_type, subject_id FROM tokens WHERE token_id = ? OR refresh_token = ? ORDER BY token_type ASC",
		tokenID,
		refreshToken,
	).Scan(&tokens).Error; err != nil {
		t.Fatalf("query stored tokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 stored tokens, got %d", len(tokens))
	}
	for _, token := range tokens {
		if token.SubjectType != "user" {
			t.Fatalf("expected %s token subject_type=user, got %q", token.TokenType, token.SubjectType)
		}
		if token.SubjectID != "user-123" {
			t.Fatalf("expected %s token subject_id=user-123, got %q", token.TokenType, token.SubjectID)
		}
		if token.UserID != "user-123" {
			t.Fatalf("expected %s token user_id=user-123, got %q", token.TokenType, token.UserID)
		}
	}
}

func TestTokenRequestByRefreshToken_NotFound(t *testing.T) {
	s := newTestOIDCStorage(t)
	_, err := s.TokenRequestByRefreshToken(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent refresh token")
	}
}

func TestRevokeToken(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())

	tokenID, refreshToken, _, _ := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")

	// Revoke access token.
	if oidcErr := s.RevokeToken(ctx, tokenID, "user-123", "test-client"); oidcErr != nil {
		t.Fatalf("unexpected error revoking access token: %v", oidcErr)
	}

	// Revoke refresh token.
	if oidcErr := s.RevokeToken(ctx, refreshToken, "user-123", "test-client"); oidcErr != nil {
		t.Fatalf("unexpected error revoking refresh token: %v", oidcErr)
	}

	// Revoking nonexistent token should not error (RFC 7009).
	if oidcErr := s.RevokeToken(ctx, "nonexistent", "", ""); oidcErr != nil {
		t.Fatalf("unexpected error revoking nonexistent token: %v", oidcErr)
	}
}

func TestOIDCStorageSetIntrospectionFromTokenSupportsOAuthClientSubject(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	token := &model.Token{
		TokenID:     "client-token-1",
		SubjectType: "oauth_client",
		SubjectID:   "client-1",
		ClientID:    "client-1",
		Scopes:      "read write",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		t.Fatalf("create client token: %v", err)
	}

	resp := &oidc.IntrospectionResponse{}
	if err := s.SetIntrospectionFromToken(ctx, resp, token.TokenID, token.SubjectID, "caller-client"); err != nil {
		t.Fatalf("introspect client token: %v", err)
	}

	if !resp.Active {
		t.Fatal("expected introspection response to be active")
	}
	if resp.Subject != "client-1" {
		t.Fatalf("expected sub=client-1, got %q", resp.Subject)
	}
	if resp.ClientID != "client-1" {
		t.Fatalf("expected client_id=client-1, got %q", resp.ClientID)
	}
	if resp.Username != "" {
		t.Fatalf("expected empty username for client token, got %q", resp.Username)
	}
	if len(resp.Scope) != 2 || resp.Scope[0] != "read" || resp.Scope[1] != "write" {
		t.Fatalf("expected scope [read write], got %v", []string(resp.Scope))
	}
}

func TestOIDCStorageSetIntrospectionFromTokenUsesPersistedTokenClientID(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	user := &model.User{
		Username:   "introspect-user",
		Email:      "introspect-user@example.com",
		Provider:   "local",
		ProviderID: "introspect-user",
		InviteCode: "INV-introspect-user",
	}
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token := &model.Token{
		TokenID:     "user-token-1",
		UserID:      user.ID,
		SubjectType: "user",
		SubjectID:   user.ID,
		ClientID:    "stored-client",
		Scopes:      "openid profile",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		t.Fatalf("create user token: %v", err)
	}

	resp := &oidc.IntrospectionResponse{}
	if err := s.SetIntrospectionFromToken(ctx, resp, token.TokenID, user.ID, "caller-client"); err != nil {
		t.Fatalf("introspect user token: %v", err)
	}

	if resp.ClientID != "stored-client" {
		t.Fatalf("expected client_id=stored-client, got %q", resp.ClientID)
	}
}

func TestOIDCStorageRevokeTokenOnlyDeletesOwnedClientToken(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	owned := &model.Token{
		TokenID:     "owned-token",
		SubjectType: "oauth_client",
		SubjectID:   "client-a",
		ClientID:    "client-a",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	foreign := &model.Token{
		TokenID:     "foreign-token",
		SubjectType: "oauth_client",
		SubjectID:   "client-b",
		ClientID:    "client-b",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(owned).Error; err != nil {
		t.Fatalf("create owned token: %v", err)
	}
	if err := s.db.WithContext(ctx).Create(foreign).Error; err != nil {
		t.Fatalf("create foreign token: %v", err)
	}

	if oidcErr := s.RevokeToken(ctx, owned.TokenID, "", "client-a"); oidcErr != nil {
		t.Fatalf("revoke owned token: %v", oidcErr)
	}
	if oidcErr := s.RevokeToken(ctx, foreign.TokenID, "", "client-a"); oidcErr != nil {
		t.Fatalf("revoke foreign token: %v", oidcErr)
	}

	var ownedCount int64
	if err := s.db.WithContext(ctx).Model(&model.Token{}).Where("token_id = ?", owned.TokenID).Count(&ownedCount).Error; err != nil {
		t.Fatalf("count owned token: %v", err)
	}
	if ownedCount != 0 {
		t.Fatalf("expected owned token to be deleted, count=%d", ownedCount)
	}

	var foreignCount int64
	if err := s.db.WithContext(ctx).Model(&model.Token{}).Where("token_id = ?", foreign.TokenID).Count(&foreignCount).Error; err != nil {
		t.Fatalf("count foreign token: %v", err)
	}
	if foreignCount != 1 {
		t.Fatalf("expected foreign token to remain, count=%d", foreignCount)
	}
}

func TestOIDCStorageFindUserByAccessTokenRejectsOAuthClientToken(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	token := &model.Token{
		TokenID:     "machine-token",
		SubjectType: "oauth_client",
		SubjectID:   "client-machine",
		ClientID:    "client-machine",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		t.Fatalf("create client token: %v", err)
	}

	if _, err := s.FindUserByAccessToken(ctx, token.TokenID); err == nil {
		t.Fatal("expected oauth client token to be rejected by user lookup")
	}
}

func TestTerminateSession(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())
	_, refreshToken, _, _ := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")

	if err := s.TerminateSession(ctx, "user-123", "test-client"); err != nil {
		t.Fatal(err)
	}

	// Refresh token should be gone.
	_, err := s.TokenRequestByRefreshToken(ctx, refreshToken)
	if err == nil {
		t.Fatal("expected refresh token to be revoked after session termination")
	}
}

func TestGetRefreshTokenInfo(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())
	_, refreshToken, _, _ := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")

	userID, token, err := s.GetRefreshTokenInfo(ctx, "test-client", refreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if userID != "user-123" {
		t.Errorf("expected userID 'user-123', got %q", userID)
	}
	if token != refreshToken {
		t.Errorf("expected token %q, got %q", refreshToken, token)
	}
}

func TestSigningKeyAndKeySet(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	sk, err := s.SigningKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sk.ID() == "" {
		t.Fatal("expected signing key ID")
	}
	if sk.Key() == nil {
		t.Fatal("expected signing key")
	}

	algs, err := s.SignatureAlgorithms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(algs) != 1 {
		t.Fatalf("expected 1 algorithm, got %d", len(algs))
	}

	keys, err := s.KeySet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].Use() != "sig" {
		t.Errorf("expected use 'sig', got %q", keys[0].Use())
	}
}

func TestHealth(t *testing.T) {
	s := newTestOIDCStorage(t)
	if err := s.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestOidcAuthRequest_Methods(t *testing.T) {
	ar := &oidcAuthRequest{
		id:           "req-1",
		clientID:     "client-1",
		redirectURI:  "http://localhost/cb",
		scopes:       []string{"openid", "email"},
		responseType: oidc.ResponseTypeCode,
		nonce:        "nonce-1",
		state:        "state-1",
		userID:       "user-1",
		done:         true,
		authTime:     time.Now(),
	}

	if ar.GetACR() != "" {
		t.Error("expected empty ACR")
	}
	if ar.GetAMR() != nil {
		t.Error("expected nil AMR")
	}
	if len(ar.GetAudience()) != 1 || ar.GetAudience()[0] != "client-1" {
		t.Errorf("unexpected audience: %v", ar.GetAudience())
	}
	if ar.GetCodeChallenge() != nil {
		t.Error("expected nil code challenge")
	}
	if ar.GetResponseMode() != "" {
		t.Error("expected empty response mode")
	}
}

func TestOidcClient_Methods(t *testing.T) {
	c := &oidcClient{
		client: &model.OAuthClient{
			ClientID:     "client-1",
			RedirectURIs: "http://localhost/cb,http://localhost/cb2",
			GrantTypes:   "authorization_code,refresh_token",
			Scopes:       "openid,profile,email",
		},
		loginURL:   "http://localhost:3206/login",
		idTokenTTL: 5 * time.Minute,
	}

	if c.GetID() != "client-1" {
		t.Errorf("expected ID 'client-1', got %q", c.GetID())
	}

	uris := c.RedirectURIs()
	if len(uris) != 2 {
		t.Fatalf("expected 2 redirect URIs, got %d", len(uris))
	}

	if c.PostLogoutRedirectURIs() != nil {
		t.Error("expected nil post-logout redirect URIs")
	}

	grantTypes := c.GrantTypes()
	if len(grantTypes) != 2 {
		t.Fatalf("expected 2 grant types, got %d", len(grantTypes))
	}

	loginURL := c.LoginURL("req-123")
	if loginURL != "http://localhost:3206/login?id=req-123" {
		t.Errorf("unexpected login URL: %s", loginURL)
	}

	if c.IDTokenLifetime() != 5*time.Minute {
		t.Errorf("unexpected ID token lifetime: %v", c.IDTokenLifetime())
	}

	if c.DevMode() {
		t.Error("expected DevMode to be false")
	}

	if !c.IsScopeAllowed("openid") {
		t.Error("expected 'openid' scope to be allowed")
	}
	if c.IsScopeAllowed("admin") {
		t.Error("expected 'admin' scope to not be allowed")
	}

	if c.IDTokenUserinfoClaimsAssertion() {
		t.Error("expected IDTokenUserinfoClaimsAssertion to be false")
	}

	if c.ClockSkew() != 0 {
		t.Errorf("expected zero clock skew, got %v", c.ClockSkew())
	}

	if c.RestrictAdditionalIdTokenScopes() != nil {
		t.Error("expected nil RestrictAdditionalIdTokenScopes")
	}
	if c.RestrictAdditionalAccessTokenScopes() != nil {
		t.Error("expected nil RestrictAdditionalAccessTokenScopes")
	}
}

func TestOidcClient_EmptyGrantTypes(t *testing.T) {
	c := &oidcClient{
		client: &model.OAuthClient{
			GrantTypes: "",
		},
	}
	grantTypes := c.GrantTypes()
	if len(grantTypes) != 1 || grantTypes[0] != oidc.GrantTypeCode {
		t.Errorf("expected default grant type [authorization_code], got %v", grantTypes)
	}
}

func TestOidcRefreshToken_Methods(t *testing.T) {
	rt := &oidcRefreshToken{
		id:       "rt-1",
		token:    "token-value",
		userID:   "user-1",
		clientID: "client-1",
		scopes:   []string{"openid"},
		authTime: time.Now(),
	}

	if rt.GetAMR() != nil {
		t.Error("expected nil AMR")
	}
	if len(rt.GetAudience()) != 1 || rt.GetAudience()[0] != "client-1" {
		t.Errorf("unexpected audience: %v", rt.GetAudience())
	}
	if rt.GetSubject() != "user-1" {
		t.Errorf("expected subject 'user-1', got %q", rt.GetSubject())
	}
	if rt.GetClientID() != "client-1" {
		t.Errorf("expected clientID 'client-1', got %q", rt.GetClientID())
	}

	rt.SetCurrentScopes([]string{"openid", "email"})
	if len(rt.GetScopes()) != 2 {
		t.Errorf("expected 2 scopes after SetCurrentScopes, got %d", len(rt.GetScopes()))
	}
}

func TestSplitTrimmed(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		got := splitTrimmed(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitTrimmed(%q): expected %v, got %v", tt.input, tt.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitTrimmed(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

func TestPopulateUserinfo(t *testing.T) {
	user := &model.User{
		Username:  "testuser",
		Email:     "test@example.com",
		AvatarURL: "https://example.com/avatar.png",
	}
	user.ID = "user-123"

	info := &oidc.UserInfo{}
	populateUserinfo(info, user, []string{"openid", "email", "profile"})

	if info.Subject != "user-123" {
		t.Errorf("expected subject 'user-123', got %q", info.Subject)
	}
	if info.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", info.Email)
	}
	if info.PreferredUsername != "testuser" {
		t.Errorf("expected preferred_username 'testuser', got %q", info.PreferredUsername)
	}
	if info.Picture != "https://example.com/avatar.png" {
		t.Errorf("expected picture URL, got %q", info.Picture)
	}
}

func TestGetPrivateClaimsFromScopes(t *testing.T) {
	// This test requires a UserStore, so we test the nil-user path.
	s := newTestOIDCStorage(t)
	claims, err := s.GetPrivateClaimsFromScopes(context.Background(), "nonexistent", "client", []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	// User not found returns empty claims, no error.
	if len(claims) != 0 {
		t.Errorf("expected empty claims for nonexistent user, got %v", claims)
	}
}

func TestValidateJWTProfileScopes(t *testing.T) {
	s := newTestOIDCStorage(t)
	scopes, err := s.ValidateJWTProfileScopes(context.Background(), "user", []string{"openid", "profile"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(scopes))
	}
}

func TestGetKeyByIDAndClientID(t *testing.T) {
	s := newTestOIDCStorage(t)
	_, err := s.GetKeyByIDAndClientID(context.Background(), "key", "client")
	if err == nil {
		t.Fatal("expected error for unimplemented method")
	}
}

func TestCreateAuthRequest_WithCodeChallenge(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost/callback",
		Scopes:              oidc.SpaceDelimitedArray{"openid"},
		ResponseType:        oidc.ResponseTypeCode,
		CodeChallenge:       "challenge-value",
		CodeChallengeMethod: oidc.CodeChallengeMethodS256,
	}

	ar, err := s.CreateAuthRequest(ctx, req, "")
	if err != nil {
		t.Fatal(err)
	}
	cc := ar.GetCodeChallenge()
	if cc == nil {
		t.Fatal("expected code challenge to be set")
	}
	if cc.Challenge != "challenge-value" {
		t.Errorf("expected challenge 'challenge-value', got %q", cc.Challenge)
	}
	if cc.Method != oidc.CodeChallengeMethodS256 {
		t.Errorf("expected method S256, got %v", cc.Method)
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	s := newTestOIDCStorage(t)
	ctx := context.Background()

	req := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	ar, _ := s.CreateAuthRequest(ctx, req, "")
	_ = s.CompleteAuthRequest(ctx, ar.GetID(), "user-123")
	ar, _ = s.AuthRequestByID(ctx, ar.GetID())

	// Create initial tokens.
	_, oldRefresh, _, _ := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), "")

	// Rotate: create new tokens with old refresh token.
	_, newRefresh, _, err := s.CreateAccessAndRefreshTokens(ctx, ar.(op.TokenRequest), oldRefresh)
	if err != nil {
		t.Fatal(err)
	}

	// Old refresh token should be revoked.
	_, err = s.TokenRequestByRefreshToken(ctx, oldRefresh)
	if err == nil {
		t.Fatal("expected old refresh token to be revoked")
	}

	// New refresh token should work.
	_, err = s.TokenRequestByRefreshToken(ctx, newRefresh)
	if err != nil {
		t.Fatalf("expected new refresh token to be valid: %v", err)
	}
}
