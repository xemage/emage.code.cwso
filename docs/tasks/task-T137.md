# Task T137 — Polar REST API + trainer e2e

> **ID note:** roadmap **placeholder T111**. Active **T137** (see `active-tasks.md`).

- **Status:** done
- **Merged:** MR !46 → `develop` @ `c1c56d6` (pipeline #2579885204)
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T134 (trajectory store), T136 (programmatic rewards)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/artifacts/rollout-architecture-v1.md` §8, `schemas/rollout_task_*.json`

## Objective

Expose Polar-compatible REST routes on the orchestrator HTTP server for rollout task
submission, status polling, fleet status, node registration, and trainer callbacks.

## Deliverables

- **`orchestrator/internal/rollout/service.go`** — in-memory task/node store, reward aggregation
- **`orchestrator/internal/rollout/api_handler.go`** — HTTP routes per architecture §8
- **Config** — `CWSO_ROLLOUT_API_ENABLED`, `CWSO_ROLLOUT_SOCKET` (optional trajectory drain)
- **Transport** — `WithRolloutAPI` HTTPOption mounts routes with JWT auth
- **Tests** — service + HTTP handler tests

## Routes (v1)

| Method | Path |
|--------|------|
| POST | `/rollout/task/submit` |
| GET | `/rollout/task/{task_id}` |
| GET | `/rollout/status` |
| POST | `/callbacks/session_result` |
| POST | `/nodes/register` |
| POST | `/nodes/{id}/heartbeat` |
| POST | `/v1/chat/completions` | stub → 501 (sidecar proxy) |

## Acceptance Criteria

- [x] Submit + poll conform to JSON schemas
- [x] GET task attaches merge rewards from `rollout/reward` broker topic
- [x] Optional trajectory drain when `CWSO_ROLLOUT_SOCKET` set
- [x] JWT auth on rollout routes (same as `/mcp`)
- [x] `go test ./...` green locally
- [x] CI green on MR !46

## Notes

- Trainer e2e validation deferred to **T138** gate.
- KV prefix prewarm returns synthetic `prefix_key` until **T135** lands.
