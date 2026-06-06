# Checkpoint 011 — Phase 9 Complete (Rollout-as-a-Service)

**Date:** 2026-06-05  
**Phase:** 9 — Features E + F + G (Rollout / Polar substrate)  
**develop tip:** `c1c56d6` (post T137 MR !46)

## Completed tasks

| ID | Title | Merge |
|----|-------|-------|
| T131 | Rollout architecture (ADR-010) | !40 `2d40413` |
| T132 | Hyper proxy + zero-copy capture | !41 `267922c` |
| T133 | Trajectory builder + prefix merging | !42 `18b5a40` |
| T134 | Parquet trajectory store | !43 `26761ab` |
| T136 | Programmatic merge rewards | !45 `892142f` |
| T137 | Polar REST API | !46 `c1c56d6` |
| T138 | Phase 9 QA + security gate | pending (!47) |

## Key decisions

- Rollout proxy on Rust `cwso-rollout` sidecar; Polar REST on Go orchestrator HTTP.
- Merge rewards deterministic (+1/−1); published to `rollout/reward` broker topic.
- CI uses socket-mounted Docker runners (`docker` / `dind` tags); DinD sidecars removed.

## Deferred / debt

- **T135** KV prefix router (P1, parallel path).
- Orchestrator `/v1/chat/completions` stub (501); proxy on sidecar.
- Trainer fleet benchmark (proxy p95 ≤ 5 ms) — unit-tested, not fleet-validated.

## Blockers

None.

## Next steps

1. Merge T138 gate MR !47.
2. **T139** — v0.3.0 release readiness + docs.

## Token metrics

Planning + implementation tracked within Phase 9 budgets per AGENTS.md.
