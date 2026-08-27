#!/usr/bin/env bash
# check-ipc-gid-drift.sh — fail when deploy/docker-compose.yml's
# CWSO_IPC_ALLOWED_UIDS/CWSO_IPC_ALLOWED_GIDS on the git-shadow and
# merge-engine services no longer match the orchestrator image's actual,
# live `cwso` uid/gid.
#
# Background (T197): deploy/docker-compose.yml hardcodes these allowlists as
# literal CSV strings because the values originate in a *different* image
# (deploy/Dockerfile.orchestrator's `addgroup -S cwso && adduser -S -G cwso
# cwso`, which Alpine assigns dynamically based on base-image state) than the
# one that reads them (git-shadow/merge-engine's own Dockerfiles). A rebuild
# of the orchestrator image can silently shift its `cwso` uid/gid without
# touching docker-compose.yml at all -- exactly how "0,100" drifted out of
# sync with the orchestrator's real gid=101 (found during T191, fixed under
# T197). This script closes that blind spot by asserting the two never
# diverge again.
#
# Note on severity: services/cwso-git-shadow/src/main.rs and
# services/cwso-merge-engine/src/ipc.rs both implement `allows()` as
# `allowed_uids.contains(uid) || allowed_gids.contains(gid)` (an OR). As long
# as CWSO_IPC_ALLOWED_UIDS also matches (it does today), a wrong GID entry is
# latent drift, not an active access-control failure -- this script still
# flags it as a FAIL because a config value that silently doesn't do what its
# name claims should never be left uncorrected, but a failure here is not, by
# itself, evidence of a live outage.
#
# What this checks, for both the git-shadow and merge-engine services:
#   1. The orchestrator image's live `cwso` uid is present in that service's
#      CWSO_IPC_ALLOWED_UIDS CSV.
#   2. The orchestrator image's live `cwso` gid is present in that service's
#      CWSO_IPC_ALLOWED_GIDS CSV.
#
# Requires: docker, bash, sed, grep. Builds cwso/orchestrator:dev locally if
# it is not already present (same command as `make build-orchestrator`).
#
# Exit code: 0 if both services' allowlists cover the live uid and gid; 1 on
# a genuine mismatch (drift); 2 on a setup/environment error (docker
# unavailable, compose file unreadable, etc.).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

COMPOSE_FILE="deploy/docker-compose.yml"
DOCKERFILE="deploy/Dockerfile.orchestrator"
IMAGE="cwso/orchestrator:dev"

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker is required but not found on PATH" >&2
  exit 2
fi

if [ ! -f "$COMPOSE_FILE" ]; then
  echo "ERROR: $COMPOSE_FILE not found (run from repo root or via scripts/)" >&2
  exit 2
fi

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "==> $IMAGE not found locally, building from $DOCKERFILE ..."
  docker build -t "$IMAGE" -f "$DOCKERFILE" . >&2
fi

id_output="$(docker run --rm --entrypoint id "$IMAGE" cwso)"
live_uid="$(echo "$id_output" | sed -E 's/^uid=([0-9]+).*/\1/')"
live_gid="$(echo "$id_output" | sed -E 's/.*[[:space:]]gid=([0-9]+).*/\1/')"

if [ -z "$live_uid" ] || [ -z "$live_gid" ]; then
  echo "ERROR: could not parse uid/gid from '$id_output'" >&2
  exit 2
fi

echo "Live orchestrator 'cwso' identity: uid=$live_uid gid=$live_gid"

# Extracts the value of an env var (as a bare CSV string, quotes stripped)
# for the given top-level compose service. Scoped to the block of lines
# between the service's own "  <service>:" header and the next top-level
# "  <key>:" header (or EOF), so a var name that happens to repeat under a
# different service is never picked up by mistake.
extract_env_value() {
  local service="$1" var="$2"
  local start next_offset end
  start="$(grep -n "^  ${service}:[[:space:]]*\$" "$COMPOSE_FILE" | head -n1 | cut -d: -f1)"
  if [ -z "$start" ]; then
    echo "ERROR: service '$service' not found in $COMPOSE_FILE" >&2
    exit 2
  fi
  next_offset="$(tail -n "+$((start + 1))" "$COMPOSE_FILE" | grep -n "^  [A-Za-z0-9_-]\+:[[:space:]]*\$" | head -n1 | cut -d: -f1 || true)"
  if [ -n "$next_offset" ]; then
    end=$((start + next_offset - 1))
  else
    end="$(wc -l < "$COMPOSE_FILE")"
  fi
  sed -n "${start},${end}p" "$COMPOSE_FILE" \
    | grep "${var}:" \
    | head -n1 \
    | sed -E "s/.*${var}:[[:space:]]*\"([^\"]*)\".*/\1/"
}

csv_contains() {
  local csv="$1" needle="$2"
  local IFS=','
  local item
  for item in $csv; do
    if [ "$item" = "$needle" ]; then
      return 0
    fi
  done
  return 1
}

status=0

for service in git-shadow merge-engine; do
  uids_csv="$(extract_env_value "$service" "CWSO_IPC_ALLOWED_UIDS")"
  gids_csv="$(extract_env_value "$service" "CWSO_IPC_ALLOWED_GIDS")"

  if [ -z "$uids_csv" ]; then
    echo "ERROR: $service: CWSO_IPC_ALLOWED_UIDS not found or empty in $COMPOSE_FILE" >&2
    exit 2
  fi
  if [ -z "$gids_csv" ]; then
    echo "ERROR: $service: CWSO_IPC_ALLOWED_GIDS not found or empty in $COMPOSE_FILE" >&2
    exit 2
  fi

  if csv_contains "$uids_csv" "$live_uid"; then
    echo "OK: $service CWSO_IPC_ALLOWED_UIDS=\"$uids_csv\" contains live uid=$live_uid"
  else
    echo "DRIFT: $service CWSO_IPC_ALLOWED_UIDS=\"$uids_csv\" does NOT contain live uid=$live_uid" >&2
    status=1
  fi

  if csv_contains "$gids_csv" "$live_gid"; then
    echo "OK: $service CWSO_IPC_ALLOWED_GIDS=\"$gids_csv\" contains live gid=$live_gid"
  else
    echo "DRIFT: $service CWSO_IPC_ALLOWED_GIDS=\"$gids_csv\" does NOT contain live gid=$live_gid" >&2
    status=1
  fi
done

if [ "$status" -ne 0 ]; then
  echo >&2
  echo "One or more CWSO_IPC_ALLOWED_UIDS/GIDS entries in $COMPOSE_FILE no longer match" >&2
  echo "the orchestrator image's live 'cwso' identity. Update the literal CSV value(s)" >&2
  echo "on the affected service(s) in $COMPOSE_FILE to include uid=$live_uid / gid=$live_gid." >&2
  echo "See T197 (docs/tasks/completed-tasks.md) for the originating context." >&2
fi

exit "$status"
