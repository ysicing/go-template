# go-template

`go-template` 是一个带嵌入式控制台的身份与权限服务模板，使用 `Go + React` 实现。

后端提供认证、授权、管理 API；前端提供默认控制台，构建后嵌入最终二进制。

## 当前范围

- 本地账号登录、刷新、注销
- Social Login Provider 管理
- MFA / WebAuthn
- 用户、权限、系统设置、审计日志
- 积分示例模块

## 技术栈

- 后端：Go、Fiber、GORM
- 前端：React、Vite、TypeScript
- 默认存储：SQLite
- 交付形态：单二进制，可选 Docker 运行

## 快速开始

```bash
cp config.example.yaml config.yaml
$EDITOR config.yaml
task web:install
git config core.hooksPath .githooks
task build
./id
```

默认启动后访问 `http://localhost:3206`。

首次本地开发如果只想直接跑后端，也可以执行：

```bash
task run
```

`task run` 默认会注入 `SECURITY_ALLOW_INSECURE=true`，便于在本地快速启动；生产环境请显式配置 `jwt.secret`，并保持 `security.allow_insecure: false`。

如需启用仓库内置的提交前格式化 hook，可执行：

```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

## Docker 运行

镜像会先修正 `/data` 权限，再以非 root 用户启动应用，并将运行时数据目录固定在 `/data`。

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

### Docker Compose 本地预览

仓库内置 `docker-compose.yaml`，用于本地构建镜像并快速预览完整控制台。Compose 会同时启动 PostgreSQL 18 和带密码的 Redis；PostgreSQL 作为默认数据库，Redis 用于缓存和异步任务队列。

```bash
docker compose up --build
```

启动后访问：

- 控制台：`http://localhost:3206`
- Swagger UI：`http://localhost:3206/swagger/index.html`
- 默认管理员：`admin` / `admin123456`

停止服务：

```bash
docker compose down
```

如需清空本地数据：

```bash
docker compose down -v
```

## 控制台与 API

- 控制台首页：`http://localhost:3206/`
- Swagger UI：`http://localhost:3206/swagger/index.html`
- OpenAPI JSON：`http://localhost:3206/openapi.json`
- API Base：`http://localhost:3206/api`

Swagger 文档会按当前登录用户权限动态裁剪：

- 未登录：仅显示公开接口
- 已登录普通用户：显示公开接口与本人可访问接口
- 管理员 / 具备对应权限的用户：额外显示相关管理接口

## 核心配置

从 `config.example.yaml` 开始，优先关注这些字段：

- `server.addr`
- `database.driver`
- `database.dsn`
- `jwt.secret`
- `jwt.issuer`
- `log.file.enabled`
- `log.file.path`
- `security.allow_insecure`
- `security.encryption_key`
- `admin.username`
- `admin.password`（留空会跳过初始管理员自动创建）
- `admin.email`

系统设置中的站点标题、注册开关、邮件验证、Turnstile 等能力可在控制台里继续配置。

## 开发命令

```bash
task web:install
task run
task dev
task build
task fmt
task test
task verify
```

- `task run`：启动后端
- `task dev`：启动前端开发服务器，默认 `http://localhost:3000`
- `task build`：构建前端并打包后端二进制
- `task test`：执行后端和前端测试
- `task verify`：执行接近 CI 的完整校验

如需只跑某一侧：

```bash
go test ./...
cd web && pnpm test
cd web && pnpm lint
cd web && pnpm build
```

## OpenAPI 维护

- 新增或修改受管路由后，运行 `task openapi:check`
- 需要添加文档元数据时，可用：

```bash
task openapi:scaffold METHOD=GET PATH=/api/example SUMMARY='示例接口' TAG=example AUTH=true
```

## 参考

- 示例配置：`config.example.yaml`
- 前端说明：`web/README.md`
