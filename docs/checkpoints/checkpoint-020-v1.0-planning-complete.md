# Checkpoint 020 — v1.0 planning complete

## Phase summary

Planning phase for `plan-cwso-v1.0-roadmap.md` produced the full delegation layer:
7 phase plans and 39 task briefs (C001–C063) with explicit rails for cheap-model
execution, plus the C-series registered in the task ledger. No task has been
dispatched — the roadmap and all phase plans remain `proposed`, awaiting human approval.

## Completed tasks (this phase)

| ID | Title | Owner | Outcome |
|----|-------|-------|---------|
| — | 7 phase plans (Phase 0–6) | orchestrator | `docs/plans/plan-cwso-v1.0-phase{0..6}-*-v1.md` |
| — | 39 task briefs with rails | orchestrator | `docs/tasks/task-C001.md` … `task-C063.md` |
| — | C-series ledger registration | orchestrator | `docs/tasks/active-tasks.md` (39 new rows, all `pending`) |

## Open / carried over

| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|
| T010 | SE: Security audit (auth, secret leakage) | security-engineer | in_review | Closed by C061 in Phase 6 |
| C001–C063 | v1.0 roadmap tasks | various | pending | Awaiting roadmap approval |

## Key decisions

- **C-prefix task IDs** adopted per the roadmap proposal (avoids collision with the T### series spanning two repos).
- **Owner mapping** follows the permission classification in security-guidelines.md: devops-engineer (compose/CI/scripts), backend-developer (Go/Rust code), solution-architect (ADRs), qa-engineer (tests/verification), technical-writer (docs), security-engineer (audit, read-only).
- **C025 is conditional**: activates only on an ADR-012 NO-GO.
- **Hard ordering constraints**: C020/C021 (ADR before implementation), C030→C031→C032 (gap table before decision before execution), C052 ⇄ emage.code T403 (paired handover), emage.code T422 must not start before CG3 closes.

## Artifacts produced

- `docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md`
- `docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v1.md`
- `docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md`
- `docs/plans/plan-cwso-v1.0-phase3-protocol-conformance-v1.md`
- `docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md`
- `docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md`
- `docs/plans/plan-cwso-v1.0-phase6-release-v1.md`
- `docs/tasks/task-C001.md` … `task-C063.md` (39 briefs)
- `docs/tasks/active-tasks.md` (updated)

## Blockers (active)

| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|
| — | none | — | — | — | — |

## Token usage

| Phase | Budget | Spent | % |
|-------|--------|-------|---|
| Planning (this checkpoint) | 80k | ~75k | ~94% |
| Phase 0–6 (planned) | 1,360k | — | — |

## Next steps

- Phase: approval gate — human reviews the roadmap + phase plans (plan-approve-execute).
- On approval: dispatch Phase 0 (C001–C005, all parallel) and Phase 3's C030 (depends only on CG0).
- Open questions carried from the roadmap (need human answers before Phase 2):
  1. Filesystem projection (B2) in or out of v1.0? (Plans assume **in**, with C025 escape hatch.)
  2. Official MCP SDK vs hand-rolled + conformance suite? (C031 decides from the C030 gap table.)
  3. Read-write mount of the user's repository? (C015 assumes **yes** with documented rails.)
- Inputs to delegate forward: the relevant phase plan + task brief + this checkpoint.

## Compression note

This checkpoint is the canonical handoff for the next phase. Subsequent agents receive **only**: this checkpoint + their task brief + referenced artifact versions.
