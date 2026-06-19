#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
if [[ -z "$TAG" ]]; then
  echo "Usage: scripts/release-assets.sh <tag>"
  echo "Example: scripts/release-assets.sh v0.1.2"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/$TAG"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command not found: $1"
    exit 1
  fi
}

require_cmd docker
require_cmd glab

mkdir -p "$DIST_DIR"

echo "==> Building Linux binaries for $TAG"
docker run --rm \
  -v "$ROOT_DIR":/workspace \
  -w /workspace/orchestrator \
  golang:1.23-bookworm \
  sh -lc 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -buildvcs=false -o /workspace/dist/'"$TAG"'/cwso-orchestrator-linux-amd64 ./cmd/cwso-orchestrator'

docker run --rm \
  -v "$ROOT_DIR":/workspace \
  -w /workspace/services \
  rust:1.86-bookworm \
  sh -lc 'apt-get update -qq && apt-get install -y -qq build-essential pkg-config cmake ca-certificates >/dev/null && /usr/local/cargo/bin/cargo build --release -p cwso-git-shadow -p cwso-merge-engine && cp target/release/cwso-git-shadow /workspace/dist/'"$TAG"'/cwso-git-shadow-linux-amd64 && cp target/release/cwso-merge-engine /workspace/dist/'"$TAG"'/cwso-merge-engine-linux-amd64'

echo "==> Building container image archives for $TAG"
docker build -t "cwso/orchestrator:$TAG" -f "$ROOT_DIR/deploy/Dockerfile.orchestrator" "$ROOT_DIR"
docker save "cwso/orchestrator:$TAG" | gzip -c > "$DIST_DIR/cwso-orchestrator-image-$TAG.tar.gz"

docker build -t "cwso/git-shadow:$TAG" -f "$ROOT_DIR/deploy/Dockerfile.git-shadow" "$ROOT_DIR"
docker save "cwso/git-shadow:$TAG" | gzip -c > "$DIST_DIR/cwso-git-shadow-image-$TAG.tar.gz"

docker build -t "cwso/merge-engine:$TAG" -f "$ROOT_DIR/deploy/Dockerfile.merge-engine" "$ROOT_DIR"
docker save "cwso/merge-engine:$TAG" | gzip -c > "$DIST_DIR/cwso-merge-engine-image-$TAG.tar.gz"

echo "==> Uploading release assets to tag $TAG"
glab release upload "$TAG" \
  "$DIST_DIR/cwso-orchestrator-linux-amd64#cwso-orchestrator-linux-amd64#package" \
  "$DIST_DIR/cwso-git-shadow-linux-amd64#cwso-git-shadow-linux-amd64#package" \
  "$DIST_DIR/cwso-merge-engine-linux-amd64#cwso-merge-engine-linux-amd64#package" \
  "$DIST_DIR/cwso-orchestrator-image-$TAG.tar.gz#cwso-orchestrator-image-$TAG.tar.gz#package" \
  "$DIST_DIR/cwso-git-shadow-image-$TAG.tar.gz#cwso-git-shadow-image-$TAG.tar.gz#package" \
  "$DIST_DIR/cwso-merge-engine-image-$TAG.tar.gz#cwso-merge-engine-image-$TAG.tar.gz#package"

echo "==> Done. Assets uploaded for $TAG"
