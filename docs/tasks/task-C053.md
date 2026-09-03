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

**CONTRIBUTING.md — did not exist before this task.** Created fresh at repo
root.

**Inventory of contributor-facing material and where each topic's link now
points:**

- Build/test: no dedicated contributor doc existed for this; sourced
  directly from `Makefile` (`make build`, `make test`, `make lint`, `make
  fmt`, `make help`, plus the Docker-first "no local toolchain" fact already
  stated in `README.md`'s Quick start section). `CONTRIBUTING.md` § "Build &
  test" links `Makefile`.
- Branching: `docs/branching.md` already existed (GitFlow layout, branch
  naming, merge/commit policy) and cross-references
  `.github/instructions/git-workflow.instructions.md` (the fuller git-workflow
  doc, itself sourced from `.claude/rules/git-workflow.md` via the sync
  projection). `CONTRIBUTING.md` § "Branching & merge requests" links both,
  plus `.gitlab/issue_templates/` (confirmed present) for issue filing.
- Task process: `docs/tasks/active-tasks.md` and
  `docs/tasks/completed-tasks.md` both existed and were read to confirm
  their column conventions; `AGENTS.md` (root) documents the full task
  lifecycle/ID/delegation protocol. `CONTRIBUTING.md` § "Task process" links
  all three plus the `docs/tasks/task-<ID>.md` brief convention.
- Debt register: `docs/DEBT-REGISTER.md` existed (from C003, confirmed live
  and current). `CONTRIBUTING.md` § "Debt register" links it and restates
  the `POC-DEBT` inline-tag convention already documented there.
- Docs-vs-code layout: no single existing doc stated this explicitly;
  synthesized from `README.md`'s "Layout" section plus direct observation of
  the repo tree (`docs/user/` for user docs; root + `docs/{branching.md,
  DEBT-REGISTER.md, tasks/, decisions/, artifacts/, checkpoints/}` for
  contributor docs; `orchestrator/`, `services/`, `schemas/`, `deploy/`,
  `scripts/` for code). Confirmed no `docs/dev/` directory exists (a `Read`
  against `docs/dev/README.md` returned "File does not exist") — per the
  rails, none was created; contributor docs stay at root + `docs/`.
- Also linked: `SECURITY.md` (existed, read in full) and
  `.github/instructions/coding-standards.instructions.md` under a
  "Security" section — not one of the five required topics, but directly
  relevant contributor material referenced from `AGENTS.md`; added as a
  bonus link, not in place of any required topic.

**Cross-link text added (exactly one each way):**

- `docs/user/README.md` → `CONTRIBUTING.md`: added one sentence at the end
  of the file (next to the existing link to the root `README.md`):
  `See [contributing](../../CONTRIBUTING.md).` — anchor text "contributing".
- `CONTRIBUTING.md` → `docs/user/README.md`: one sentence inside the "Docs
  vs. code layout" section: `...in the single guide to
  [using CWSO](docs/user/README.md).` — anchor text "using CWSO".

Verified by reading both files in full after editing: `docs/user/README.md`
contains the literal string `CONTRIBUTING` exactly once (inside the new
link's `../../CONTRIBUTING.md` path); `CONTRIBUTING.md` contains the literal
string `docs/user/README.md` exactly once (inside the new link's target).
An earlier draft of `CONTRIBUTING.md` briefly had two references to
`docs/user/README.md` (one in the intro paragraph, one in "Docs vs. code
layout") — caught and fixed before finalizing, down to the single required
link.

**Leaked contributor content in `docs/user/README.md`:** none found.
Read the file in full (269 lines, prerequisites → install → configure
MCP client → verify → daily use → troubleshoot → known limitations). It
contains no references to task briefs, `docs/tasks/`, the debt register's
disposition mechanics, or contributor-facing branching/worktree
conventions — it is a user-only guide throughout. No content was moved out
of it; only the one cross-link sentence was added.

**File ownership respected:** only `CONTRIBUTING.md` (created),
`docs/user/README.md` (one added sentence), `CHANGELOG.md` (one new entry
under `## Unreleased`), and this task brief's own execution notes were
touched. `README.md` (root), `docs/user/deployment/*`, and all code were
left untouched, per the rails.

**Verification commands from the brief:** not run directly (Technical
Writer has no Bash access in this task's tooling configuration) — verified
by reading the files instead: `CONTRIBUTING.md` exists at repo root (`Read`
succeeded); the "CONTRIBUTING" / "docs/user/README.md" grep-count
equivalents were confirmed at 1 each by full-file reads as described above.
The orchestrator should independently re-run the literal grep commands
before merge.

**No blockers.**
