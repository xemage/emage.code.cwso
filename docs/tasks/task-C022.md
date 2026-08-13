# Task C022 — Write-back: filesystem mutations flow into the git ODB

**ID:** C022
**Owner:** backend-developer
**Status:** pending
**Priority:** P0
**Depends on:** C021
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B2); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md; docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md

## Objective

Edits made through the projected filesystem path must flow into the in-memory git
object store — not just the page cache — so that `commit_shadow` captures what an
agent actually changed with an ordinary editor.

## Inputs

- C021's projection implementation
- `services/cwso-git-shadow/src/repo.rs` (ODB write paths, `commit`)
- ADR-012 (the mechanism determines the write-back hook: overlay upperdir diff, FUSE write handler, or tmpfs sync)

## Rails (read before starting)

### You MUST
- Implement the write-back path appropriate to the ADR-012 mechanism (upperdir diff scan, FUSE write handler, or explicit sync-on-commit)
- Ensure `commit_shadow` after a filesystem edit produces a commit containing that edit
- Handle: file create, modify, delete, and rename through the filesystem
- Add tests: edit via filesystem → `commit_shadow` → assert the commit tree contains the change (for all four mutation types)
- Update `docs/DEBT-REGISTER.md` if any new shortcut is introduced (with a `POC-DEBT` tag per poc-guidelines.md)

### You MUST NOT
- Change the `commit_shadow` tool signature or the MCP surface
- Buffer writes without a durability story for the crash path (C023 tests crash; don't make its job impossible)
- Implement Merkle/incremental indexing (v1.1)
- Touch orchestrator Go code or other services

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `docs/DEBT-REGISTER.md` (only if new debt)
- **Must NOT touch:** `orchestrator/*`, other services, deploy files (unless C021 already flagged a capability need)

## Steps (execute in order)

1. Read C021's implementation and the ADR mechanism.
2. Implement write-back for the four mutation types.
3. Tests for create/modify/delete/rename → commit captures each.
4. Run verification.

## Expected outputs

- Write-back path in `cwso-git-shadow`
- Tests proving filesystem edits land in commits

## Acceptance criteria

1. Edit a file via the projected path → `commit_shadow` → commit tree contains the edit
2. All four mutation types covered by tests
3. `cargo test -p cwso-git-shadow` passes

## Verification commands

```bash
cargo test -p cwso-git-shadow
# E2E: create workspace, write via projected path, commit, inspect tree
docker compose -f deploy/docker-compose.yml up -d --build git-shadow
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/backend-developer/C022` from `develop` (rebased on merged C021)
- Commit: `feat(git-shadow): write filesystem mutations back into git ODB`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
Silent write loss is the worst outcome of this task — if a mutation type cannot be
captured reliably, that is `technical` / `critical`, not a TODO comment.

## Execution notes

<filled during execution>
