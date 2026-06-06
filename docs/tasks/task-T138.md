# Task T138 — Phase 9 integration QA + security gate

> **ID note:** roadmap **placeholder T112**. Active **T138** (see `active-tasks.md`).

- **Status:** done
- **Owner:** qa-engineer / security-engineer / tech-lead
- **Priority:** P0
- **Depends on:** T137 (Polar REST API)
- **Phase:** 9 — Rollout-as-a-Service
- **Based on:** `rollout-architecture-v1.md`, ADR-010, T125/T130 gate patterns

## Objective

Close Phase 9 with structured QA evidence and Tech-Lead + Security validation gates
(PASS verdicts) covering T132–T137 rollout stack.

## Deliverables

- **`docs/artifacts/qa-phase9-report-v1.md`** — integration budgets + test mapping
- **`docs/artifacts/gate-phase9-feature-efg-2026-06-05.md`** — combined gate verdicts
- **`docs/artifacts/security-phase9-checklist-v1.md`** — OWASP-oriented sign-off
- **`docs/checkpoints/checkpoint-011-phase9-complete.md`** — phase boundary
- **`orchestrator/internal/rollout/integration_test.go`** — trainer e2e tests

## Acceptance Criteria

- [x] Trainer e2e flow tested (submit → reward → poll → callback)
- [x] Gate artifacts: Implementation **PASS**, Security **PASS**
- [x] `go test ./... -race` green
- [x] CI green on T138 MR (!47 → `5d2cfca`, squash `011d8c8`)

## Notes

- Full trainer fleet e2e with live sidecar deferred to ops validation.
- **T139** release readiness follows this gate.
