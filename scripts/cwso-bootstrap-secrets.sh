#!/usr/bin/env bash
# Bootstrap the dev-only JWT secret file consumed by deploy/docker-compose.yml's
# `secrets: jwt_secret` mount (C012).
#
# Without this, a fresh checkout has no `.env.jwt.dev` and the orchestrator
# container refuses to start with a "JWT secret must be set" config error.
#
# Usage: scripts/cwso-bootstrap-secrets.sh
#
# Behavior:
#   - If .env.jwt.dev is absent: generate a cryptographically random 64-hex-char
#     secret, write `JWT_SECRET=<64 hex chars>` to .env.jwt.dev, chmod 600 it,
#     and print "[OK] generated .env.jwt.dev".
#   - If .env.jwt.dev is present: leave it untouched and print
#     "[OK] .env.jwt.dev exists". Exits 0 either way.
#
# The secret value is NEVER written to stdout/stderr — only the [OK] status
# lines above. Do not add debug output that echoes $JWT_SECRET or file
# contents.
#
# Secret generation prefers `openssl rand -hex 32`; if openssl is unavailable
# on the host, falls back to /dev/urandom:
#   head -c 32 /dev/urandom | od -A n -t x1 | tr -d ' \n'
#
# NOTE (scope): this script intentionally does not wire itself into the
# Makefile `up` target — that hook is owned by a separate task (C016). Until
# that lands, run this script manually (or via CI) before `docker compose up`.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT/.env.jwt.dev"

# Verify .env.jwt.dev is gitignored before generating anything. deploy/ secrets
# files must never be committable.
if git -C "$ROOT" check-ignore -q ".env.jwt.dev" 2>/dev/null; then
  : # already ignored — nothing to do
else
  GITIGNORE="$ROOT/.gitignore"
  if [[ -f "$GITIGNORE" ]] && grep -qxF '.env.jwt.dev' "$GITIGNORE"; then
    : # explicit entry already present but check-ignore failed for another
      # reason (e.g. not a git repo checkout) — do not treat as an error here
  else
    echo '.env.jwt.dev' >> "$GITIGNORE"
    echo "[OK] added .env.jwt.dev to .gitignore (was not already covered)"
  fi
fi

if [[ -f "$ENV_FILE" ]]; then
  echo "[OK] .env.jwt.dev exists"
  exit 0
fi

if command -v openssl >/dev/null 2>&1; then
  secret="$(openssl rand -hex 32)"
else
  # Fallback when openssl is unavailable on the host matrix.
  secret="$(head -c 32 /dev/urandom | od -A n -t x1 | tr -d ' \n')"
fi

umask 077
printf 'JWT_SECRET=%s\n' "$secret" > "$ENV_FILE"
chmod 600 "$ENV_FILE"
unset secret

echo "[OK] generated .env.jwt.dev"
