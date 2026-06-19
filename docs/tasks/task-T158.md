# Task T158 - Fix local Phase 2 integration auth mismatch

- **Status:** pending
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T157
- **Based on:** `scripts/phase2-integration.py`, `deploy/docker-compose.yml`, `deploy/docker-compose.ci.yml`

## Objective

Eliminate local JWT secret drift between orchestrator runtime and integration test token minting so
`scripts/phase2-integration.py` can pass deterministically in local (non-CI) mode.

## Acceptance Criteria

- [ ] Local integration path and orchestrator share one JWT source of truth by default.
- [ ] CI path remains compatible with env-driven secret injection.
- [ ] `python3 scripts/phase2-integration.py` passes on a clean local run.
- [ ] No security regression (JWT validation remains strict; no bypasses).
