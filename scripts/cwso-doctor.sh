#!/usr/bin/env bash
# scripts/cwso-doctor.sh — CWSO one-command-stack pre-flight/post-flight doctor.
#
# Checks everything the stack depends on and prints one [OK]/[WARN]/[FAIL] line
# per check, in a fixed order, with a one-line suggested fix after every
# [WARN]/[FAIL]. Diagnose-only: never mutates host state, never fixes anything.
#
# Exit code: 0 if no [FAIL] lines were printed, 1 otherwise. [WARN] never
# fails the run. Runtime-only checks (sidecar sockets, /healthz, token
# acceptance) are skipped with a single informational [OK] line when the
# stack is not running, so this script is always safe to run pre-flight on a
# clean host.
#
# Never prints secrets or tokens.

set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.yml"
JWT_DEV_FILE="${ROOT_DIR}/.env.jwt.dev"
TOKEN_SCRIPT="${ROOT_DIR}/scripts/cwso-token.sh"

ORCH_CONTAINER="cwso-orchestrator"
HEALTHZ_URL="http://127.0.0.1:8080/healthz"
MCP_URL="http://127.0.0.1:8080/mcp"
PORT=8080

# KVM / vhost-net device paths mirror orchestrator/internal/config/config.go
# defaults (CWSO_FIRECRACKER_KVM_DEVICE / CWSO_FIRECRACKER_VHOST_DEVICE), so
# this script honors the same env overrides the orchestrator itself does —
# this also gives us a clean way to simulate an absent device in tests.
KVM_DEVICE="${CWSO_FIRECRACKER_KVM_DEVICE:-/dev/kvm}"
VHOST_NET_DEVICE="${CWSO_FIRECRACKER_VHOST_DEVICE:-/dev/vhost-net}"

OK_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0

ok() {
  printf '[OK]   %s\n' "$1"
  OK_COUNT=$((OK_COUNT + 1))
}

warn() {
  printf '[WARN] %s\n' "$1"
  printf '       fix: %s\n' "$2"
  WARN_COUNT=$((WARN_COUNT + 1))
}

fail() {
  printf '[FAIL] %s\n' "$1"
  printf '       fix: %s\n' "$2"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

# ---------------------------------------------------------------------------
# 1. docker + docker compose available
# ---------------------------------------------------------------------------
check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    fail "docker CLI not found on PATH" \
      "Install Docker (https://docs.docker.com/engine/install/) and re-run."
    return
  fi

  if ! docker info >/dev/null 2>&1; then
    fail "docker CLI found but the daemon is unreachable" \
      "Start the Docker daemon (e.g. 'sudo systemctl start docker') and verify your user is in the 'docker' group."
    return
  fi

  if ! docker compose version >/dev/null 2>&1; then
    fail "'docker compose' (v2 plugin) not found" \
      "Install the compose plugin (https://docs.docker.com/compose/install/) — the legacy 'docker-compose' binary is not sufficient."
    return
  fi

  ok "docker and docker compose are available"
}

# ---------------------------------------------------------------------------
# 2. Port 8080 free, or owned by cwso-orchestrator
# ---------------------------------------------------------------------------
port_in_use() {
  # Pure-bash TCP probe; avoids depending on ss/lsof/nc being installed.
  ( exec 3<>"/dev/tcp/127.0.0.1/${PORT}" ) 2>/dev/null
  local rc=$?
  exec 3<&- 2>/dev/null
  exec 3>&- 2>/dev/null
  return $rc
}

check_port() {
  local owner=""
  if command -v docker >/dev/null 2>&1; then
    owner="$(docker ps --filter "publish=${PORT}" --format '{{.Names}}' 2>/dev/null | head -n1)"
  fi

  if ! port_in_use; then
    ok "Port ${PORT} is free"
    return
  fi

  if [[ "${owner}" == "${ORCH_CONTAINER}" ]]; then
    ok "Port ${PORT} is in use by ${ORCH_CONTAINER} (stack running)"
    return
  fi

  fail "Port ${PORT} is occupied by something other than ${ORCH_CONTAINER}" \
    "Find and stop the process holding port ${PORT} (e.g. 'lsof -i :${PORT}'), or 'make down' if a stale stack is still up."
}

# ---------------------------------------------------------------------------
# 3. /dev/kvm presence — mirrors sandbox.TierRouter degraded-mode conclusion
#    (orchestrator/internal/sandbox/router.go: resolveFirecracker() falls
#    back to gVisor with reason DEGRADED_FALLBACK_GVISOR when KVM/vhost-net
#    are absent; server.go sets SandboxDegradedMode from this same check).
# ---------------------------------------------------------------------------
check_kvm() {
  if [[ -e "${KVM_DEVICE}" ]]; then
    ok "${KVM_DEVICE} is present (Firecracker tier available)"
    return
  fi

  warn "${KVM_DEVICE} is absent — sandbox runs in degraded mode (Firecracker requests fall back to gVisor, reason DEGRADED_FALLBACK_GVISOR)" \
    "Enable KVM virtualization on the host (check '/dev/kvm' permissions, nested-virt settings) if firecracker-secure-isolation sandboxing is required; gVisor-only operation is otherwise fully supported."
}

# ---------------------------------------------------------------------------
# 4. vhost-net presence
# ---------------------------------------------------------------------------
check_vhost_net() {
  if [[ -e "${VHOST_NET_DEVICE}" ]]; then
    ok "${VHOST_NET_DEVICE} is present"
    return
  fi

  warn "${VHOST_NET_DEVICE} is absent — Firecracker networking requires it when RequireVhostNet is set" \
    "Load the vhost_net kernel module ('sudo modprobe vhost_net') or accept degraded gVisor-only sandboxing."
}

# ---------------------------------------------------------------------------
# 5. .env.jwt.dev exists and is gitignored
# ---------------------------------------------------------------------------
check_jwt_dev_file() {
  if [[ ! -f "${JWT_DEV_FILE}" ]]; then
    warn ".env.jwt.dev not found at repo root" \
      "Create it before starting the stack, e.g. 'head -c32 /dev/urandom | base64 > .env.jwt.dev' (never commit it)."
    return
  fi

  if git -C "${ROOT_DIR}" check-ignore -q -- ".env.jwt.dev" 2>/dev/null; then
    ok ".env.jwt.dev exists and is gitignored"
    return
  fi

  fail ".env.jwt.dev exists but is NOT gitignored — risk of committing a secret" \
    "Add '.env.jwt.dev' (or a matching '.env*' pattern) to .gitignore immediately; rotate the secret if it was ever committed."
}

# ---------------------------------------------------------------------------
# Runtime helpers
# ---------------------------------------------------------------------------
stack_running() {
  command -v docker >/dev/null 2>&1 || return 1
  [[ "$(docker inspect -f '{{.State.Running}}' "${ORCH_CONTAINER}" 2>/dev/null)" == "true" ]]
}

# ---------------------------------------------------------------------------
# 6. Sidecar sockets (only when stack is running)
# ---------------------------------------------------------------------------
check_sockets() {
  local sock
  local all_ok=1
  for sock in /run/cwso/git-shadow.sock /run/cwso/merge-engine.sock; do
    if docker exec "${ORCH_CONTAINER}" sh -c "test -S '${sock}'" >/dev/null 2>&1; then
      ok "Sidecar socket ${sock} is present"
    else
      all_ok=0
      fail "Sidecar socket ${sock} is missing inside ${ORCH_CONTAINER}" \
        "Check 'docker compose -f deploy/docker-compose.yml logs git-shadow merge-engine' for sidecar startup failures."
    fi
  done
  return $((1 - all_ok))
}

# ---------------------------------------------------------------------------
# 7. http://127.0.0.1:8080/healthz returns 200 (only when stack is running)
# ---------------------------------------------------------------------------
check_healthz() {
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "${HEALTHZ_URL}" 2>/dev/null || true)"
  if [[ "${code}" == "200" ]]; then
    ok "${HEALTHZ_URL} returned 200"
    return
  fi

  fail "${HEALTHZ_URL} did not return 200 (got '${code:-no response}')" \
    "Check orchestrator container health: 'docker compose -f deploy/docker-compose.yml logs orchestrator'."
}

# ---------------------------------------------------------------------------
# 8. A freshly minted token is accepted (only when stack is running)
# ---------------------------------------------------------------------------
check_token() {
  if [[ ! -x "${TOKEN_SCRIPT}" && ! -f "${TOKEN_SCRIPT}" ]]; then
    warn "scripts/cwso-token.sh not found (C013 not yet merged on this branch) — skipping token-acceptance check" \
      "Re-run this doctor once scripts/cwso-token.sh (C013) lands to exercise the full auth path."
    return
  fi

  local token
  token="$(bash "${TOKEN_SCRIPT}" 2>/dev/null)"
  if [[ -z "${token}" ]]; then
    fail "scripts/cwso-token.sh produced no token" \
      "Run 'bash scripts/cwso-token.sh' directly and inspect stderr for the underlying error."
    return
  fi

  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "${MCP_URL}" \
    -H "Authorization: Bearer ${token}" \
    -H "Origin: http://localhost" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' 2>/dev/null || true)"
  # Token value never leaves this function/scope in any printed output.
  unset token

  if [[ "${code}" == "200" ]]; then
    ok "Freshly minted token was accepted by ${MCP_URL}"
    return
  fi

  fail "Freshly minted token was rejected by ${MCP_URL} (got HTTP '${code:-no response}')" \
    "Verify CWSO_JWT_SECRET matches between scripts/cwso-token.sh and the running orchestrator, and check iss/aud claims."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "CWSO doctor — pre-flight/post-flight diagnostics"
  echo "--------------------------------------------------"

  check_docker
  check_port
  check_kvm
  check_vhost_net
  check_jwt_dev_file

  if stack_running; then
    check_sockets
    check_healthz
    check_token
  else
    ok "Stack not running (${ORCH_CONTAINER} not found) — skipping runtime checks (sidecar sockets, /healthz, token acceptance)"
  fi

  echo "--------------------------------------------------"
  echo "Summary: ${OK_COUNT} OK, ${WARN_COUNT} WARN, ${FAIL_COUNT} FAIL"

  if [[ "${FAIL_COUNT}" -gt 0 ]]; then
    exit 1
  fi
  exit 0
}

main "$@"
