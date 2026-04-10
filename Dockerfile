FROM node:22-alpine AS web-builder
WORKDIR /workspace
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json ./web/package.json
RUN corepack enable && cd web && pnpm install --frozen-lockfile
COPY web ./web
RUN cd web && pnpm build

FROM golang:1.26.2 AS go-builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /workspace/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X github.com/ysicing/go-template/internal/buildinfo.Version=${VERSION} -X github.com/ysicing/go-template/internal/buildinfo.Commit=${COMMIT} -X github.com/ysicing/go-template/internal/buildinfo.BuildTime=${BUILD_TIME}" \
  -o /out/server ./app/server

FROM gcr.io/distroless/base-debian12
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
WORKDIR /app
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.created="${BUILD_TIME}"
COPY --from=go-builder /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
