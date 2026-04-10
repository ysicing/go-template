#!/usr/bin/env bash

set -euo pipefail

version="${VERSION:-}"
if [[ -z "${version}" ]]; then
  version="$(git branch --show-current 2>/dev/null || true)"
fi
if [[ -z "${version}" || "${version}" == "HEAD" ]]; then
  version="${GITHUB_REF_NAME:-dev}"
fi

commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
build_time="${BUILD_TIME:-$(date -u +%Y%m%dT%H%M%SZ)}"
full_version="${version}-${commit}-${build_time}"
go_ldflags="-X github.com/ysicing/go-template/internal/buildinfo.Version=${version} -X github.com/ysicing/go-template/internal/buildinfo.Commit=${commit} -X github.com/ysicing/go-template/internal/buildinfo.BuildTime=${build_time}"

printf "VERSION='%s'\n" "${version}"
printf "COMMIT='%s'\n" "${commit}"
printf "BUILD_TIME='%s'\n" "${build_time}"
printf "FULL_VERSION='%s'\n" "${full_version}"
printf "GO_LDFLAGS='%s'\n" "${go_ldflags}"
