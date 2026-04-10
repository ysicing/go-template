#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
eval "$(./hack/build-metadata.sh)"
docker build \
  --build-arg VERSION="${VERSION}" \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  -t go-template:local .
