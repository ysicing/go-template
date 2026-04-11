# Frontend Workspace

这里是嵌入到后端二进制中的 React 控制台。

## 命令

```bash
pnpm install
pnpm dev
pnpm test
pnpm lint
pnpm build
```

## 本地开发

- `pnpm dev` 在 `http://localhost:3000` 启动前端
- `/api` 代理到 `http://localhost:8080`
- `task build` 会将 `web/dist` 嵌入最终二进制

## 当前模板前端范围

- 登录、注册、MFA、Consent
- Dashboard
- Applications
- Users / Clients / Providers
- Settings / Audit Logs
- Profile / Security / Points

已移出模板的页面不再作为默认控制台能力维护。

## 约定

- 必需首屏数据页面必须实现 `loading / error / content`
- 启动失败使用 `PageErrorState`
- 操作错误统一通过 `getErrorMessage(...)` 处理

更多约束见 `../docs/FRONTEND_ERROR_HANDLING.md`
