#!/bin/sh
# Reference Polar shell-command harness (T144). Routes model traffic through cwso-rollout.
set -eu

BASE="${OPENAI_BASE_URL:-http://127.0.0.1:8787}"
PROMPT="${CWSO_HARNESS_PROMPT:-ping}"

if command -v curl >/dev/null 2>&1; then
  HTTP_CLIENT=curl
elif command -v wget >/dev/null 2>&1; then
  HTTP_CLIENT=wget
else
  echo "shell-command harness requires curl or wget" >&2
  exit 1
fi

PAYLOAD=$(printf '{"model":"gpt-4","messages":[{"role":"user","content":"%s"}],"stream":false}' "$PROMPT")

if [ "$HTTP_CLIENT" = curl ]; then
  curl -sS "${BASE}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${OPENAI_API_KEY:-cwso-harness}" \
    -d "$PAYLOAD"
else
  wget -qO- "${BASE}/v1/chat/completions" \
    --header="Content-Type: application/json" \
    --header="Authorization: Bearer ${OPENAI_API_KEY:-cwso-harness}" \
    --post-data="$PAYLOAD"
fi
