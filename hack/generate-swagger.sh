#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${ROOT_DIR}"

go run github.com/swaggo/swag/cmd/swag@v1.16.6 init \
  -g app/server/main.go \
  -o internal/apidocs \
  --parseDependency \
  --parseInternal
