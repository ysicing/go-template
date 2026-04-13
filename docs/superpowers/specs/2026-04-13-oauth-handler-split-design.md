# OAuth Handler Split Design

**Goal**: 拆分 `handler/oauth.go`，按职责拆到多个文件，并抽出最小公共 helper，保持 HTTP 接口、行为和测试语义不变。

**Architecture**:
- 保留 `OAuthHandler` 作为统一入口，拆出 provider-specific、social-link、webauthn、shared helper 四类职责。
- GitHub/Google 回调通过共享 callback helper 收敛重复逻辑；provider 特有差异保留在各自文件。
- 不修改路由、不修改返回结构、不引入新的跨包依赖。

**Files**:
- Modify: `handler/oauth.go`
- Create: `handler/oauth_provider.go`
- Create: `handler/oauth_github.go`
- Create: `handler/oauth_google.go`
- Create: `handler/oauth_social_link.go`
- Create: `handler/oauth_webauthn.go`
- Test: `handler/oauth_test.go`

**Constraints**:
- 保持现有导出 API 与路由行为不变。
- 不改变测试断言语义，只允许因 helper 抽取产生的内部重排。
- 公共 helper 仅抽取已存在的重复逻辑，不额外扩展功能。
