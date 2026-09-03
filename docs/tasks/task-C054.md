# Task C054 — Verify every command in the guide on a clean machine

**ID:** C054
**Owner:** qa-engineer
**Status:** pending
**Priority:** P1
**Depends on:** C050, C051, C052, C053
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C054 row); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Execute **every command** in `docs/user/README.md` on a clean machine (or a clean VM /
container / fresh user account), top to bottom, exactly as written. A command that has
not been run is a claim, not a document. Attach the verification log to the MR.

## Inputs

- `docs/user/README.md` (the single guide, post-C050/C052/C053)
- A clean environment (fresh clone, no `.env.jwt.dev`, no cwso containers/images)

## Rails (read before starting)

### You MUST
- Extract every command block from the guide and execute them in order, as a new user would
- Use a genuinely clean environment: fresh `git clone`, no pre-existing `.env.jwt.dev`, `docker system prune -f` for cwso images first
- Record a verification log: command → result (pass/fail + output excerpt) for every command
- Attach the log to the MR and note any command that failed (a single failure blocks CG4 — the guide gets fixed, not the test)
- Verify the guide's troubleshoot section by triggering one deliberate failure (e.g., port occupied) and following the guide's own remedy

### You MUST NOT
- Fix the guide yourself — a failing command is a finding; file it for the orchestrator/technical-writer
- Skip "obvious" commands (prerequisites included)
- Use your shell history, aliases, or pre-existing state — clean env only
- Modify code or the guide

## File ownership

- **May create/modify:** `docs/artifacts/user-guide-verification-v1.md` (the log)
- **Must NOT touch:** the guide, code, scripts

## Steps (execute in order)

1. Build the clean environment.
2. Extract and execute every command in order; log results.
3. Trigger one deliberate failure and follow the guide's remedy.
4. Write the verification log artifact; attach to MR.

## Expected outputs

- `docs/artifacts/user-guide-verification-v1.md` — complete command-by-command log

## Acceptance criteria

1. Every command in the guide executed on a clean host, logged
2. Zero unexplained failures (any failure is filed, not ignored)
3. Troubleshoot section validated via one deliberate failure
4. Log attached to the MR

## Verification commands

```bash
grep -c "PASS\|FAIL" docs/artifacts/user-guide-verification-v1.md
grep -c "FAIL" docs/artifacts/user-guide-verification-v1.md   # = 0 (or each FAIL is filed)
```

## Git rails

- Branch: `agent/qa-engineer/C054` from `develop`
- Commit: `test: verify user guide commands on a clean host`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A failing guide command is `technical` / `critical` against the guide — file it; do
not work around it and call the guide verified.

## Execution notes

<filled during execution>
