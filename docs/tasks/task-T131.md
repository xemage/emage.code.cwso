# Task T131 — Rollout architecture (proxy boundary + Polar API)

> **ID note:** roadmap **Feature E+F+G / placeholder T105**. Active **T131** (see `active-tasks.md`).

- **Status:** in_review
- **Owner:** solution-architect (reviewers: tech-lead, backend-developer, security-engineer)
- **Priority:** P0
- **Depends on:** T130 (Phase 8 gate)
- **Phase:** 9 — Rollout-as-a-Service (Polar)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` §3.7, §5.2–5.4; `gate-phase8-feature-d-2026-06-04.md`

## Objective

Define the Phase 9 rollout substrate architecture: Rust `cwso-rollout` proxy sidecar, Go Polar
REST API, trajectory builder/store, KV prefix router, and programmatic reward hooks — gating T132–T138
implementation.

## Deliverables

- **`docs/decisions/ADR-010-rollout-proxy-boundary.md`** — proxy boundary, sidecar split, feature flags.
- **`docs/artifacts/rollout-architecture-v1.md`** — topology, capture pipeline, prefix merge, API surface,
  T132–T139 breakdown.
- **`schemas/rollout_task_submit.json`**, **`schemas/rollout_task_status.json`** — REST contract stubs.
- **`orchestrator/internal/rollout/schema_test.go`** — schema parse guards.

## Acceptance Criteria

- [x] ADR-010 documents Go/Rust split, security boundary, and alternatives.
- [x] Design v1 specifies capture pipeline, trajectory builder, store, KV router, rewards.
- [x] Polar REST routes enumerated with schemas.
- [x] Implementation breakdown T132–T139 with owners, priorities, dependencies.
- [x] Board + reconciliation mapping updated for Phase 9 kickoff.
- [ ] CI green on T131 MR

## Notes

- Docs/architecture only; proxy implementation lands in **T132**.
- CWSO is rollout substrate only — no embedded trainer.
