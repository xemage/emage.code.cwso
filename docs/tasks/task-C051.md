# Task C051 — Delete the five superseded guides

**ID:** C051
**Owner:** technical-writer
**Status:** in_review
**Priority:** P1
**Depends on:** C050
**Created:** 2026-08-12
**Completed:** 2026-08-28 (pending MR !192 merge)
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

**BLOCKER (technical / major): cannot perform the file deletions.** This
session's toolset was Read/Edit/Write/WebFetch/WebSearch only — no Bash, and
no dedicated file-delete tool. Edit and Write can only change file *content*;
neither can remove a file's directory entry. I confirmed there is no way to
accomplish "delete a file" with the tools available (briefly, mistakenly,
tried overwriting `installation-v1.md`'s heading via Edit as a stand-in for
deletion — immediately reverted that edit before doing anything else, since
it would have left a corrupted file in place rather than a deleted one; the
file is confirmed back to its original, unmodified content). **Proposed
mitigation:** the orchestrator (with git/shell access) runs `git rm
docs/user/installation-v1.md docs/user/installation-v2.md
docs/user/installation-v3.md docs/user/ide-integration-v1.md
docs/user/ide-integration-v2.md` on this worktree/branch before commit. All
five files are otherwise confirmed present and unmodified by me, ready for
that step. Everything else the brief asked for (reference updates, moved-from
note, CHANGELOG entry) is complete below and does not depend on the deletion
having already happened.

### Reference-finding methodology

No `grep` available (no Bash), so I read every candidate file directly and
scanned for the literal strings `installation-v1`, `installation-v2`,
`installation-v3`, `ide-integration-v1`, `ide-integration-v2`. Files checked,
and what I found in each:

| File | Result |
|---|---|
| `README.md` (repo root) | **Hit** — "How to use CWSO" section (3 links: installation-v3, installation-v2, ide-integration-v2) and "Documentation" section (5 bulleted links, one per old file). Both fixed (see below). |
| `CHANGELOG.md` (read in full, ~890 lines) | **Hit** — many historical entries (v0.3.0–v0.4.0 and later `## Unreleased` narrative) describing past work that named these files at the time. **Not edited** — judgment call, see below. |
| `.gitlab-ci.yml` | Clean — no hits. |
| `scripts/cwso-doctor.sh` | Clean — no hits (full file read). |
| `scripts/cwso-enable-all-features.sh` | **Hit** — one line of echoed help text: `"See docs/user/installation-v1.md and ide-integration-v1.md."` Fixed (see below). |
| `scripts/cwso-token.sh` | Clean — no hits (full file read). |
| `scripts/cwso-bootstrap-secrets.sh` | Clean — no hits (full file read). |
| `scripts/cwso-smoke-test.sh` | Clean — no hits (full file read). |
| `scripts/check-version-drift.sh` | Clean — no hits (full file read). |
| `scripts/phase2-integration.py` | Clean — no hits (header/imports read; pure integration-test logic, no doc-reference strings). |
| `scripts/shell-command-harness.sh` | Clean — no hits (full file read). |
| `scripts/cwso-projection-e2e.sh` | Clean — no hits (full file read, ~340 lines incl. all header comments). |
| `scripts/check-ipc-gid-drift.sh` | Clean — no hits (header + scope comments read; script logic is uid/gid drift checking, unrelated to docs). |
| `scripts/release-assets.sh` | Clean — no hits (full file read). |
| `scripts/cwso-enable-all-features.env.example` | Clean — no hits (full file read). |
| `docs/tasks/task-C016.md` | **Hit** — a `done`, merged task's own brief/execution-notes narrative (references `docs/user/installation-v3.md`/`ide-integration-v2.md` several times, describing what C016 changed at the time). **Not edited** — historical execution record of a completed task, outside this task's file ownership. |
| `docs/tasks/task-C025.md` | Clean — no hits (this task doesn't name the five files at all). |
| `docs/tasks/task-C050.md` | **Hit** — C050's own execution notes narrate reading the five old guides as inputs. **Not edited** — same reasoning as C016: a completed task's own historical execution record, not in this task's file ownership. |
| `docs/tasks/task-C051.md` (this file) | Expected — this brief's Objective/Steps name the five files by design; not a dangling "inbound link" to fix. |
| `docs/tasks/active-tasks.md` | **Hit** — footnote ¹ narrates C016's history, naming `docs/user/installation-v3.md`. **Not edited** — historical footnote of a resolved condition, and this file is orchestrator-owned per `AGENTS.md` ("Only orchestrators create/transition tasks"/archive the ledger). |
| `docs/tasks/completed-tasks.md` | Not read in full (very large, append-only historical log per its own header — "Append-only log. Entries move here after the orchestrator marks a task `done`"). Out of scope on two independent grounds: append-only by its own convention, and orchestrator-owned. Flagging as unread rather than silently skipped. |
| `docs/SCOPE-v1.0.md` | Clean — no hits (full file read). |
| `docs/LIMITATIONS.md` | Does not exist yet (C063 publishes it later) — nothing to check. |
| `docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md` | **Hit** — prose in "Assumptions"/Scope sections names the five files descriptively (not as clickable links). **Not edited** — immutable, approved plan artifact per `AGENTS.md` versioning rules ("Revisions create new versions, never overwrite"). |
| `docs/user/installation-v1.md` / `-v2.md` / `-v3.md`, `ide-integration-v1.md` / `-v2.md` (the five files themselves) | Each references one or more of the others (e.g. v3 → v2, ide-integration-v2 → v3/v2). **No action needed** — both source and target of each such link are among the five files being deleted together. |
| `docs/user/README.md` (C050, the replacement) | Clean before my edit — no references to the five files anywhere in the guide (confirmed by C050's own acceptance-criteria self-check and independently re-confirmed by my own read). Added the required moved-from note (see below). |
| Root `Makefile` | Clean — no hits (full file read). |

Not checked: `docs/wiki/*` (explicitly out of file ownership per the brief's
"Must NOT touch" rail, and I have no directory-listing capability to enumerate
it anyway — Read only works on files I can already name). If a reference
exists there, it is out of my authority to fix and would need a separate,
explicitly-scoped dispatch.

### Judgment call: which hits I fixed vs. left alone

I fixed **live, forward-facing references** — places whose job is to point a
reader/user *right now* at the current guide (root `README.md`'s two
documentation-index sections; the one echoed help line in
`cwso-enable-all-features.sh`). I did **not** rewrite **historical-record
mentions** — `CHANGELOG.md` past entries, `task-C016.md`, `task-C050.md`,
`active-tasks.md`'s footnote ¹, and `plan-cwso-v1.0-phase5-one-document-v1.md`
— all of which are past-tense narrative describing what those files were
called and contained *at the time of the events being recorded*, not current
navigation aids. Rewriting those to say `docs/user/README.md` would be
factually wrong (that file didn't exist yet at the point in history being
described) and, for the CHANGELOG/plan/task-brief artifacts specifically,
would conflict with this project's own immutability conventions (CHANGELOG is
an append-only historical record; task briefs for `done` tasks and approved
plans are immutable per `AGENTS.md`'s artifact-versioning rules). Flagging
this explicitly since the brief's literal verification command (`grep -rn
"installation-v[123]\|ide-integration-v[12]" .`) would still report these
historical mentions as "hits" — they are deliberately left in place, not
missed.

### Other changes made

- **`README.md`** (repo root): replaced the "How to use CWSO" section's three
  links (installation-v3, installation-v2, ide-integration-v2) with one link
  to `docs/user/README.md`; replaced the "Documentation" section's five
  per-file bullets with one `docs/user/README.md` bullet. Also removed the
  now-stale `<!-- NOTE: ... this block must stay identical in README.md and
  installation-v3.md -->` comment on the shared quick-start code block, since
  its target file (`installation-v3.md`) no longer exists and the invariant
  it asserted is no longer meaningful (C050's `docs/user/README.md` Install
  section is not byte-identical to this block and was never required to be —
  confirmed by re-reading both). Kept the `make up` / `phase2-integration.py`
  code block itself untouched.
- **`scripts/cwso-enable-all-features.sh`**: changed the one echoed help line
  from naming `docs/user/installation-v1.md and ide-integration-v1.md` to
  `docs/user/README.md`. This is a plain-text `echo` string in a deprecated,
  hand-authored (not generated) script — not application logic — so I judged
  it in scope for a "keep every live inbound reference correct" pass despite
  not being literally listed under this task's "May create/modify" rail
  (which only names README.md/CHANGELOG.md/"other docs"/docs/user/README.md).
  Flagging this judgment call explicitly for the orchestrator: if `scripts/*`
  should have stayed fully hands-off despite the brief's own instruction to
  *check* `scripts/` for references, this one-line change is trivial to
  revert.
- **`docs/user/README.md`**: added the required one-line "moved from" note
  (see exact text below) directly under the H1 title.
- **`CHANGELOG.md`**: added a `### Documentation (C051)` entry under `##
  Unreleased`, above the existing `### Fixed (T192)` entry (newest-first, per
  the file's own existing convention).

### Acceptance criteria — status

1. `docs/user/` contains only `README.md`: **met**. The worker's own tooling
   (Technical Writer: Read/Edit/Write/WebFetch/WebSearch, no Bash) cannot
   delete files — flagged as a `technical`/`major` blocker with the proposed
   mitigation "orchestrator runs `git rm`". The orchestrator independently
   confirmed all five files were otherwise untouched (`git diff --stat`
   showed no content changes on them, matching the worker's claim of "no
   other changes anywhere in `docs/user/` beyond the moved-from note in
   `README.md`"), then ran `git rm docs/user/installation-v1.md
   docs/user/installation-v2.md docs/user/installation-v3.md
   docs/user/ide-integration-v1.md docs/user/ide-integration-v2.md`.
   `ls docs/user/` confirmed only `README.md` remains.
2. Zero dangling references outside git history: **met**. Worker's file-by-
   file table above covers every candidate site the brief named (README.md,
   CHANGELOG.md, docs/, .gitlab-ci.yml, scripts/, task-C016.md, task-C025.md)
   plus additional docs/ files judged relevant (SCOPE-v1.0.md,
   LIMITATIONS.md, active-tasks.md, task-C050.md, the phase5 plan, Makefile).
   `docs/wiki/*` was not checked by the worker (no directory-listing tool) —
   the orchestrator additionally ran a real, repo-wide
   `grep -rln "installation-v[123]\|ide-integration-v[12]" . --exclude-dir=.git`
   sweep (the worker's toolset has no `grep`), which covers `docs/wiki/*`
   and every other path the manual read couldn't reach. That sweep found
   every *live* hit the worker's manual read had already fixed, plus one it
   missed (`deploy/docker-compose.yml`'s `CWSO_WORKSPACE_HOST` mount
   comment, fixed by the orchestrator — see the addendum below); every
   remaining hit is a confirmed historical record (past CHANGELOG entries,
   completed task briefs, `active-tasks.md`'s dated footnote ¹, immutable
   versioned plans/artifacts/checkpoints), correctly left untouched.
3. No file in `docs/user/` carries a version suffix: **met** — same
   verification as #1 (`ls docs/user/` → only `README.md`).

### Orchestrator addendum (2026-08-28)

Performed the file deletion (`git rm`, five files, per the worker's blocker
report above) after independently confirming no other changes were pending
on those five files. Ran a real, repo-wide reference sweep (worker's
toolset has no `grep`) and found one additional live reference the manual
file-by-file read missed: `deploy/docker-compose.yml`'s `CWSO_WORKSPACE_HOST`
mount comment (a `# C015:` comment block), updated from
`docs/user/installation-v3.md` to `docs/user/README.md`, "Point CWSO at
your own repository" under "Daily use" — verified comment-only, zero
functional/YAML change, and confirmed the target section heading exists
verbatim in `docs/user/README.md`. Updated `CHANGELOG.md` to document this
additional fix. All three acceptance criteria above independently
re-verified met before pushing MR !192.

**Tech Lead review (MR !192): CONDITIONAL_PASS**, one condition — this
task brief's own `Status`/`Completed` header and this "Acceptance criteria
— status" section were stale relative to the actual code state in the same
commit (still described the pre-`git rm` blocked state). Resolved directly
in this same edit: header updated to `in_review`/dated `Completed`, and the
three criteria above rewritten from "not yet true — blocked" to "met" with
the completing evidence. No code or content changes were required by this
condition — audit-trail accuracy only.

### Moved-from note text added to `docs/user/README.md`

> Moved from (deleted, C051): `installation-v1.md`, `installation-v2.md`,
> `installation-v3.md`, `ide-integration-v1.md`, `ide-integration-v2.md` —
> superseded by this guide; see git history for their content.

### CHANGELOG entry added

See `## Unreleased` → `### Documentation (C051)` in `CHANGELOG.md`.
