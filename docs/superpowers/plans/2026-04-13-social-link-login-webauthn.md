# Social Link Login WebAuthn Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为登录页补齐社交账号二次确认绑定流程，并支持密码、TOTP、Passkey 三种验证方式。

**Architecture:** 在 `web/src/pages/login/index.tsx` 内识别 `link_required` 参数，切换到同一卡片内的“确认绑定”第二步；新增 `authApi` 的 social-link 请求封装，并复用现有登录成功跳转逻辑。若登录页逻辑开始臃肿，则抽离一个专用的确认绑定子组件。

**Tech Stack:** React、React Router、shadcn/ui、Axios API 封装、Vitest（若项目已配置）

---

### Task 1: 盘点前端测试与登录页边界

**Files:**
- Modify: `web/src/pages/login/index.tsx`
- Modify: `web/src/api/services.ts`
- Test: `web/package.json`

- [ ] **Step 1: 找出前端测试基建与现有登录页依赖**

Run: `cat web/package.json && rg -n "vitest|jest|testing-library|playwright" web -g'package.json' -g'*.test.*' -g'*.spec.*'`
Expected: 能确认是否已有可复用测试框架与样例

- [ ] **Step 2: 标记登录页需要承接的新状态与 API**

需要确认最终最小状态：

```ts
const [linkRequired, setLinkRequired] = useState(false)
const [linkToken, setLinkToken] = useState("")
const [linkProvider, setLinkProvider] = useState("")
const [linkMethod, setLinkMethod] = useState<"password" | "totp" | "webauthn">("password")
const [linkLoading, setLinkLoading] = useState(false)
const [linkPassword, setLinkPassword] = useState("")
const [linkTotpCode, setLinkTotpCode] = useState("")
```

- [ ] **Step 3: 提交当前认知（可选，不单独 commit）**

本任务仅确认基建，不单独提交。

### Task 2: 为 social-link API 先写失败测试或最小验证点

**Files:**
- Modify: `web/src/api/services.ts`
- Test: `web/src/pages/login/*.test.tsx` 或现有前端测试目录

- [ ] **Step 1: 先写一个失败测试，验证 `link_required` 时会进入确认绑定模式**

示例断言：

```tsx
it('shows social link confirm mode when link_required params exist', async () => {
  renderWithRouter('/login?link_required=true&link_token=abc&provider=github')
  expect(await screen.findByText(/确认绑定/i)).toBeInTheDocument()
  expect(screen.getByText(/github/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: 运行该测试确认先失败**

Run: `cd web && pnpm test -- --runInBand login`
Expected: FAIL，提示页面尚未渲染确认绑定 UI

- [ ] **Step 3: 为 API 封装补齐最小接口**

```ts
confirmSocialLink: (link_token: string, payload: { password?: string; totp_code?: string }) =>
  api.post('/auth/social/confirm-link', { link_token, ...payload }),
socialLinkWebAuthnBegin: (link_token: string) =>
  api.post('/auth/social/confirm-link/webauthn/begin', { link_token }),
socialLinkWebAuthnFinish: (link_token: string, body: unknown) =>
  api.post(`/auth/social/confirm-link/webauthn/finish?link_token=${encodeURIComponent(link_token)}`, body),
```

- [ ] **Step 4: 若无前端测试基建，则记录并转为手工验证 + 类型构建验证**

Run: `cd web && pnpm build`
Expected: 至少能用构建校验类型与页面代码

### Task 3: 实现登录页确认绑定模式

**Files:**
- Modify: `web/src/pages/login/index.tsx`
- Create: `web/src/components/auth/login-link-confirm-card.tsx`（仅当需要拆分）

- [ ] **Step 1: 解析 URL 参数并建立 link mode 状态**

```ts
useEffect(() => {
  const required = searchParams.get('link_required') === 'true'
  const token = searchParams.get('link_token') || ''
  const provider = searchParams.get('provider') || ''

  if (required && token) {
    setLinkRequired(true)
    setLinkToken(token)
    setLinkProvider(provider)
    setLinkMethod('password')
    return
  }

  setLinkRequired(false)
  setLinkToken('')
  setLinkProvider('')
}, [searchParams])
```

- [ ] **Step 2: 在登录卡片中切换普通登录 / 确认绑定 UI**

```tsx
{linkRequired ? (
  <ConfirmLinkPanel />
) : (
  <LoginForm />
)}
```

- [ ] **Step 3: 使用 shadcn 风格组件实现确认绑定面板**

```tsx
<Tabs value={linkMethod} onValueChange={(value) => setLinkMethod(value as LinkMethod)}>
  <TabsList className="grid w-full grid-cols-3">
    <TabsTrigger value="password">密码</TabsTrigger>
    <TabsTrigger value="totp">TOTP</TabsTrigger>
    <TabsTrigger value="webauthn">Passkey</TabsTrigger>
  </TabsList>
</Tabs>
```

- [ ] **Step 4: 添加返回普通登录动作并清理 URL**

```ts
const exitLinkMode = () => {
  setLinkRequired(false)
  setLinkToken('')
  setLinkPassword('')
  setLinkTotpCode('')
  navigate('/login', { replace: true })
}
```

### Task 4: 接通密码、TOTP、Passkey 三种提交

**Files:**
- Modify: `web/src/pages/login/index.tsx`
- Modify: `web/src/api/services.ts`

- [ ] **Step 1: 先写密码提交失败测试或最小行为断言**

```tsx
it('submits password confirmation for social link', async () => {
  // mock authApi.confirmSocialLink
  // fill password
  // click confirm
  // expect called with link_token + password
})
```

- [ ] **Step 2: 实现密码 / TOTP 提交逻辑**

```ts
const submitLinkConfirm = async () => {
  if (linkMethod === 'password') {
    const res = await authApi.confirmSocialLink(linkToken, { password: linkPassword })
    return handleLoginResult(res.data)
  }
  if (linkMethod === 'totp') {
    const res = await authApi.confirmSocialLink(linkToken, { totp_code: linkTotpCode })
    return handleLoginResult(res.data)
  }
}
```

- [ ] **Step 3: 实现 Passkey begin/finish 流程**

```ts
const beginRes = await authApi.socialLinkWebAuthnBegin(linkToken)
const { publicKey } = beginRes.data
const credential = await navigator.credentials.get({ publicKey: normalizePublicKey(publicKey) })
const body = toAssertionBody(credential)
const finishRes = await authApi.socialLinkWebAuthnFinish(linkToken, body)
handleLoginResult(finishRes.data)
```

- [ ] **Step 4: 复用统一登录后处理**

```ts
const handleLoginResult = (data: { user?: User; mfa_required?: boolean; mfa_token?: string }) => {
  if (data.mfa_required && data.mfa_token) {
    navigate(`/mfa-verify?mfa_token=${encodeURIComponent(data.mfa_token)}`)
    return
  }
  handleLogin(data as { user: User })
}
```

### Task 5: 补验证并收尾

**Files:**
- Modify: `web/src/pages/login/index.tsx`
- Modify: `web/src/api/services.ts`
- Test: 前端测试文件（若存在）

- [ ] **Step 1: 运行新增/相关测试**

Run: `cd web && pnpm test`
Expected: PASS；若项目无测试基建则跳过并记录原因

- [ ] **Step 2: 运行前端构建验证**

Run: `cd web && pnpm build`
Expected: PASS

- [ ] **Step 3: 运行仓库格式化**

Run: `task fmt`
Expected: PASS

- [ ] **Step 4: 运行回归验证**

Run: `go test ./handler ./internal/app ./store`
Expected: PASS，确保本轮前端改动未影响现有后端联调接口编译/生成流程

- [ ] **Step 5: 提交**

```bash
git add web/src/api/services.ts web/src/pages/login/index.tsx docs/superpowers/specs/2026-04-13-social-link-login-webauthn-design.md docs/superpowers/plans/2026-04-13-social-link-login-webauthn.md
git commit -m "feat(web): 增加社交绑定确认登录流程"
```
