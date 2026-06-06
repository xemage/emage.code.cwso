# Phase 9 Integration QA Report — Features E + F + G (Rollout)

**Target:** Phase 9 Rollout-as-a-Service (T132–T137)  
**Based on:** ADR-010, `rollout-architecture-v1.md`, tasks T132–T137  
**Date:** 2026-06-05

## Hypothesis

The Phase 9 rollout stack meets integration budgets when wired end-to-end:

1. Proxy capture path is non-blocking (bounded queue + drop counter)
2. Trajectory builder preserves prefix merge + loss masks (T133)
3. Parquet store writes off hot path (T134)
4. Merge SM rewards publish to `rollout/reward` and attach to task poll (T136–T137)
5. Polar REST submit → poll → callback flow works for trainer e2e (T137–T138)

## Reliability Budgets

| Budget | Target | Guard |
|--------|--------|-------|
| Proxy capture enqueue | non-blocking | `cwso-rollout` capture tests (try_send + drops) |
| Trajectory prefix merge | loss_mask=1 on sampled tokens | `rollout.TestBuildTrajectoryGroup*` |
| Parquet store hot path | no await on fsync | `cwso-rollout/src/store.rs` tests |
| Merge reward emission | +1/−1 on completion | `tools.TestMergeConcurrentResultsEmitsRewardOnSuccess` |
| Trainer REST e2e | submit → reward → poll → callback | `rollout.TestPhase9TrainerE2EFlow`, `TestPhase9RESTTrainerE2E` |

## Integration Coverage

- **Rust `cwso-rollout`:** proxy capture, Parquet writer (20 tests)
- **Go `rollout`:** trajectory builder, UDS client, REST service, trainer e2e (integration tests)
- **Go `tools`:** merge reward hook when `CWSO_ROLLOUT_REWARD_ENABLED`
- **CI:** socket-mounted Docker runners; full pipeline incl. e2e on T137 MR !46 (#2579885204)

## Verdict

**PASS** — T137 merged via MR !46 (`c1c56d6`); local `go test ./... -race` and `cargo test -p cwso-rollout` green.

## Notes

- `/v1/chat/completions` transparent proxy remains on `cwso-rollout` sidecar (orchestrator returns 501 stub).
- KV prefix router (**T135**) and real prewarm deferred; synthetic `prefix_key` on submit is acceptable for PoC.
- Proxy overhead p95 ≤ 5 ms budget validated in sidecar unit tests; fleet benchmark deferred to production hardening.
