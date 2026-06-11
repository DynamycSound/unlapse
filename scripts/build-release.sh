#!/usr/bin/env sh
# Builds release binaries for every supported platform into ./dist.
# Usage: scripts/build-release.sh [version]
set -eu

VERSION="${1:-dev}"
LDFLAGS="-s -w -X github.com/DynamycSound/unlapse/internal/server.Version=${VERSION}"

rm -rf dist
mkdir -p dist

build() {
  goos="$1"; goarch="$2"; suffix="$3"
  out="dist/unlapse-${VERSION}-${goos}-${goarch}${suffix}"
  echo "building ${out}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" ./cmd/unlapse
}

build linux   amd64 ""
build linux   arm64 ""
build windows amd64 ".exe"
build darwin  amd64 ""
build darwin  arm64 ""

(cd dist && sha256sum unlapse-* > SHA256SUMS.txt)
echo "done:"
ls -l dist
