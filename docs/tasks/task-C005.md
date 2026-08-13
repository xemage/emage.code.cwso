# Task C005 — Publish docs/SCOPE-v1.0.md

**ID:** C005
**Owner:** technical-writer
**Status:** pending
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (§1.5, §2.4); docs/plans/plan-cwso-v1.0-phase0-honest-baseline-v1.md

## Objective

Create `docs/SCOPE-v1.0.md`: the single, quotable statement of what v1.0 means and what
it explicitly excludes. Every later phase, review, and release decision cites this file
instead of re-litigating scope. It exists to stop scope drift — the roadmap's §1.4
pattern — from recurring.

## Inputs

- `docs/plans/plan-cwso-v1.0-roadmap.md` — §1.5 ("What v1.0 should mean") and §2.4 ("Explicitly not in v1.0")

## Rails (read before starting)

### You MUST
- Quote the roadmap §1.5 definition **verbatim** (the blockquote beginning "A developer with Docker and one supported MCP client…")
- Reproduce the §2.4 non-goals table **verbatim** (Deferred | Status | Re-entry)
- Add a short header: purpose (2–3 sentences), source link to the roadmap, and a rule that changing this file requires a new version of the roadmap plan
- Add one closing section: "How to use this file" — cite it in MR reviews when a change exceeds v1.0 scope

### You MUST NOT
- Editorialize, summarize, or "improve" the quoted sections — verbatim means verbatim
- Add new non-goals or remove existing ones
- Touch any other file

## File ownership

- **May create/modify:** `docs/SCOPE-v1.0.md` (new file only)
- **Must NOT touch:** everything else

## Steps (execute in order)

1. Copy §1.5 and §2.4 verbatim from the roadmap.
2. Add the header and "How to use this file" sections.
3. Verify the quoted text matches the roadmap character-for-character.

## Expected outputs

- `docs/SCOPE-v1.0.md`

## Acceptance criteria

1. §1.5 definition appears verbatim (diff against roadmap shows no changes inside the quote)
2. §2.4 table appears verbatim
3. File contains the change-control rule and usage section

## Verification commands

```bash
grep -c "A developer with Docker and one supported MCP client" docs/SCOPE-v1.0.md   # = 1
grep -c "Deferred to v1.1" docs/SCOPE-v1.0.md docs/plans/plan-cwso-v1.0-roadmap.md  # both present
git diff --stat   # exactly 1 new file
```

## Git rails

- Branch: `agent/technical-writer/C005` from `develop`
- Commit: `docs: publish v1.0 scope statement`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
