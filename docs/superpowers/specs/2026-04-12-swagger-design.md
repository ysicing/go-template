# Swagger 设计说明

## 目标

为 `go-template` 增加 Swagger UI 与 OpenAPI JSON 输出，并支持按当前用户权限裁剪接口文档。

## 设计

- 提供 `GET /swagger/*` 作为 Swagger UI 入口。
- 提供 `GET /openapi.json` 作为动态 OpenAPI 文档出口。
- OpenAPI 文档不从静态文件读取，而是在服务端根据路由元数据生成。
- 每个 operation 声明：方法、路径、标签、摘要、是否需要登录、所需权限列表。
- 未登录用户仅看到公开接口；已登录用户看到公开接口和登录后可访问接口；管理员看到全部接口；具备指定权限的用户看到对应管理接口。
- Swagger UI 使用官方 Fiber Swagger 中间件，并将文档 URL 指向 `/openapi.json`。

## 范围

首版覆盖当前主要 API 路由与系统路由：
- `/health`
- `/api/version`
- `/api/auth/*`
- `/api/users/*`
- `/api/sessions/*`
- `/api/mfa/*`
- `/api/apps/*`
- `/api/points/*`
- `/api/admin/*`
- GitHub 兼容 OAuth 路由

## 权衡

- 不做请求/响应 schema 的完整建模，首版先保证接口可见性和基本说明。
- 采用代码内元数据而非手写超大 `openapi.json`，以降低漂移风险。
- 暂不自动清理未引用 `components`，因为首版几乎不定义复杂组件。
