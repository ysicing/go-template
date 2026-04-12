# OpenAPI Guardrails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 OpenAPI 增加新增路由必补文档的硬约束，并提供 Taskfile 脚手架辅助。

**Architecture:** 通过 Fiber 路由枚举和 `openAPIRoutes()` 元数据做双向比对，保证“注册了的路由必须被文档覆盖、文档声明也不能漂移”；Taskfile 提供本地检查与模板生成功能，CI 执行同一校验。

**Tech Stack:** Fiber v3, Go tests, Taskfile, GitHub Actions

---

### Task 1: 增加路由覆盖一致性测试

**Files:**
- Create: `internal/app/openapi_consistency_test.go`
- Modify: `internal/app/openapi.go`

- [ ] 写失败测试覆盖未文档化路由与全量一致性
- [ ] 运行测试确认失败
- [ ] 实现路由比对辅助函数
- [ ] 运行测试确认通过

### Task 2: 提供本地开发工具

**Files:**
- Modify: `Taskfile.yml`
- Modify: `README.md`

- [ ] 增加 `task openapi:check`
- [ ] 增加 `task openapi:scaffold`
- [ ] 更新 README 使用说明

### Task 3: 接入 CI

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

- [ ] 在 CI 中执行 OpenAPI 覆盖检查
- [ ] 运行定向与回归测试确认通过
