# Task C052 — Receive emage.code deployment docs (T403 ⇄ C052)

**ID:** C052
**Owner:** technical-writer
**Status:** pending
**Priority:** P1
**Depends on:** C050; emage.code T403 (paired handover)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C052 row, §2.7); docs/plans/plan-cwso-v1.0-phase5-one-document-v1.md

## Objective

Receive the six deployment guides relocated from emage.code (their T403) and fold them
into this repo's single documentation tree. This is a **paired handover**: neither repo
lands its side until both are ready — the docs must not be dropped between the repos.

## Inputs

- emage.code T403 (the sending side — coordinate timing with the orchestrator)
- The six incoming deployment guides (from emage.code)
- `docs/user/README.md` (C050 — the tree they fold into)

## Rails (read before starting)

### You MUST
- Confirm with the orchestrator that emage.code's T403 is ready before starting (hard ordering constraint, roadmap §2.7)
- Place the received guides under `docs/user/deployment/` and link them from the single guide's deployment section
- Normalize filenames to this repo's conventions (no version suffixes; kebab-case)
- Record the provenance of each received file (source repo + original path) in a `docs/user/deployment/README.md` index
- Add a CHANGELOG entry

### You MUST NOT
- Land this before emage.code's T403 side is ready (and vice versa — coordinate via orchestrator)
- Rewrite the received content substantively — fold in, normalize names, link; content edits are a separate task if needed
- Leave duplicates: if a received guide overlaps the single guide, link rather than duplicate
- Touch code

## File ownership

- **May create/modify:** `docs/user/deployment/` (new files), `docs/user/README.md` (deployment section links), `CHANGELOG.md`
- **Must NOT touch:** code, other docs, the emage.code repo (that is T403's side)

## Steps (execute in order)

1. Confirm T403 readiness with the orchestrator.
2. Receive the six guides; place under `docs/user/deployment/`.
3. Normalize names; write the provenance index.
4. Link from the single guide; CHANGELOG.
5. Confirm with the orchestrator that both sides landed.

## Expected outputs

- `docs/user/deployment/` with the six guides + provenance index
- Single guide links them
- CHANGELOG entry

## Acceptance criteria

1. Six guides present, normalized, indexed with provenance
2. Single guide's deployment section links them
3. Both repos agree the handover completed (orchestrator confirms T403 landed)

## Verification commands

```bash
ls docs/user/deployment/ | wc -l   # = 7 (6 guides + index)
grep -c "deployment/" docs/user/README.md
git diff --stat
```

## Git rails

- Branch: `agent/technical-writer/C052` from `develop` (rebased on merged C050)
- Commit: `docs(user): receive deployment guides from emage.code (T403)`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If T403 is not ready, this task is `blocked` / `dependency` / `critical` — do not
proceed with a partial handover.

## Execution notes

**Confirmation of T403 receipt.** Per the orchestrator's dispatch, emage.code's T403 was
already confirmed `done` before this task started; the six guides were staged, ready to
receive, at
`/home/emage/Code/emage/emage.code/docs/archiv/cwso-deployment-guides-pending-t473-handoff/`
on this machine. All six were read directly from that absolute path (plus the staging
directory's own `README.md`, a handoff explanation — correctly not copied in as a
seventh guide) before any other work began. Content received and folded in: **yes** —
see below. Confirming both repos consider the handover fully complete (i.e. that
emage.code's staging directory can now be deleted) is the orchestrator's job per the
dispatch brief, not this task's.

**Received filenames vs. final filenames.** All six guides already conformed to this
repo's naming convention (kebab-case, no version suffixes) — **no renames were needed**.
Original (emage.code, `docs/deployment/`) and final (this repo,
`docs/user/deployment/`) filenames are identical:

| File | Renamed? |
|---|---|
| `local-docker-desktop-guide.md` | No |
| `gcp-cloud-run-guide.md` | No |
| `proxmox-lxc-guide.md` | No |
| `cwso-overview-and-agent-integration-guide.md` | No |
| `cwso-emage-orchestrator-connection-guide.md` | No |
| `troubleshooting-guide.md` | No |

**Provenance index.** Written to `docs/user/deployment/README.md` — a source-repo /
original-path table for all six files, a "which guide do I need" comparison
(environment, validated status), and a normalization log. Full content is in that file;
see it directly rather than duplicating here.

**Internal cross-link audit and fixes.** Checked every one of the six guides for links
to (a) each other and (b) the old emage.code `docs/deployment/README.md` index, per the
brief's explicit scope:

- Cross-links **among the six guides themselves** (e.g.
  `local-docker-desktop-guide.md` → `proxmox-lxc-guide.md`, `gcp-cloud-run-guide.md`,
  `troubleshooting-guide.md`; and the reverse links in those files; plus
  `troubleshooting-guide.md`'s "Related Guides" list) were already relative,
  same-directory markdown links (e.g. `(local-docker-desktop-guide.md)`). Since all six
  guides now live together, flat, in `docs/user/deployment/`, **these needed no changes**
  — they resolve correctly as-is.
- Two guides (`cwso-overview-and-agent-integration-guide.md`,
  `troubleshooting-guide.md`) link to `README.md` (the old emage.code deployment index).
  That link target was already a same-directory relative link and is **still
  syntactically valid** — it now resolves to the new `docs/user/deployment/README.md`
  provenance index created by this task, which took over that role. No target edit was
  needed; the provenance index's "which guide do I need" comparison table was written
  deliberately to keep `cwso-overview-and-agent-integration-guide.md`'s surrounding
  description of that link ("compares Docker Desktop, Proxmox LXC, and GCP Cloud Run
  side by side") reasonably accurate at the new target, without editing that guide's own
  prose.
- `cwso-overview-and-agent-integration-guide.md` additionally contained **three
  path-prefix text mentions** of `docs/deployment/...` (in its own §2 "Deployment"
  section: the index link's display path, the local-docker-desktop-guide.md link's
  display path, and a plain-text backtick mention of
  `cwso-emage-orchestrator-connection-guide.md`'s old path) describing the guides' old
  location in emage.code. These were mechanically updated to `docs/user/deployment/...`
  to match the new location — a path-string fix, not a content rewrite. The link
  *targets* themselves were already correct relative paths and were not touched.
- **Not fixed, flagged instead (out of C052's explicit scope):**
  `cwso-overview-and-agent-integration-guide.md` also contains several relative links
  into the emage.code repository itself (`implementation/runtime/cwso/README.md`,
  `docs/artifacts/role-mapping-cwso-v1.md`, plan/task files under `docs/plans/` and
  `docs/tasks/`, and three `implementation/knowledge/agents/*.md` files) that will not
  resolve from this repo — they point into a different repository, not to a relocated
  sibling file, so no local rewrite target exists. Left as-is per the brief's scope
  (cross-links among the six guides + the old README only). Also left as-is: one plain-
  text mention of the old `docs/deployment/local-docker-desktop-guide.md` path in that
  guide's closing "Summary" table (not a link). Both are logged in
  `docs/user/deployment/README.md`'s "Normalization performed on receipt" section for
  visibility.

**Content overlap flagged (not resolved unilaterally).** `local-docker-desktop-guide.md`
documents an older, more manual local-deployment flow
(`deploy/docker-compose-t226.yml`, `scripts/deploy/cwso-docker-desktop.sh`, hand-exported
`JWT_SECRET`) that is materially different from `docs/user/README.md`'s current default
flow (`make up`, a single command wrapping bootstrap/build/start/health-wait/token-mint).
Both describe local Docker deployment of the same system via different mechanisms. Per
the brief ("if a received guide's content overlaps docs/user/README.md's own content,
link rather than duplicate — note any such overlap... rather than deciding unilaterally
to cut content"), this was **linked and flagged, not merged or cut** — see the "Overlap
note" in `docs/user/deployment/README.md` and the "Deployment guides" section note in
`docs/user/README.md`. Recommend a follow-up content task (Tech Lead / orchestrator call)
to decide whether `local-docker-desktop-guide.md` should be reconciled with, or marked
superseded by, the current `make up` flow.

**Confirmation: content not substantively rewritten.** All six guides were written
verbatim from the content read at the staging path, with the sole exception of the three
mechanical path-prefix substitutions in `cwso-overview-and-agent-integration-guide.md`
described above. No prose, code blocks, commands, tables, or structural content were
added, removed, or reworded in any of the six guides. `docs/user/deployment/README.md`
(the provenance index) and the new "Deployment guides" section in `docs/user/README.md`
are new files/sections authored by this task, not edits to received content.

**File ownership respected.** Only created files under `docs/user/deployment/` (new),
modified `docs/user/README.md`'s deployment section, and `CHANGELOG.md`. No code, other
docs, or the emage.code repo were touched (emage.code was only read from, at the
absolute path given in the dispatch, never written to).

**Acceptance criteria status:**
1. Six guides present under `docs/user/deployment/`, normalized (no renames needed),
   indexed with provenance in `docs/user/deployment/README.md` — **met** (7 files total:
   6 guides + index).
2. `docs/user/README.md`'s new "Deployment guides" section links all six — **met**.
3. Content received and folded in from T403 — **confirmed above**; both-repos-agree
   confirmation (i.e. that emage.code's staging directory is now safe to delete) is
   explicitly left to the orchestrator, per the dispatch brief.

**Blockers:** none. No git operations were attempted — worktree left with changes in the
working tree for the orchestrator to review and commit, per the dispatch's tooling note.
