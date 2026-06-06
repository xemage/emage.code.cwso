# Task T139 — v0.3.0 release readiness (Phases 6–9)

> **ID note:** roadmap **placeholder T113**. Active **T139** (see `active-tasks.md`).

- **Status:** done
- **Owner:** release-manager / technical-writer
- **Priority:** P0
- **Depends on:** T138 (Phase 9 QA + security gate)
- **Phase:** 9 closure — Next-Gen release packaging
- **Based on:** `plan-cwso-nextgen-phase6plus.md`, checkpoints 007–011, gate artifacts Phases 6–9

## Objective

Package Phases 6–9 (Features A–G) as **v0.3.0-rc1**: consolidated changelog, release readiness
artifact, plan status update, and task-board reconciliation after T138 merge.

## Deliverables

- **`docs/artifacts/release-v0.3.0-rc1.md`** — release readiness for Phases 6–9
- **`CHANGELOG.md`** — `v0.3.0-rc1 - 2026-06-06` section (Phase 6–9 highlights)
- **`docs/plans/plan-cwso-nextgen-phase6plus.md`** — Phase 9 marked complete
- **`docs/tasks/active-tasks.md`** — T138 done, T139 in_review
- **`docs/tasks/task-T139.md`** — this brief

## Acceptance Criteria

- [x] T138 merge recorded (`!47` → `5d2cfca`)
- [x] Release artifact references gate PASS/PASS for Phases 6–9
- [x] Changelog covers Features A–G at RC level
- [x] Plan status reflects Phase 9 complete
- [x] CI green on T139 MR !48 (pipeline #2581160040)
- [x] Merged !48 → `d693c3f`; tagged `v0.3.0-rc1`

## Notes

- RC only — GA promotion (`v0.3.0`) requires stakeholder validation and audit hardening (T094 `allow_failure`).
- Deferred items: T135 KV prefix router, trainer fleet benchmark, orchestrator chat stub.
