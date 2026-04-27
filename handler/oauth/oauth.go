package oauthhandler

import (
	"context"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

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
	CreateUserWithSocialAccount(ctx context.Context, user *model.User, account *model.SocialAccount) error
	Create(ctx context.Context, account *model.SocialAccount) error
	Update(ctx context.Context, account *model.SocialAccount) error
}

type oauthSettingStore interface {
	Get(key, defaultVal string) string
	GetBool(key string, defaultVal bool) bool
}

type refreshTokenCreator interface {
	Create(ctx context.Context, rt *model.APIRefreshToken) error
}

type mfaReader interface {
	GetByUserID(ctx context.Context, userID string) (*model.MFAConfig, error)
}

// OAuthDeps aggregates dependencies required by OAuthHandler.
type OAuthDeps struct {
	Users          oauthUserStore
	Providers      oauthProviderStore
	SocialAccounts oauthSocialAccountStore
	Audit          *store.AuditLogStore
	RefreshTokens  refreshTokenCreator
	Sessions       *sessionservice.SessionService
	MFA            mfaReader
	Cache          store.Cache
	Settings       oauthSettingStore
	WebAuthnCreds  oauthWebAuthnCredStore
	TokenConfig    handlercommon.TokenConfig
}

// OAuthHandler handles social login flows using database-managed provider configs.
type OAuthHandler struct {
	users          oauthUserStore
	providers      oauthProviderStore
	socialAccounts oauthSocialAccountStore
	audit          *store.AuditLogStore
	sessions       *sessionservice.SessionService
	mfa            mfaReader
	cache          store.Cache
	settings       oauthSettingStore
	webAuthnCreds  oauthWebAuthnCredStore
	webAuthn       oauthWebAuthnManager
	tokenConfig    handlercommon.TokenConfig
}

// NewOAuthHandler creates an OAuthHandler with database-backed social providers.
func NewOAuthHandler(deps OAuthDeps) *OAuthHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = sessionservice.NewSessionService(deps.RefreshTokens, deps.TokenConfig.ToServiceConfig())
	}

	return &OAuthHandler{
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

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
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

	ip, ua := handlercommon.GetRealIPAndUA(c)
	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), sessionservice.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: h.tokenConfig.RefreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}
	// Set tokens in cookies for web clients
	handlercommon.SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, h.tokenConfig.RefreshTTL)

	return c.JSON(fiber.Map{
		"user": handlercommon.NewUserResponse(user),
	})
}
