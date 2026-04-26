package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"
)

type mfaUserStore interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type mfaStore interface {
	GetByUserID(ctx context.Context, userID string) (*model.MFAConfig, error)
	Upsert(ctx context.Context, cfg *model.MFAConfig) error
	Delete(ctx context.Context, userID string) error
}

type oidcAuthCompleter interface {
	CompleteAuthRequest(ctx context.Context, id, userID string) error
	AssignAuthRequestUser(ctx context.Context, id, userID string) error
	AuthRequestRequiresConsent(ctx context.Context, id string) bool
	AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error)
}

// MFADeps aggregates dependencies required by MFAHandler.
type MFADeps struct {
	Users         mfaUserStore
	MFA           mfaStore
	Audit         *store.AuditLogStore
	RefreshTokens refreshTokenCreator
	Sessions      *service.SessionService
	Cache         store.Cache
	OIDC          oidcAuthCompleter
	Clients       *store.OAuthClientStore
	ConsentGrants *store.OAuthConsentGrantStore
	TokenConfig   TokenConfig
}

// MFAHandler handles MFA endpoints.
type MFAHandler struct {
	users         mfaUserStore
	mfa           mfaStore
	audit         *store.AuditLogStore
	sessions      *service.SessionService
	cache         store.Cache
	oidc          oidcAuthCompleter
	clients       *store.OAuthClientStore
	consentGrants *store.OAuthConsentGrantStore
	tokenConfig   TokenConfig
}

const (
	mfaVerifyThreshold = 5
	mfaVerifyWindow    = 5 * time.Minute
)

// NewMFAHandler creates a MFAHandler.
func NewMFAHandler(deps MFADeps) *MFAHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = service.NewSessionService(deps.RefreshTokens, deps.TokenConfig.ToServiceConfig())
	}

	return &MFAHandler{
		users:         deps.Users,
		mfa:           deps.MFA,
		audit:         deps.Audit,
		sessions:      sessions,
		cache:         deps.Cache,
		oidc:          deps.OIDC,
		clients:       deps.Clients,
		consentGrants: deps.ConsentGrants,
		tokenConfig:   deps.TokenConfig,
	}
}

func mfaVerifyFailKey(token string) string { return "mfa_verify_fail:" + token }
func mfaVerifyLockKey(token string) string { return "mfa_verify_lock:" + token }
func mfaConsumedKey(token string) string   { return "mfa_consumed:" + token }

func (h *MFAHandler) isMFAVerifyLocked(ctx context.Context, token string) bool {
	val, err := h.cache.Get(ctx, mfaVerifyLockKey(token))
	return err == nil && val != ""
}

func (h *MFAHandler) recordFailedMFAVerify(ctx context.Context, token string) bool {
	key := mfaVerifyFailKey(token)
	count, _ := h.cache.Incr(ctx, key, mfaVerifyWindow)
	if count >= int64(mfaVerifyThreshold) {
		_ = h.cache.Set(ctx, mfaVerifyLockKey(token), "1", mfaVerifyWindow)
		return true
	}
	return false
}

func (h *MFAHandler) clearMFAVerifyFailures(ctx context.Context, token string) {
	_ = h.cache.Del(ctx, mfaVerifyFailKey(token))
	_ = h.cache.Del(ctx, mfaVerifyLockKey(token))
}

// Status handles GET /api/mfa/status.
func (h *MFAHandler) Status(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.JSON(fiber.Map{"totp_enabled": false})
	}
	return c.JSON(fiber.Map{"totp_enabled": cfg.TOTPEnabled})
}

// TOTPSetup handles POST /api/mfa/totp/setup.
func (h *MFAHandler) TOTPSetup(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      h.tokenConfig.Issuer,
		AccountName: user.Email,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate TOTP secret"})
	}

	_ = h.cache.Set(c.Context(), "totp_setup:"+userID, key.Secret(), 10*time.Minute)
	return c.JSON(fiber.Map{"secret": key.Secret(), "url": key.URL()})
}

func (h *MFAHandler) verifyAndConsumeBackupCode(ctx context.Context, cfg *model.MFAConfig, code string) bool {
	var hashes []string
	if err := json.Unmarshal([]byte(cfg.BackupCodes), &hashes); err != nil {
		return false
	}

	for i, hash := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			hashes = append(hashes[:i], hashes[i+1:]...)
			codesJSON, err := json.Marshal(hashes)
			if err != nil {
				return false
			}
			cfg.BackupCodes = string(codesJSON)
			if err := h.mfa.Upsert(ctx, cfg); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func generateBackupCodes(n int) []string {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		codes[i] = fmt.Sprintf("%016x", b)
	}
	return codes
}

func hashBackupCodes(codes []string) []string {
	hashed := make([]string, len(codes))
	for i, code := range codes {
		h, _ := bcrypt.GenerateFromPassword([]byte(code), 12)
		hashed[i] = string(h)
	}
	return hashed
}
