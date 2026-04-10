# 用户管理 CRUD 子功能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有模板中补齐用户管理 CRUD 与当前登录用户改密能力，并把前端组织调整为统一后台布局 + 子系统导航 + shadcn 通用组件优先的结构。

**Architecture:** 后端新增 `internal/user/service.go` 作为用户管理服务中心，在 `internal/httpserver` 下增加管理员用户路由和改密接口。前端保持统一后台布局，新增 `subsystems/admin-users` 与 `subsystems/auth` 子系统目录，由统一 `admin-layout` 承载各子系统导航和内容区。

**Tech Stack:** Go, Fiber v3, GORM, React, Vite, React Query, React Router, shadcn/ui style components, Vitest

---

## File Structure

本次实现预计创建或修改以下文件：

- `internal/user/service.go`：用户列表、详情、创建、更新、启停用、软删除、改密服务
- `internal/user/service_test.go`：用户服务单元测试
- `internal/httpserver/routes_admin_users.go`：管理员用户管理 API
- `internal/httpserver/routes_auth.go`：增加 `change-password`
- `internal/httpserver/server.go`：注册管理员用户路由
- `internal/httpserver/server_test.go`：管理员 API 与改密集成测试
- `web/src/app/layouts/admin-layout.tsx`：统一后台布局
- `web/src/app/router.tsx`：切换到子系统路由与统一布局
- `web/src/shared/navigation/types.ts`：导航项定义
- `web/src/shared/ui/table.tsx`：通用表格容器
- `web/src/shared/ui/badge.tsx`：通用状态徽标
- `web/src/shared/ui/select.tsx`：通用选择组件
- `web/src/subsystems/admin-users/navigation.ts`：用户管理子系统导航
- `web/src/subsystems/admin-users/types.ts`：用户管理 DTO
- `web/src/subsystems/admin-users/api/users.ts`：用户管理 API 客户端
- `web/src/subsystems/admin-users/hooks/use-users.ts`：列表查询 hook
- `web/src/subsystems/admin-users/components/user-filters.tsx`：筛选栏
- `web/src/subsystems/admin-users/components/user-table.tsx`：用户表格
- `web/src/subsystems/admin-users/components/user-form-dialog.tsx`：新建 / 编辑表单
- `web/src/subsystems/admin-users/components/user-view-dialog.tsx`：详情弹层
- `web/src/subsystems/admin-users/pages/user-management-page.tsx`：用户管理页
- `web/src/subsystems/auth/components/change-password-card.tsx`：改密卡片
- `web/src/pages/profile.tsx`：接入改密卡片
- `web/src/lib/api.ts`：扩展通用请求能力
- `web/src/pages/__tests__/app.test.tsx`：前端基础测试
- `web/src/subsystems/admin-users/components/__tests__/user-management.test.tsx`：用户管理页面测试
- `web/src/subsystems/auth/components/__tests__/change-password-card.test.tsx`：改密表单测试

---

### Task 1: 实现用户服务与单元测试

**Files:**
- Create: `internal/user/service.go`
- Create: `internal/user/service_test.go`
- Test: `internal/user/service_test.go`

- [ ] **Step 1: 先写用户列表筛选测试**

```go
func TestServiceListUsersByKeywordRoleStatus(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	createTestUser(t, conn, "alice", "alice@example.com", RoleUser, "active")
	createTestUser(t, conn, "bob", "bob@example.com", RoleAdmin, "disabled")

	result, err := service.ListUsers(ListUsersQuery{
		Keyword: "bob",
		Role:    string(RoleAdmin),
		Status:  "disabled",
		Page:    1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Username != "bob" {
		t.Fatalf("expected bob, got %s", result.Items[0].Username)
	}
}
```

- [ ] **Step 2: 先写禁止停用自己测试**

```go
func TestServiceDisableUserRejectsSelf(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	admin := createTestUser(t, conn, "admin", "admin@example.com", RoleAdmin, "active")

	err := service.DisableUser(admin.ID, admin.ID)
	if !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected ErrCannotDisableSelf, got %v", err)
	}
}
```

- [ ] **Step 3: 先写修改密码测试**

```go
func TestServiceChangePassword(t *testing.T) {
	service, conn := newUserServiceForTest(t)
	account := createTestUserWithPassword(t, conn, "user1", "user1@example.com", "oldpass123")

	err := service.ChangePassword(account.ID, ChangePasswordInput{
		OldPassword: "oldpass123",
		NewPassword: "newpass123",
		ConfirmNewPassword: "newpass123",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `go test ./internal/user -v`
Expected: FAIL with `undefined: Service` and missing methods

- [ ] **Step 5: 实现用户服务**

```go
type ListUsersQuery struct {
	Keyword  string
	Role     string
	Status   string
	Page     int
	PageSize int
}

type CreateUserInput struct {
	Username string
	Email    string
	Password string
	Role     string
	Status   string
}

type UpdateUserInput struct {
	Username string
	Email    string
	Role     string
	Status   string
}

type ChangePasswordInput struct {
	OldPassword        string
	NewPassword        string
	ConfirmNewPassword string
}
```

```go
func (s *Service) ListUsers(query ListUsersQuery) (ListUsersResult, error) {
	// 组合 keyword / role / status / pagination 查询
}

func (s *Service) CreateUser(input CreateUserInput) (*User, error) {
	// 唯一性检查 + 密码哈希 + 创建
}

func (s *Service) UpdateUser(userID uint, input UpdateUserInput) (*User, error) {
	// 更新 username / email / role / status
}

func (s *Service) DisableUser(actorID uint, userID uint) error {
	// 拒绝停用自己
}

func (s *Service) DeleteUser(actorID uint, userID uint) error {
	// 拒绝删除自己 + 软删除
}

func (s *Service) ChangePassword(userID uint, input ChangePasswordInput) error {
	// 校验旧密码 + 新密码确认 + 更新 hash
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/user -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/user/service.go internal/user/service_test.go
git commit -m "feat(user): 添加用户管理服务"
```

---

### Task 2: 暴露管理员用户 API 与改密接口

**Files:**
- Create: `internal/httpserver/routes_admin_users.go`
- Modify: `internal/httpserver/routes_auth.go`
- Modify: `internal/httpserver/server.go`
- Modify: `internal/httpserver/server_test.go`
- Test: `internal/httpserver/server_test.go`

- [ ] **Step 1: 先写管理员列表接口测试**

```go
func TestAdminUsersRouteRequiresAdmin(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: 先写改密接口测试**

```go
func TestChangePasswordRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/httpserver -v`
Expected: FAIL with missing `/api/admin/users` or `/api/auth/change-password`

- [ ] **Step 4: 实现管理员用户路由**

```go
app.Get("/api/admin/users", requireAuth(state.Tokens()), requireAdmin, listUsersHandler)
app.Get("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, getUserHandler)
app.Post("/api/admin/users", requireAuth(state.Tokens()), requireAdmin, createUserHandler)
app.Put("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, updateUserHandler)
app.Post("/api/admin/users/:id/enable", requireAuth(state.Tokens()), requireAdmin, enableUserHandler)
app.Post("/api/admin/users/:id/disable", requireAuth(state.Tokens()), requireAdmin, disableUserHandler)
app.Delete("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, deleteUserHandler)
```

- [ ] **Step 5: 在认证路由中新增改密接口**

```go
app.Post("/api/auth/change-password", requireAuth(state.Tokens()), func(c fiber.Ctx) error {
	userID, _ := c.Locals(localUserID).(uint)
	var payload user.ChangePasswordInput
	if err := c.Bind().Body(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", "invalid request body"))
	}
	if err := state.UserService().ChangePassword(userID, payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(shared.Err("CHANGE_PASSWORD_FAILED", err.Error()))
	}
	return c.JSON(shared.OK(map[string]any{"changed": true}))
})
```

- [ ] **Step 6: 注册用户服务到服务状态容器**

```go
type Dependencies struct {
	DB          *gorm.DB
	UserService *user.Service
	Auth        *auth.Service
	Tokens      *auth.TokenManager
	Setup       *setup.Service
	SetupRequired bool
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/httpserver/routes_admin_users.go internal/httpserver/routes_auth.go internal/httpserver/server.go internal/httpserver/server_test.go
git commit -m "feat(api): 添加用户管理与改密接口"
```

---

### Task 3: 建立统一后台布局与共享 UI 组件

**Files:**
- Create: `web/src/app/layouts/admin-layout.tsx`
- Create: `web/src/shared/navigation/types.ts`
- Create: `web/src/shared/ui/table.tsx`
- Create: `web/src/shared/ui/badge.tsx`
- Create: `web/src/shared/ui/select.tsx`
- Modify: `web/src/app/router.tsx`
- Test: `web/src/pages/__tests__/app.test.tsx`

- [ ] **Step 1: 先写后台布局测试**

```tsx
it("renders admin layout navigation area", () => {
  render(
    <AdminLayout
      title="Users"
      navigation={[{ label: "用户列表", to: "/admin/users" }]}
    >
      <div>content</div>
    </AdminLayout>
  );

  expect(screen.getByText("用户列表")).toBeInTheDocument();
  expect(screen.getByText("content")).toBeInTheDocument();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test`
Expected: FAIL with missing `AdminLayout`

- [ ] **Step 3: 实现统一后台布局**

```tsx
export function AdminLayout({ title, description, navigation, children }: AdminLayoutProps) {
  return (
    <div className="grid min-h-[calc(100vh-5rem)] grid-cols-[240px_1fr] gap-6">
      <aside className="rounded-xl border border-border bg-card p-4">
        <nav className="space-y-2">
          {navigation.map((item) => (
            <Link key={item.to} to={item.to}>{item.label}</Link>
          ))}
        </nav>
      </aside>
      <section className="space-y-6">
        <header>
          <h1>{title}</h1>
          {description ? <p>{description}</p> : null}
        </header>
        {children}
      </section>
    </div>
  );
}
```

- [ ] **Step 4: 实现共享 UI 组件**

```tsx
export function Table({ className, ...props }: React.TableHTMLAttributes<HTMLTableElement>) {
  return (
    <div className="overflow-hidden rounded-xl border border-border">
      <table className={cn("w-full text-sm", className)} {...props} />
    </div>
  );
}
```

```tsx
export function StatusBadge({ status }: { status: "active" | "disabled" }) {
  return <span className={cn("rounded-full px-2 py-1 text-xs", status === "active" ? "bg-emerald-100 text-emerald-700" : "bg-slate-200 text-slate-700")}>{status}</span>;
}
```

- [ ] **Step 5: 将后台路由切换到统一布局**

```tsx
<Route
  path="/admin/users"
  element={
    <AdminLayout title="用户管理" navigation={adminUsersNavigation}>
      <UserManagementPage />
    </AdminLayout>
  }
/>
```

- [ ] **Step 6: 运行测试与构建确认通过**

Run: `cd web && pnpm test && pnpm build`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add web/src/app/layouts/admin-layout.tsx web/src/shared/navigation/types.ts web/src/shared/ui/table.tsx web/src/shared/ui/badge.tsx web/src/shared/ui/select.tsx web/src/app/router.tsx web/src/pages/__tests__/app.test.tsx
git commit -m "feat(web): 添加统一后台布局"
```

---

### Task 4: 实现 `admin-users` 子系统

**Files:**
- Create: `web/src/subsystems/admin-users/navigation.ts`
- Create: `web/src/subsystems/admin-users/types.ts`
- Create: `web/src/subsystems/admin-users/api/users.ts`
- Create: `web/src/subsystems/admin-users/hooks/use-users.ts`
- Create: `web/src/subsystems/admin-users/components/user-filters.tsx`
- Create: `web/src/subsystems/admin-users/components/user-table.tsx`
- Create: `web/src/subsystems/admin-users/components/user-form-dialog.tsx`
- Create: `web/src/subsystems/admin-users/components/user-view-dialog.tsx`
- Create: `web/src/subsystems/admin-users/pages/user-management-page.tsx`
- Create: `web/src/subsystems/admin-users/components/__tests__/user-management.test.tsx`
- Test: `web/src/subsystems/admin-users/components/__tests__/user-management.test.tsx`

- [ ] **Step 1: 先写用户管理页渲染测试**

```tsx
it("renders user filters and table headers", () => {
  render(<UserManagementPage />);
  expect(screen.getByPlaceholderText("搜索用户名或邮箱")).toBeInTheDocument();
  expect(screen.getByText("用户名")).toBeInTheDocument();
  expect(screen.getByText("邮箱")).toBeInTheDocument();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test web/src/subsystems/admin-users/components/__tests__/user-management.test.tsx`
Expected: FAIL with missing `UserManagementPage`

- [ ] **Step 3: 定义类型与 API 客户端**

```ts
export type AdminUser = {
  id: number;
  username: string;
  email: string;
  role: "user" | "admin";
  status: "active" | "disabled";
  last_login_at?: string | null;
};

export type ListUsersResponse = {
  items: AdminUser[];
  total: number;
  page: number;
  page_size: number;
};
```

```ts
export async function listUsers(params: URLSearchParams) {
  const response = await api.get(`/admin/users?${params.toString()}`);
  return response.data.data as ListUsersResponse;
}
```

- [ ] **Step 4: 实现筛选栏与表格**

```tsx
export function UserFilters({ keyword, role, status, onKeywordChange, onRoleChange, onStatusChange }: UserFiltersProps) {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      <Input placeholder="搜索用户名或邮箱" value={keyword} onChange={(event) => onKeywordChange(event.target.value)} />
      <Select value={role} onValueChange={onRoleChange} options={[{ label: "全部角色", value: "" }, { label: "管理员", value: "admin" }, { label: "普通用户", value: "user" }]} />
      <Select value={status} onValueChange={onStatusChange} options={[{ label: "全部状态", value: "" }, { label: "启用", value: "active" }, { label: "停用", value: "disabled" }]} />
    </div>
  );
}
```

- [ ] **Step 5: 实现管理页容器**

```tsx
export function UserManagementPage() {
  const [keyword, setKeyword] = useState("");
  const [role, setRole] = useState("");
  const [status, setStatus] = useState("");
  const query = useUsers({ keyword, role, status, page: 1, pageSize: 10 });

  return (
    <div className="space-y-4">
      <UserFilters keyword={keyword} role={role} status={status} onKeywordChange={setKeyword} onRoleChange={setRole} onStatusChange={setStatus} />
      <UserTable items={query.data?.items ?? []} />
    </div>
  );
}
```

- [ ] **Step 6: 补齐查看 / 新建 / 编辑弹层**

```tsx
<UserFormDialog mode="create" />
<UserFormDialog mode="edit" user={selectedUser} />
<UserViewDialog user={selectedUser} />
```

- [ ] **Step 7: 运行测试与构建确认通过**

Run: `cd web && pnpm test && pnpm build`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add web/src/subsystems/admin-users
git commit -m "feat(web): 添加用户管理子系统"
```

---

### Task 5: 在个人中心接入修改密码卡片

**Files:**
- Create: `web/src/subsystems/auth/components/change-password-card.tsx`
- Create: `web/src/subsystems/auth/components/__tests__/change-password-card.test.tsx`
- Modify: `web/src/pages/profile.tsx`
- Modify: `web/src/lib/api.ts`
- Test: `web/src/subsystems/auth/components/__tests__/change-password-card.test.tsx`

- [ ] **Step 1: 先写改密表单测试**

```tsx
it("validates confirm password before submit", async () => {
  render(<ChangePasswordCard />);
  await userEvent.type(screen.getByLabelText("旧密码"), "oldpass123");
  await userEvent.type(screen.getByLabelText("新密码"), "newpass123");
  await userEvent.type(screen.getByLabelText("确认新密码"), "different123");
  await userEvent.click(screen.getByRole("button", { name: "提交" }));
  expect(screen.getByText("两次输入的新密码不一致")).toBeInTheDocument();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test web/src/subsystems/auth/components/__tests__/change-password-card.test.tsx`
Expected: FAIL with missing `ChangePasswordCard`

- [ ] **Step 3: 扩展 API 客户端**

```ts
export async function changePassword(payload: {
  old_password: string;
  new_password: string;
  confirm_new_password: string;
}) {
  const response = await api.post("/auth/change-password", payload);
  return response.data.data as { changed: boolean };
}
```

- [ ] **Step 4: 实现改密卡片**

```tsx
export function ChangePasswordCard() {
  // 旧密码 / 新密码 / 确认新密码表单
}
```

- [ ] **Step 5: 接入个人中心**

```tsx
export function ProfilePage() {
  return (
    <div className="space-y-6">
      <ProfileSummaryCard />
      <ChangePasswordCard />
    </div>
  );
}
```

- [ ] **Step 6: 运行测试与构建确认通过**

Run: `cd web && pnpm test && pnpm build`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add web/src/subsystems/auth/components/change-password-card.tsx web/src/subsystems/auth/components/__tests__/change-password-card.test.tsx web/src/pages/profile.tsx web/src/lib/api.ts
git commit -m "feat(auth): 添加当前用户改密功能"
```

---

### Task 6: 补齐前后端联调与最终验证

**Files:**
- Modify: `web/src/app/router.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `internal/httpserver/server_test.go`
- Modify: `Taskfile.yml`
- Test: `internal/httpserver/server_test.go`
- Test: `web/src/pages/__tests__/app.test.tsx`

- [ ] **Step 1: 先写自己不能删除 / 停用的前端提示测试**

```tsx
it("shows self-protection action state", () => {
  render(<UserTable items={[{ id: 1, username: "admin", email: "admin@example.com", role: "admin", status: "active" }]} currentUserId={1} />);
  expect(screen.getByText("不可删除自己")).toBeInTheDocument();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test`
Expected: FAIL with missing self-protection UI

- [ ] **Step 3: 完善前端提示与刷新逻辑**

```tsx
// 在用户表格中针对当前用户渲染禁用操作态
disabled={row.id === currentUserId}
title={row.id === currentUserId ? "不可删除自己" : "删除用户"}
```

- [ ] **Step 4: 完善 Taskfile**

```yaml
  test:web:
    dir: web
    cmds:
      - pnpm install --frozen-lockfile
      - pnpm test

  verify:
    cmds:
      - go test ./...
      - task test:web
      - task build:web
      - go build ./app/server
```

- [ ] **Step 5: 运行最终验证**

Run: `task verify`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add web/src/app/router.tsx web/src/lib/api.ts internal/httpserver/server_test.go Taskfile.yml
git commit -m "test(admin-users): 完善联调与验证"
```

---

## Self-Review

### Spec coverage

- 统一后台布局：Task 3
- 子系统导航：Task 3、Task 4
- shadcn 通用组件优先：Task 3、Task 4、Task 5
- 用户列表 + 关键词/角色/状态/分页：Task 1、Task 2、Task 4
- 查看 / 新建 / 编辑 / 启停用 / 软删除：Task 1、Task 2、Task 4
- 不能停用 / 删除自己：Task 1、Task 2、Task 6
- 当前登录用户改密：Task 1、Task 2、Task 5
- 忘记密码仅预留：本计划未实现邮件流程，符合规格

### Placeholder scan

- 已检查本计划中不存在未完成占位内容或模糊执行描述

### Type consistency

- 后端用户管理统一使用 `internal/user/service.go` 中的 DTO
- 管理员用户前端类型统一使用 `web/src/subsystems/admin-users/types.ts`
- 当前用户改密统一使用 `ChangePasswordInput`

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-10-admin-users.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
