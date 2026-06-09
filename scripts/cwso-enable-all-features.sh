#!/usr/bin/env bash
# Enable all CWSO orchestrator feature flags for local PoC demos (T155).
# Usage: source scripts/cwso-enable-all-features.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${CWSO_FEATURES_ENV:-$ROOT/deploy/cwso-all-features.env}"
EXAMPLE="$ROOT/scripts/cwso-enable-all-features.env.example"

if [[ ! -f "$ENV_FILE" ]]; then
  mkdir -p "$(dirname "$ENV_FILE")"
  cp "$EXAMPLE" "$ENV_FILE"
  echo "Created $ENV_FILE from example."
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

echo "CWSO feature flags loaded from $ENV_FILE"
echo ""
echo "Docker Compose (phase2 + phase4 core sidecars):"
echo "  docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d"
echo ""
echo "Pass orchestrator env (example):"
echo "  docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 \\"
echo "    up -d --force-recreate orchestrator"
echo ""
echo "Note: HAL, sparse, and rollout sidecars need separate containers/sockets."
echo "See docs/user/installation-v1.md and ide-integration-v1.md."
