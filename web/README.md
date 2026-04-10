# Frontend workspace

This directory contains the embedded React control center for `go-template`.

## Commands

```bash
pnpm install
pnpm dev
pnpm test
pnpm lint
pnpm build
```

## Local development

- `pnpm dev --host 0.0.0.0` exposes the frontend on the current LAN IP at port `3000`
- `/api` is proxied to the Go backend on port `3206`
- override the backend target with `VITE_API_PROXY_TARGET=http://<内网IP>:3206`
- production assets build into `web/dist` and are embedded by `task build:go`

## Testing

- `Vitest` runs in `jsdom`
- shared test setup lives in `src/test/setup.ts`
- core console and setup coverage live in `src/pages/__tests__/app.test.tsx` and `src/pages/__tests__/setup-page.test.tsx`

## Conventions

- Keep pages split by subsystem and route ownership
- Prefer shadcn/ui components under `src/components/ui`
- Use `@/` imports only; avoid relative `../` chains
