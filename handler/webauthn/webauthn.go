package webauthnhandler

import (
	"strings"
	"sync"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	rootstore "github.com/ysicing/go-template/store"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

const (
	mfaFailureTTL = 5 * time.Minute // TTL for MFA failure counter
)

// WebAuthnDeps aggregates dependencies required by WebAuthnHandler.
type WebAuthnDeps struct {
	Settings      *rootstore.SettingStore
	Users         *rootstore.UserStore
	Creds         *webauthnstore.WebAuthnStore
	MFA           *rootstore.MFAStore
	Audit         *rootstore.AuditLogStore
	RefreshTokens *rootstore.APIRefreshTokenStore
	Sessions      *sessionservice.SessionService
	Cache         rootstore.Cache
	TokenConfig   handlercommon.TokenConfig
}

// WebAuthnHandler handles WebAuthn/Passkey endpoints.
type WebAuthnHandler struct {
	settings    *rootstore.SettingStore
	users       *rootstore.UserStore
	creds       *webauthnstore.WebAuthnStore
	mfa         *rootstore.MFAStore
	audit       *rootstore.AuditLogStore
	sessions    *sessionservice.SessionService
	cache       rootstore.Cache
	tokenConfig handlercommon.TokenConfig

	mu        sync.RWMutex
	cachedWA  *webauthn.WebAuthn
	cachedAt  time.Time
	cachedCfg string
}

// NewWebAuthnHandler creates a WebAuthnHandler.
func NewWebAuthnHandler(deps WebAuthnDeps) *WebAuthnHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = sessionservice.NewSessionService(deps.RefreshTokens, sessionservice.TokenConfig{
			Secret:        deps.TokenConfig.Secret,
			Issuer:        deps.TokenConfig.Issuer,
			AccessTTL:     deps.TokenConfig.AccessTTL,
			RefreshTTL:    deps.TokenConfig.RefreshTTL,
			RememberMeTTL: deps.TokenConfig.RememberMeTTL,
		})
	}

	return &WebAuthnHandler{
		settings:    deps.Settings,
		users:       deps.Users,
		creds:       deps.Creds,
		mfa:         deps.MFA,
		audit:       deps.Audit,
		sessions:    sessions,
		cache:       deps.Cache,
		tokenConfig: deps.TokenConfig,
	}
}

// getWebAuthn returns a cached *webauthn.WebAuthn instance, rebuilding when settings change.
func (h *WebAuthnHandler) getWebAuthn() (*webauthn.WebAuthn, error) {
	rpID := h.settings.Get(rootstore.SettingWebAuthnRPID, "")
	rpDisplay := h.settings.Get(rootstore.SettingWebAuthnRPDisplay, "ID Service")
	rpOrigins := h.settings.Get(rootstore.SettingWebAuthnRPOrigins, "")
	cfgKey := rpID + "|" + rpDisplay + "|" + rpOrigins

	h.mu.RLock()
	if h.cachedWA != nil && h.cachedCfg == cfgKey && time.Since(h.cachedAt) < 30*time.Second {
		cached := h.cachedWA
		h.mu.RUnlock()
		return cached, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cachedWA != nil && h.cachedCfg == cfgKey && time.Since(h.cachedAt) < 30*time.Second {
		return h.cachedWA, nil
	}

	if rpID == "" {
		h.cachedWA = nil
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	var origins []string
	for o := range strings.SplitSeq(rpOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplay,
		RPID:          rpID,
		RPOrigins:     origins,
	})
	if err != nil {
		return nil, err
	}

	h.cachedWA = wa
	h.cachedAt = time.Now()
	h.cachedCfg = cfgKey
	return wa, nil
}

func (h *WebAuthnHandler) loadWebAuthnUser(c fiber.Ctx, userID string) (*webauthnstore.WebAuthnUser, error) {
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	creds, _ := h.creds.ListByUserID(c.Context(), userID)
	return &webauthnstore.WebAuthnUser{User: user, Creds: creds}, nil
}
