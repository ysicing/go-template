# --- Frontend build ---
FROM node:lts-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable pnpm
RUN --mount=type=cache,id=pnpm,target=/pnpm-store \
    pnpm install --frozen-lockfile --store-dir /pnpm-store
COPY web/ .
RUN pnpm run build

# --- Backend build ---
FROM golang:1.26.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY *.go ./
COPY handler/ handler/
COPY model/ model/
COPY store/ store/
COPY pkg/ pkg/
COPY internal/ internal/
COPY --from=frontend /app/web/dist ./web/dist
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -o id .

# --- Runtime ---
FROM alpine
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app
WORKDIR /app
ENV TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12"
COPY --from=builder --chown=app:app /app/id .
EXPOSE 8080
USER app
CMD ["/app/id"]
