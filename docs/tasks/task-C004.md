# Task C004 — Reconcile the task ledger

**ID:** C004
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C004 row); docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md

## Objective

`docs/tasks/active-tasks.md` lists exactly one row (T010) while ~25 briefs in
`docs/tasks/` sit at `in_review` and ~4 at `pending`. The ledger must reflect reality:
every non-`done` brief gets a row with its actual status. This task makes the ledger
**honest** — it does not close, complete, or judge any task.

## Inputs

- `docs/tasks/active-tasks.md` (currently 1 row)
- `docs/tasks/completed-tasks.md` (the archive)
- Every `docs/tasks/task-T*.md` brief (read the `**Status:**` header of each)

## Rails (read before starting)

### You MUST
- Read the status header of **every** `task-T*.md` file in `docs/tasks/` (not `docs/archive/tasks/`)
- Rewrite the `active-tasks.md` table so it contains exactly one row per brief whose status is `pending`, `in_progress`, `blocked`, or `in_review` — with the status copied verbatim from the brief
- Preserve the existing T010 row's data
- Produce `docs/artifacts/task-ledger-reconciliation-v1.md`: a discrepancy report listing (a) briefs whose status header is missing or malformed, (b) briefs at `in_review` with no linked review artifact, (c) any brief whose status you could not determine
- Follow the ledger format exactly: `| ID | Title | Owner | Status | Priority | Depends on | Last update |`

### You MUST NOT
- Mark any task `done` or move any row to `completed-tasks.md` — terminal transitions are orchestrator-only, no exceptions
- Modify any task brief's status header — report discrepancies instead
- Guess a status: if a brief's status is ambiguous, list it as `blocked` in the ledger and record it in the discrepancy report
- Touch `docs/archive/tasks/*` — those are already archived history
- Add the new C-series tasks to the ledger (the orchestrator does that separately)

## File ownership

- **May create/modify:** `docs/tasks/active-tasks.md`, `docs/artifacts/task-ledger-reconciliation-v1.md` (new)
- **Must NOT touch:** `docs/tasks/task-*.md`, `docs/tasks/completed-tasks.md`, `docs/archive/*`

## Steps (execute in order)

1. List all `docs/tasks/task-T*.md` files.
2. Extract `**Status:**`, title, owner, priority, and depends-on from each brief's header.
3. Cross-check against `completed-tasks.md` — a brief at `done` that was never archived is a discrepancy (report it; do not archive it yourself).
4. Rewrite `active-tasks.md` with the full honest table.
5. Write the discrepancy report.
6. Run the verification commands.

## Expected outputs

- `docs/tasks/active-tasks.md` — row count equals the number of non-`done` briefs
- `docs/artifacts/task-ledger-reconciliation-v1.md` — discrepancy report

## Acceptance criteria

1. `active-tasks.md` row count = number of briefs with status `pending`/`in_progress`/`blocked`/`in_review`
2. Every ledger row's status matches its brief's status header verbatim
3. Discrepancy report exists and covers the three categories above (even if a category is empty)
4. No brief was modified; no row was archived

## Verification commands

```bash
grep -l "Status:\*\* done" docs/tasks/task-T*.md | wc -l
grep -h "Status:\*\*" docs/tasks/task-T*.md | grep -c "in_review\|pending\|in_progress\|blocked"
grep -c "^| T" docs/tasks/active-tasks.md   # must equal the previous count
git diff --stat docs/tasks/                  # only active-tasks.md changed
```

## Git rails

- Branch: `agent/technical-writer/C004` from `develop`
- Commit: `docs(tasks): reconcile active ledger with brief statuses`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
The expected outcome of this task is that the orchestrator and human decide
dispositions for the reconciled rows — your job ends at an honest ledger plus report.

## Execution notes

<filled during execution>
