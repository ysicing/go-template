# go-template

一个可直接复用的 Go 全栈模板，默认提供：

- 后端：`Go + Fiber v3 + GORM`
- 认证：登录、安装向导、修改密码、管理员用户管理
- 数据库：`sqlite(nocgo)` / `PostgreSQL` / `MySQL`
- 缓存：`memory` / `Redis`
- 前端：`React + Vite + shadcn/ui + pnpm`
- 工程化：`Taskfile` / `Dockerfile` / `GitHub Actions`
- 发布：默认将前端静态资源嵌入 Go 二进制

## 默认端口

- 后端监听：`0.0.0.0:3206`
- 前端开发：`0.0.0.0:5173`

前后端分离开发时，Vite 会把 `/api` 代理到 `http://127.0.0.1:3206`。

## 首次启动

第一次启动时如果不存在 `configs/config.yaml`（或环境变量 `APP_CONFIG_PATH` 指向的配置文件），系统会进入安装向导。

安装向导会完成：

- 生成核心配置文件
- 校验数据库 / 缓存连接
- 执行 `gorm.AutoMigrate`
- 创建管理员账号
- 写入初始化状态与默认系统设置

## 常用命令

```bash
task dev:backend
task dev:web
task build:web
task build:go
task docker:build
task verify
```

## 开发模式

### 前后端分离

终端一：

```bash
task dev:backend
```

终端二：

```bash
task dev:web
```

### 单二进制

```bash
task build:web
task build:go
./server
```

## Swagger / OpenAPI

生成文档：

```bash
task docs:swagger
```

访问地址：

- Swagger UI：`/swagger/index.html`
- OpenAPI JSON：`/swagger/doc.json`

## 版本规则

构建版本通过 `ldflags` 注入，统一用于：

- 启动日志
- Swagger 版本
- 前端页脚
- Docker 镜像构建标签元信息

默认规则：

- 日常开发：`master-<shortsha>-<timestamp>`
- 正式发版：`<tag>-<shortsha>-<timestamp>`

构建辅助脚本：

```bash
./hack/build-metadata.sh
```

## Docker

本地构建：

```bash
task docker:build
```

镜像为多阶段构建：

1. 构建前端静态资源
2. 构建 Go 二进制
3. 使用 distroless 运行镜像

容器默认约定：

- 配置文件路径：`/data/config.yaml`
- 持久化目录：`/data`
- SQLite 默认数据文件：`/data/app.db`

## CI / Release

- PR：跑 Go 测试、前端测试与构建、Docker 校验构建，不推镜像
- `master` / `main`：推送 `linux/amd64` 镜像
- Tag：推送多架构镜像

## 注意事项

- `web/dist/embed-placeholder.txt` 用于保证干净 checkout 时 `go:embed` 可编译
- 真正发布前仍应执行一次前端构建，确保嵌入最新静态资源
