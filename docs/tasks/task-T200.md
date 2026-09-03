# Task T200 — Reconcile `local-docker-desktop-guide.md` with the current `make up` flow

**ID:** T200
**Owner:** technical-writer
**Status:** pending
**Priority:** P2
**Depends on:** C052 (merged — the guide must exist in this repo before it can be reconciled)
**Created:** 2026-08-28
**Completed:** —
**Based on:** Discovered during C052 (receive emage.code deployment docs), flagged in
`docs/user/deployment/README.md`'s "Overlap note" and `docs/tasks/task-C052.md`'s
execution notes — not resolved there, per C052's brief scope (receive/normalize/link
only, no content rewrites). Logged separately per the established T197/T198/T199
cross-boundary-gap pattern.

## Objective

`docs/user/deployment/local-docker-desktop-guide.md` (received from emage.code via T403 ⇄
C052) documents an older, more manual local-deployment procedure —
`deploy/docker-compose-t226.yml`, `scripts/deploy/cwso-docker-desktop.sh`, a hand-exported
`JWT_SECRET` — that materially diverges from this repository's current default flow
(`make up`, documented in `docs/user/README.md`, C050). **Both referenced files
(`deploy/docker-compose-t226.yml`, `scripts/deploy/cwso-docker-desktop.sh`) were confirmed
absent from this repository** (verified independently by both the orchestrator and Tech
Lead review during C052) — every command-line example in this guide that invokes either
file will fail verbatim on a clean checkout.

Reconcile this guide with reality: either update it to reflect the current `make up` flow,
or clearly demarcate it as a superseded/legacy path with a pointer to the current guide —
whichever is the more accurate framing once you've investigated. Do not let a
`docs/user/deployment/*` guide silently contradict `docs/user/README.md`'s documented
flow.

## Inputs

- `docs/user/deployment/local-docker-desktop-guide.md` (the guide to reconcile)
- `docs/user/deployment/README.md` (the provenance index — its "Overlap note" section has
  the original finding; update or remove that note once this task resolves it)
- `docs/user/README.md` (C050 — the current, actively-maintained flow this guide must not
  contradict)
- `Makefile`, `deploy/docker-compose.yml`, `scripts/cwso-bootstrap-secrets.sh`,
  `scripts/cwso-token.sh` (the actual current mechanism this guide's older equivalents —
  `docker-compose-t226.yml`, `cwso-docker-desktop.sh`, hand-exported `JWT_SECRET` — were
  superseded by)

## Rails (read before starting)

### You MUST
- Investigate first: is `local-docker-desktop-guide.md`'s content genuinely obsolete
  (fully superseded by `make up`), or does it contain content specific to a Docker
  Desktop deployment scenario that `docs/user/README.md`'s generic flow doesn't cover
  (e.g. Docker Desktop-specific settings, resource limits, networking quirks)? Do not
  assume — read both documents fully before deciding the reconciliation approach.
- If the content is genuinely superseded: rewrite the guide's commands/flow to match
  `make up` (or delete it and redirect to `docs/user/README.md`, if it adds nothing
  `docs/user/README.md` doesn't already cover for a Docker Desktop user specifically) —
  your judgment, with reasoning disclosed in your MR.
- If there's genuinely Docker-Desktop-specific value to preserve: keep that value, but
  fix every broken command reference (`docker-compose-t226.yml` → the real compose file;
  `cwso-docker-desktop.sh` → the real bootstrap/token scripts; hand-exported `JWT_SECRET`
  → `scripts/cwso-bootstrap-secrets.sh`'s actual mechanism) so every command in the
  reconciled guide is genuinely runnable, per this project's standing bar for guide
  commands (see C050's and C054's acceptance criteria for the standard this repo holds
  itself to).
- Update or remove `docs/user/deployment/README.md`'s "Overlap note" once resolved.
- Add a CHANGELOG entry.

### You MUST NOT
- Silently delete the guide without a disclosed reason and a redirect — a reader following
  a link into `docs/user/deployment/` should never hit a 404 without explanation
- Touch the other five received guides (`gcp-cloud-run-guide.md`, `proxmox-lxc-guide.md`,
  `cwso-overview-and-agent-integration-guide.md`,
  `cwso-emage-orchestrator-connection-guide.md`, `troubleshooting-guide.md`) unless you
  find they share the exact same stale-reference problem — if so, flag that as a new
  finding rather than silently expanding this task's scope
- Touch `docs/user/README.md` itself (that's C050's territory) beyond, if needed, a single
  cross-reference update if the reconciliation changes what `docs/user/README.md`'s
  "Deployment guides" section says about this specific guide

## File ownership

- **May create/modify:** `docs/user/deployment/local-docker-desktop-guide.md`,
  `docs/user/deployment/README.md` (Overlap note, plus — see optional nit below — the
  `local-docker-desktop-guide.md` validation-status row wording), `CHANGELOG.md`
- **Must NOT touch:** the other five deployment guides, `docs/user/README.md`'s substance
  (a single link-description tweak is fine if needed), code

## Optional minor nit (bundle in if convenient, non-blocking)

C052's Tech Lead review separately flagged: `docs/user/deployment/README.md`'s provenance
table cites `local-docker-desktop-guide.md`'s "Yes — validated end-to-end" claim as "(see
guide header)", but that guide's own header doesn't literally use the phrase "validated
end-to-end" — it's inferable from a `Based on:` line and from the *other two* guides'
cross-references to it. Tighten the wording while you're already touching this file for
the Overlap note; not worth a separate task on its own.

## Steps (execute in order)

1. Read `local-docker-desktop-guide.md` in full and `docs/user/README.md`'s current flow
   in full; decide the reconciliation approach (rewrite vs. redirect) with reasoning.
2. Execute the chosen approach — every remaining command must be genuinely runnable
   against this repo's current state.
3. Update/remove the provenance index's Overlap note.
4. CHANGELOG entry.

## Expected outputs

- A reconciled `local-docker-desktop-guide.md` (rewritten or redirected) with zero
  references to nonexistent files/scripts
- Updated provenance index
- CHANGELOG entry

## Acceptance criteria

1. Every command in the reconciled guide is genuinely runnable against this repo's
   current state (no `docker-compose-t226.yml`, no `cwso-docker-desktop.sh`, no
   hand-exported `JWT_SECRET` instructions that contradict `scripts/cwso-bootstrap-secrets.sh`)
2. The reconciliation approach (rewrite vs. redirect) is disclosed with reasoning in the MR
3. `docs/user/deployment/README.md`'s Overlap note is updated or removed to match the
   resolved state
4. The other five received guides are untouched (unless a new, separately-flagged finding
   justifies otherwise)

## Verification commands

```bash
grep -n "docker-compose-t226\|cwso-docker-desktop.sh" docs/user/deployment/local-docker-desktop-guide.md
# either zero hits (fully rewritten) or explicitly framed as historical/legacy, not live instructions
ls deploy/docker-compose-t226.yml scripts/deploy/cwso-docker-desktop.sh 2>&1  # confirms they still don't exist
```

## Git rails

- Branch: `agent/technical-writer/T200` from `develop`
- Commit: `docs(deployment): reconcile local-docker-desktop-guide.md with make up`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries. If it's
genuinely ambiguous which reconciliation approach is right (e.g. you find real,
non-obvious Docker-Desktop-specific value worth preserving but can't tell whether it's
still accurate), report `unclear_requirements` / `minor` and propose your best-judgment
default rather than blocking indefinitely.

## Execution notes

<filled during execution>
