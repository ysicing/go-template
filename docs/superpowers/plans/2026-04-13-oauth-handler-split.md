# OAuth Handler Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 OAuth handler 按职责拆分并收敛 provider callback 重复逻辑。

**Architecture:** 共享结构、state、callback helper 留在公共文件；GitHub/Google、社交绑定、WebAuthn 分拆到独立文件。整个过程保持同包内方法调用，不改对外接口。

**Tech Stack:** Go, Fiber, oauth2, WebAuthn, GORM

---

### Task 1: 拆出公共 helper
**Files:**
- Modify: `handler/oauth.go`
- Create: `handler/oauth_provider.go`
- Test: `handler/oauth_test.go`

- [ ] 提取 provider/state/shared callback helper
- [ ] 保留现有行为与错误响应
- [ ] 运行 `go test ./handler -run 'TestGitHub|TestGoogle'`

### Task 2: 拆出 provider 文件
**Files:**
- Create: `handler/oauth_github.go`
- Create: `handler/oauth_google.go`
- Modify: `handler/oauth.go`
- Test: `handler/oauth_test.go`

- [ ] 将 GitHub/Google 配置与用户信息拉取逻辑迁移到独立文件
- [ ] 使用共享 callback helper 接线
- [ ] 运行 `go test ./handler -run 'TestGitHub|TestGoogle'`

### Task 3: 拆出社交绑定与 WebAuthn
**Files:**
- Create: `handler/oauth_social_link.go`
- Create: `handler/oauth_webauthn.go`
- Modify: `handler/oauth.go`
- Test: `handler/oauth_test.go`

- [ ] 迁移 social link / WebAuthn 相关逻辑
- [ ] 保持绑定确认、MFA、WebAuthn 流程不变
- [ ] 运行 `go test ./handler -run 'Test.*Social|Test.*WebAuthn'`

### Task 4: 完整验证
**Files:**
- Modify: `handler/oauth.go`
- Test: `handler/oauth_test.go`

- [ ] 执行 `task fmt`
- [ ] 执行 `go test ./handler ./internal/app ./store ./internal/service`
