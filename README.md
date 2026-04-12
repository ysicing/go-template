# Auth Template

`Auth Template` 是一个面向个人项目和团队内部脚手架的 `Go + React` 全栈模板。

它保留了开箱即用的认证与控制台底座：

- 本地账号登录、刷新、注销
- OIDC Provider
- GitHub Compatible OAuth 端点
- Social Login Provider 管理
- MFA / WebAuthn
- 用户、权限、系统设置、审计日志
- 应用管理与积分示例模块

它刻意移除了与模板定位不匹配的业务域：

- 组织与工作空间
- Webhook、监控、Telegram 管理
- 报价与其他强业务模块

## 快速开始

```bash
cp config.example.yaml config.yaml
$EDITOR config.yaml
task build
./id
```

默认启动后访问 `http://localhost:3206`。

## 模板能力

- 控制台首页：`http://localhost:3206/`
- API Base：`http://localhost:3206/api`
- OIDC Discovery：`http://localhost:3206/.well-known/openid-configuration`
- Authorization：`http://localhost:3206/authorize`
- Token：`http://localhost:3206/oauth/token`
- UserInfo：`http://localhost:3206/oauth/userinfo`
- JWKS：`http://localhost:3206/oauth/keys`
- GitHub OAuth Authorize：`http://localhost:3206/login/oauth/authorize`
- GitHub OAuth Token：`http://localhost:3206/login/oauth/access_token`

## 核心配置

从 `config.example.yaml` 开始，优先关注：

- `server.addr`
- `database.driver`
- `database.dsn`
- `jwt.secret`
- `jwt.issuer`
- `security.allow_insecure`
- `security.encryption_key`
- `security.oidc_secret`
- `admin.username`
- `admin.password`
- `admin.email`

系统设置中的站点标题、注册开关、邮件验证、Turnstile 等能力可在控制台里继续配置。

## 开发命令

```bash
task run
task dev
task build
task fmt
go test ./...
cd web && pnpm test
```

`task run` 默认按本地开发模式启动：`Taskfile.yml` 会在未显式设置 `SECURITY_ALLOW_INSECURE` 时默认注入 `true`，便于在没有 `config.yaml` 的情况下直接运行。生产环境请显式配置 `jwt.secret`，并保持 `security.allow_insecure: false`。

## 文档

- 快速启动：`docs/QUICKSTART.md`
- 前端说明：`web/README.md`
- 前端错误态约束：`docs/FRONTEND_ERROR_HANDLING.md`
