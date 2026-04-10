# Go 全栈模板设计说明

## 1. 概述

本项目目标是构建一个可复用的 Go 全栈模板，满足以下核心要求：

- 后端使用 `Go + Fiber v3 + GORM`
- 数据库支持 `sqlite3(nocgo)`、`PostgreSQL`、`MySQL`
- 缓存支持 `memory` 或 `Redis`
- 前端使用 `React + Vite + shadcn/ui`，目录固定为 `web/`
- 支持前后端分离开发
- 生产默认将前端构建产物嵌入 Go 二进制
- 使用 `Taskfile` 统一开发、构建、测试、打包命令
- 内置首次安装向导，负责初始化核心配置、数据库、缓存与管理员账号
- 内置基础登录能力，支持 `JWT access token + refresh token`
- 提供基础后台壳、用户中心、多语言、明暗模式、预设主题色切换
- 提供 `Dockerfile` 与 GitHub Actions 流水线

该模板定位为“少而完整”的工程模板，而不是大而全的后台框架。

## 2. 技术决策

### 2.1 后端

- Web 框架：`gofiber/fiber/v3`
- ORM：`gorm`
- 数据库驱动：
  - `modernc.org/sqlite` + GORM sqlite driver
  - GORM postgres driver
  - GORM mysql driver
- 配置：`YAML 配置文件 + 环境变量覆盖`
- 认证：`JWT access token + refresh token`
- 日志：可配置日志级别与输出策略

### 2.2 前端

- 工具链：`Vite`
- 框架：`React`
- 包管理器：`pnpm`
- 组件体系：优先使用 `shadcn/ui`
- 样式主题：
  - `light / dark / system`
  - 预设主题色切换
- 国际化：首版内置 `zh-CN` 与 `en-US`

### 2.3 工程化

- 任务执行：`Taskfile.yml`
- 镜像构建：`Dockerfile`
- CI/CD：GitHub Actions
- 发布形态：单二进制 + 可选容器镜像

## 3. 目录结构

建议目录结构如下：

```text
.
├── app/
│   └── server/                    # main 入口
├── internal/
│   ├── auth/                      # 登录、密码、JWT、刷新
│   ├── bootstrap/                 # 启动编排
│   ├── cache/                     # memory / redis 抽象与实现
│   ├── config/                    # YAML 读取与 env 覆盖
│   ├── db/                        # GORM 初始化与驱动选择
│   ├── httpserver/                # Fiber、routes、middleware、handlers
│   ├── setup/                     # 安装向导、初始化检查
│   ├── shared/                    # 响应模型、错误、通用工具
│   ├── system/                    # 系统初始化状态、系统设置
│   └── user/                      # 用户领域
├── web/
│   ├── src/                       # React + Vite + shadcn/ui 源码
│   ├── public/
│   ├── dist/                      # 前端构建产物
│   ├── web.go                     # embed 前端资源
│   ├── package.json
│   ├── pnpm-lock.yaml
│   └── components.json
├── configs/
│   ├── config.example.yaml
│   └── config.yaml
├── hack/                          # 构建、发布、辅助脚本
├── .github/
│   └── workflows/
├── Dockerfile
├── Taskfile.yml
├── go.mod
├── pnpm-workspace.yaml
└── .gitignore
```

### 目录边界约束

- `app/server` 仅放启动入口
- `web/web.go` 仅负责嵌入静态资源
- `hack/` 仅放工程辅助脚本，不放业务逻辑
- 业务逻辑优先放在按领域聚合的 `internal/*` 目录下
- 不引入 `pkg/`，避免过早公共化
- 不做重型 `service/repository/model` 模板化分层，优先按领域收敛

## 4. 配置分层

### 4.1 核心启动配置

核心启动配置写入 `configs/config.yaml`，并允许环境变量覆盖。此类配置仅包含启动必须的信息：

- 数据库连接配置
- 缓存配置
- 监听地址与端口
- 日志配置
- JWT 签名密钥等启动必须项

### 4.2 运行期业务配置

非启动必须配置写入数据库中的系统设置，例如：

- 站点名称
- 注册开关
- Token 时长
- 默认头像策略

该类配置由后台管理，不要求回写配置文件。

## 5. 启动流程与安装向导

### 5.1 启动流程

应用启动按以下顺序执行：

1. 读取 `configs/config.yaml`
2. 若配置文件存在，则加载并应用环境变量覆盖
3. 初始化日志
4. 根据当前状态决定是否初始化数据库与缓存
5. 初始化 Fiber 应用
6. 根据系统状态挂载不同的页面路由与 API 路由
7. 若前端已构建，则托管嵌入的静态资源

### 5.2 系统状态

系统只保留两个核心状态：

- `setup_required`
  - 配置文件不存在；或
  - 配置存在但数据库中没有初始化标记
- `ready`
  - 配置存在、数据库可连接且初始化标记存在

### 5.3 安装向导职责

首次访问安装向导时，需要完成以下步骤：

- 采集数据库配置：
  - `sqlite(nocgo)` / `postgres` / `mysql`
- 采集缓存配置：
  - `memory` / `redis`
- 采集启动配置：
  - 监听地址与端口
  - 日志级别
- 采集管理员信息：
  - 用户名
  - 邮箱
  - 密码
- 校验数据库与缓存连接
- 生成 `configs/config.yaml`
- 使用新配置初始化数据库与缓存
- 执行 `gorm.AutoMigrate`
- 写入系统初始化状态
- 创建管理员账号
- 写入默认系统设置

### 5.4 初始化标记

数据库中维护一张极小的系统初始化状态表，用于判断系统是否已安装完成。其职责是：

- 避免仅凭配置文件判断是否安装完成
- 支持初始化失败后的再次提交
- 防止配置已写入但数据库尚未完成初始化时误判为可用

### 5.5 错误处理策略

- 配置写入前必须先完成输入校验与依赖连通性检测
- 创建管理员与初始化标记放在同一数据库事务中
- 初始化失败时不写入“已初始化”标记
- 允许用户重新执行安装流程
- 首版不实现复杂配置文件回滚机制

## 6. 数据模型

### 6.1 用户表

首版用户表至少包含：

- `id`
- `username`
- `email`
- `password_hash`
- `role`，取值为 `user` 或 `admin`
- `status`
- `last_login_at`
- `created_at`
- `updated_at`
- `deleted_at`

其中 `deleted_at` 使用 GORM 软删除能力。

### 6.2 系统初始化状态表

该表至少包含：

- `id`
- `initialized_at`
- `version`

### 6.3 系统设置表

该表采用键值形式，至少包含：

- `key`
- `value`
- `group`

用途是存放运行期可调整的业务设置，避免模板首版过早引入复杂设置模型。

## 7. 数据库与缓存策略

### 7.1 数据库支持

通过统一配置切换数据库方言：

- `sqlite`
- `postgres`
- `mysql`

约束如下：

- 使用 GORM 管理模型与连接
- 使用 `AutoMigrate`
- 默认支持软删除
- 数据库差异优先收敛在 `internal/db`

### 7.2 缓存支持

缓存层提供统一接口，底层可选：

- `memory`
- `redis`

默认承担这些职责：

- 验证码
- 限流计数
- 临时会话或短期状态
- 通用 KV

模板不得强绑定 Redis，仅使用内存缓存时也应可完整运行。

## 8. 认证与权限

### 8.1 登录能力

支持以下登录方式：

- 用户名 + 密码
- 邮箱 + 密码

前端可在同一个输入框内输入用户名或邮箱，由后端统一识别。

### 8.2 Token 模型

登录成功后签发：

- `access token`
- `refresh token`

约束如下：

- `access token` 为短期有效
- `refresh token` 为长期有效
- 首版使用签名型 token
- 不首版实现复杂 token 黑名单或多设备会话表
- 为后续结合 Redis 做 session 管理预留扩展空间

### 8.3 权限模型

模板首版只支持简单角色：

- `user`
- `admin`

不首版实现完整 RBAC。

## 9. 前端能力

### 9.1 页面范围

前端默认包含这些页面：

- `/setup`：安装向导
- `/login`：登录页
- `/`：受保护首页
- `/profile`：用户中心
- `/admin`：后台壳页
- `/admin/settings`：基础系统设置页

### 9.2 国际化

首版内置语言：

- `zh-CN`
- `en-US`

默认优先级：

1. 用户偏好（后续可扩展）
2. 本地存储
3. 浏览器语言
4. 默认 `zh-CN`

### 9.3 明暗模式与主题色

首版支持：

- `light`
- `dark`
- `system`

主题色采用预设方案切换，不支持后台输入任意品牌色。预设主题色至少包含：

- `slate` 或 `zinc`
- `blue`
- `green`
- `violet`

主题切换基于 CSS variables 与 shadcn token 体系实现。

### 9.4 偏好存储

首版默认将语言、主题模式、主题色保存在浏览器本地存储中。

模板可在用户中心预留偏好设置入口，但首版不强制实现数据库持久化。

## 10. 路由与前后端交互

### 10.1 路由分层

页面路由由前端 SPA 管理，API 路由统一使用 `/api` 前缀。

示例 API：

- `/api/setup/status`
- `/api/setup/install`
- `/api/auth/login`
- `/api/auth/refresh`
- `/api/auth/logout`
- `/api/auth/me`
- `/api/system/settings`

### 10.2 安装向导交互

前端启动后优先调用 `/api/setup/status`：

- 若返回 `setup_required`，强制跳转 `/setup`
- 若返回 `ready`，进入正常登录与业务流程

安装成功后跳转 `/login`。

### 10.3 登录态检查

前端应用初始化时调用 `/api/auth/me` 获取当前用户与角色信息，并据此做：

- 是否已登录判断
- 是否 admin 判断

路由守卫至少校验：

- 系统是否已初始化
- 用户是否已登录
- 当前路由是否需要管理员权限

## 11. 开发与发布模式

### 11.1 开发模式

主推前后端分离开发：

- Go API 独立运行
- Vite Dev Server 独立运行
- Vite 通过代理转发 `/api`

同时保留一个辅助开发模式：

- 先构建前端
- 再由 Go 托管静态资源

该模式适合快速联调与 smoke test，但不是默认开发方式。

### 11.2 生产模式

生产构建时：

- 前端构建生成 `web/dist`
- `web/web.go` 使用 `embed` 打包静态资源
- Fiber 托管嵌入静态资源
- SPA 路由 fallback 到 `index.html`

## 12. Taskfile

`Taskfile.yml` 至少覆盖以下命令：

- 后端开发启动
- 前端开发启动
- 前后端联调启动
- 前端构建
- Go 单元测试
- 全量构建
- Docker 构建
- 示例配置生成
- 质量检查

命令命名应清晰、职责单一，便于 CI 和本地复用。

## 13. Docker 设计

默认提供多阶段 `Dockerfile`：

1. Node 阶段：
   - 安装 `pnpm`
   - 安装前端依赖
   - 构建前端产物
2. Go 阶段：
   - 编译后端
   - 嵌入前端资源
3. Runtime 阶段：
   - 产出轻量运行镜像

镜像默认面向生产部署，配置文件通过挂载或环境变量覆盖。

## 14. GitHub Actions 设计

### 14.1 `pull_request`

PR 流水线执行：

- 前端依赖安装与构建
- Go 单元测试
- Docker 单架构构建验证

约束：

- 不推送镜像
- 只校验镜像可构建

### 14.2 主干 `push`

主干开发分支推送时执行：

- 前端构建
- Go 单元测试
- 构建并推送 `linux/amd64` 镜像

镜像 tag 使用分支名。

### 14.3 Git Tag 发布

Git tag 触发正式发布时执行：

- 前端构建
- Go 单元测试
- 构建并推送多架构镜像

多架构至少包含：

- `linux/amd64`
- `linux/arm64`

镜像 tag 同时包含：

- Git tag
- 分支名

这里“tag 是分支名”指 Docker 镜像 tag 规则，而不是 Git tag 命名规则。

## 15. 测试策略

### 15.1 Go 单元测试

至少覆盖：

- 配置加载与 env 覆盖
- 数据库驱动选择
- 内存缓存实现
- 密码校验
- JWT 签发与刷新
- setup 状态判断与初始化服务

### 15.2 Go API 集成测试

至少覆盖：

- `/api/setup/status`
- `/api/setup/install`
- `/api/auth/login`
- `/api/auth/refresh`
- 受保护路由访问

### 15.3 前端测试

首版保持轻量，重点验证：

- app shell
- 路由守卫
- setup 跳转
- 登录流程

CI 中至少保证：

- Go 单元测试
- 前端构建
- Docker 构建

## 16. 非目标

首版明确不包含以下内容：

- 完整 RBAC
- 独立 migration 文件体系
- 复杂 token 撤销机制
- 多设备会话管理
- 后台自定义任意品牌色
- 大而全的业务后台模块

## 17. 风险与约束

### 17.1 安装向导与运行态耦合风险

通过集中式系统状态服务与 setup/ready 路由分离规避。

### 17.2 配置来源混乱风险

通过“启动配置只认 YAML + env，运行期配置只认数据库”的分层规避。

### 17.3 多数据库支持散落风险

通过在 `internal/db` 收敛驱动选择与差异处理规避。

### 17.4 前端开发态与生产态割裂风险

通过明确区分“前后端分离开发”与“生产嵌入发布”，并在 Taskfile、Dockerfile、CI 中统一表达来规避。

### 17.5 模板过重风险

通过限制角色模型、限制设置模型、限制认证复杂度来控制范围。

## 18. 最终范围总结

本模板最终交付目标为：

- 一个可直接启动开发的 Go 全栈模板
- 开箱支持首次安装向导
- 开箱支持登录、用户中心、后台壳
- 开箱支持多数据库、双缓存实现
- 开箱支持前后端分离开发与嵌入式生产发布
- 开箱支持 Taskfile、Docker、多架构 CI/CD

模板重点是为后续业务项目提供稳固起点，而不是一次性覆盖全部业务能力。
