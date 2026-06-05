# Task T133 — Trajectory builder + prefix merging

> **ID note:** roadmap **Feature E / placeholder T107**. Active **T133** (see `active-tasks.md`).

- **Status:** done
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T132 (cwso-rollout capture)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/decisions/ADR-010-rollout-proxy-boundary.md`, `docs/artifacts/rollout-architecture-v1.md` §4

## Objective

Assemble token-faithful trajectory groups from `CompletionRecord` streams drained from
`cwso-rollout`, applying Polar prefix merging (append-only chains, `loss_mask=1` on sampled
assistant tokens only).

## Deliverables

- **`orchestrator/internal/rollout/trajectory.go`** — `BuildTrajectoryGroup`, prefix merge, validation
- **`orchestrator/internal/rollout/types.go`** — `CompletionRecord`, `Chain`, `TrajectoryGroup`
- **`orchestrator/internal/rollout/client.go`** — UDS client (`drain_capture`, `capture_stats`, `stat`)
- **Tests** — prefix merge, parallel branches, IPC envelope shape

## Acceptance Criteria

- [x] Stable sort by `(timestamp_ns, request_id)` before merge
- [x] Extend chain when prompt equals `prefix || prior sampled tokens`
- [x] Parallel divergent prompts spawn separate chains
- [x] `loss_mask=1` on all sampled assistant tokens in steps
- [x] `go test ./internal/rollout/...` green
- [x] CI green on T133 MR (pipeline #2578342413 at `59026df`)
- [x] MR !42 merged to `develop` (`18b5a40`)

## Notes

- Parquet trajectory store lands in **T134**; programmatic rewards in **T136**.
- Polar REST routes land in **T137**.
