# Task T180 — Close Resolved Debt Rows In Register

**ID:** T180
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-08
**Based on:** docs/plans/plan-TD-remediation-v1.md

## Objective

Close already-fixed debt items TD-05, TD-06, and TD-08 in `TECHNICAL-DEBT.md` without changing other active debt rows.

## Inputs

- `TECHNICAL-DEBT.md`
- `orchestrator/internal/jobs/manager.go`
- `orchestrator/internal/jobs/manager_test.go`

## Constraints

- Edit only `TECHNICAL-DEBT.md`.
- Do not change wording for unrelated TD rows.
- Keep markdown table formatting valid.

## Steps

1. Open `TECHNICAL-DEBT.md`.
2. Remove rows `TD-05`, `TD-06`, and `TD-08` from the active debt table.
3. Add a new section `## Closed Items` above the legend.
4. Add a table with columns: `ID | Resolved by | Resolution summary | Closed on`.
5. Add rows:
   - `TD-05 | T162 — orchestrator/internal/jobs/manager.go | publish() logs publish failures at Debug level | 2026-08-08`
   - `TD-06 | T162 — orchestrator/internal/jobs/manager.go | Close() drains queued jobs and publishes cancelled transitions | 2026-08-08`
   - `TD-08 | T162 — orchestrator/internal/jobs/manager.go | publishTransition() redacts sensitive error text via sanitizeErrorForBroadcast() | 2026-08-08`
6. Save file.

## Verification

Run:
```bash
cd /home/emage/Code/emage/CWSO
rg "TD-05|TD-06|TD-08" TECHNICAL-DEBT.md
```
Expected:
- Items appear in `## Closed Items` only.
- They no longer appear in the active debt table.

## Acceptance Criteria

1. Active table excludes TD-05/06/08.
2. `## Closed Items` section exists with the 3 rows.
3. No other debt row content changed.

## Blocker Protocol

If blocked, report: blocker type, severity, exact failing step, and one mitigation attempt.