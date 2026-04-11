package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"

	jose "github.com/go-jose/go-jose/v4"
)

// ---------------------------------------------------------------------------
// oidcAuthRequest implements op.AuthRequest
// ---------------------------------------------------------------------------

type oidcAuthRequest struct {
	id            string
	creationDate  time.Time
	clientID      string
	redirectURI   string
	scopes        []string
	prompt        []string
	responseType  oidc.ResponseType
	responseMode  oidc.ResponseMode
	nonce         string
	state         string
	codeChallenge *oidc.CodeChallenge
	userID        string
	done          bool
	authTime      time.Time
	loginURL      string
}

func (a *oidcAuthRequest) GetID() string                         { return a.id }
func (a *oidcAuthRequest) GetACR() string                        { return "" }
func (a *oidcAuthRequest) GetAMR() []string                      { return nil }
func (a *oidcAuthRequest) GetAudience() []string                 { return []string{a.clientID} }
func (a *oidcAuthRequest) GetAuthTime() time.Time                { return a.authTime }
func (a *oidcAuthRequest) GetClientID() string                   { return a.clientID }
func (a *oidcAuthRequest) GetCodeChallenge() *oidc.CodeChallenge { return a.codeChallenge }
func (a *oidcAuthRequest) GetNonce() string                      { return a.nonce }
func (a *oidcAuthRequest) GetRedirectURI() string                { return a.redirectURI }
func (a *oidcAuthRequest) GetResponseType() oidc.ResponseType    { return a.responseType }
func (a *oidcAuthRequest) GetResponseMode() oidc.ResponseMode    { return a.responseMode }
func (a *oidcAuthRequest) GetScopes() []string                   { return a.scopes }
func (a *oidcAuthRequest) GetState() string                      { return a.state }
func (a *oidcAuthRequest) GetSubject() string                    { return a.userID }
func (a *oidcAuthRequest) Done() bool                            { return a.done }

// ---------------------------------------------------------------------------
// oidcClient implements op.Client
// ---------------------------------------------------------------------------

type oidcClient struct {
	client     *model.OAuthClient
	loginURL   string
	idTokenTTL time.Duration
}

func (c *oidcClient) GetID() string { return c.client.ClientID }

func (c *oidcClient) RedirectURIs() []string {
	return splitTrimmed(c.client.RedirectURIs)
}

func (c *oidcClient) PostLogoutRedirectURIs() []string { return nil }

func (c *oidcClient) ApplicationType() op.ApplicationType { return op.ApplicationTypeWeb }

func (c *oidcClient) AuthMethod() oidc.AuthMethod { return oidc.AuthMethodBasic }

func (c *oidcClient) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}

func (c *oidcClient) GrantTypes() []oidc.GrantType {
	raw := splitTrimmed(c.client.GrantTypes)
	types := make([]oidc.GrantType, 0, len(raw))
	for _, g := range raw {
		types = append(types, oidc.GrantType(g))
	}
	if len(types) == 0 {
		types = append(types, oidc.GrantTypeCode)
	}
	return types
}

func (c *oidcClient) LoginURL(id string) string {
	return c.loginURL + "?id=" + id
}

func (c *oidcClient) AccessTokenType() op.AccessTokenType { return op.AccessTokenTypeBearer }

func (c *oidcClient) IDTokenLifetime() time.Duration { return c.idTokenTTL }

func (c *oidcClient) DevMode() bool { return false }

func (c *oidcClient) RestrictAdditionalIdTokenScopes() func([]string) []string { return nil }

func (c *oidcClient) RestrictAdditionalAccessTokenScopes() func([]string) []string { return nil }

func (c *oidcClient) IsScopeAllowed(scope string) bool {
	allowed := splitTrimmed(c.client.Scopes)
	for _, s := range allowed {
		if s == scope {
			return true
		}
	}
	return false
}

func (c *oidcClient) IDTokenUserinfoClaimsAssertion() bool { return false }

func (c *oidcClient) ClockSkew() time.Duration { return 0 }

// ---------------------------------------------------------------------------
// oidcRefreshToken implements op.RefreshTokenRequest
// ---------------------------------------------------------------------------

type oidcRefreshToken struct {
	id       string
	token    string
	userID   string
	clientID string
	scopes   []string
	authTime time.Time
	expiry   time.Time
}

func (r *oidcRefreshToken) GetAMR() []string                 { return nil }
func (r *oidcRefreshToken) GetAudience() []string            { return []string{r.clientID} }
func (r *oidcRefreshToken) GetAuthTime() time.Time           { return r.authTime }
func (r *oidcRefreshToken) GetClientID() string              { return r.clientID }
func (r *oidcRefreshToken) GetScopes() []string              { return r.scopes }
func (r *oidcRefreshToken) GetSubject() string               { return r.userID }
func (r *oidcRefreshToken) SetCurrentScopes(scopes []string) { r.scopes = scopes }

// ---------------------------------------------------------------------------
// signingKeyData implements op.SigningKey
// ---------------------------------------------------------------------------

type signingKeyData struct {
	id         string
	algorithm  jose.SignatureAlgorithm
	privateKey *rsa.PrivateKey
}

func (s *signingKeyData) SignatureAlgorithm() jose.SignatureAlgorithm { return s.algorithm }
func (s *signingKeyData) Key() any                                    { return s.privateKey }
func (s *signingKeyData) ID() string                                  { return s.id }

// ---------------------------------------------------------------------------
// publicKeyData implements op.Key
// ---------------------------------------------------------------------------

type publicKeyData struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PublicKey
}

func (p *publicKeyData) ID() string                         { return p.id }
func (p *publicKeyData) Algorithm() jose.SignatureAlgorithm { return p.algorithm }
func (p *publicKeyData) Use() string                        { return "sig" }
func (p *publicKeyData) Key() any                           { return p.key }

// ---------------------------------------------------------------------------
// OIDCStorage implements op.Storage (DB-backed)
// ---------------------------------------------------------------------------

const (
	oidcSigningKeyRotationInterval = 30 * 24 * time.Hour
	oidcSigningKeyRetainDuration   = 90 * 24 * time.Hour
)

type OIDCStorage struct {
	db              *gorm.DB
	cache           Cache
	signingKey      signingKeyData
	encPassphrase   string // Passphrase for encrypting signing key at rest
	users           *UserStore
	clients         *OAuthClientStore
	loginURL        string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	authRequestTTL  time.Duration

	signingMu sync.RWMutex
}

// NewOIDCStorage creates a new DB-backed OIDCStorage.
func NewOIDCStorage(ctx context.Context, db *gorm.DB, cache Cache, users *UserStore, clients *OAuthClientStore, loginURL string, encryptionKey string, accessTTL, refreshTTL, authReqTTL time.Duration) (*OIDCStorage, error) {
	if accessTTL == 0 {
		accessTTL = 5 * time.Minute
	}
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	if authReqTTL == 0 {
		authReqTTL = 10 * time.Minute
	}
	s := &OIDCStorage{
		db:              db,
		cache:           cache,
		users:           users,
		clients:         clients,
		loginURL:        loginURL,
		encPassphrase:   encryptionKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
		authRequestTTL:  authReqTTL,
	}
	if err := s.loadOrCreateSigningKey(ctx); err != nil {
		return nil, err
	}
	go s.cleanupLoop(ctx)
	return s, nil
}

func (s *OIDCStorage) loadOrCreateSigningKey(ctx context.Context) error {
	// Try loading existing key first.
	var sk model.SigningKey
	if err := s.db.WithContext(ctx).Order("created_at desc").First(&sk).Error; err == nil {
		return s.parseSigningKey(&sk)
	}

	return s.rotateSigningKey(ctx, time.Now())
}

func (s *OIDCStorage) rotateSigningKey(ctx context.Context, now time.Time) error {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("generate rsa key: %w", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}
	pemData := string(pem.EncodeToMemory(pemBlock))

	storedKey := pemData
	if s.encPassphrase != "" {
		encrypted, err := crypto.Encrypt(s.encPassphrase, pemData)
		if err != nil {
			return fmt.Errorf("encrypt signing key: %w", err)
		}
		storedKey = encrypted
	}

	newSigningKey := model.SigningKey{
		ID:         uuid.New().String(),
		Algorithm:  string(jose.RS256),
		PrivateKey: storedKey,
	}

	if err := s.db.WithContext(ctx).Create(&newSigningKey).Error; err != nil {
		return fmt.Errorf("store signing key: %w", err)
	}

	s.signingMu.Lock()
	s.signingKey = signingKeyData{
		id:         newSigningKey.ID,
		algorithm:  jose.RS256,
		privateKey: key,
	}
	s.signingMu.Unlock()

	if err := s.db.WithContext(ctx).Where("created_at < ?", now.Add(-oidcSigningKeyRetainDuration)).Delete(&model.SigningKey{}).Error; err != nil {
		return fmt.Errorf("cleanup old signing keys: %w", err)
	}

	return nil
}

func (s *OIDCStorage) ensureSigningKeyFresh(ctx context.Context, now time.Time) error {
	s.signingMu.RLock()
	currentID := s.signingKey.id
	s.signingMu.RUnlock()
	if currentID == "" {
		return s.loadOrCreateSigningKey(ctx)
	}

	var current model.SigningKey
	if err := s.db.WithContext(ctx).Where("id = ?", currentID).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.loadOrCreateSigningKey(ctx)
		}
		return fmt.Errorf("load signing key metadata: %w", err)
	}

	if current.CreatedAt.IsZero() {
		seeded := now.Add(-oidcSigningKeyRotationInterval - time.Minute)
		if err := s.db.WithContext(ctx).Model(&model.SigningKey{}).Where("id = ?", current.ID).Update("created_at", seeded).Error; err != nil {
			return fmt.Errorf("seed legacy signing key created_at: %w", err)
		}
		current.CreatedAt = seeded
	}

	if now.Sub(current.CreatedAt) < oidcSigningKeyRotationInterval {
		return nil
	}

	return s.rotateSigningKey(ctx, now)
}

func (s *OIDCStorage) parseSigningKey(sk *model.SigningKey) error {
	// Decrypt if encrypted, otherwise use as-is (backward compatible).
	pemStr := sk.PrivateKey
	if s.encPassphrase != "" {
		decrypted, err := crypto.DecryptOrPlaintext(s.encPassphrase, pemStr)
		if err != nil {
			return fmt.Errorf("decrypt signing key: %w", err)
		}
		pemStr = decrypted
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	s.signingMu.Lock()
	s.signingKey = signingKeyData{
		id:         sk.ID,
		algorithm:  jose.SignatureAlgorithm(sk.Algorithm),
		privateKey: key,
	}
	s.signingMu.Unlock()
	return nil
}

func (s *OIDCStorage) parsePublicKey(sk *model.SigningKey) (op.Key, error) {
	// Decrypt if encrypted, otherwise use as-is.
	pemStr := sk.PrivateKey
	if s.encPassphrase != "" {
		decrypted, err := crypto.DecryptOrPlaintext(s.encPassphrase, pemStr)
		if err != nil {
			return nil, fmt.Errorf("decrypt signing key: %w", err)
		}
		pemStr = decrypted
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &publicKeyData{
		id:        sk.ID,
		algorithm: jose.SignatureAlgorithm(sk.Algorithm),
		key:       &privateKey.PublicKey,
	}, nil
}

// cleanupLoop periodically removes expired auth requests and tokens.
func (s *OIDCStorage) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		now := time.Now()
		if err := s.ensureSigningKeyFresh(ctx, now); err != nil {
			// Keep serving with the current key if rotation fails.
		}
		s.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&model.Token{})
	}
}

// ---------------------------------------------------------------------------
// DB ↔ adapter helpers
// ---------------------------------------------------------------------------

func authRequestFromDB(ar *model.AuthRequest, loginURL string) *oidcAuthRequest {
	scopes := splitTrimmed(ar.Scopes)
	req := &oidcAuthRequest{
		id:           ar.ID,
		creationDate: ar.CreatedAt,
		clientID:     ar.ClientID,
		redirectURI:  ar.RedirectURI,
		scopes:       scopes,
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

func refreshTokenFromDB(t *model.Token) *oidcRefreshToken {
	subjectID := t.SubjectID
	if subjectID == "" {
		subjectID = t.UserID
	}
	return &oidcRefreshToken{
		id:       t.ID,
		token:    t.RefreshToken,
		userID:   subjectID,
		clientID: t.ClientID,
		scopes:   splitTrimmed(t.Scopes),
		authTime: t.CreatedAt,
		expiry:   t.ExpiresAt,
	}
}

// ---------------------------------------------------------------------------
// GetAuthRequestClientID / CompleteAuthRequest (used by login handler)
// ---------------------------------------------------------------------------

// GetAuthRequestClientID returns the OAuth client ID for the given auth request.
func (s *OIDCStorage) GetAuthRequestClientID(id string) string {
	ar, err := s.loadAuthRequest(context.Background(), id)
	if err != nil {
		return ""
	}
	return ar.ClientID
}

// CompleteAuthRequest marks an auth request as done with the given userID.
func (s *OIDCStorage) CompleteAuthRequest(id, userID string) error {
	ar, err := s.loadAuthRequest(context.Background(), id)
	if err != nil {
		return err
	}
	ar.UserID = userID
	ar.Done = true
	ar.AuthTime = time.Now()
	return s.storeAuthRequest(context.Background(), ar)
}

func (s *OIDCStorage) AssignAuthRequestUser(id, userID string) error {
	ar, err := s.loadAuthRequest(context.Background(), id)
	if err != nil {
		return err
	}
	ar.UserID = userID
	ar.Done = false
	ar.AuthTime = time.Now()
	return s.storeAuthRequest(context.Background(), ar)
}

func (s *OIDCStorage) AuthRequestRequiresConsent(id string) bool {
	ar, err := s.loadAuthRequest(context.Background(), id)
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

// --- op.Storage: AuthStorage methods ---

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

func (s *OIDCStorage) CreateAccessToken(ctx context.Context, req op.TokenRequest) (string, time.Time, error) {
	tokenID := uuid.New().String()
	expiry := time.Now().Add(s.accessTokenTTL)

	clientID := ""
	if aud := req.GetAudience(); len(aud) > 0 {
		clientID = aud[0]
	}

	subjectID := req.GetSubject()
	t := model.Token{
		TokenID:     tokenID,
		UserID:      subjectID,
		SubjectType: "user",
		SubjectID:   subjectID,
		ClientID:    clientID,
		Scopes:      strings.Join(req.GetScopes(), ","),
		TokenType:   "access",
		ExpiresAt:   expiry,
	}
	if err := s.db.WithContext(ctx).Create(&t).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create access token: %w", err)
	}
	return tokenID, expiry, nil
}

func (s *OIDCStorage) CreateAccessAndRefreshTokens(ctx context.Context, req op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	tokenID := uuid.New().String()
	expiry := time.Now().Add(s.accessTokenTTL)

	clientID := ""
	if aud := req.GetAudience(); len(aud) > 0 {
		clientID = aud[0]
	}
	scopes := strings.Join(req.GetScopes(), ",")
	subjectID := req.GetSubject()

	accessToken := model.Token{
		TokenID:     tokenID,
		UserID:      subjectID,
		SubjectType: "user",
		SubjectID:   subjectID,
		ClientID:    clientID,
		Scopes:      scopes,
		TokenType:   "access",
		ExpiresAt:   expiry,
	}

	refreshTokenStr := uuid.New().String()
	refreshToken := model.Token{
		TokenID:      uuid.New().String(),
		UserID:       subjectID,
		SubjectType:  "user",
		SubjectID:    subjectID,
		ClientID:     clientID,
		Scopes:       scopes,
		TokenType:    "refresh",
		RefreshToken: refreshTokenStr,
		ExpiresAt:    time.Now().Add(s.refreshTokenTTL),
	}

	// Use transaction to ensure atomicity
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Revoke old refresh token if provided (token rotation)
		if currentRefreshToken != "" {
			if err := tx.Unscoped().Where("refresh_token = ? AND token_type = ?", currentRefreshToken, "refresh").Delete(&model.Token{}).Error; err != nil {
				return fmt.Errorf("revoke old refresh token: %w", err)
			}
		}

		// Create access token
		if err := tx.Create(&accessToken).Error; err != nil {
			return fmt.Errorf("create access token: %w", err)
		}

		// Create refresh token
		if err := tx.Create(&refreshToken).Error; err != nil {
			return fmt.Errorf("create refresh token: %w", err)
		}

		return nil
	})

	if err != nil {
		return "", "", time.Time{}, err
	}

	return tokenID, refreshTokenStr, expiry, nil
}

func (s *OIDCStorage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("refresh_token = ? AND token_type = ? AND revoked = false", refreshToken, "refresh").First(&t).Error; err != nil {
		return nil, errors.New("refresh token not found")
	}
	if time.Now().After(t.ExpiresAt) {
		s.db.WithContext(ctx).Where("id = ?", t.ID).Delete(&model.Token{})
		return nil, errors.New("refresh token expired")
	}
	return refreshTokenFromDB(&t), nil
}

func (s *OIDCStorage) TerminateSession(ctx context.Context, userID, clientID string) error {
	q := s.db.WithContext(ctx).Unscoped().Where("user_id = ?", userID)
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	return q.Delete(&model.Token{}).Error
}

func (s *OIDCStorage) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
	query := s.db.WithContext(ctx).Where("token_id = ? OR refresh_token = ?", tokenOrTokenID, tokenOrTokenID)
	if clientID != "" {
		query = query.Where("client_id = ?", clientID)
	}

	var token model.Token
	if err := query.First(&token).Error; err != nil {
		return nil // RFC 7009: invalid tokens are not an error
	}

	_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.Token{}, "id = ?", token.ID).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{
			Action:     model.AuditOAuthTokenRevoke,
			Resource:   "oauth_token",
			ResourceID: token.TokenID,
			ClientID:   clientID,
			Detail:     "oauth token revoked",
			Status:     "success",
		}).Error
	})
	return nil // RFC 7009: invalid tokens are not an error
}

func (s *OIDCStorage) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (string, string, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("refresh_token = ?", token).First(&t).Error; err != nil {
		return "", "", errors.New("refresh token not found")
	}
	return t.UserID, t.RefreshToken, nil
}

func (s *OIDCStorage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}

	s.signingMu.RLock()
	key := s.signingKey
	s.signingMu.RUnlock()
	return &key, nil
}

func (s *OIDCStorage) SignatureAlgorithms(ctx context.Context) ([]jose.SignatureAlgorithm, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}

	s.signingMu.RLock()
	algorithm := s.signingKey.algorithm
	s.signingMu.RUnlock()
	return []jose.SignatureAlgorithm{algorithm}, nil
}

func (s *OIDCStorage) KeySet(ctx context.Context) ([]op.Key, error) {
	if err := s.ensureSigningKeyFresh(ctx, time.Now()); err != nil {
		return nil, err
	}

	var signingKeys []model.SigningKey
	if err := s.db.WithContext(ctx).
		Where("created_at >= ?", time.Now().Add(-oidcSigningKeyRetainDuration)).
		Order("created_at desc").
		Find(&signingKeys).Error; err != nil {
		return nil, fmt.Errorf("load signing key set: %w", err)
	}

	keys := make([]op.Key, 0, len(signingKeys))
	for i := range signingKeys {
		pub, err := s.parsePublicKey(&signingKeys[i])
		if err != nil {
			return nil, err
		}
		keys = append(keys, pub)
	}

	if len(keys) == 0 {
		s.signingMu.RLock()
		current := s.signingKey
		s.signingMu.RUnlock()
		if current.privateKey != nil {
			keys = append(keys, &publicKeyData{
				id:        current.id,
				algorithm: current.algorithm,
				key:       &current.privateKey.PublicKey,
			})
		}
	}

	return keys, nil
}

// --- op.Storage: OPStorage methods ---

func (s *OIDCStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	client, err := s.clients.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	return &oidcClient{client: client, loginURL: s.loginURL, idTokenTTL: s.accessTokenTTL}, nil
}

func (s *OIDCStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	client, err := s.clients.GetByClientID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("client not found: %w", err)
	}
	if !client.CheckSecret(clientSecret) {
		return errors.New("invalid client secret")
	}
	return nil
}

func (s *OIDCStorage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, _ string, scopes []string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	populateUserinfo(userinfo, user, scopes)
	return nil
}

func (s *OIDCStorage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, _ string) error {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenID).First(&t).Error; err != nil {
		return errors.New("token not found")
	}
	if !isUserSubjectToken(&t) {
		return errors.New("token subject is not a user")
	}

	user, err := s.users.GetByID(ctx, persistedSubjectID(&t, subject))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	populateUserinfo(userinfo, user, splitTrimmed(t.Scopes))
	return nil
}

func (s *OIDCStorage) SetIntrospectionFromToken(ctx context.Context, resp *oidc.IntrospectionResponse, tokenID, subject, _ string) error {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenID).First(&t).Error; err != nil {
		return nil
	}
	if time.Now().After(t.ExpiresAt) || t.Revoked {
		return nil
	}

	resp.Active = true
	resp.Subject = persistedSubjectID(&t, subject)
	resp.ClientID = t.ClientID
	resp.Scope = oidc.SpaceDelimitedArray(splitTrimmed(t.Scopes))
	resp.TokenType = t.TokenType

	if !isUserSubjectToken(&t) {
		return nil
	}

	user, err := s.users.GetByID(ctx, persistedSubjectID(&t, subject))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	resp.Subject = user.ID
	resp.Username = user.Username
	return nil
}

func (s *OIDCStorage) GetPrivateClaimsFromScopes(ctx context.Context, userID, _ string, scopes []string) (map[string]any, error) {
	if s.users == nil {
		return nil, nil
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil
	}
	claims := make(map[string]any)
	for _, scope := range scopes {
		if scope == "admin" && user.IsAdmin {
			claims["is_admin"] = true
		}
	}
	return claims, nil
}

func (s *OIDCStorage) GetKeyByIDAndClientID(_ context.Context, keyID, _ string) (*jose.JSONWebKey, error) {
	return nil, errors.New("not implemented")
}

func (s *OIDCStorage) ValidateJWTProfileScopes(_ context.Context, _ string, scopes []string) ([]string, error) {
	return scopes, nil
}

func (s *OIDCStorage) Health(_ context.Context) error {
	return nil
}

// FindUserByAccessToken looks up the user associated with an OIDC access token.
func (s *OIDCStorage) FindUserByAccessToken(ctx context.Context, tokenID string) (*model.User, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).
		Where("token_id = ? AND expires_at > ?", tokenID, time.Now()).
		Where("(subject_type = ? OR subject_type = ? OR subject_type IS NULL)", "", "user").
		First(&t).Error; err != nil {
		return nil, errors.New("token not found")
	}
	if t.UserID == "" {
		return nil, errors.New("token subject is not a user")
	}
	return s.users.GetByID(ctx, t.UserID)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func populateUserinfo(info *oidc.UserInfo, user *model.User, scopes []string) {
	info.Subject = user.ID
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			info.Subject = user.ID
		case oidc.ScopeEmail:
			info.Email = user.Email
			info.EmailVerified = oidc.Bool(user.EmailVerified)
		case oidc.ScopeProfile:
			info.PreferredUsername = user.Username
			info.Name = user.Username
			if user.AvatarURL != "" {
				info.Picture = user.AvatarURL
			}
		}
	}
}

func splitTrimmed(s string) []string {
	if s == "" {
		return nil
	}
	// Support both comma and space as separators (OIDC standard uses spaces).
	normalized := strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(normalized)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func isUserSubjectToken(token *model.Token) bool {
	return token.SubjectType == "" || token.SubjectType == "user"
}

func persistedSubjectID(token *model.Token, fallback string) string {
	if token.SubjectID != "" {
		return token.SubjectID
	}
	if token.UserID != "" {
		return token.UserID
	}
	return fallback
}
