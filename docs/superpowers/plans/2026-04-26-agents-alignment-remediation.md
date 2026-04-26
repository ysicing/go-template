# AGENTS 对齐整改 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让代码实现符合项目 `AGENTS.md` 的事务边界、`store.ErrNotFound`、分层依赖和函数体量要求，同时保持现有行为与测试稳定。

**Architecture:** 采用“先契约统一、再边界收拢、最后结构拆分”的顺序推进。先在 `store` 层建立统一错误语义，再把 `service` / `handler` 的事务与 `gorm.DB` 依赖下沉到 `store`，最后拆分超长函数和超大文件，避免把行为变更与结构重构混在同一批次里。

**Tech Stack:** Go 1.25+, GORM, Fiber, Testify, pnpm/vitest/eslint

---

## Execution Status

- 已完成：`store.ErrNotFound` 契约统一、上层 `store.ErrNotFound` 对齐、client credentials 事务下沉、social login 创建绑定事务下沉、管理员删用户事务下沉。
- 已完成：`handler/auth.go`、`handler/user.go`、`handler/admin.go`、`handler/mfa.go`、`handler/oauth_social_login.go` 的结构拆分，当前相关函数体均已压到 50 行以内，相关文件均低于 300 行。
- 已验证：`go test ./...` 通过；`web` 下 `pnpm lint` 通过。
- 待处理：`web` 下 `pnpm test` 因现有测试环境问题失败，报错为 `localStorage.clear is not a function`，本轮未改前端测试代码。

---

## File Map

- Modify: `store/user.go`
  - 统一单条查询的未找到错误。
- Modify: `store/oauth_client.go`
  - 统一 OAuth client 单条查询的未找到错误。
- Modify: `store/oauth_consent_grant.go`
  - 统一 consent grant 单条查询与删除未找到错误。
- Modify: `store/social_account.go`
  - 统一 social account 单条查询未找到错误。
- Modify: `store/social_provider.go`
  - 统一 provider 单条查询未找到错误，并保留 secret 解密错误语义。
- Modify: `store/webauthn.go`, `store/mfa.go`, `store/api_refresh_token.go`
  - 对单条读取接口统一错误语义。
- Create: `store/errors.go`
  - 定义 `ErrNotFound`、`normalizeNotFound`。
- Create: `store/not_found_test.go`
  - 统一覆盖单条查询未找到行为。
- Modify: `handler/user.go`, `handler/admin_provider.go`, `internal/app/runtime.go`, `internal/service/client_credentials_service.go`, `handler/oauth_social_login.go`
  - 上层改为只依赖 `store.ErrNotFound`。
- Modify: `store/oauth_client.go`
  - 新增 client credentials 令牌写入 / 删除事务封装。
- Modify: `store/social_account.go`, `store/user.go`
  - 新增 social login 用户创建绑定事务封装。
- Modify: `store/user.go` or create `store/user_cleanup.go`
  - 新增用户级联删除事务封装。
- Modify: `internal/service/client_credentials_service.go`, `handler/oauth.go`, `handler/admin.go`
  - 移除上层 `*gorm.DB` 字段与依赖注入。
- Modify: `internal/app/bootstrap.go`, `handler/client_credentials_test.go`, `handler/oauth_test.go`, `handler/auth_flow_test.go`, `internal/service/client_credentials_service_test.go`
  - 适配新的依赖注入与 store 接口。
- Modify: `handler/auth.go`, `handler/user.go`, `handler/admin.go`, `handler/mfa.go`, `handler/oauth_social_login.go`
  - 按职责拆分超长函数。
- Create: `handler/*_helpers.go`（按实际拆分结果命名）
  - 承载解析、校验、响应组装等纯辅助逻辑。

### Task 1: 统一 `store.ErrNotFound` 契约

**Files:**
- Create: `store/errors.go`
- Create: `store/not_found_test.go`
- Modify: `store/user.go`
- Modify: `store/oauth_client.go`
- Modify: `store/oauth_consent_grant.go`
- Modify: `store/social_account.go`
- Modify: `store/social_provider.go`
- Modify: `store/webauthn.go`
- Modify: `store/mfa.go`
- Modify: `store/api_refresh_token.go`

- [ ] **Step 1: 写统一错误定义**

```go
package store

import (
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("store: not found")

func normalizeNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
```

- [ ] **Step 2: 为单条查询接入 `normalizeNotFound`**

```go
func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &user, nil
}
```

- [ ] **Step 3: 处理 `RowsAffected == 0` 的接口**

```go
if result.RowsAffected == 0 {
	return nil, ErrNotFound
}
```

- [ ] **Step 4: 新增集中测试**

```go
func TestNormalizeNotFound(t *testing.T) {
	require.ErrorIs(t, normalizeNotFound(gorm.ErrRecordNotFound), ErrNotFound)
}
```

- [ ] **Step 5: 跑 store 定向测试**

Run: `go test ./store -run 'TestNormalizeNotFound|TestOAuthConsentGrantStore|TestUserStore|TestSocialProviderStore|TestAPIRefreshTokenStore'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add store/errors.go store/not_found_test.go store/user.go store/oauth_client.go store/oauth_consent_grant.go store/social_account.go store/social_provider.go store/webauthn.go store/mfa.go store/api_refresh_token.go
git commit -m "refactor(store): 统一未找到错误语义"
```

### Task 2: 上层改为只依赖 `store.ErrNotFound`

**Files:**
- Modify: `handler/user.go`
- Modify: `handler/admin_provider.go`
- Modify: `internal/app/runtime.go`
- Modify: `internal/service/client_credentials_service.go`
- Modify: `handler/oauth_social_login.go`
- Test: `handler/user_test.go`
- Test: `handler/oauth_test.go`
- Test: `internal/service/client_credentials_service_test.go`

- [ ] **Step 1: 替换 `gorm.ErrRecordNotFound` 判断**

```go
if errors.Is(err, store.ErrNotFound) {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
}
```

- [ ] **Step 2: 清理上层 `gorm` 依赖**

```go
import (
	"errors"

	"github.com/ysicing/go-template/store"
)
```

- [ ] **Step 3: 修正测试断言**

```go
require.ErrorIs(t, err, store.ErrNotFound)
```

- [ ] **Step 4: 跑 handler/service 定向测试**

Run: `go test ./handler ./internal/service ./internal/app -run 'TestClientCredentials|TestConfirmSocialLink|TestUserHandler|TestSocialProvider|TestSeed'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add handler/user.go handler/admin_provider.go internal/app/runtime.go internal/service/client_credentials_service.go handler/oauth_social_login.go handler/user_test.go handler/oauth_test.go internal/service/client_credentials_service_test.go
git commit -m "refactor(app): 统一使用 store 未找到错误"
```

### Task 3: 收拢 client credentials 事务边界

**Files:**
- Modify: `store/oauth_client.go`
- Modify: `internal/service/client_credentials_service.go`
- Modify: `internal/app/bootstrap.go`
- Test: `internal/service/client_credentials_service_test.go`
- Test: `handler/client_credentials_test.go`

- [ ] **Step 1: 在 `store` 层新增事务骨架方法**

```go
func (s *OAuthClientStore) IssueClientAccessToken(ctx context.Context, token *model.Token, auditLog *model.AuditLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		return tx.Create(auditLog).Error
	})
}
```

- [ ] **Step 2: 新增撤销事务方法**

```go
func (s *OAuthClientStore) RevokeClientAccessToken(ctx context.Context, tokenID uint, auditLog *model.AuditLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.Token{}, "id = ?", tokenID).Error; err != nil {
			return err
		}
		return tx.Create(auditLog).Error
	})
}
```

- [ ] **Step 3: 从 service 移除 `DB` 字段**

```go
type ClientCredentialsServiceDeps struct {
	Clients        *store.OAuthClientStore
	Audit          *store.AuditLogStore
	AccessTokenTTL time.Duration
}
```

- [ ] **Step 4: 改写 `Exchange` / `RevokeForClient` 调用 store**

```go
if err := s.clients.IssueClientAccessToken(ctx, token, auditLog); err != nil {
	return nil, err
}
```

- [ ] **Step 5: 跑定向测试**

Run: `go test ./internal/service ./handler -run 'TestClientCredentials'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add store/oauth_client.go internal/service/client_credentials_service.go internal/service/client_credentials_service_test.go internal/app/bootstrap.go handler/client_credentials_test.go
git commit -m "refactor(oauth): 下沉客户端凭证事务"
```

### Task 4: 收拢 social login 和 admin delete 事务边界

**Files:**
- Modify: `store/social_account.go`
- Modify: `store/user.go`
- Modify: `handler/oauth.go`
- Modify: `handler/oauth_social_login.go`
- Modify: `handler/admin.go`
- Modify: `handler/oauth_test.go`
- Modify: `handler/auth_flow_test.go`

- [ ] **Step 1: 为 social login 新增原子创建方法**

```go
func (s *SocialAccountStore) CreateUserWithSocialAccount(
	ctx context.Context,
	user *model.User,
	account *model.SocialAccount,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		account.UserID = user.ID
		return tx.Create(account).Error
	})
}
```

- [ ] **Step 2: 为用户删除新增级联事务方法**

```go
func (s *UserStore) DeleteCascade(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Delete(&model.User{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.APIRefreshToken{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", id).Delete(&model.CheckInRecord{}).Error
	})
}
```

- [ ] **Step 3: 从 handler 依赖中去掉 `DB`**

```go
type OAuthDeps struct {
	Users          oauthUserStore
	SocialAccounts oauthSocialAccountStore
	// no DB
}
```

- [ ] **Step 4: 让 handler 只编排，不直接开事务**

```go
if err := h.socialAccounts.CreateUserWithSocialAccount(ctx, user, newSocialAccount); err != nil {
	return nil, err
}
```

- [ ] **Step 5: 跑定向测试**

Run: `go test ./handler -run 'TestConfirmSocialLink|TestSocialLink|TestAdminHandler_DeleteUser'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add store/social_account.go store/user.go handler/oauth.go handler/oauth_social_login.go handler/admin.go handler/oauth_test.go handler/auth_flow_test.go
git commit -m "refactor(handler): 下沉社交登录和删用户事务"
```

### Task 5: 清理上层 `*gorm.DB` 持有与注入

**Files:**
- Modify: `handler/oauth.go`
- Modify: `handler/admin.go`
- Modify: `internal/service/client_credentials_service.go`
- Modify: `internal/app/bootstrap.go`
- Modify: `handler/client_credentials_test.go`
- Modify: `handler/oauth_test.go`
- Modify: `handler/auth_flow_test.go`

- [ ] **Step 1: 删除不再需要的字段**

```go
type AdminHandler struct {
	users adminUserStore
	// no db field
}
```

- [ ] **Step 2: 清理构造参数和测试注入**

```go
h := NewAdminHandler(AdminDeps{
	Users: userStore,
	Audit: auditStore,
})
```

- [ ] **Step 3: 跑全量后端测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add handler/oauth.go handler/admin.go internal/service/client_credentials_service.go internal/app/bootstrap.go handler/client_credentials_test.go handler/oauth_test.go handler/auth_flow_test.go
git commit -m "refactor(layer): 移除上层 gorm 依赖"
```

### Task 6: 拆分超长函数与超大文件

**Files:**
- Modify: `handler/auth.go`
- Modify: `handler/user.go`
- Modify: `handler/admin.go`
- Modify: `handler/mfa.go`
- Modify: `handler/oauth_social_login.go`
- Create: `handler/auth_register_helpers.go`
- Create: `handler/user_sessions_helpers.go`
- Create: `handler/admin_user_helpers.go`
- Create: `handler/mfa_verify_helpers.go`
- Test: `handler/auth_flow_test.go`
- Test: `handler/user_test.go`

- [ ] **Step 1: 先拆输入解析和响应组装**

```go
type registerRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

func readRegisterRequest(c fiber.Ctx) (registerRequest, error) {
	var req registerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return registerRequest{}, err
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	return req, nil
}
```

- [ ] **Step 2: 抽纯业务判断为小函数**

```go
func shouldBumpTokenVersion(user *model.User, permissionChanged bool) bool {
	return permissionChanged || user.TokenVersion < 1
}

func normalizeAdminPermissions(perms []string) ([]string, error) {
	normalized := make([]string, 0, len(perms))
	for _, perm := range perms {
		perm = strings.TrimSpace(perm)
		if perm == "" {
			continue
		}
		if !model.IsValidPermission(perm) {
			return nil, fmt.Errorf("invalid permission: %s", perm)
		}
		normalized = append(normalized, perm)
	}
	return normalized, nil
}
```

- [ ] **Step 3: 保持 handler 主函数只做编排**

```go
func (h *AuthHandler) Register(c fiber.Ctx) error {
	req, err := readRegisterRequest(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	user, emailVerificationRequired, err := h.registerLocalUser(c, req)
	if err != nil {
		return err
	}
	return h.finishRegisterResponse(c, user, emailVerificationRequired)
}
```

- [ ] **Step 4: 每拆一个文件跑对应测试**

Run: `go test ./handler -run 'TestRegister|TestUserHandler|TestMFAVerify|TestAdminUpdateUser|TestSocialLink'`
Expected: PASS

- [ ] **Step 5: 跑静态体量检查**

Run: `find . -name '*.go' -not -path './vendor/*' -exec wc -l {} + | sort -nr | sed -n '1,20p'`
Expected: 重点目标文件行数下降，新增 helper 文件承担拆分后的职责

- [ ] **Step 6: Commit**

```bash
git add handler/auth.go handler/user.go handler/admin.go handler/mfa.go handler/oauth_social_login.go handler/auth_register_helpers.go handler/user_sessions_helpers.go handler/admin_user_helpers.go handler/mfa_verify_helpers.go
git commit -m "refactor(handler): 拆分超长函数职责"
```

### Task 7: 最终验证与收尾

**Files:**
- Modify: `AGENTS.md`（仅当本轮又发现可复用经验时）
- Modify: `CHANGELOG.md`（若仓库要求记录）

- [ ] **Step 1: 跑后端全量验证**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 2: 安装前端依赖并跑前端验证**

Run: `cd web && pnpm install`
Expected: 安装成功，生成 `node_modules`

- [ ] **Step 3: 跑前端 lint/test**

Run: `cd web && pnpm lint && pnpm test`
Expected: PASS

- [ ] **Step 4: 检查遗留的上层事务和 `gorm.ErrRecordNotFound`**

Run: `rg -n '\\.Transaction\\(|gorm\\.ErrRecordNotFound' handler internal store --glob '*.go'`
Expected: 只剩 `store` 内事务；上层不再直接依赖 `gorm.ErrRecordNotFound`

- [ ] **Step 5: 检查工作区并整理提交顺序**

Run: `git status --short`
Expected: 仅包含本轮预期改动
