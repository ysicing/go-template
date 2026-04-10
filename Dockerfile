FROM node:22-alpine AS web-builder
WORKDIR /workspace
COPY pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json ./web/package.json
RUN corepack enable && cd web && pnpm install --frozen-lockfile
COPY web ./web
RUN cd web && pnpm build

FROM golang:1.26 AS go-builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /workspace/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./app/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=go-builder /out/server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
