# Task T136 — Programmatic reward emission (merge SM hook)

> **ID note:** roadmap **Feature G / placeholder T110**. Active **T136** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T133 (trajectory builder)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/artifacts/rollout-architecture-v1.md` §7, `ADR-010-rollout-proxy-boundary.md`

## Objective

Emit deterministic programmatic rewards when `merge_concurrent_results` completes, publishing
to the `rollout/reward` memory-broker topic for trainer consumption (T137 attaches rewards
to trajectory groups).

## Deliverables

- **`orchestrator/internal/rollout/reward.go`** — reward kinds, classifier, emitter
- **Merge tool hook** — optional `rollout_session_id` arg; output fields `merge_reward`, `reward_kind`
- **Config** — `CWSO_ROLLOUT_REWARD_ENABLED` (default off)
- **Server wiring** — `registerMergeTools` passes `RewardEmitter` when enabled
- **Tests** — classifier unit tests + merge tool integration test

## Reward table (v1)

| Event | Reward | Condition |
|-------|--------|-----------|
| `merge_success` | +1.0 | Outcome `success` |
| `merge_conflict` | −1.0 | Outcome `conflict` or non-syntax error |
| `syntax_fail` | −1.0 | Outcome `error` with `invalid_engine_payload` |

## Acceptance Criteria

- [x] Rewards published to `rollout/reward` when flag enabled
- [x] Tool response includes reward fields for downstream REST (T137)
- [x] Disabled by default; no-op when flag off
- [x] `go test ./...` green
- [ ] CI green on T136 MR

## Notes

- `test_pass` bonus deferred to v2 per architecture doc.
- T137 depends on T134 + T136 for Polar REST + trainer e2e.
