package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"

	oauth2github "golang.org/x/oauth2/github"
)

// googleEndpoint defines Google OAuth2 endpoints.
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

type oauthUserStore interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
}

type oauthProviderStore interface {
	GetByName(ctx context.Context, name string) (*model.SocialProvider, error)
}

type oauthSocialAccountStore interface {
	GetByProviderAndID(ctx context.Context, provider, providerID string) (*model.SocialAccount, error)
	Create(ctx context.Context, account *model.SocialAccount) error
	Update(ctx context.Context, account *model.SocialAccount) error
}

type oauthWebAuthnCredStore interface {
	ListByUserID(ctx context.Context, userID string) ([]model.WebAuthnCredential, error)
	UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error
}

type oauthSettingStore interface {
	Get(key, defaultVal string) string
	GetBool(key string, defaultVal bool) bool
}

type oauthWebAuthnManager interface {
	BeginLogin(user *store.WebAuthnUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
	FinishLogin(user *store.WebAuthnUser, session webauthn.SessionData, body []byte) (*webauthn.Credential, error)
}

type defaultOAuthWebAuthnManager struct {
	settings oauthSettingStore
}

func (m defaultOAuthWebAuthnManager) BeginLogin(user *store.WebAuthnUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	wa, err := m.build()
	if err != nil {
		return nil, nil, err
	}
	return wa.BeginLogin(user)
}

func (m defaultOAuthWebAuthnManager) FinishLogin(user *store.WebAuthnUser, session webauthn.SessionData, body []byte) (*webauthn.Credential, error) {
	wa, err := m.build()
	if err != nil {
		return nil, err
	}
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return wa.ValidateLogin(user, session, parsedResponse)
}

func (m defaultOAuthWebAuthnManager) build() (*webauthn.WebAuthn, error) {
	if m.settings == nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	rpID := strings.TrimSpace(m.settings.Get(store.SettingWebAuthnRPID, ""))
	if rpID == "" {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	rpDisplay := m.settings.Get(store.SettingWebAuthnRPDisplay, "ID Service")
	rpOrigins := m.settings.Get(store.SettingWebAuthnRPOrigins, "")
	origins := make([]string, 0, 4)
	for origin := range strings.SplitSeq(rpOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	return webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplay,
		RPID:          rpID,
		RPOrigins:     origins,
	})
}

type socialLinkPendingData struct {
	UserID     string `json:"user_id"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Email      string `json:"email"`
	AvatarURL  string `json:"avatar_url"`
}

// OAuthDeps aggregates dependencies required by OAuthHandler.
type OAuthDeps struct {
	DB             *gorm.DB
	Users          oauthUserStore
	Providers      oauthProviderStore
	SocialAccounts oauthSocialAccountStore
	Audit          *store.AuditLogStore
	RefreshTokens  refreshTokenCreator
	Sessions       *service.SessionService
	MFA            mfaReader
	Cache          store.Cache
	Settings       oauthSettingStore
	WebAuthnCreds  oauthWebAuthnCredStore
	TokenConfig    TokenConfig
}

// OAuthHandler handles social login flows using database-managed provider configs.
type OAuthHandler struct {
	db             *gorm.DB
	users          oauthUserStore
	providers      oauthProviderStore
	socialAccounts oauthSocialAccountStore
	audit          *store.AuditLogStore
	sessions       *service.SessionService
	mfa            mfaReader
	cache          store.Cache
	settings       oauthSettingStore
	webAuthnCreds  oauthWebAuthnCredStore
	webAuthn       oauthWebAuthnManager
	tokenConfig    TokenConfig
}

// NewOAuthHandler creates an OAuthHandler with database-backed social providers.
func NewOAuthHandler(deps OAuthDeps) *OAuthHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = service.NewSessionService(deps.RefreshTokens, service.TokenConfig{
			Secret:        deps.TokenConfig.Secret,
			Issuer:        deps.TokenConfig.Issuer,
			AccessTTL:     deps.TokenConfig.AccessTTL,
			RefreshTTL:    deps.TokenConfig.RefreshTTL,
			RememberMeTTL: deps.TokenConfig.RememberMeTTL,
		})
	}

	return &OAuthHandler{
		db:             deps.DB,
		users:          deps.Users,
		providers:      deps.Providers,
		socialAccounts: deps.SocialAccounts,
		audit:          deps.Audit,
		sessions:       sessions,
		mfa:            deps.MFA,
		cache:          deps.Cache,
		settings:       deps.Settings,
		webAuthnCreds:  deps.WebAuthnCreds,
		webAuthn:       defaultOAuthWebAuthnManager{settings: deps.Settings},
		tokenConfig:    deps.TokenConfig,
	}
}

// linkOrCreateSocialUser handles user lookup, linking, or creation for social login.
func (h *OAuthHandler) linkOrCreateSocialUser(ctx context.Context, provider, providerID, email, username, avatarURL string) (*model.User, error) {
	// First, check if this social account already exists
	socialAccount, err := h.socialAccounts.GetByProviderAndID(ctx, provider, providerID)
	if err == nil {
		// Social account exists, return the linked user
		user, err := h.users.GetByID(ctx, socialAccount.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user")
		}
		// Update avatar if changed
		if avatarURL != "" && socialAccount.AvatarURL != avatarURL {
			socialAccount.AvatarURL = avatarURL
			_ = h.socialAccounts.Update(ctx, socialAccount)
		}
		return user, nil
	}

	// Social account doesn't exist, check if we should link or create user
	registerEnabled := h.settings.GetBool(store.SettingRegisterEnabled, true)

	if email == "" {
		return nil, fmt.Errorf("email_required")
	}

	// Try to find existing user by email
	existingUser, err := h.users.GetByEmail(ctx, email)
	if err == nil {
		// SECURITY: User exists with this email. To prevent account takeover via social login,
		// we require explicit confirmation before linking.
		//
		// Attack scenario: Attacker registers GitHub/Google account with victim's email,
		// then uses social login to hijack victim's account.
		//
		// Solution: Generate a pending link token and require password/TOTP verification.

		// Generate pending link token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return nil, fmt.Errorf("failed to generate link token")
		}
		linkToken := hex.EncodeToString(tokenBytes)

		// Store pending link data in cache (5 minutes)
		pendingData := map[string]string{
			"user_id":     existingUser.ID,
			"provider":    provider,
			"provider_id": providerID,
			"email":       email,
			"avatar_url":  avatarURL,
		}
		dataJSON, _ := json.Marshal(pendingData)
		if err := h.cache.Set(ctx, "social_link_pending:"+linkToken, string(dataJSON), 5*time.Minute); err != nil {
			return nil, fmt.Errorf("failed to store pending link")
		}

		// Return special error with link token
		return nil, fmt.Errorf("account_link_required:%s", linkToken)
	}

	// User doesn't exist, check if registration is enabled
	if !registerEnabled {
		return nil, fmt.Errorf("registration_disabled")
	}

	// Create new user with social login in a transaction
	user := &model.User{
		Username:      username,
		Email:         email,
		Provider:      provider,
		ProviderID:    providerID,
		AvatarURL:     avatarURL,
		EmailVerified: true,
	}

	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create user
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Create social account binding
		newSocialAccount := &model.SocialAccount{
			UserID:     user.ID,
			Provider:   provider,
			ProviderID: providerID,
			Email:      email,
			AvatarURL:  avatarURL,
		}
		if err := tx.Create(newSocialAccount).Error; err != nil {
			return fmt.Errorf("failed to create social account binding: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}

// saveOAuthState stores state in cache.
// linkUserID is optional; when set it is stored server-side so the state string itself
// does not embed sensitive user information.
func (h *OAuthHandler) saveOAuthState(c fiber.Ctx, state, linkUserID string) {
	val := "1"
	if linkUserID != "" {
		val = "link:" + linkUserID
	}
	_ = h.cache.Set(c.Context(), oauthStateKey(state), val, 5*time.Minute)
}

// verifyOAuthState validates state and returns (valid, linkUserID).
// Cookie fallback is intentionally removed to prevent CSRF attacks.
// SetNX ensures only one callback can consume a given state value (replay protection).
func (h *OAuthHandler) verifyOAuthState(c fiber.Ctx, state string) (bool, string) {
	if state == "" {
		return false, ""
	}
	val, err := h.cache.Get(c.Context(), oauthStateKey(state))
	if err != nil || val == "" {
		return false, ""
	}
	consumed, err := h.cache.SetNX(c.Context(), oauthStateUsedKey(state), "1", 5*time.Minute)
	if err != nil || !consumed {
		return false, ""
	}
	_ = h.cache.Del(c.Context(), oauthStateKey(state))

	linkUserID := ""
	if strings.HasPrefix(val, "link:") {
		linkUserID = strings.TrimPrefix(val, "link:")
	}
	return true, linkUserID
}

func oauthStateKey(state string) string     { return "oauth_state:" + state }
func oauthStateUsedKey(state string) string { return "oauth_state_used:" + state }
func socialLinkPendingKey(token string) string {
	return "social_link_pending:" + token
}
func socialLinkWebAuthnKey(token string) string {
	return "social_link_webauthn:" + token
}

// githubOAuth2Config builds an oauth2.Config for GitHub from a SocialProvider.
func (h *OAuthHandler) githubOAuth2Config(provider *model.SocialProvider, c fiber.Ctx) *oauth2.Config {
	redirectURL := provider.RedirectURL
	if redirectURL == "" {
		redirectURL = deriveRedirectURL(c, "/api/auth/github/callback")
	}
	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user:email"},
		Endpoint:     oauth2github.Endpoint,
	}
}

// googleOAuth2Config builds an oauth2.Config for Google from a SocialProvider.
func (h *OAuthHandler) googleOAuth2Config(provider *model.SocialProvider, c fiber.Ctx) *oauth2.Config {
	redirectURL := provider.RedirectURL
	if redirectURL == "" {
		redirectURL = deriveRedirectURL(c, "/api/auth/google/callback")
	}
	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     googleEndpoint,
	}
}

// generateOAuthState creates a cryptographically random state string for CSRF protection.
// The userID (for link operations) is stored server-side via saveOAuthState, not in the state value.
func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// deriveRedirectURL builds a redirect URL from the request's scheme and host.
func deriveRedirectURL(c fiber.Ctx, path string) string {
	scheme := c.Scheme()
	host := c.Hostname()
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// errProviderHandled is a sentinel indicating the response was already sent.
var errProviderHandled = errors.New("provider response handled")

// loadProvider loads a social provider by name and checks it is enabled.
// On failure it writes the HTTP response and returns errProviderHandled.
func (h *OAuthHandler) loadProvider(c fiber.Ctx, name string) (*model.SocialProvider, error) {
	provider, err := h.providers.GetByName(c.Context(), name)
	if err != nil {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": name + " oauth not configured"})
		return nil, errProviderHandled
	}
	if !provider.Enabled {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": name + " oauth not configured"})
		return nil, errProviderHandled
	}
	return provider, nil
}

// GitHubLogin redirects to GitHub OAuth authorization page.
func (h *OAuthHandler) GitHubLogin(c fiber.Ctx) error {
	provider, err := h.loadProvider(c, "github")
	if err != nil {
		return nil
	}
	cfg := h.githubOAuth2Config(provider, c)

	// Check if this is a link operation (user is already logged in)
	userID := ""
	if uid, ok := c.Locals("user_id").(string); ok {
		userID = uid
	}

	state, err := generateOAuthState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate state"})
	}
	h.saveOAuthState(c, state, userID)
	url := cfg.AuthCodeURL(state)
	return c.Redirect().To(url)
}

// GitHubCallback handles the GitHub OAuth callback.
func (h *OAuthHandler) GitHubCallback(c fiber.Ctx) error {
	provider, err := h.loadProvider(c, "github")
	if err != nil {
		return nil
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
	}

	state := c.Query("state")
	valid, linkUserID := h.verifyOAuthState(c, state)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid oauth state"})
	}
	isLink := linkUserID != ""

	cfg := h.githubOAuth2Config(provider, c)
	token, tokenErr := cfg.Exchange(c.Context(), code)
	if tokenErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to exchange token"})
	}

	ghUser, fetchErr := fetchGitHubUser(c.Context(), cfg, token)
	if fetchErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user info"})
	}

	providerID := fmt.Sprintf("%d", ghUser.ID)

	// Handle link operation
	if isLink && linkUserID != "" {
		return h.handleSocialLink(c, linkUserID, "github", providerID, ghUser.Email, ghUser.AvatarURL)
	}

	// Handle login operation
	user, lookupErr := h.users.GetByProviderID(c.Context(), "github", providerID)
	if lookupErr != nil {
		var err error
		user, err = h.linkOrCreateSocialUser(c.Context(), "github", providerID, ghUser.Email, "gh_"+ghUser.Login, ghUser.AvatarURL)
		if err != nil {
			errMsg := err.Error()
			if errMsg == "email_required" {
				return c.Redirect().To("/login?error=email_required")
			}
			if errMsg == "registration_disabled" {
				return c.Redirect().To("/login?error=registration_disabled")
			}
			// Check for account link required
			if strings.HasPrefix(errMsg, "account_link_required:") {
				linkToken := strings.TrimPrefix(errMsg, "account_link_required:")
				return c.Redirect().To("/login?link_required=true&link_token=" + linkToken + "&provider=github")
			}
			logger.L.Error().Err(err).Str("provider", "github").Msg("social login failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "social login failed"})
		}
	}

	return h.handleSocialCallback(c, user, "github")
}

// GoogleLogin redirects to Google OAuth authorization page.
func (h *OAuthHandler) GoogleLogin(c fiber.Ctx) error {
	provider, err := h.loadProvider(c, "google")
	if err != nil {
		return nil
	}
	cfg := h.googleOAuth2Config(provider, c)

	// Check if this is a link operation (user is already logged in)
	userID := ""
	if uid, ok := c.Locals("user_id").(string); ok {
		userID = uid
	}

	state, err := generateOAuthState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate state"})
	}
	h.saveOAuthState(c, state, userID)
	url := cfg.AuthCodeURL(state)
	return c.Redirect().To(url)
}

// GoogleCallback handles the Google OAuth callback.
func (h *OAuthHandler) GoogleCallback(c fiber.Ctx) error {
	provider, err := h.loadProvider(c, "google")
	if err != nil {
		return nil
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
	}

	state := c.Query("state")
	valid, linkUserID := h.verifyOAuthState(c, state)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid oauth state"})
	}
	isLink := linkUserID != ""

	cfg := h.googleOAuth2Config(provider, c)
	token, tokenErr := cfg.Exchange(c.Context(), code)
	if tokenErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to exchange token"})
	}

	gUser, fetchErr := fetchGoogleUser(c.Context(), cfg, token)
	if fetchErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user info"})
	}

	// Handle link operation
	if isLink && linkUserID != "" {
		return h.handleSocialLink(c, linkUserID, "google", gUser.ID, gUser.Email, gUser.Picture)
	}

	// Handle login operation
	user, lookupErr := h.users.GetByProviderID(c.Context(), "google", gUser.ID)
	if lookupErr != nil {
		var err error
		user, err = h.linkOrCreateSocialUser(c.Context(), "google", gUser.ID, gUser.Email, "g_"+gUser.ID, gUser.Picture)
		if err != nil {
			errMsg := err.Error()
			if errMsg == "email_required" {
				return c.Redirect().To("/login?error=email_required")
			}
			if errMsg == "registration_disabled" {
				return c.Redirect().To("/login?error=registration_disabled")
			}
			// Check for account link required
			if strings.HasPrefix(errMsg, "account_link_required:") {
				linkToken := strings.TrimPrefix(errMsg, "account_link_required:")
				return c.Redirect().To("/login?link_required=true&link_token=" + linkToken + "&provider=google")
			}
			logger.L.Error().Err(err).Str("provider", "google").Msg("social login failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "social login failed"})
		}
	}

	return h.handleSocialCallback(c, user, "google")
}

// handleSocialCallback stores user info in a temporary code and redirects to the SPA.
func (h *OAuthHandler) handleSocialCallback(c fiber.Ctx, user *model.User, provider string) error {
	if isAccountLocked(c.Context(), h.cache, user.ID) {
		return c.Redirect().To("/login?error=account_locked")
	}

	// Store a temporary exchange code in cache.
	code := store.GenerateRandomToken()
	_ = h.cache.Set(c.Context(), "oauth_code:"+code, user.ID+"|"+provider, 2*time.Minute)
	return c.Redirect().To("/login/callback?code=" + code)
}

// handleSocialLink handles linking a social account to an existing user.
func (h *OAuthHandler) handleSocialLink(c fiber.Ctx, userID, provider, providerID, email, avatarURL string) error {
	// Verify the user exists
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Redirect().To("/profile?error=user_not_found")
	}

	// Check if this social account is already linked to another user
	existingAccount, err := h.socialAccounts.GetByProviderAndID(c.Context(), provider, providerID)
	if err == nil && existingAccount.UserID != userID {
		return c.Redirect().To("/profile?error=account_already_linked")
	}

	// Create or update social account binding
	if err != nil {
		// Create new binding
		newAccount := &model.SocialAccount{
			UserID:     userID,
			Provider:   provider,
			ProviderID: providerID,
			Email:      email,
			AvatarURL:  avatarURL,
		}
		if err := h.socialAccounts.Create(c.Context(), newAccount); err != nil {
			return c.Redirect().To("/profile?error=failed_to_link")
		}
	} else {
		// Update existing binding
		existingAccount.Email = email
		existingAccount.AvatarURL = avatarURL
		_ = h.socialAccounts.Update(c.Context(), existingAccount)
	}

	// Update user avatar if empty
	if user.AvatarURL == "" && avatarURL != "" {
		user.AvatarURL = avatarURL
		_ = h.users.Update(c.Context(), user)
	}

	// Audit log
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   userID,
		Action:   model.AuditSocialAccountLink,
		Resource: "social_account",
		Status:   "success",
		Detail:   "social account linked",
		Metadata: map[string]string{
			"provider": provider,
		},
	})

	return c.Redirect().To("/profile?success=account_linked")
}

func (h *OAuthHandler) loadPendingSocialLink(ctx context.Context, linkToken string) (*socialLinkPendingData, error) {
	val, err := h.cache.Get(ctx, socialLinkPendingKey(linkToken))
	if err != nil || val == "" {
		return nil, errors.New("invalid_or_expired_link_token")
	}

	var pending socialLinkPendingData
	if err := json.Unmarshal([]byte(val), &pending); err != nil {
		return nil, errors.New("invalid_link_data")
	}
	return &pending, nil
}

func (h *OAuthHandler) loadSocialLinkWebAuthnUser(c fiber.Ctx, userID string) (*store.WebAuthnUser, error) {
	if h.webAuthnCreds == nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	creds, err := h.webAuthnCreds.ListByUserID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	return &store.WebAuthnUser{User: user, Creds: creds}, nil
}

func (h *OAuthHandler) completeSocialLink(c fiber.Ctx, user *model.User, pending *socialLinkPendingData, verificationMethod string, cleanupKeys ...string) error {
	newAccount := &model.SocialAccount{
		UserID:     user.ID,
		Provider:   pending.Provider,
		ProviderID: pending.ProviderID,
		Email:      pending.Email,
		AvatarURL:  pending.AvatarURL,
	}
	if err := h.socialAccounts.Create(c.Context(), newAccount); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to link social account"})
	}

	updated := false
	if user.AvatarURL == "" && pending.AvatarURL != "" {
		user.AvatarURL = pending.AvatarURL
		updated = true
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		updated = true
	}
	if updated {
		_ = h.users.Update(c.Context(), user)
	}

	for _, key := range cleanupKeys {
		if key == "" {
			continue
		}
		_ = h.cache.Del(c.Context(), key)
	}

	clearFailedAuthAttempts(c.Context(), h.cache, user.ID)

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     user.ID,
		Action:     model.AuditSocialAccountLink,
		Resource:   "social_account",
		ResourceID: newAccount.ID,
		Status:     "success",
		Detail:     "social account linked after verification",
		Metadata: map[string]string{
			"provider":            pending.Provider,
			"provider_id":         pending.ProviderID,
			"verification_method": verificationMethod,
		},
	})

	return h.respondWithTokens(c, user, pending.Provider)
}

// ExchangeCode handles POST /api/auth/social/exchange — exchanges a temporary code for tokens.
func (h *OAuthHandler) ExchangeCode(c fiber.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	val, err := h.cache.Get(c.Context(), "oauth_code:"+req.Code)
	if err != nil || val == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired code"})
	}
	_ = h.cache.Del(c.Context(), "oauth_code:"+req.Code)

	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid code data"})
	}
	userID, provider := parts[0], parts[1]

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	return h.respondWithTokens(c, user, provider)
}

// ConfirmSocialLink handles POST /api/auth/social/confirm-link — confirms linking a social account.
// Supports password or TOTP verification.
func (h *OAuthHandler) ConfirmSocialLink(c fiber.Ctx) error {
	var req struct {
		LinkToken string `json:"link_token"`
		Password  string `json:"password,omitempty"`
		TOTPCode  string `json:"totp_code,omitempty"`
		Challenge string `json:"challenge,omitempty"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.LinkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token is required"})
	}
	if req.Challenge != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "challenge verification is not supported"})
	}

	// At least one verification method required
	if req.Password == "" && req.TOTPCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "verification required: password or totp_code"})
	}

	// Retrieve pending link data
	pending, err := h.loadPendingSocialLink(c.Context(), req.LinkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}

	// Verify user exists
	user, err := h.users.GetByID(c.Context(), pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	// Check account lockout
	if isAccountLocked(c.Context(), h.cache, user.ID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	verified := false
	verificationMethod := ""

	// Try password verification
	if req.Password != "" {
		if user.PasswordHash == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "password_not_set",
				"hint":  "This account was created via social login. Please use TOTP or set a password first.",
			})
		}
		if user.CheckPassword(req.Password) {
			verified = true
			verificationMethod = "password"
		} else {
			_ = recordAuditFromFiber(c, h.audit, AuditEvent{
				UserID:   user.ID,
				Action:   model.AuditSocialAccountLink,
				Resource: "social_account",
				Status:   "failed",
				Detail:   "social link verification failed",
				Metadata: map[string]string{
					"reason": "invalid_password",
				},
			})
			recordFailedAuthAttempt(c.Context(), h.cache, user.ID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
		}
	}

	// Try TOTP verification
	if !verified && req.TOTPCode != "" {
		if h.mfa == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "MFA not available"})
		}
		mfaCfg, err := h.mfa.GetByUserID(c.Context(), user.ID)
		if err != nil || !mfaCfg.TOTPEnabled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "totp_not_enabled",
				"hint":  "TOTP is not enabled for this account. Please use password or set up TOTP first.",
			})
		}
		if totp.Validate(req.TOTPCode, mfaCfg.TOTPSecret) {
			verified = true
			verificationMethod = "totp"
		} else {
			_ = recordAuditFromFiber(c, h.audit, AuditEvent{
				UserID:   user.ID,
				Action:   model.AuditSocialAccountLink,
				Resource: "social_account",
				Status:   "failed",
				Detail:   "social link verification failed",
				Metadata: map[string]string{
					"reason": "invalid_totp",
				},
			})
			recordFailedAuthAttempt(c.Context(), h.cache, user.ID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid TOTP code"})
		}
	}

	if !verified {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "verification failed"})
	}

	return h.completeSocialLink(c, user, pending, verificationMethod, socialLinkPendingKey(req.LinkToken))
}

// SocialLinkWebAuthnBegin handles POST /api/auth/social/confirm-link/webauthn/begin.
func (h *OAuthHandler) SocialLinkWebAuthnBegin(c fiber.Ctx) error {
	var req struct {
		LinkToken string `json:"link_token"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.LinkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token is required"})
	}
	if h.webAuthn == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	pending, err := h.loadPendingSocialLink(c.Context(), req.LinkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}
	if isAccountLocked(c.Context(), h.cache, pending.UserID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	waUser, err := h.loadSocialLinkWebAuthnUser(c, pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	if len(waUser.Creds) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webauthn_not_enabled",
			"hint":  "WebAuthn is not enabled for this account. Please use password or TOTP first.",
		})
	}

	options, session, err := h.webAuthn.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	sessionJSON, _ := json.Marshal(session)
	if err := h.cache.Set(c.Context(), socialLinkWebAuthnKey(req.LinkToken), string(sessionJSON), 5*time.Minute); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist webauthn session"})
	}

	return c.JSON(fiber.Map{"publicKey": options.Response})
}

// SocialLinkWebAuthnFinish handles POST /api/auth/social/confirm-link/webauthn/finish.
func (h *OAuthHandler) SocialLinkWebAuthnFinish(c fiber.Ctx) error {
	linkToken := c.Query("link_token")
	if linkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token query param is required"})
	}
	if h.webAuthn == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	pending, err := h.loadPendingSocialLink(c.Context(), linkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}
	if isAccountLocked(c.Context(), h.cache, pending.UserID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	sessionJSON, err := h.cache.Get(c.Context(), socialLinkWebAuthnKey(linkToken))
	if err != nil || sessionJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending authentication"})
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}

	waUser, err := h.loadSocialLinkWebAuthnUser(c, pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	credential, err := h.webAuthn.FinishLogin(waUser, session, c.Body())
	if err != nil {
		recordFailedAuthAttempt(c.Context(), h.cache, pending.UserID)
		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:   pending.UserID,
			Action:   model.AuditSocialAccountLink,
			Resource: "social_account",
			Status:   "failed",
			Detail:   "social link verification failed",
			Metadata: map[string]string{
				"reason": "invalid_webauthn",
			},
		})
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	if h.webAuthnCreds != nil {
		_ = h.webAuthnCreds.UpdateSignCount(c.Context(), credential.ID, credential.Authenticator.SignCount)
	}

	return h.completeSocialLink(c, waUser.User, pending, "webauthn",
		socialLinkPendingKey(linkToken),
		socialLinkWebAuthnKey(linkToken),
	)
}

// respondWithTokens generates JWT tokens and returns them with user info.
// Audit success is only logged when tokens are actually issued (not when MFA is pending).
func (h *OAuthHandler) respondWithTokens(c fiber.Ctx, user *model.User, provider string) error {
	// Check if MFA is enabled for this user.
	if h.mfa != nil {
		mfaCfg, _ := h.mfa.GetByUserID(c.Context(), user.ID)
		if mfaCfg != nil && mfaCfg.TOTPEnabled {
			mfaToken := store.GenerateRandomToken()
			_ = h.cache.Set(c.Context(), "mfa_pending:"+mfaToken, user.ID, 5*time.Minute)
			return c.JSON(fiber.Map{
				"mfa_required": true,
				"mfa_token":    mfaToken,
			})
		}
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     user.ID,
		Action:     model.AuditLogin,
		Resource:   "user",
		ResourceID: user.ID,
		Status:     "success",
		Detail:     "user login",
		Metadata: map[string]string{
			"provider": provider,
		},
	})

	issuedSession, err := issueBrowserSession(c, h.sessions, user, h.tokenConfig.RefreshTTL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, h.tokenConfig.RefreshTTL)

	return c.JSON(fiber.Map{
		"user": user,
	})
}

// githubUser represents the GitHub user API response.
type githubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// fetchGitHubUser calls the GitHub user API with the given token.
func fetchGitHubUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*githubUser, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}

	// If email is empty (user set it to private), try /user/emails endpoint.
	if u.Email == "" {
		u.Email = fetchGitHubPrimaryEmail(client)
	}

	return &u, nil
}

// fetchGitHubPrimaryEmail calls /user/emails to get the primary verified email.
func fetchGitHubPrimaryEmail(client *http.Client) string {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return ""
	}

	// Prefer primary+verified, then any verified.
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}

// googleUser represents the Google userinfo API response.
type googleUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// fetchGoogleUser calls the Google userinfo API with the given token.
func fetchGoogleUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*googleUser, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var u googleUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
