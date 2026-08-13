# Task C060 — Debt register: zero unclassified rows

**ID:** C060
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** C050–C054 (gate CG4)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C060 row); docs/plans/plan-cwso-v1.0-phase6-release-v1.md

## Objective

Full debt-register review before release: every row in `docs/DEBT-REGISTER.md`
reclassified `fixed` / `documented-limitation` / `v1.1`. **No row may remain
unclassified.** This is the release's honesty check.

## Inputs

- `docs/DEBT-REGISTER.md` (C003, kept current by every debt-closing task since)
- `docs/LIMITATIONS.md` (C063 — cross-check target)
- The closing tasks' evidence (C021, C032, C040–C044 MRs)

## Rails (read before starting)

### You MUST
- Re-verify every `fixed` row against the code (the marker is gone, the test exists) — do not trust the register's own claim
- Reclassify every remaining row: `fixed` (verified), `documented-limitation` (has a LIMITATIONS.md entry), or `v1.1` (explicitly deferred)
- Enforce the cross-check: any row marked `documented-limitation` MUST have a corresponding `docs/LIMITATIONS.md` entry — if it doesn't, either add the limitation (coordinate with C063) or reclassify
- Produce a summary header in the register: counts per classification, and a plain statement that zero rows are unclassified

### You MUST NOT
- Mark a row `fixed` without re-verifying in code
- Use `documented-limitation` to avoid work — the cross-check exists to catch exactly that
- Reclassify a v1.0-blocker as `v1.1` without orchestrator + human sign-off (that is a scope change — cite SCOPE-v1.0.md)
- Modify code (verification is read-only)

## File ownership

- **May create/modify:** `docs/DEBT-REGISTER.md`
- **Must NOT touch:** code, `docs/LIMITATIONS.md` (C063 owns it — coordinate)

## Steps (execute in order)

1. Read the full register.
2. Re-verify each `fixed` row in code.
3. Reclassify every remaining row.
4. Run the limitation cross-check.
5. Write the summary header.

## Expected outputs

- `docs/DEBT-REGISTER.md` with zero unclassified rows + summary header

## Acceptance criteria

1. Every row is `fixed` / `documented-limitation` / `v1.1` — no blanks, no `unclear`
2. Every `documented-limitation` row has a LIMITATIONS.md entry
3. Summary header with counts present

## Verification commands

```bash
grep -c "unclear\|TBD\|—$" docs/DEBT-REGISTER.md   # = 0 unclassified
grep "documented-limitation" docs/DEBT-REGISTER.md | wc -l
grep -c "." docs/LIMITATIONS.md   # cross-check entries exist
```

## Git rails

- Branch: `agent/technical-writer/C060` from `develop`
- Commit: `docs: reclassify all debt register rows for v1.0.0`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A row that cannot be verified is not `fixed` — report `unclear_requirements` / `major`.

## Execution notes

<filled during execution>
