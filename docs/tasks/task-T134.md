# Task T134 — Trajectory store (Arrow + LZ4 + Parquet)

> **ID note:** roadmap **Feature E / placeholder T108**. Active **T134** (see `active-tasks.md`).

- **Status:** done
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T133 (trajectory builder)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/artifacts/rollout-architecture-v1.md` §5, `ADR-010-rollout-proxy-boundary.md`
- **Merged:** MR !43 → `develop` @ `26761ab` (source `374d672`, pipeline #2579771042)

## Objective

Persist captured `CompletionRecord` artifacts from the proxy hot path to columnar Parquet files
with LZ4 compression on a dedicated I/O thread, without blocking the capture enqueue path.

## Deliverables

- **`services/cwso-rollout/src/store.rs`** — Arrow schema, Parquet writer, retention sweep
- **Config** — `CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED`, `CWSO_ROLLOUT_STORE_PATH`,
  `CWSO_ROLLOUT_STORE_RETENTION_DAYS`, `CWSO_ROLLOUT_DEFAULT_SESSION_ID`
- **Fan-out enqueue** — capture queue + store queue (IPC `drain_capture` unchanged)
- **Tests** — round-trip write/read, retention, non-blocking enqueue under saturated store queue
- **CI** — socket-mounted Docker runner layout (`.gitlab-ci.yml`)

## Acceptance Criteria

- [x] Layout `rollout_store/YYYY-MM-DD/{session_id}.parquet.lz4`
- [x] Dedicated thread drains store channel; proxy never awaits fsync
- [x] Saturated store queue drops with metric (non-blocking hot path)
- [x] `cargo test -p cwso-rollout` green (20 tests)
- [x] CI green on MR !43 (all 11 jobs including e2e)

## Notes

- Chain columns (post-prefix-merge) may be added when T137 wires session lifecycle.
- Programmatic rewards (**T136**, P0) proceeds after T134 merge.
