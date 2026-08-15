# Task Ledger Reconciliation — v1

**Task:** C004 (technical-writer)
**Date:** 2026-08-13
**Scope:** Reconciled `docs/tasks/active-tasks.md` against the `**Status:**` header of every
`docs/tasks/task-T*.md` brief (88 files). `docs/archive/tasks/*` was not touched, per rails.
This task makes the ledger **honest** — it does not close, complete, archive, or judge any task.

## Method

- Read the `**Status:**` header of all 88 `task-T*.md` briefs.
- Classified each brief: 48 `done`, 40 non-`done` (`in_review`/`pending`/`in_progress`/`blocked`).
- Rewrote `active-tasks.md` so it has exactly one row per non-`done` brief, status copied verbatim.
- Preserved the pre-existing T010 row and all 40 C-series rows unchanged.
- Cross-checked every brief against `completed-tasks.md`.

Result: 40 T-rows added to `active-tasks.md` (T010 preserved). The active ledger now has
**41 T-rows** (1 pre-existing + 40 added) and **81 rows total** (41 T + 40 C).

## (a) Briefs with missing or malformed status headers

**None.** All 88 `task-T*.md` briefs carry a parseable `**Status:**` header.

## (b) `in_review` briefs with no linked review artifact

An `in_review` status implies the work is awaiting/under review, so a linked review artifact
(gate report, tech-lead review, QA verdict, etc.) is expected. The following `in_review` briefs
reference **no** `docs/artifacts/*` file at all (no review artifact, and no artifact of any kind):

| Brief | Title |
|-------|-------|
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) |
| T145 | Rollout `num_samples` session fan-out |
| T146 | Gateway async staging + partial trace recovery |
| T148 | Evaluator registry + SWE-bench hook |
| T154 | IDE integration guide (VS Code / Cursor) |
| T155 | Enable-all-features script |

Note: the other 20 `in_review` briefs (T082–T094, T115–T122) do reference `docs/artifacts/*`
files, but those references are **input/blueprint/gate** documents (e.g.
`cwso-nextgen-blueprint-v1.md`, `gate-phase6-feature-a-2026-06-02.md`,
`wasm-sparse-agent-design-v1.md`) cited under "Based on" — **not** review/verdict artifacts for
the brief itself. If the acceptance bar requires a dedicated *review* artifact (tech-lead / QA /
security verdict) per `in_review` brief, then **all 26** `in_review` briefs lack one; only the 6
above lack even a generic artifact link. This distinction is reported, not adjudicated, per rails.

## (c) Briefs whose status could not be determined

**None.** Every brief's status was determinable from its header. No brief was listed as
`blocked` on account of ambiguity.

## Additional discrepancy: ledger overlap (brief vs completed-tasks.md)

A cross-check surfaced a **significant pre-existing inconsistency** that is outside this task's
authority to fix (terminal transitions and archival are orchestrator-only), so it is **reported
here, not acted on**:

- **All 40 non-`done` briefs also appear as rows in `completed-tasks.md`** (T082–T094, T114,
  T115–T122, T145–T148, T154, T155, T165–T168, T180–T189). These briefs carry live statuses
  (`in_review` / `pending`) yet were previously written into the completed ledger.
  - The `completed-tasks.md` rows for T082–T122/T145–T155 are "Migrated from active-tasks.md
    board cleanup" entries (dated 2026-06-02 … 2026-06-09) that assert completion, but the briefs
    were later flipped back to `in_review` without removing the completed rows.
  - T165–T168 are recorded in `completed-tasks.md` as "**Ledger-hygiene retroactive archival**"
    entries (the work was done and released as v0.5.0), yet their briefs still read `pending`.
  - T180–T189 are recorded in `completed-tasks.md` (dated 2026-08-08) with concrete
    implementation evidence, yet their briefs still read `pending`.
- Conversely, **no `done` brief is missing from `completed-tasks.md`** (all 48 `done` briefs are
  archived) — so there is no un-archived `done` brief to report under Step 3.

This active/completed overlap, and the brief-status-vs-completed-row contradiction, is a
disposition decision for the orchestrator and human. This task did **not** modify any brief, did
**not** move any row to `completed-tasks.md`, and did **not** mark anything `done`.
