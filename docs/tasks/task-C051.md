# Task C051 — Delete the five superseded guides

**ID:** C051
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C050
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B8); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Delete `installation-v1.md`, `installation-v2.md`, `installation-v3.md`,
`ide-integration-v1.md`, and `ide-integration-v2.md` from `docs/user/`. **Delete, not
archive** — the emage.code audit showed archived docs still surface in searches. Git
history preserves them.

## Inputs

- `docs/user/README.md` (C050 — the replacement, must be merged first)
- The five files to delete
- Repo-wide references to the deleted files

## Rails (read before starting)

### You MUST
- Delete exactly these five files: `docs/user/installation-v1.md`, `docs/user/installation-v2.md`, `docs/user/installation-v3.md`, `docs/user/ide-integration-v1.md`, `docs/user/ide-integration-v2.md`
- Grep the whole repo for references to the deleted filenames and update every inbound link to point at `docs/user/README.md` (check: README.md, CHANGELOG, docs/, .gitlab-ci.yml, scripts/)
- Add a "moved from" note in `docs/user/README.md` listing the five superseded files (one line)
- Add a CHANGELOG entry

### You MUST NOT
- Delete anything else in `docs/user/` (the wiki stays)
- Archive copies anywhere (no `docs/archive/user/` — git history is the archive)
- Delete before C050 is merged — a user must never land on an empty docs/user/
- Modify the deleted files' git history (no filter-branch, no rebase)

## File ownership

- **May create/modify:** delete the 5 named files; modify inbound links in `README.md`, `CHANGELOG.md`, other docs, `docs/user/README.md` (moved-from note)
- **Must NOT touch:** code, `docs/wiki/*`, `docs/archive/*`

## Steps (execute in order)

1. Confirm C050 is merged.
2. Grep for all inbound references to the five filenames.
3. Delete the five files.
4. Update every inbound link to `docs/user/README.md`.
5. Add the moved-from note + CHANGELOG.
6. Verify zero dangling references.

## Expected outputs

- Five deletions + updated inbound links
- Moved-from note in the single guide
- CHANGELOG entry

## Acceptance criteria

1. `docs/user/` contains only `README.md` (and the pre-existing wiki content, untouched)
2. `grep -rn "installation-v[123]\|ide-integration-v[12]" . --exclude-dir=.git` returns zero hits outside git history
3. No file in `docs/user/` carries a version suffix

## Verification commands

```bash
ls docs/user/
grep -rn "installation-v[123]\|ide-integration-v[12]" . --exclude-dir=.git | wc -l   # = 0
git log --oneline -3 -- docs/user/   # history preserved
```

## Git rails

- Branch: `agent/technical-writer/C051` from `develop` (rebased on merged C050)
- Commit: `docs(user): delete superseded installation and IDE guides`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If an inbound reference is found in a generated file, do not edit the generated file —
report `dependency` / `minor` naming the generator.

## Execution notes

<filled during execution>
