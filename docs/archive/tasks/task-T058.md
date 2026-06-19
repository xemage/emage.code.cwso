# Task T058 — Harden sidecar socket permissions and peer auth

- Phase: **4 (Security Fix)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T051 · Blocks: T051
- Status: done

## Objective
Eliminate sidecar IPC authorization bypass by enforcing least-privilege Unix socket permissions and validating caller identity for sidecar connections.

## Inputs
- [checkpoint-018-phase4-t050-conditional-pass.md](../checkpoints/checkpoint-018-phase4-t050-conditional-pass.md)
- T051 security finding references
- [services/cwso-git-shadow/src/main.rs](../../services/cwso-git-shadow/src/main.rs)
- [services/cwso-merge-engine/src/ipc.rs](../../services/cwso-merge-engine/src/ipc.rs)
- [deploy/docker-compose.yml](../../deploy/docker-compose.yml)

## Constraints
- Preserve orchestrator-to-sidecar functionality.
- Enforce least-privilege socket permissions (`0600`/`0660`) and identity checks.
- Add negative tests for unauthorized socket clients.

## Expected outputs
- Hardened sidecar socket setup and peer-credential authorization checks.
- Tests proving unauthorized access is rejected.

## Acceptance criteria
1. Sidecar sockets are no longer world-writable.
2. Unauthorized local peers cannot invoke sidecar APIs.
3. Existing integration/e2e flow remains functional.

## Blocker protocol
If platform-level peer credential checks differ by environment, report portability impact and provide secure fallback design.

## Completion notes (2026-05-16)
- Tightened sidecar socket permissions from world-writable to group-restricted `0660` for both sidecars.
- Added Linux peer-credential authorization using `SO_PEERCRED`, with allowlists sourced from `CWSO_IPC_ALLOWED_UIDS` and `CWSO_IPC_ALLOWED_GIDS`.
- Rejected unauthorized peers before any framed request handling in both sidecars.
- Added negative unit tests proving unauthorized peers are denied by `authorize_stream`.

Validation evidence:
- `cd /home/emage/Code/emage/CWSO && docker run --rm -v "$PWD":/workspace -w /workspace/services rust:1.86-slim cargo test -p cwso-git-shadow -p cwso-merge-engine`: PASS

Residual notes:
- Non-security warnings remain in `cwso-git-shadow` tests (`TempDir::into_path` deprecation, unused `base_tree` field), but they do not affect T058 acceptance criteria.
