package handler

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/errgroup"
)

type adminUserStore interface {
	Create(ctx context.Context, user *model.User) error
	List(ctx context.Context, page, pageSize int) ([]model.User, int64, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	DeleteCascade(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

type adminClientStore interface {
	Count(ctx context.Context) (int64, error)
}

type adminAuditLogStore interface {
	Create(ctx context.Context, log *model.AuditLog) error
	CountLogin(ctx context.Context) (int64, error)
	CountLoginToday(ctx context.Context) (int64, error)
	ListLoginAllPaged(ctx context.Context, page, pageSize int) ([]store.LoginRow, int64, error)
	ListAuditLogsPaged(ctx context.Context, filter store.AuditLogFilter, page, pageSize int) ([]store.AuditLogRow, int64, error)
}

type adminCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	DelIfValue(ctx context.Context, key, value string) (bool, error)
}

// AdminDeps aggregates dependencies required by AdminHandler.
type AdminDeps struct {
	Users          adminUserStore
	Clients        adminClientStore
	Audit          adminAuditLogStore
	RefreshTokens  *store.APIRefreshTokenStore
	MFA            *store.MFAStore
	WebAuthnCreds  *store.WebAuthnStore
	SocialAccounts *store.SocialAccountStore
	PasswordHist   *store.PasswordHistoryStore
	Cache          adminCache
}

// AdminHandler handles admin user management endpoints.
type AdminHandler struct {
	users          adminUserStore
	clients        adminClientStore
	audit          adminAuditLogStore
	refreshTokens  *store.APIRefreshTokenStore
	mfa            *store.MFAStore
	webauthnCreds  *store.WebAuthnStore
	socialAccounts *store.SocialAccountStore
	passwordHist   *store.PasswordHistoryStore
	cache          adminCache
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(deps AdminDeps) *AdminHandler {
	return &AdminHandler{
		users:          deps.Users,
		clients:        deps.Clients,
		audit:          deps.Audit,
		refreshTokens:  deps.RefreshTokens,
		mfa:            deps.MFA,
		webauthnCreds:  deps.WebAuthnCreds,
		socialAccounts: deps.SocialAccounts,
		passwordHist:   deps.PasswordHist,
		cache:          deps.Cache,
	}
}

// GetUser handles GET /api/admin/users/:id.
func (h *AdminHandler) GetUser(c fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.users.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}

// GetStats handles GET /api/admin/stats.
func (h *AdminHandler) GetStats(c fiber.Ctx) error {
	var totalUsers, totalClients, totalLogins, todayLogins int64

	g, ctx := errgroup.WithContext(c.Context())
	g.Go(func() error {
		var err error
		totalUsers, err = h.users.Count(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		totalClients, err = h.clients.Count(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		totalLogins, err = h.audit.CountLogin(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		todayLogins, err = h.audit.CountLoginToday(ctx)
		return err
	})
	if err := g.Wait(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get stats"})
	}

	return c.JSON(fiber.Map{
		"total_users":   totalUsers,
		"total_clients": totalClients,
		"total_logins":  totalLogins,
		"today_logins":  todayLogins,
	})
}

// generatePassword creates a random password with the given length,
// guaranteed to contain upper, lower, digit and special characters.
// GeneratePassword generates a cryptographically random password of the given length
// with at least one uppercase, lowercase, digit, and special character.
func GeneratePassword(length int) string {
	const (
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "!@#$%^&*"
		all     = upper + lower + digits + special
	)

	randChar := func(charset string) byte {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		return charset[n.Int64()]
	}

	buf := make([]byte, length)
	buf[0] = randChar(upper)
	buf[1] = randChar(lower)
	buf[2] = randChar(digits)
	buf[3] = randChar(special)
	for i := 4; i < length; i++ {
		buf[i] = randChar(all)
	}

	for i := length - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		buf[i], buf[j.Int64()] = buf[j.Int64()], buf[i]
	}

	return string(buf)
}

// GetLoginHistory handles GET /api/admin/login-history.
func (h *AdminHandler) GetLoginHistory(c fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	rows, total, err := h.audit.ListLoginAllPaged(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list login history"})
	}
	return c.JSON(fiber.Map{
		"events":    rows,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetAuditLogs handles GET /api/admin/audit-logs.
func (h *AdminHandler) GetAuditLogs(c fiber.Ctx) error {
	page, pageSize := parsePagination(c)

	filter := store.AuditLogFilter{
		UserID:   strings.TrimSpace(c.Query("user_id")),
		Action:   strings.TrimSpace(c.Query("action")),
		Resource: strings.TrimSpace(c.Query("resource")),
		Source:   strings.TrimSpace(c.Query("source")),
		Status:   strings.TrimSpace(c.Query("status")),
		IP:       strings.TrimSpace(c.Query("ip")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	}

	createdFrom, err := parseAuditTimeQuery(c.Query("created_from"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid created_from"})
	}
	createdTo, err := parseAuditTimeQuery(c.Query("created_to"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid created_to"})
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo

	rows, total, err := h.audit.ListAuditLogsPaged(c.Context(), filter, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list audit logs"})
	}

	return c.JSON(fiber.Map{
		"logs":      rows,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func parseAuditTimeQuery(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	layouts := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}

	return nil, fiber.ErrBadRequest
}
