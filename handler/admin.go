package handler

import (
	"context"
	"crypto/rand"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type adminUserStore interface {
	Create(ctx context.Context, user *model.User) error
	List(ctx context.Context, page, pageSize int) ([]model.User, int64, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
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
	DB             *gorm.DB
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
	db             *gorm.DB
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
		db:             deps.DB,
	}
}

// CreateUser handles POST /api/admin/users.
func (h *AdminHandler) CreateUser(c fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and email are required"})
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username must be 3-32 characters"})
	}
	if !isValidEmail(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid email format"})
	}

	user := &model.User{
		Username:   req.Username,
		Email:      req.Email,
		Provider:   "local",
		ProviderID: req.Username,
		IsAdmin:    req.IsAdmin,
	}
	if req.IsAdmin {
		user.SetPermissions(model.AllAdminPermissions())
	}

	if err := h.users.Create(c.Context(), user); err != nil {
		if store.IsUniqueViolation(err) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username or email already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
	}

	adminID, _ := c.Locals("user_id").(string)
	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: model.AuditUserCreate, Resource: "user", ResourceID: user.ID,
		IP: ip, UserAgent: ua, Status: "success",
	})

	setupToken := store.GenerateRandomToken()
	if err := store.NewEphemeralTokenStore(h.cache).IssueString(c.Context(), "password_setup", "user", setupToken, user.ID, 24*time.Hour); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create password setup token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":                      NewUserResponse(user),
		"password_setup_token":      setupToken,
		"password_setup_expires_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	})
}

// ListUsers handles GET /api/admin/users.
func (h *AdminHandler) ListUsers(c fiber.Ctx) error {
	page, pageSize := parsePagination(c)

	users, total, err := h.users.List(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list users"})
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	socialProvidersByUserID := map[string][]string{}
	if h.socialAccounts != nil {
		socialProvidersByUserID, err = h.socialAccounts.ListProvidersByUserIDs(c.Context(), userIDs)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list user social accounts"})
		}
	}

	type userListResp struct {
		ID              string   `json:"id"`
		Username        string   `json:"username"`
		Email           string   `json:"email"`
		Provider        string   `json:"provider"`
		IsAdmin         bool     `json:"is_admin"`
		CreatedAt       string   `json:"created_at"`
		SocialProviders []string `json:"social_providers"`
	}

	result := make([]userListResp, 0, len(users))
	for _, user := range users {
		socialProviders := slices.Clone(socialProvidersByUserID[user.ID])
		result = append(result, userListResp{
			ID:              user.ID,
			Username:        user.Username,
			Email:           user.Email,
			Provider:        user.Provider,
			IsAdmin:         user.IsAdmin,
			CreatedAt:       user.CreatedAt.Format("2006-01-02T15:04:05Z"),
			SocialProviders: socialProviders,
		})
	}

	return c.JSON(fiber.Map{
		"users":     result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
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

// UpdateUser handles PUT /api/admin/users/:id.
func (h *AdminHandler) UpdateUser(c fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.users.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	var req struct {
		IsAdmin     *bool     `json:"is_admin"`
		Permissions *[]string `json:"permissions"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	permissionChanged := false

	if req.IsAdmin != nil {
		// Prevent admin from demoting themselves.
		if currentID, _ := c.Locals("user_id").(string); currentID == id && !*req.IsAdmin {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot remove your own admin role"})
		}
		permissionChanged = true
		user.IsAdmin = *req.IsAdmin
		if user.IsAdmin {
			user.SetPermissions(model.AllAdminPermissions())
		} else {
			user.Permissions = ""
		}
	}

	if req.Permissions != nil {
		permissionChanged = true
		perms := make([]string, 0, len(*req.Permissions))
		for _, perm := range *req.Permissions {
			perm = strings.TrimSpace(perm)
			if perm == "" {
				continue
			}
			if !model.IsValidPermission(perm) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid permission: " + perm})
			}
			perms = append(perms, perm)
		}
		user.SetPermissions(perms)
		user.IsAdmin = false
	}

	if permissionChanged {
		if user.TokenVersion < 1 {
			user.TokenVersion = 1
		}
		user.TokenVersion++
	}

	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}
	if h.cache != nil {
		for _, perm := range model.AllAdminPermissions() {
			_ = h.cache.Del(c.Context(), "perm_check:"+id+":"+perm)
		}
		if permissionChanged {
			_ = h.cache.Del(c.Context(), "token_ver:"+id)
		}
	}

	adminID, _ := c.Locals("user_id").(string)
	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: model.AuditUserUpdate, Resource: "user", ResourceID: id,
		IP: ip, UserAgent: ua, Status: "success",
	})

	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}

// DeleteUser handles DELETE /api/admin/users/:id.
func (h *AdminHandler) DeleteUser(c fiber.Ctx) error {
	id := c.Params("id")
	// Prevent admin from deleting themselves.
	if currentID, _ := c.Locals("user_id").(string); currentID == id {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete yourself"})
	}
	if _, err := h.users.GetByID(c.Context(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	// Use transaction to ensure atomicity
	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		// Delete user (soft delete with GORM)
		if err := tx.Where("id = ?", id).Delete(&model.User{}).Error; err != nil {
			return err
		}

		// Cascade cleanup: delete all related data
		// 1. Refresh tokens (sessions)
		if err := tx.Where("user_id = ?", id).Delete(&model.APIRefreshToken{}).Error; err != nil {
			return err
		}

		// 2. MFA configuration
		if err := tx.Where("user_id = ?", id).Delete(&model.MFAConfig{}).Error; err != nil {
			return err
		}

		// 3. WebAuthn credentials
		if err := tx.Where("user_id = ?", id).Delete(&model.WebAuthnCredential{}).Error; err != nil {
			return err
		}

		// 4. Social account bindings
		if err := tx.Where("user_id = ?", id).Delete(&model.SocialAccount{}).Error; err != nil {
			return err
		}

		// 5. Password history
		if err := tx.Where("user_id = ?", id).Delete(&model.PasswordHistory{}).Error; err != nil {
			return err
		}

		// 6. User points
		if err := tx.Where("user_id = ?", id).Delete(&model.UserPoints{}).Error; err != nil {
			return err
		}

		// 7. Check-in records
		if err := tx.Where("user_id = ?", id).Delete(&model.CheckInRecord{}).Error; err != nil {
			return err
		}

		// Note: AuditLog is intentionally NOT deleted for compliance/audit trail
		// Note: OIDC tokens (authorization_codes, access_tokens, refresh_tokens)
		// are managed by zitadel/oidc library and will be cleaned up by their TTL

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete user"})
	}

	delAdminID, _ := c.Locals("user_id").(string)
	delIP, delUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: delAdminID, Action: model.AuditUserDelete, Resource: "user", ResourceID: id,
		IP: delIP, UserAgent: delUA, Status: "success",
	})

	return c.JSON(fiber.Map{"message": "user deleted"})
}

// GetStats handles GET /api/admin/stats.
func (h *AdminHandler) GetStats(c fiber.Ctx) error {
	var (
		totalUsers, totalClients, totalLogins, todayLogins int64
	)
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

	// Shuffle (Fisher-Yates).
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
