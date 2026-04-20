# go-template

`go-template` 是一个面向个人项目和团队内部脚手架的 `Go + React` 全栈模板。

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

## Docker 运行

镜像会先修正 `/data` 权限，再以非 root 用户启动应用，并将运行时数据目录固定在 `/data`：

- 默认 SQLite 数据库：`/data/go-template.db`
- 默认配置文件路径：`/data/config.yaml`
- 可选文件日志：`/data/logs/go-template/app.log`
- 默认监听端口：`3206`

最小示例：

```bash
docker run -d \
  --name go-template \
  -p 3206:3206 \
  -e JWT_SECRET='replace-with-a-long-random-secret' \
  -v go-template-data:/data \
  ghcr.io/ysicing/go-template:master
```

如需挂载配置文件：

```bash
docker run -d \
  --name go-template \
  -p 3206:3206 \
  -e JWT_SECRET='replace-with-a-long-random-secret' \
  -v $(pwd)/config.yaml:/data/config.yaml:ro \
  -v go-template-data:/data \
  ghcr.io/ysicing/go-template:master
```

说明：

- 生产环境请始终显式设置 `JWT_SECRET`
- 若使用默认 SQLite，请务必挂载 `/data`，否则容器重建后数据会丢失
- 绑定宿主机目录到 `/data` 时，入口脚本会自动修正权限
- 若改用 MySQL / Postgres，可通过 `/data/config.yaml` 指定数据库连接

## 模板能力

- 控制台首页：`http://localhost:3206/`
- Swagger UI：`http://localhost:3206/swagger/index.html`
- OpenAPI JSON：`http://localhost:3206/openapi.json`
- API Base：`http://localhost:3206/api`
- OIDC Discovery：`http://localhost:3206/.well-known/openid-configuration`
- Authorization：`http://localhost:3206/authorize`
- Token：`http://localhost:3206/oauth/token`
- UserInfo：`http://localhost:3206/oauth/userinfo`
- JWKS：`http://localhost:3206/oauth/keys`
- GitHub OAuth Authorize：`http://localhost:3206/login/oauth/authorize`
- GitHub OAuth Token：`http://localhost:3206/login/oauth/access_token`

Swagger 文档会按当前登录用户权限动态裁剪：

- 未登录：仅显示公开接口
- 已登录普通用户：显示公开接口与本人可访问接口
- 管理员 / 具备对应权限的用户：额外显示相关管理接口

## 核心配置

从 `config.example.yaml` 开始，优先关注：

- `server.addr`
- `database.driver`
- `database.dsn`
- `jwt.secret`
- `jwt.issuer`
- `log.file.enabled`
- `log.file.path`
- `security.allow_insecure`
- `security.encryption_key`
- `security.oidc_secret`
- `admin.username`
- `admin.password`（留空会跳过初始管理员自动创建）
- `admin.email`

系统设置中的站点标题、注册开关、邮件验证、Turnstile 等能力可在控制台里继续配置。

## 开发命令

```bash
task run
task dev
task build
task fmt
task openapi:check
go test ./...
cd web && pnpm test
```

`task run` 默认按本地开发模式启动：`Taskfile.yml` 会在未显式设置 `SECURITY_ALLOW_INSECURE` 时默认注入 `true`，便于在没有 `config.yaml` 的情况下直接运行。生产环境请显式配置 `jwt.secret`，并保持 `security.allow_insecure: false`。

OpenAPI 维护约束：

- 新增或修改受管路由后，先运行 `task openapi:check`
- 需要添加文档元数据时，可用 `task openapi:scaffold METHOD=GET PATH=/api/example SUMMARY='示例接口' TAG=example AUTH=true`
- 若接口需要权限，可追加 `PERMISSIONS='admin.users.read,admin.stats.read'`

## 参考

- 示例配置：`config.example.yaml`
- 前端说明：`web/README.md`
