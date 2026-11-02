#!/usr/bin/env bash
# scripts/check-image-pins.sh -- image pin guard (T728).
#
# Fails when
#   1. an external image reference is not pinned by digest (SEC-T696-04):
#      every `image:` / `*_IMAGE:` value in .gitlab-ci.yml, every external
#      `image:` in deploy/docker-compose.yml, every `FROM` and `# syntax=`
#      line in deploy/Dockerfile.*, and every rust/golang image in the two
#      scripts that run builds must read `<name>:<tag>@sha256:<64 hex>`;
#   2. the Rust or Go toolchain differs between CI, the Dockerfiles and
#      scripts/release-assets.sh (SEC-T695-09, SEC-T696-01): all rust images
#      must name one Rust version, all golang images one Go version.
#
# Locally built images (`cwso/<svc>:dev` in docker-compose.yml) are skipped:
# compose builds them from the pinned Dockerfiles. Offline; needs bash, grep,
# sed, sort. Run from anywhere: scripts/check-image-pins.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIN_RE='^[a-z0-9._/-]+(:[A-Za-z0-9._-]+)?@sha256:[0-9a-f]{64}$'
FAILED=0

# Print "<file>:<line>: <image ref>" for every image reference to check.
collect_refs() {
  cd "$ROOT"
  grep -nE '^[[:space:]]*(image|[A-Z_]+_IMAGE):[[:space:]]*[^[:space:]]' .gitlab-ci.yml |
    sed -E 's/^([0-9]+):[[:space:]]*[A-Za-z_]+:[[:space:]]*"?([^"[:space:]]+)"?.*/.gitlab-ci.yml:\1: \2/'
  grep -nE '^[[:space:]]*image:' deploy/docker-compose.yml | grep -vE 'image:[[:space:]]*cwso/' |
    sed -E 's/^([0-9]+):[[:space:]]*image:[[:space:]]*([^[:space:]]+).*/deploy\/docker-compose.yml:\1: \2/'
  local f
  for f in deploy/Dockerfile.*; do
    grep -nE '^FROM ' "$f" | sed -E "s#^([0-9]+):FROM ([^[:space:]]+).*#$f:\\1: \\2#"
    grep -nE '^# syntax=' "$f" | sed -E "s#^([0-9]+):\\# syntax=([^[:space:]]+).*#$f:\\1: \\2#"
  done
  for f in scripts/release-assets.sh scripts/cwso-projection-e2e.sh; do
    grep -nE '^[^#]*\b(rust|golang):[0-9]' "$f" |
      sed -E "s#^([0-9]+):[^a-z]*((rust|golang):[^[:space:]]+).*#$f:\\1: \\2#"
  done
}

check_pinned() {
  local loc ref
  while IFS= read -r line; do
    loc="${line%%: *}"
    ref="${line#*: }"
    if [[ ! "$ref" =~ $PIN_RE ]]; then
      echo "NOT PINNED BY DIGEST: $loc: $ref" >&2
      FAILED=1
    fi
  done
}

# $1 = image name (rust | golang). All references must share one version.
check_one_version() {
  local versions
  versions="$(collect_refs | sed -nE "s#.*: $1:([0-9]+\\.[0-9]+(\\.[0-9]+)?).*#\\1#p" | sort -u)"
  if [[ "$(printf '%s\n' "$versions" | grep -c .)" -ne 1 ]]; then
    echo "MORE THAN ONE (OR NO) $1 VERSION: $(printf '%s ' $versions)" >&2
    collect_refs | grep -E ": $1:" >&2 || true
    FAILED=1
  fi
}

main() {
  local refs
  refs="$(collect_refs)"
  echo "checked $(printf '%s\n' "$refs" | grep -c .) image references"
  check_pinned <<<"$refs"
  check_one_version rust
  check_one_version golang
  if [[ "$FAILED" -ne 0 ]]; then
    echo "image pin check FAILED" >&2
    exit 1
  fi
  echo "image pin check OK"
}

main "$@"
