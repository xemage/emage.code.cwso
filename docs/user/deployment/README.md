# CWSO deployment guides — index and provenance

**Last updated:** 2026-08-28
**Received via:** emage.code task T403 → CWSO task C052 (paired handover; see
`docs/tasks/task-C052.md`)

These six guides were relocated out of emage.code's `docs/deployment/` (emage.code is a
knowledge-projection layer, not the CWSO orchestrator, so full CWSO deployment
documentation does not belong there) and folded into this repository's single
documentation tree by task C052. Content is unchanged from the received files except for
the internal-cross-link normalization described below — see
[`docs/user/README.md`](../README.md) for the current, actively maintained
getting-started flow (`make up`), which is **not** described by these guides.

## Which guide do I need?

| Environment / topic | Guide | Validated? |
|---|---|---|
| Local development (older, manual flow) | [local-docker-desktop-guide.md](local-docker-desktop-guide.md) | Yes — validated end-to-end (see guide header) |
| Google Cloud Run | [gcp-cloud-run-guide.md](gcp-cloud-run-guide.md) | No — explicitly marked "not yet validated end-to-end" in its own header |
| Proxmox LXC (on-premises) | [proxmox-lxc-guide.md](proxmox-lxc-guide.md) | No — explicitly marked "not yet validated end-to-end" in its own header |
| CWSO overview + emage.code agent integration | [cwso-overview-and-agent-integration-guide.md](cwso-overview-and-agent-integration-guide.md) | N/A — narrative overview, not a deployment procedure |
| Wiring a running CWSO stack to an emage.code orchestrator | [cwso-emage-orchestrator-connection-guide.md](cwso-emage-orchestrator-connection-guide.md) | Yes — validated 2026-08-05 (see guide's "Tested evidence") |
| Cross-environment troubleshooting | [troubleshooting-guide.md](troubleshooting-guide.md) | N/A — reference document |

**Overlap note (flagged for the orchestrator, not resolved unilaterally by this task):**
`local-docker-desktop-guide.md` documents an older, more manual local-deployment
procedure — `deploy/docker-compose-t226.yml`, `scripts/deploy/cwso-docker-desktop.sh`,
a hand-exported `JWT_SECRET` — than this repository's current default, single-command
flow (`make up`, documented in [`docs/user/README.md`](../README.md)). Both describe
local Docker-based deployment of the same system but via different scripts, compose
files, and secret-bootstrap mechanisms. C052's brief scoped this task to receiving,
normalizing, and linking the six guides, not reconciling their content with the current
flow — this discrepancy was not resolved and is called out here per the brief's
instruction to link/flag overlap rather than unilaterally cut or rewrite content.

## Provenance

| File | Source repo | Original path | Handoff task |
|---|---|---|---|
| `local-docker-desktop-guide.md` | emage.code | `docs/deployment/local-docker-desktop-guide.md` | T403 → C052 |
| `gcp-cloud-run-guide.md` | emage.code | `docs/deployment/gcp-cloud-run-guide.md` | T403 → C052 |
| `proxmox-lxc-guide.md` | emage.code | `docs/deployment/proxmox-lxc-guide.md` | T403 → C052 |
| `cwso-overview-and-agent-integration-guide.md` | emage.code | `docs/deployment/cwso-overview-and-agent-integration-guide.md` | T403 → C052 |
| `cwso-emage-orchestrator-connection-guide.md` | emage.code | `docs/deployment/cwso-emage-orchestrator-connection-guide.md` | T403 → C052 |
| `troubleshooting-guide.md` | emage.code | `docs/deployment/troubleshooting-guide.md` | T403 → C052 |

All six files were relocated out of emage.code's `docs/deployment/` by emage.code's T403
into a staging handoff directory
(`docs/archiv/cwso-deployment-guides-pending-t473-handoff/` in the emage.code repo,
copied there verbatim, no edits) and received here by CWSO's C052. emage.code's own
`docs/deployment/README.md` was replaced, in the same T403 change, with a short,
usage-only pointer back to this repository — it no longer contains deployment
instructions.

## Normalization performed on receipt (C052)

- **Filenames:** all six already followed this repository's convention (kebab-case, no
  version suffixes) — no renames were needed.
- **Internal cross-links between the six guides:** these were already relative,
  same-directory links (e.g. `local-docker-desktop-guide.md` ↔ `proxmox-lxc-guide.md` ↔
  `gcp-cloud-run-guide.md` ↔ `troubleshooting-guide.md`) and continue to resolve
  correctly unchanged now that all six live together in this directory.
- **Links to the old emage.code deployment index (`docs/deployment/README.md`):**
  `cwso-overview-and-agent-integration-guide.md` and `troubleshooting-guide.md` both
  contain a same-directory link targeting `README.md` — this was already syntactically
  valid before and after the move (it now resolves to this file, replacing the old
  emage.code index) and needed no target change. `cwso-overview-and-agent-integration-guide.md`
  additionally contained three **path-prefix mentions** of `docs/deployment/...`
  (describing the old emage.code location of this index and two of the sibling guides)
  that were updated to `docs/user/deployment/...` to match the new location — this was a
  mechanical path-string fix, not a content rewrite.
- **Not touched (out of C052's scope):** `cwso-overview-and-agent-integration-guide.md`
  also contains several relative links into the emage.code repository itself
  (`implementation/runtime/cwso/README.md`, `docs/artifacts/role-mapping-cwso-v1.md`,
  plan/task files under `docs/plans/` and `docs/tasks/`, and three
  `implementation/knowledge/agents/*.md` files) that will **not** resolve from this
  location, since they point into a different repository entirely rather than to a
  relocated sibling file. These were left as-is: C052's brief scoped link-fixing to
  cross-links among the six guides and to the old emage.code deployment index, not to
  this guide's broader emage.code-internal references. Its closing "Summary" table also
  still states the old `docs/deployment/local-docker-desktop-guide.md` path in one answer
  cell (plain text, not a link) — left unchanged for the same reason. Flagging both for a
  possible follow-up content task if this guide needs further adjustment for its new home.
- **Content:** no substantive content edits were made to any of the six guides — only the
  mechanical path-prefix normalization described above. No content was cut; the one
  identified overlap with `docs/user/README.md` (see "Overlap note" above) was flagged,
  not resolved, per the brief.
