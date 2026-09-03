# Checkpoint 021 — v1.0 roadmap approved; decisions recorded

## Phase summary

The human approved `plan-cwso-v1.0-roadmap.md` on 2026-08-13 and answered the three
open questions. The approval was processed per plan-approve-execute: roadmap and all
phase plans advanced to `approved`, the three decisions recorded in the roadmap's
Approval section, affected briefs amended, one new task created (C019), and the ledger
updated. The plan is now in the Execute phase — the first dispatchable set is ready.

## Completed tasks (this phase)

| ID | Title | Owner | Outcome |
|----|-------|-------|---------|
| — | Roadmap approved + decisions recorded | orchestrator | `plan-cwso-v1.0-roadmap.md` Approval section |
| — | 6 phase plans approved in place | orchestrator | phase{0,2,3,4,5,6} v1 files |
| — | Phase 1 plan revised | orchestrator | `plan-cwso-v1.0-phase1-one-command-stack-v2.md` (v1 superseded) |
| — | C019 created (decision 3) | orchestrator | `docs/tasks/task-C019.md` |
| — | Briefs amended for decisions | orchestrator | task-C015, task-C020, task-C031, task-C032 |
| — | Ledger updated | orchestrator | `active-tasks.md` (40 C-rows) |

## Open / carried over

| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|
| T010 | SE: Security audit | security-engineer | in_review | Closed by C061 in Phase 6 |
| C001–C005 | Phase 0 (Honest Baseline) | devops/writer | pending | **Dispatchable now** — all parallel |
| C030 | MCP gap table | backend-developer | pending | **Dispatchable in parallel** (depends only on CG0) |
| C010–C063 | Remaining phases | various | pending | Gate-ordered per phase plans |

## Key decisions (human, 2026-08-13)

1. **Filesystem projection (B2) is IN v1.0.** ADR-012 (C020) selects the mechanism;
   C025 remains escape-hatch-only on proven infeasibility.
2. **Keep the hand-rolled MCP kernel and prove it.** ADR-013 (C031) documents the
   decision + scopes the conformance suite; C032 executes keep-and-prove; SDK recorded
   as considered-and-rejected.
3. **Read-write user-repo mount**, conditional on non-KVM sandbox trustworthiness →
   new task **C019** (Phase 1, P0); C015 depends on C019's evidence artifact.
   Phase 1 budget 200k → 240k (total 1,360k → 1,400k).

## Artifacts produced

- `docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md` (supersedes v1)
- `docs/tasks/task-C019.md`
- Amended: roadmap (Approval), 6 phase plans (status), task-C015/C020/C031/C032, `active-tasks.md`

## Blockers (active)

| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|
| — | none | — | — | — | — |

## Token usage

| Phase | Budget | Spent | % |
|-------|--------|-------|---|
| Planning + approval (checkpoints 020–021) | 80k | ~120k | ~150% (overrun: 40 briefs + approval round-trip) |
| Phase 0–6 (planned, amended) | 1,400k | — | — |

## Next steps

- Phase: Execute — dispatch the first unblocked set: **C001–C005** (parallel worktrees)
  and **C030** (parallel; depends only on CG0).
- Worktree per agent per `git-workflow.md`: `agent/<role>/<CID>` from `develop`.
- All planning + approval artifacts are uncommitted on branch `chore/harness-sync-2026-08-09`;
  land them via a dedicated branch + MR to `develop` before dispatch (protected-branch rules).
- Watch items: C019 NO-GO escalation path (decision 3 condition); C052 ⇄ emage.code
  T403 handover timing; emage.code T422 must not start before CG3 closes.
- Inputs to delegate forward: the relevant phase plan + task brief + this checkpoint.

## Compression note

This checkpoint is the canonical handoff for the next phase. Subsequent agents receive **only**: this checkpoint + their task brief + referenced artifact versions.
