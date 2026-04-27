package handler

import (
	"strings"
	"sync"
	"time"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/store"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

const (
	mfaFailureTTL = 5 * time.Minute // TTL for MFA failure counter
)

// WebAuthnDeps aggregates dependencies required by WebAuthnHandler.
type WebAuthnDeps struct {
	Settings      *store.SettingStore
	Users         *store.UserStore
	Creds         *store.WebAuthnStore
	MFA           *store.MFAStore
	Audit         *store.AuditLogStore
	RefreshTokens *store.APIRefreshTokenStore
	Sessions      *service.SessionService
	Cache         store.Cache
	TokenConfig   TokenConfig
}

// WebAuthnHandler handles WebAuthn/Passkey endpoints.
type WebAuthnHandler struct {
	settings    *store.SettingStore
	users       *store.UserStore
	creds       *store.WebAuthnStore
	mfa         *store.MFAStore
	audit       *store.AuditLogStore
	sessions    *service.SessionService
	cache       store.Cache
	tokenConfig TokenConfig

	mu        sync.RWMutex
	cachedWA  *webauthn.WebAuthn
	cachedAt  time.Time
	cachedCfg string
}

// NewWebAuthnHandler creates a WebAuthnHandler.
func NewWebAuthnHandler(deps WebAuthnDeps) *WebAuthnHandler {
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
	rpID := h.settings.Get(store.SettingWebAuthnRPID, "")
	rpDisplay := h.settings.Get(store.SettingWebAuthnRPDisplay, "ID Service")
	rpOrigins := h.settings.Get(store.SettingWebAuthnRPOrigins, "")
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

func (h *WebAuthnHandler) loadWebAuthnUser(c fiber.Ctx, userID string) (*store.WebAuthnUser, error) {
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	creds, _ := h.creds.ListByUserID(c.Context(), userID)
	return &store.WebAuthnUser{User: user, Creds: creds}, nil
}
