# Task C053 — Contributor vs user documentation separation

**ID:** C053
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C050
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C053 row); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Contributor docs (`CONTRIBUTING.md`, build, branching, debt register) stay strictly
separate from user docs. One cross-link each way, no more. A user reading the guide
should never wade through contributor process; a contributor should find process docs
without hunting through the user guide.

## Inputs

- `docs/user/README.md` (C050)
- `docs/branching.md`, `docs/DEBT-REGISTER.md` (C003), `SECURITY.md`, existing contributor-facing material
- Whether a `CONTRIBUTING.md` exists at repo root (check; create if missing)

## Rails (read before starting)

### You MUST
- Ensure a root `CONTRIBUTING.md` exists covering: build, branching (link `docs/branching.md`), task process (link `docs/tasks/`), debt register (link `docs/DEBT-REGISTER.md`), and the docs-vs-code layout
- Keep exactly one cross-link each way: user guide → CONTRIBUTING ("contributing"), CONTRIBUTING → user guide ("using CWSO")
- Move any contributor content that leaked into `docs/user/` out to the contributor side
- Add a CHANGELOG entry

### You MUST NOT
- Duplicate content between the two sides — link, don't copy
- Move user content into contributor docs
- Touch code or the five deleted guides' replacements beyond the cross-links
- Create new doc sprawl: contributor docs live at root + `docs/` (not a new `docs/dev/` tree unless one already exists)

## File ownership

- **May create/modify:** `CONTRIBUTING.md` (root), `docs/user/README.md` (one cross-link), `CHANGELOG.md`
- **Must NOT touch:** code, `docs/user/deployment/*` (C052 owns it)

## Steps (execute in order)

1. Inventory contributor-facing material and where it lives.
2. Create/update root `CONTRIBUTING.md`.
3. Add the single cross-link each way.
4. Move any leaked contributor content out of `docs/user/`.
5. CHANGELOG.

## Expected outputs

- Root `CONTRIBUTING.md`
- Exactly one cross-link each way
- CHANGELOG entry

## Acceptance criteria

1. `CONTRIBUTING.md` exists and covers the five topics above
2. Exactly one cross-link each direction (grep-verified)
3. No contributor process content inside `docs/user/README.md`

## Verification commands

```bash
test -f CONTRIBUTING.md && echo "PASS: exists"
grep -c "CONTRIBUTING" docs/user/README.md   # = 1
grep -c "docs/user/README.md" CONTRIBUTING.md   # = 1
```

## Git rails

- Branch: `agent/technical-writer/C053` from `develop` (rebased on merged C050)
- Commit: `docs: separate contributor and user documentation`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

<filled during execution>
