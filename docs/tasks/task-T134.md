# Task T134 — Trajectory store (Arrow + LZ4 + Parquet)

> **ID note:** roadmap **Feature E / placeholder T108**. Active **T134** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T133 (trajectory builder)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/artifacts/rollout-architecture-v1.md` §5, `ADR-010-rollout-proxy-boundary.md`

## Objective

Persist captured `CompletionRecord` artifacts from the proxy hot path to columnar Parquet files
with LZ4 compression on a dedicated I/O thread, without blocking the capture enqueue path.

## Deliverables

- **`services/cwso-rollout/src/store.rs`** — Arrow schema, Parquet writer, retention sweep
- **Config** — `CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED`, `CWSO_ROLLOUT_STORE_PATH`,
  `CWSO_ROLLOUT_STORE_RETENTION_DAYS`, `CWSO_ROLLOUT_DEFAULT_SESSION_ID`
- **Fan-out enqueue** — capture queue + store queue (IPC `drain_capture` unchanged)
- **Tests** — round-trip write/read, retention, non-blocking enqueue under saturated store queue
- **CI** — `cargo test -p cwso-rollout` green

## Acceptance Criteria

- [x] Layout `rollout_store/YYYY-MM-DD/{session_id}.parquet.lz4`
- [x] Dedicated thread drains store channel; proxy never awaits fsync
- [x] Saturated store queue drops with metric (non-blocking hot path)
- [x] `cargo test -p cwso-rollout` green locally (20 tests)
- [ ] CI green on T134 MR !43 (do not merge unless user requests)

## Notes

- Chain columns (post-prefix-merge) may be added when T137 wires session lifecycle; v1 stores
  raw completion records for trainer ingest.
- Programmatic rewards (**T136**, P0) can proceed in parallel.
