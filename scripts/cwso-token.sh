#!/usr/bin/env bash
# cwso-token.sh — mint a short-lived HS256 JWT for the CWSO MCP server.
#
# Usage:
#   scripts/cwso-token.sh [--role orchestrator|worker] [--ttl <seconds>]
#
# Defaults: --role orchestrator --ttl 3600
#
# The signing secret is read from .env.jwt.dev in the repo root. If that
# file is missing, run scripts/cwso-bootstrap-secrets.sh first to generate
# it, then re-run this script.
#
# Only the signed token is written to stdout. All diagnostics, usage, and
# error output go to stderr, so this composes cleanly:
#
#   TOKEN=$(scripts/cwso-token.sh)
#   TOKEN=$(scripts/cwso-token.sh --role worker --ttl 900)
#
# Claims minted (must match orchestrator/internal/transport/http.go
# verifyJWT()/authMiddleware() exactly):
#   alg: HS256
#   sub: vscode-dev
#   role: <--role>
#   iss:  cwso           (CWSO_JWT_ISSUER default)
#   aud:  [cwso-mcp]      (CWSO_JWT_AUDIENCE default)
#   iat:  now
#   exp:  now + <--ttl>

set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: cwso-token.sh [--role orchestrator|worker] [--ttl <seconds>]

Mint a short-lived HS256 JWT for the CWSO MCP server, signed with the
development secret in .env.jwt.dev (repo root).

Options:
  --role orchestrator|worker   Role claim to embed (default: orchestrator)
  --ttl <seconds>              Token lifetime in seconds (default: 3600)
  -h, --help                   Show this help and exit

Output:
  The signed JWT is printed on stdout only. Everything else goes to stderr.

Example:
  TOKEN=$(scripts/cwso-token.sh --role worker --ttl 900)
EOF
}

role="orchestrator"
ttl="3600"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --role)
      role="${2:-}"
      shift 2
      ;;
    --ttl)
      ttl="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "cwso-token.sh: unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ "$role" != "orchestrator" && "$role" != "worker" ]]; then
  echo "cwso-token.sh: --role must be 'orchestrator' or 'worker' (got: '$role')" >&2
  exit 2
fi

if ! [[ "$ttl" =~ ^[0-9]+$ ]] || [[ "$ttl" -le 0 ]]; then
  echo "cwso-token.sh: --ttl must be a positive integer number of seconds (got: '$ttl')" >&2
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
secret_file="$repo_root/.env.jwt.dev"

if [[ ! -f "$secret_file" ]]; then
  cat <<EOF >&2
cwso-token.sh: secret file not found: $secret_file

Run scripts/cwso-bootstrap-secrets.sh first to generate the development
JWT signing secret, then re-run this script.
EOF
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "cwso-token.sh: python3 is required but was not found on PATH" >&2
  exit 1
fi

CWSO_TOKEN_ROLE="$role" \
CWSO_TOKEN_TTL="$ttl" \
CWSO_TOKEN_SECRET_FILE="$secret_file" \
python3 - <<'PY'
import base64
import hashlib
import hmac
import json
import os
import sys
import time

secret_file = os.environ["CWSO_TOKEN_SECRET_FILE"]
role = os.environ["CWSO_TOKEN_ROLE"]
ttl = int(os.environ["CWSO_TOKEN_TTL"])

try:
    with open(secret_file, "r", encoding="utf-8") as f:
        secret = f.read().strip().encode()
except OSError as exc:
    print(f"cwso-token.sh: failed to read secret file {secret_file}: {exc}", file=sys.stderr)
    sys.exit(1)

if not secret:
    print(f"cwso-token.sh: secret file {secret_file} is empty", file=sys.stderr)
    sys.exit(1)

now = int(time.time())
header = {"alg": "HS256", "typ": "JWT"}
claims = {
    "sub": "vscode-dev",
    "role": role,
    "iss": "cwso",
    "aud": ["cwso-mcp"],
    "iat": now,
    "exp": now + ttl,
}


def b64(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


signing_input = (
    f"{b64(json.dumps(header, separators=(',', ':')).encode())}"
    f".{b64(json.dumps(claims, separators=(',', ':')).encode())}"
)
signature = b64(hmac.new(secret, signing_input.encode(), hashlib.sha256).digest())
print(f"{signing_input}.{signature}")
PY
