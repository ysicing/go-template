# 社交账号绑定登录页交互设计

## 背景

当前后端在社交登录命中已存在邮箱账号时，会回跳到登录页并附带 `link_required=true`、`link_token`、`provider` 参数。后端现已支持三类确认绑定方式：

- 密码：`POST /api/auth/social/confirm-link`
- TOTP：`POST /api/auth/social/confirm-link`
- WebAuthn：
  - `POST /api/auth/social/confirm-link/webauthn/begin`
  - `POST /api/auth/social/confirm-link/webauthn/finish`

当前前端 `web/src/pages/login/index.tsx` 尚未承接这条二阶段流程。

## 目标

在不跳离现有登录页的前提下，将登录页扩展为“登录 / 确认绑定”双阶段卡片流程：

1. 默认展示现有登录表单。
2. 当 URL 中存在 `link_required=true` 且带有有效 `link_token` 与 `provider` 时，切换为“确认绑定”视图。
3. 在确认绑定视图中提供三种验证入口：密码、TOTP、Passkey（WebAuthn）。
4. 绑定成功后沿用现有登录成功逻辑：更新用户状态并跳转首页或 MFA 验证页。

## 设计结论

采用**同一卡片内切换第二步**的方案，而不是额外弹 `Dialog/Sheet`。

### 原因

- 当前登录页本身就是居中单卡片结构，直接切换内容最自然。
- 移动端体验更稳定，不会出现弹层高度、键盘顶起、焦点切换等额外复杂度。
- URL 已经明确表达这是登录流的一部分，继续留在登录卡片里语义更一致。
- 更适合复用现有 shadcn 组件：`Card`、`Button`、`Input`、`Tabs`/分段按钮、`Alert`。

## 页面结构

### 阶段 1：普通登录

维持现状：

- 用户名/邮箱输入框
- 密码输入框
- remember me
- 登录按钮
- Passkey 登录按钮
- 注册入口（若允许注册）

### 阶段 2：确认绑定

当检测到 `link_required` 后，登录卡片切换为确认绑定视图，包含：

- 标题：`确认绑定 {provider}`
- 副标题：提示检测到当前邮箱已有账号，需要先验证身份
- 验证方式切换器（推荐用 shadcn `Tabs` 或等价 segmented UI）
  - 密码验证
  - TOTP 验证
  - Passkey 验证
- 对应方式的操作区域
  - 密码：展示密码输入框 + 提交按钮
  - TOTP：展示 6 位验证码输入框 + 提交按钮
  - Passkey：展示说明文案 + “使用 Passkey 验证并绑定”按钮
- 次要操作：返回普通登录
- 提示文案：链接有效期较短，失败后可重新发起社交登录

## 交互细节

### URL 解析

登录页初始化时读取：

- `link_required`
- `link_token`
- `provider`
- `error`

规则：

- 若 `link_required=true` 且 `link_token` 非空，则进入确认绑定模式。
- 若参数不完整，则仍停留在普通登录模式，并给出错误提示。
- 用户点击“返回普通登录”时：
  - 清空本地 `linkToken`、`provider`、验证方式等状态
  - 使用 `navigate('/login', { replace: true })` 或移除相关查询参数，避免刷新后再次进入绑定模式

### 方式切换

推荐默认选中：

1. 密码
2. TOTP
3. Passkey

原因：

- 密码是最通用的兜底方式。
- TOTP 适合已有 MFA 用户。
- Passkey 虽然体验最好，但并非所有用户都已配置。

### Passkey 流程

前端流程：

1. 调用 `authApi.socialLinkWebAuthnBegin(linkToken)`。
2. 将返回的 `publicKey.challenge`、`allowCredentials[].id` 做 base64url -> ArrayBuffer 转换。
3. 调用 `navigator.credentials.get({ publicKey })`。
4. 将 assertion 响应转换为后端需要的 JSON。
5. 调用 `authApi.socialLinkWebAuthnFinish(linkToken, body)`。
6. 若返回 `mfa_required`，跳转到 MFA 验证页；否则按普通登录成功处理。

### 密码 / TOTP 流程

- 密码调用：`authApi.confirmSocialLink(linkToken, { password })`
- TOTP 调用：`authApi.confirmSocialLink(linkToken, { totp_code })`
- 成功后的跳转与普通登录成功一致。

### 错误处理

错误提示统一使用 toast，并结合当前上下文做更明确文案：

- `invalid or expired link token`：绑定请求已过期，请重新发起社交登录
- `password_not_set`：当前账号未设置密码，请使用 TOTP 或 Passkey
- `totp_not_enabled`：当前账号未启用 TOTP，请使用密码或 Passkey
- `webauthn_not_enabled`：当前账号未配置 Passkey，请改用密码或 TOTP
- `authentication failed` / `invalid password` / `invalid TOTP code`：保持原样或映射为更友好的提示

Passkey 分支需要单独 loading 态，避免重复触发系统认证弹窗。

## 组件建议

优先复用现有 shadcn 组件：

- `Card`：整体容器
- `Alert`：说明“为什么需要确认绑定”
- `Tabs`：验证方式切换
- `Input`：密码 / TOTP 输入
- `Button`：提交 / 返回
- `Separator`：与普通登录视觉保持一致

不新增自定义复杂组件；若逻辑开始变重，可将“确认绑定面板”抽为 `LoginLinkConfirmCard` 子组件。

## 状态建议

登录页新增最小状态集合：

- `linkRequired: boolean`
- `linkToken: string`
- `linkProvider: string`
- `linkMethod: 'password' | 'totp' | 'webauthn'`
- `linkLoading: boolean`
- `linkPassword: string`
- `linkTotpCode: string`

普通登录与确认绑定 loading 最好拆开，避免按钮状态互相污染。

## API 建议

在 `web/src/api/services.ts` 中为社交绑定补齐显式 API：

- `confirmSocialLink(link_token, password?, totp_code?)`
- `socialLinkWebAuthnBegin(link_token)`
- `socialLinkWebAuthnFinish(link_token, body)`

避免在页面中直接拼接 URL 或手写请求体结构。

## 测试建议

### 前端行为测试

至少覆盖：

1. 登录页在 `link_required=true` 时切换到确认绑定模式。
2. 点击“返回普通登录”后能退出绑定模式并清理 URL。
3. 密码提交调用正确 API。
4. TOTP 提交调用正确 API。
5. Passkey begin/finish 按顺序调用，并在成功后走统一登录处理。

### 手工验证

- 普通登录不受影响
- OIDC 登录页模式不受影响
- 社交登录回跳到绑定模式显示正确 provider
- 密码 / TOTP / Passkey 三种路径都可闭环
- MFA required 返回仍能跳去 `/mfa-verify`
- 移动端卡片高度与按钮布局正常

## 风险与约束

- 浏览器若不支持 WebAuthn，需要在 Passkey 按钮点击前做能力检测并给出提示。
- OIDC 登录模式（带 `id` 参数）与确认绑定模式不能互相打架；若两者同时出现，以确认绑定优先，并禁用普通 OIDC 表单语义。
- 若后端返回 token 过期，必须引导重新发起社交登录，而不是停留在死状态。

## 范围外

本次不做：

- 单独的确认绑定路由页
- 弹窗 / Sheet 方案
- 绑定方式记忆
- provider 图标视觉增强
- 社交登录入口本身的重构

## 实施文件

- `web/src/pages/login/index.tsx`
- `web/src/api/services.ts`
- `web/src/pages/login/__tests__/...`（若项目已有前端测试基建则补测试）
- 可能新增：`web/src/components/auth/login-link-confirm-card.tsx`

## 自检结论

- 方案边界清晰，仅影响登录页与 auth API 封装。
- 与已完成后端 WebAuthn social-link 两阶段接口保持一致。
- 优先复用现有 shadcn 风格和登录页结构，没有额外引入重交互容器。
