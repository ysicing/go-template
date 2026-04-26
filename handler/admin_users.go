package handler

import (
	"slices"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type createAdminUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
}

// CreateUser handles POST /api/admin/users.
func (h *AdminHandler) CreateUser(c fiber.Ctx) error {
	req, err := parseCreateAdminUserRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	user := newAdminCreatedUser(req)
	if err := h.users.Create(c.Context(), user); err != nil {
		if store.IsUniqueViolation(err) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username or email already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
	}

	h.auditAdminUserMutation(c, model.AuditUserCreate, user.ID)
	setupToken, expiresAt, err := h.issuePasswordSetupToken(c, user.ID)
	if err != nil {
		return finishHandlerError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":                      NewUserResponse(user),
		"password_setup_token":      setupToken,
		"password_setup_expires_at": expiresAt,
	})
}

func parseCreateAdminUserRequest(c fiber.Ctx) (*createAdminUserRequest, error) {
	var req createAdminUserRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" || req.Email == "" {
		return nil, jsonError(fiber.StatusBadRequest, "username and email are required")
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return nil, jsonError(fiber.StatusBadRequest, "username must be 3-32 characters")
	}
	if !isValidEmail(req.Email) {
		return nil, jsonError(fiber.StatusBadRequest, "invalid email format")
	}
	return &req, nil
}

func newAdminCreatedUser(req *createAdminUserRequest) *model.User {
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
	return user
}

func (h *AdminHandler) issuePasswordSetupToken(c fiber.Ctx, userID string) (string, string, error) {
	setupToken := store.GenerateRandomToken()
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	if err := store.NewEphemeralTokenStore(h.cache).IssueString(c.Context(), "password_setup", "user", setupToken, userID, 24*time.Hour); err != nil {
		return "", "", jsonError(fiber.StatusInternalServerError, "failed to create password setup token")
	}
	return setupToken, expiresAt, nil
}

// ListUsers handles GET /api/admin/users.
func (h *AdminHandler) ListUsers(c fiber.Ctx) error {
	page, pageSize := parsePagination(c)
	users, total, err := h.users.List(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list users"})
	}

	socialProvidersByUserID, err := h.listUserSocialProviders(c, users)
	if err != nil {
		return finishHandlerError(c, err)
	}

	return c.JSON(fiber.Map{
		"users":     buildAdminUserListResponse(users, socialProvidersByUserID),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) listUserSocialProviders(c fiber.Ctx, users []model.User) (map[string][]string, error) {
	if h.socialAccounts == nil || len(users) == 0 {
		return map[string][]string{}, nil
	}
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	result, err := h.socialAccounts.ListProvidersByUserIDs(c.Context(), userIDs)
	if err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to list user social accounts")
	}
	return result, nil
}

func buildAdminUserListResponse(users []model.User, socialProvidersByUserID map[string][]string) []fiber.Map {
	result := make([]fiber.Map, 0, len(users))
	for _, user := range users {
		result = append(result, fiber.Map{
			"id":               user.ID,
			"username":         user.Username,
			"email":            user.Email,
			"provider":         user.Provider,
			"is_admin":         user.IsAdmin,
			"created_at":       user.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"social_providers": slices.Clone(socialProvidersByUserID[user.ID]),
		})
	}
	return result
}

type updateAdminUserRequest struct {
	IsAdmin     *bool     `json:"is_admin"`
	Permissions *[]string `json:"permissions"`
}

// UpdateUser handles PUT /api/admin/users/:id.
func (h *AdminHandler) UpdateUser(c fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.users.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	req, err := parseUpdateAdminUserRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}
	permissionChanged, err := h.applyAdminUserUpdate(c, id, user, req)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}

	h.clearAdminUserCaches(c, id, permissionChanged)
	h.auditAdminUserMutation(c, model.AuditUserUpdate, id)
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}

func parseUpdateAdminUserRequest(c fiber.Ctx) (*updateAdminUserRequest, error) {
	var req updateAdminUserRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	return &req, nil
}

func (h *AdminHandler) applyAdminUserUpdate(c fiber.Ctx, id string, user *model.User, req *updateAdminUserRequest) (bool, error) {
	permissionChanged, err := h.applyAdminRoleUpdate(c, id, user, req.IsAdmin)
	if err != nil {
		return false, err
	}
	permListChanged, err := applyAdminPermissionUpdate(user, req.Permissions)
	if err != nil {
		return false, err
	}
	if permissionChanged || permListChanged {
		if user.TokenVersion < 1 {
			user.TokenVersion = 1
		}
		user.TokenVersion++
	}
	return permissionChanged || permListChanged, nil
}

func (h *AdminHandler) applyAdminRoleUpdate(c fiber.Ctx, id string, user *model.User, isAdmin *bool) (bool, error) {
	if isAdmin == nil {
		return false, nil
	}
	if currentID, _ := c.Locals("user_id").(string); currentID == id && !*isAdmin {
		return false, jsonError(fiber.StatusBadRequest, "cannot remove your own admin role")
	}
	user.IsAdmin = *isAdmin
	if user.IsAdmin {
		user.SetPermissions(model.AllAdminPermissions())
	} else {
		user.Permissions = ""
	}
	return true, nil
}

func applyAdminPermissionUpdate(user *model.User, permissions *[]string) (bool, error) {
	if permissions == nil {
		return false, nil
	}
	perms := make([]string, 0, len(*permissions))
	for _, perm := range *permissions {
		perm = strings.TrimSpace(perm)
		if perm == "" {
			continue
		}
		if !model.IsValidPermission(perm) {
			return false, jsonError(fiber.StatusBadRequest, "invalid permission: "+perm)
		}
		perms = append(perms, perm)
	}
	user.SetPermissions(perms)
	user.IsAdmin = false
	return true, nil
}

func (h *AdminHandler) clearAdminUserCaches(c fiber.Ctx, id string, permissionChanged bool) {
	if h.cache == nil {
		return
	}
	for _, perm := range model.AllAdminPermissions() {
		_ = h.cache.Del(c.Context(), "perm_check:"+id+":"+perm)
	}
	if permissionChanged {
		_ = h.cache.Del(c.Context(), "token_ver:"+id)
	}
}

// DeleteUser handles DELETE /api/admin/users/:id.
func (h *AdminHandler) DeleteUser(c fiber.Ctx) error {
	id := c.Params("id")
	if currentID, _ := c.Locals("user_id").(string); currentID == id {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete yourself"})
	}
	if _, err := h.users.GetByID(c.Context(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	if err := h.users.DeleteCascade(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete user"})
	}

	h.auditAdminUserMutation(c, model.AuditUserDelete, id)
	return c.JSON(fiber.Map{"message": "user deleted"})
}

func (h *AdminHandler) auditAdminUserMutation(c fiber.Ctx, action, resourceID string) {
	adminID, _ := c.Locals("user_id").(string)
	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: action, Resource: "user", ResourceID: resourceID,
		IP: ip, UserAgent: ua, Status: "success",
	})
}
