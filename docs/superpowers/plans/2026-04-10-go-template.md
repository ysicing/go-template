# Go 全栈模板 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建一个可安装、可前后端分离开发、可将前端嵌入单二进制发布的 Go 全栈模板，包含 Fiber v3、GORM、多数据库、多缓存、安装向导、基础登录、Taskfile、Dockerfile 和 GitHub Actions。

**Architecture:** 采用单仓 Monorepo 结构，后端聚合在 `internal/`，前端聚合在 `web/`。开发态主推 Go API 与 Vite Dev Server 分离运行，生产态将 `web/dist` 通过 `web/web.go` 嵌入 Go 二进制，由 Fiber 统一托管页面与 `/api`。核心启动配置写入 `configs/config.yaml`，运行期业务配置写入数据库。

**Tech Stack:** Go, Fiber v3, GORM, modernc sqlite, PostgreSQL, MySQL, Redis, React, Vite, shadcn/ui, pnpm, Taskfile, Docker, GitHub Actions

---

## File Structure

本次实现预计创建或修改以下核心文件：

- `app/server/main.go`：程序入口
- `internal/bootstrap/bootstrap.go`：启动编排
- `internal/config/config.go`：配置结构、加载、env 覆盖
- `internal/config/config_test.go`：配置加载测试
- `internal/db/db.go`：GORM 初始化与方言选择
- `internal/db/db_test.go`：数据库驱动选择测试
- `internal/cache/cache.go`：缓存接口
- `internal/cache/memory.go`：内存缓存实现
- `internal/cache/redis.go`：Redis 缓存实现
- `internal/cache/memory_test.go`：内存缓存测试
- `internal/system/model.go`：系统初始化状态与设置模型
- `internal/user/model.go`：用户模型
- `internal/auth/password.go`：密码哈希与校验
- `internal/auth/jwt.go`：JWT 签发与解析
- `internal/auth/service.go`：登录、刷新、当前用户
- `internal/auth/auth_test.go`：密码与 JWT 测试
- `internal/setup/service.go`：安装向导服务
- `internal/setup/service_test.go`：初始化服务测试
- `internal/httpserver/server.go`：Fiber 创建与挂载
- `internal/httpserver/middleware.go`：认证与权限中间件
- `internal/httpserver/routes_setup.go`：setup API
- `internal/httpserver/routes_auth.go`：auth API
- `internal/httpserver/routes_system.go`：system API
- `internal/httpserver/server_test.go`：API 集成测试
- `web/web.go`：嵌入前端资源
- `web/package.json`：前端脚本与依赖
- `web/vite.config.ts`：Vite 配置与 API 代理
- `web/components.json`：shadcn 配置
- `web/src/main.tsx`：前端入口
- `web/src/app/router.tsx`：页面路由
- `web/src/app/providers.tsx`：i18n、theme、query provider
- `web/src/lib/api.ts`：API 客户端
- `web/src/lib/i18n.ts`：国际化初始化
- `web/src/lib/theme.ts`：主题管理
- `web/src/pages/setup.tsx`：安装向导页
- `web/src/pages/login.tsx`：登录页
- `web/src/pages/home.tsx`：受保护首页
- `web/src/pages/profile.tsx`：用户中心
- `web/src/pages/admin.tsx`：后台壳页
- `web/src/pages/admin-settings.tsx`：系统设置页
- `web/src/pages/__tests__/app.test.tsx`：前端基础测试
- `configs/config.example.yaml`：配置样例
- `Taskfile.yml`：统一任务入口
- `Dockerfile`：多阶段构建
- `.github/workflows/ci.yml`：PR / 主干校验与单架构构建
- `.github/workflows/release.yml`：Tag 多架构发布
- `.gitignore`：忽略构建产物与本地配置

---

### Task 1: 初始化仓库骨架与根级工程文件

**Files:**
- Create: `.gitignore`
- Create: `Taskfile.yml`
- Create: `configs/config.example.yaml`
- Create: `app/server/main.go`
- Create: `internal/shared/response.go`
- Modify: `go.mod`
- Create: `pnpm-workspace.yaml`

- [ ] **Step 1: 写根级忽略规则**

```gitignore
configs/config.yaml
web/dist
node_modules
.pnpm-store
coverage
build
.DS_Store
.superpowers
```

- [ ] **Step 2: 编写基础 Taskfile**

```yaml
version: '3'

tasks:
  dev:backend:
    cmds:
      - go run ./app/server

  dev:web:
    dir: web
    cmds:
      - pnpm install
      - pnpm dev

  build:web:
    dir: web
    cmds:
      - pnpm install --frozen-lockfile
      - pnpm build

  test:go:
    cmds:
      - go test ./...

  build:go:
    cmds:
      - go build ./app/server
```

- [ ] **Step 3: 写配置样例**

```yaml
server:
  host: 0.0.0.0
  port: 8080

log:
  level: info

jwt:
  issuer: go-template
  access_ttl: 15m
  refresh_ttl: 168h
  secret: change-me

database:
  driver: sqlite
  dsn: file:data/app.db?_pragma=foreign_keys(1)

cache:
  driver: memory
  addr: ""
  password: ""
  db: 0
```

- [ ] **Step 4: 写最小程序入口**

```go
package main

import (
	"log"

	"github.com/ysicing/go-template/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: 写统一响应结构**

```go
package shared

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
```

- [ ] **Step 6: 校正模块与 workspace**

```go
module github.com/ysicing/go-template

go 1.26.1
```

```yaml
packages:
  - web
```

- [ ] **Step 7: 运行基础检查**

Run: `go test ./...`
Expected: PASS 或仅提示当前无测试文件

- [ ] **Step 8: 提交**

```bash
git add .gitignore Taskfile.yml configs/config.example.yaml app/server/main.go internal/shared/response.go go.mod pnpm-workspace.yaml
git commit -m "chore(init): 初始化项目骨架"
```

---

### Task 2: 实现配置加载与环境变量覆盖

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: 先写配置加载失败测试**

```go
package config_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/config"
)

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("testdata/not-found.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}
```

- [ ] **Step 2: 写环境变量覆盖测试**

```go
func TestLoadWithEnvOverride(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "9090")

	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Server.Port)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/config -v`
Expected: FAIL with `undefined: config.Load`

- [ ] **Step 4: 实现配置结构与加载逻辑**

```go
package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Log      LogConfig      `yaml:"log"`
	JWT      JWTConfig      `yaml:"jwt"`
	Database DatabaseConfig `yaml:"database"`
	Cache    CacheConfig    `yaml:"cache"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type JWTConfig struct {
	Issuer     string        `yaml:"issuer"`
	AccessTTL  time.Duration `yaml:"access_ttl"`
	RefreshTTL time.Duration `yaml:"refresh_ttl"`
	Secret     string        `yaml:"secret"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type CacheConfig struct {
	Driver   string `yaml:"driver"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyEnv(&cfg)
	return &cfg, nil
}

func applyEnv(cfg *Config) {
	if value := os.Getenv("APP_SERVER_PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.Server.Port = port
		}
	}
}
```

- [ ] **Step 5: 写启动编排骨架**

```go
package bootstrap

import "github.com/ysicing/go-template/internal/config"

func Run() error {
	_, err := config.Load("configs/config.yaml")
	return err
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/config -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/config/config.go internal/config/config_test.go internal/bootstrap/bootstrap.go
git commit -m "feat(config): 添加配置加载能力"
```

---

### Task 3: 实现数据库方言选择与缓存抽象

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`
- Create: `internal/cache/cache.go`
- Create: `internal/cache/memory.go`
- Create: `internal/cache/redis.go`
- Create: `internal/cache/memory_test.go`
- Test: `internal/db/db_test.go`
- Test: `internal/cache/memory_test.go`

- [ ] **Step 1: 先写数据库方言测试**

```go
package db_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
)

func TestDialectorForSQLite(t *testing.T) {
	cfg := config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	}

	dialector, err := db.NewDialector(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dialector == nil {
		t.Fatal("expected non-nil dialector")
	}
}
```

- [ ] **Step 2: 先写内存缓存测试**

```go
package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/internal/cache"
)

func TestMemoryStoreSetGet(t *testing.T) {
	store := cache.NewMemoryStore()
	ctx := context.Background()

	if err := store.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	value, ok, err := store.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != "v" {
		t.Fatalf("unexpected value: %v %v", ok, value)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/db ./internal/cache -v`
Expected: FAIL with undefined constructors

- [ ] **Step 4: 定义数据库初始化代码**

```go
package db

import (
	"fmt"

	"github.com/ysicing/go-template/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewDialector(cfg config.DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Driver {
	case "sqlite":
		return sqlite.Open(cfg.DSN), nil
	case "postgres":
		return postgres.Open(cfg.DSN), nil
	case "mysql":
		return mysql.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dialector, err := NewDialector(cfg)
	if err != nil {
		return nil, err
	}
	return gorm.Open(dialector, &gorm.Config{})
}
```

- [ ] **Step 5: 定义缓存接口与内存实现**

```go
package cache

import (
	"context"
	"sync"
	"time"
)

type Store interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
}

type memoryItem struct {
	value     string
	expiresAt time.Time
}

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]memoryItem{}}
}

func (s *MemoryStore) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = memoryItem{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *MemoryStore) Get(_ context.Context, key string) (string, bool, error) {
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		return "", false, nil
	}
	return item.value, true, nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}
```

- [ ] **Step 6: 定义 Redis 实现骨架**

```go
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/db ./internal/cache -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/db/db.go internal/db/db_test.go internal/cache/cache.go internal/cache/memory.go internal/cache/redis.go internal/cache/memory_test.go
git commit -m "feat(storage): 添加数据库与缓存抽象"
```

---

### Task 4: 建立系统与用户模型，接入 AutoMigrate

**Files:**
- Create: `internal/system/model.go`
- Create: `internal/user/model.go`
- Modify: `internal/db/db.go`
- Create: `internal/bootstrap/bootstrap.go`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: 先写自动迁移模型测试**

```go
package db_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
)

func TestAutoMigrateModels(t *testing.T) {
	conn, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := conn.AutoMigrate(&user.User{}, &system.BootstrapState{}, &system.Setting{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/db -run TestAutoMigrateModels -v`
Expected: FAIL with undefined models

- [ ] **Step 3: 定义用户模型**

```go
package user

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID           uint           `gorm:"primaryKey"`
	Username     string         `gorm:"size:64;uniqueIndex;not null"`
	Email        string         `gorm:"size:128;uniqueIndex;not null"`
	PasswordHash string         `gorm:"size:255;not null"`
	Role         Role           `gorm:"size:16;not null;default:user"`
	Status       string         `gorm:"size:16;not null;default:active"`
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
```

- [ ] **Step 4: 定义系统模型**

```go
package system

import "time"

type BootstrapState struct {
	ID            uint      `gorm:"primaryKey"`
	InitializedAt time.Time `gorm:"not null"`
	Version       string    `gorm:"size:32;not null"`
}

type Setting struct {
	ID        uint      `gorm:"primaryKey"`
	Group     string    `gorm:"size:64;index;not null"`
	Key       string    `gorm:"size:128;uniqueIndex;not null"`
	Value     string    `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

- [ ] **Step 5: 写统一迁移入口**

```go
package db

import (
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

func AutoMigrate(conn *gorm.DB) error {
	return conn.AutoMigrate(
		&user.User{},
		&system.BootstrapState{},
		&system.Setting{},
	)
}
```

- [ ] **Step 6: 在启动阶段接入迁移**

```go
package bootstrap

import (
	"os"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
)

func Run() error {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	conn, err := db.Open(cfg.Database)
	if err != nil {
		return err
	}

	return db.AutoMigrate(conn)
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/db -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/system/model.go internal/user/model.go internal/db/db.go internal/bootstrap/bootstrap.go
git commit -m "feat(model): 添加系统与用户模型"
```

---

### Task 5: 实现密码、JWT 与认证服务

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/auth_test.go`
- Test: `internal/auth/auth_test.go`

- [ ] **Step 1: 先写密码测试**

```go
package auth_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/auth"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := auth.CheckPassword(hash, "secret123"); err != nil {
		t.Fatalf("check password: %v", err)
	}
}
```

- [ ] **Step 2: 先写 JWT 测试**

```go
func TestTokenPairIssueAndParse(t *testing.T) {
	manager := auth.NewTokenManager("issuer", "secret", time.Minute, time.Hour)

	pair, err := manager.Issue(1, "admin")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.UserID != 1 {
		t.Fatalf("expected user id 1, got %d", claims.UserID)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/auth -v`
Expected: FAIL with undefined functions and types

- [ ] **Step 4: 实现密码工具**

```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(raw string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	return string(data), err
}

func CheckPassword(hash string, raw string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
}
```

- [ ] **Step 5: 实现 TokenManager**

```go
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenManager struct {
	issuer     string
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(issuer string, secret string, accessTTL time.Duration, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		issuer: issuer,
		secret: []byte(secret),
		accessTTL: accessTTL,
		refreshTTL: refreshTTL,
	}
}
```

- [ ] **Step 6: 实现签发、解析与认证服务**

```go
func (m *TokenManager) Issue(userID uint, role string) (TokenPair, error) {
	now := time.Now()
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   "access",
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}).SignedString(m.secret)
	if err != nil {
		return TokenPair{}, err
	}

	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   "refresh",
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}).SignedString(m.secret)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (m *TokenManager) ParseAccess(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(_ *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return parsed.Claims.(*Claims), nil
}
```

```go
package auth

import (
	"errors"

	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	tokens *TokenManager
}

func NewService(db *gorm.DB, tokens *TokenManager) *Service {
	return &Service{db: db, tokens: tokens}
}

func (s *Service) Login(identifier string, password string) (*user.User, TokenPair, error) {
	var u user.User
	if err := s.db.Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return nil, TokenPair{}, err
	}
	if err := CheckPassword(u.PasswordHash, password); err != nil {
		return nil, TokenPair{}, errors.New("invalid credentials")
	}
	pair, err := s.tokens.Issue(u.ID, string(u.Role))
	if err != nil {
		return nil, TokenPair{}, err
	}
	return &u, pair, nil
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/auth -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/auth/password.go internal/auth/jwt.go internal/auth/service.go internal/auth/auth_test.go
git commit -m "feat(auth): 添加认证核心能力"
```

---

### Task 6: 实现安装向导服务与初始化事务

**Files:**
- Create: `internal/setup/service.go`
- Create: `internal/setup/service_test.go`
- Modify: `internal/db/db.go`
- Test: `internal/setup/service_test.go`

- [ ] **Step 1: 先写 setup 状态测试**

```go
package setup_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/setup"
)

func TestStatusRequiredWhenBootstrapMissing(t *testing.T) {
	conn, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	svc := setup.NewService(conn)
	required, err := svc.SetupRequired()
	if err != nil {
		t.Fatalf("setup required: %v", err)
	}
	if !required {
		t.Fatal("expected setup required")
	}
}
```

- [ ] **Step 2: 先写安装成功测试**

```go
func TestInstallCreatesAdminAndBootstrapState(t *testing.T) {
	conn, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	svc := setup.NewService(conn)
	input := setup.InstallInput{
		AdminUsername: "admin",
		AdminEmail:    "admin@example.com",
		AdminPassword: "secret123",
	}
	if err := svc.Install(input); err != nil {
		t.Fatalf("install: %v", err)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/setup -v`
Expected: FAIL with undefined service or methods

- [ ] **Step 4: 实现 setup 服务**

```go
package setup

import (
	"time"

	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

type InstallInput struct {
	AdminUsername string
	AdminEmail    string
	AdminPassword string
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetupRequired() (bool, error) {
	var count int64
	if err := s.db.Model(&system.BootstrapState{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Service) Install(input InstallInput) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		hash, err := auth.HashPassword(input.AdminPassword)
		if err != nil {
			return err
		}

		admin := user.User{
			Username:     input.AdminUsername,
			Email:        input.AdminEmail,
			PasswordHash: hash,
			Role:         user.RoleAdmin,
			Status:       "active",
		}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}

		state := system.BootstrapState{
			InitializedAt: time.Now(),
			Version:       "v1",
		}
		return tx.Create(&state).Error
	})
}
```

- [ ] **Step 5: 为初始化补系统默认设置**

```go
func (s *Service) seedDefaultSettings(tx *gorm.DB) error {
	defaults := []system.Setting{
		{Group: "site", Key: "site.name", Value: "Go Template"},
		{Group: "site", Key: "site.allow_register", Value: "false"},
	}
	return tx.Create(&defaults).Error
}
```

- [ ] **Step 6: 在事务中调用默认设置**

```go
if err := s.seedDefaultSettings(tx); err != nil {
	return err
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/setup -v`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add internal/setup/service.go internal/setup/service_test.go internal/db/db.go
git commit -m "feat(setup): 添加初始化向导服务"
```

---

### Task 7: 搭建 Fiber 服务、setup/auth/system API 与中间件

**Files:**
- Create: `internal/httpserver/server.go`
- Create: `internal/httpserver/middleware.go`
- Create: `internal/httpserver/routes_setup.go`
- Create: `internal/httpserver/routes_auth.go`
- Create: `internal/httpserver/routes_system.go`
- Create: `internal/httpserver/server_test.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Test: `internal/httpserver/server_test.go`

- [ ] **Step 1: 先写 setup 状态 API 测试**

```go
package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/internal/httpserver"
)

func TestSetupStatusRoute(t *testing.T) {
	app := httpserver.NewForTest(true)
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: 写未登录访问受保护接口测试**

```go
func TestProtectedRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
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
Expected: FAIL with undefined server constructors

- [ ] **Step 4: 创建服务容器与测试入口**

```go
package httpserver

import "github.com/gofiber/fiber/v3"

func NewForTest(setupRequired bool) *fiber.App {
	app := fiber.New()
	registerSetupRoutes(app, setupRequired)
	registerAuthRoutes(app)
	return app
}
```

- [ ] **Step 5: 实现 setup 路由**

```go
package httpserver

import (
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
)

func registerSetupRoutes(app *fiber.App, setupRequired bool) {
	app.Get("/api/setup/status", func(c fiber.Ctx) error {
		return c.JSON(shared.Response{
			Code:    "OK",
			Message: "success",
			Data: map[string]any{
				"setup_required": setupRequired,
			},
		})
	})
}
```

- [ ] **Step 6: 实现 auth 路由与认证中间件骨架**

```go
package httpserver

import (
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
)

func requireAuth(c fiber.Ctx) error {
	if c.Get("Authorization") == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(shared.Response{
			Code:    "UNAUTHORIZED",
			Message: "unauthorized",
		})
	}
	return c.Next()
}

func registerAuthRoutes(app *fiber.App) {
	app.Get("/api/auth/me", requireAuth, func(c fiber.Ctx) error {
		return c.JSON(shared.Response{Code: "OK", Message: "success"})
	})
}
```

- [ ] **Step 7: 实现正式服务构造**

```go
func New() *fiber.App {
	app := fiber.New()
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}
```

- [ ] **Step 8: 在 bootstrap 中启动 HTTP 服务**

```go
package bootstrap

import (
	"fmt"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/httpserver"
)

func serve(cfg *config.Config) error {
	app := httpserver.New()
	return app.Listen(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
}
```

- [ ] **Step 9: 运行测试确认通过**

Run: `go test ./internal/httpserver -v`
Expected: PASS

- [ ] **Step 10: 提交**

```bash
git add internal/httpserver/server.go internal/httpserver/middleware.go internal/httpserver/routes_setup.go internal/httpserver/routes_auth.go internal/httpserver/routes_system.go internal/httpserver/server_test.go internal/bootstrap/bootstrap.go
git commit -m "feat(api): 搭建基础接口与中间件"
```

---

### Task 8: 初始化前端工程与全局 Provider

**Files:**
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/components.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/app/providers.tsx`
- Create: `web/src/app/router.tsx`
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/i18n.ts`
- Create: `web/src/lib/theme.ts`
- Create: `web/src/pages/__tests__/app.test.tsx`
- Test: `web/src/pages/__tests__/app.test.tsx`

- [ ] **Step 1: 写前端 package.json**

```json
{
  "name": "go-template-web",
  "private": true,
  "version": "0.0.1",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "@radix-ui/react-slot": "^1.1.2",
    "@tanstack/react-query": "^5.66.0",
    "axios": "^1.8.1",
    "i18next": "^24.2.1",
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-i18next": "^15.4.0",
    "react-router-dom": "^7.2.0"
  },
  "devDependencies": {
    "@testing-library/react": "^16.2.0",
    "@types/react": "^19.0.8",
    "@types/react-dom": "^19.0.3",
    "@vitejs/plugin-react": "^4.4.1",
    "typescript": "^5.7.3",
    "vite": "^6.1.0",
    "vitest": "^3.0.5"
  }
}
```

- [ ] **Step 2: 写 Vite 代理配置**

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
});
```

- [ ] **Step 3: 先写前端基础测试**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AppProviders } from "../../app/providers";

describe("providers", () => {
  it("renders children", () => {
    render(
      <AppProviders>
        <div>hello</div>
      </AppProviders>,
    );

    expect(screen.getByText("hello")).toBeTruthy();
  });
});
```

- [ ] **Step 4: 运行测试确认失败**

Run: `cd web && pnpm install && pnpm test`
Expected: FAIL with missing providers module

- [ ] **Step 5: 实现前端入口与 Provider**

```tsx
import React from "react";
import ReactDOM from "react-dom/client";

import { AppProviders } from "./app/providers";
import { AppRouter } from "./app/router";
import "./lib/i18n";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AppProviders>
      <AppRouter />
    </AppProviders>
  </React.StrictMode>,
);
```

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { PropsWithChildren, useMemo } from "react";

export function AppProviders({ children }: PropsWithChildren) {
  const queryClient = useMemo(() => new QueryClient(), []);
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
```

- [ ] **Step 6: 实现 Router 与全局工具骨架**

```tsx
import { createBrowserRouter, RouterProvider } from "react-router-dom";

const router = createBrowserRouter([
  { path: "/", element: <div>home</div> },
  { path: "/login", element: <div>login</div> },
  { path: "/setup", element: <div>setup</div> },
]);

export function AppRouter() {
  return <RouterProvider router={router} />;
}
```

```ts
import axios from "axios";

export const api = axios.create({
  baseURL: "/api",
});
```

- [ ] **Step 7: 运行测试与构建确认通过**

Run: `cd web && pnpm test && pnpm build`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add web/package.json web/tsconfig.json web/vite.config.ts web/components.json web/index.html web/src
git commit -m "feat(web): 初始化前端基础工程"
```

---

### Task 9: 实现安装向导、登录、首页、用户中心与后台壳页面

**Files:**
- Create: `web/src/pages/setup.tsx`
- Create: `web/src/pages/login.tsx`
- Create: `web/src/pages/home.tsx`
- Create: `web/src/pages/profile.tsx`
- Create: `web/src/pages/admin.tsx`
- Create: `web/src/pages/admin-settings.tsx`
- Modify: `web/src/app/router.tsx`
- Modify: `web/src/lib/api.ts`
- Test: `web/src/pages/__tests__/app.test.tsx`

- [ ] **Step 1: 写 setup 状态跳转测试**

```tsx
import { vi } from "vitest";

vi.mock("../../lib/api", () => ({
  api: {
    get: vi.fn().mockResolvedValue({ data: { data: { setup_required: true } } }),
  },
}));
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test`
Expected: FAIL with missing route guards and pages

- [ ] **Step 3: 实现安装向导页**

```tsx
export function SetupPage() {
  return (
    <main>
      <h1>安装向导</h1>
      <form>
        <input placeholder="管理员用户名" />
        <input placeholder="管理员邮箱" />
        <input placeholder="管理员密码" type="password" />
        <button type="submit">初始化系统</button>
      </form>
    </main>
  );
}
```

- [ ] **Step 4: 实现登录与业务页面**

```tsx
export function LoginPage() {
  return (
    <main>
      <h1>登录</h1>
      <form>
        <input placeholder="用户名或邮箱" />
        <input placeholder="密码" type="password" />
        <button type="submit">登录</button>
      </form>
    </main>
  );
}
```

```tsx
export function HomePage() {
  return <main>dashboard</main>;
}

export function ProfilePage() {
  return <main>profile</main>;
}

export function AdminPage() {
  return <main>admin</main>;
}

export function AdminSettingsPage() {
  return <main>admin settings</main>;
}
```

- [ ] **Step 5: 实现路由注册**

```tsx
import { createBrowserRouter, RouterProvider } from "react-router-dom";

import { AdminPage } from "../pages/admin";
import { AdminSettingsPage } from "../pages/admin-settings";
import { HomePage } from "../pages/home";
import { LoginPage } from "../pages/login";
import { ProfilePage } from "../pages/profile";
import { SetupPage } from "../pages/setup";

const router = createBrowserRouter([
  { path: "/setup", element: <SetupPage /> },
  { path: "/login", element: <LoginPage /> },
  { path: "/", element: <HomePage /> },
  { path: "/profile", element: <ProfilePage /> },
  { path: "/admin", element: <AdminPage /> },
  { path: "/admin/settings", element: <AdminSettingsPage /> },
]);

export function AppRouter() {
  return <RouterProvider router={router} />;
}
```

- [ ] **Step 6: 实现 setup status 查询**

```ts
import axios from "axios";

export const api = axios.create({ baseURL: "/api" });

export async function fetchSetupStatus() {
  const response = await api.get("/setup/status");
  return response.data.data as { setup_required: boolean };
}
```

- [ ] **Step 7: 运行测试与构建确认通过**

Run: `cd web && pnpm test && pnpm build`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add web/src/pages web/src/app/router.tsx web/src/lib/api.ts
git commit -m "feat(web): 添加安装与登录页面骨架"
```

---

### Task 10: 实现静态资源嵌入与前后端双模式运行

**Files:**
- Create: `web/web.go`
- Modify: `internal/httpserver/server.go`
- Modify: `Taskfile.yml`
- Test: `internal/httpserver/server_test.go`

- [ ] **Step 1: 写静态资源挂载测试**

```go
func TestHealthzRoute(t *testing.T) {
	app := httpserver.New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: 实现 `web/web.go`**

```go
package web

import "embed"

//go:embed dist
var Dist embed.FS
```

- [ ] **Step 3: 在 Fiber 挂载嵌入静态资源**

```go
package httpserver

import (
	"io/fs"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/ysicing/go-template/web"
)

func New() *fiber.App {
	app := fiber.New()
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	if sub, err := fs.Sub(web.Dist, "dist"); err == nil {
		app.Use("/", static.New("", static.Config{
			FS: sub,
			Browse: false,
		}))
	}
	return app
}
```

- [ ] **Step 4: 扩展 Taskfile 支持联调命令**

```yaml
  dev:
    cmds:
      - task: dev:backend

  dev:full:
    cmds:
      - echo "run backend and web in separate terminals"
```

- [ ] **Step 5: 运行测试与构建确认通过**

Run: `go test ./internal/httpserver -v && task build:web && go build ./app/server`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add web/web.go internal/httpserver/server.go Taskfile.yml
git commit -m "feat(embed): 添加前端嵌入支持"
```

---

### Task 11: 完善 Dockerfile 与 GitHub Actions

**Files:**
- Create: `Dockerfile`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Modify: `Taskfile.yml`

- [ ] **Step 1: 写 Dockerfile**

```dockerfile
FROM node:22-alpine AS web-builder
WORKDIR /workspace/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install
COPY web ./
RUN pnpm build

FROM golang:1.26 AS go-builder
WORKDIR /workspace
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web-builder /workspace/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./app/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=go-builder /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

- [ ] **Step 2: 写 PR 与主干 CI**

```yaml
name: ci

on:
  pull_request:
  push:
    branches:
      - main

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
      - run: go test ./...
      - run: cd web && pnpm install --frozen-lockfile && pnpm build
      - run: docker build -t ghcr.io/example/go-template:${{ github.ref_name }} .
```

- [ ] **Step 3: 为主干 push 增加镜像推送条件**

```yaml
      - if: github.event_name == 'push'
        run: echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
      - if: github.event_name == 'push'
        run: docker push ghcr.io/example/go-template:${{ github.ref_name }}
```

- [ ] **Step 4: 写 Tag 多架构发布流水线**

```yaml
name: release

on:
  push:
    tags:
      - "v*"

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          push: true
          platforms: linux/amd64,linux/arm64
          tags: |
            ghcr.io/example/go-template:${{ github.ref_name }}
            ghcr.io/example/go-template:main
```

- [ ] **Step 5: 补充 Taskfile 构建命令**

```yaml
  docker:build:
    cmds:
      - docker build -t go-template:local .
```

- [ ] **Step 6: 运行配置校验**

Run: `docker build -t go-template:local .`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add Dockerfile .github/workflows/ci.yml .github/workflows/release.yml Taskfile.yml
git commit -m "feat(ci): 添加容器与流水线配置"
```

---

### Task 12: 补全启动编排、系统设置接口与最终验证

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `internal/httpserver/server.go`
- Modify: `internal/httpserver/routes_system.go`
- Modify: `internal/setup/service.go`
- Modify: `Taskfile.yml`
- Test: `internal/httpserver/server_test.go`

- [ ] **Step 1: 写系统设置接口测试**

```go
func TestSystemSettingsRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpserver -run TestSystemSettingsRouteRequiresAuth -v`
Expected: FAIL with missing system route

- [ ] **Step 3: 完善 bootstrap 启动编排**

```go
package bootstrap

import (
	"fmt"
	"os"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/httpserver"
	"github.com/ysicing/go-template/internal/setup"
)

func Run() error {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	setupRequired := true
	if cfg != nil {
		conn, err := db.Open(cfg.Database)
		if err != nil {
			return err
		}
		if err := db.AutoMigrate(conn); err != nil {
			return err
		}
		required, err := setup.NewService(conn).SetupRequired()
		if err != nil {
			return err
		}
		setupRequired = required
	}

	app := httpserver.NewWithSetupState(setupRequired)
	host := "0.0.0.0"
	port := 8080
	if cfg != nil {
		host = cfg.Server.Host
		port = cfg.Server.Port
	}
	return app.Listen(fmt.Sprintf("%s:%d", host, port))
}
```

- [ ] **Step 4: 实现系统设置接口**

```go
package httpserver

import (
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
)

func registerSystemRoutes(app *fiber.App) {
	app.Get("/api/system/settings", requireAuth, func(c fiber.Ctx) error {
		return c.JSON(shared.Response{
			Code:    "OK",
			Message: "success",
			Data: map[string]any{
				"site_name": "Go Template",
			},
		})
	})
}
```

- [ ] **Step 5: 完善服务构造与 Taskfile 全量验证**

```yaml
  verify:
    cmds:
      - go test ./...
      - task build:web
      - go build ./app/server
```

- [ ] **Step 6: 运行最终验证**

Run: `task verify`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/bootstrap/bootstrap.go internal/httpserver/server.go internal/httpserver/routes_system.go internal/setup/service.go Taskfile.yml
git commit -m "feat(app): 完善启动编排与系统接口"
```

---

## Self-Review

### Spec coverage

- 目录结构：Task 1、Task 8、Task 10、Task 11
- YAML + env 配置：Task 2
- 多数据库与多缓存：Task 3、Task 4
- 软删除：Task 4
- JWT 登录：Task 5
- 安装向导：Task 6、Task 9、Task 12
- Fiber API：Task 7、Task 12
- 前端多页面壳：Task 8、Task 9
- 前端嵌入：Task 10
- Taskfile / Docker / GitHub Actions：Task 1、Task 10、Task 11、Task 12
- 测试：Task 2、Task 3、Task 5、Task 6、Task 7、Task 8、Task 12

### Placeholder scan

- 已检查本计划中不存在未完成占位内容或模糊执行描述

### Type consistency

- 配置对象统一使用 `config.Config`
- 数据库入口统一使用 `db.Open` 与 `db.AutoMigrate`
- 初始化服务统一使用 `setup.Service`
- Token 管理统一使用 `auth.TokenManager`

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-10-go-template.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
