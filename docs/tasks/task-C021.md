# Task C021 — Implement the filesystem projection

**ID:** C021
**Owner:** backend-developer
**Status:** in_progress
**Priority:** P0
**Depends on:** C020 (ADR-012 approved: GO)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B2); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md; docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md

## Objective

Implement the projection mechanism chosen in ADR-012 so that every shadow workspace is
reachable at a real path inside the sandbox. This closes the largest v1.0 gap: sub-agents
that expect a real filesystem path can finally use shadow workspaces.

## Inputs

- `docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md` (the approved decision — follow it, do not re-decide)
- `services/cwso-git-shadow/src/main.rs` (P2-1 marker at line 11)
- `services/cwso-git-shadow/src/repo.rs` (in-memory libgit2 ODB)
- `deploy/Dockerfile.git-shadow`, `deploy/docker-compose.yml` (container capabilities the mechanism needs)

## Rails (read before starting)

### You MUST
- Implement exactly the mechanism ADR-012 selected
- Expose each shadow workspace at a deterministic path (e.g., `/var/lib/cwso/shadow/<workspace-id>/`) inside the git-shadow container/sandbox
- Wire projection creation into `create_shadow_workspace` and teardown into `drop_shadow_workspace`
- Remove the `POC-DEBT (P2-1)` marker from `main.rs` once the projection works, and note the removal in `docs/DEBT-REGISTER.md` (status → `fixed`, closing task C021)
- If the container needs added capabilities (e.g., `SYS_ADMIN` for mounts), request them narrowly in the compose/Dockerfile with a justifying comment — and flag the security trade-off in the MR
- Add unit/integration tests in the service's existing test layout

### You MUST NOT
- Implement write-back into the git object store — that is C022 (this task is read-side projection + lifecycle wiring only)
- Change the MCP tool surface or any Go orchestrator code
- Weaken container hardening without an explicit, MR-flagged justification
- Touch the merge-engine or rollout services
- Start before ADR-012 is human-approved

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `deploy/Dockerfile.git-shadow`, `deploy/docker-compose.yml` (git-shadow service only, if capabilities require), `docs/DEBT-REGISTER.md` (P2-1 row)
- **Must NOT touch:** `orchestrator/*`, `services/cwso-merge-engine/*`, `services/cwso-rollout/*`, `services/cwso-hal/*`, `services/cwso-sparse/*`

## Steps (execute in order)

1. Read ADR-012 and the current workspace lifecycle in `main.rs`/`repo.rs`.
2. Implement projection creation/teardown per the ADR.
3. Wire into the workspace lifecycle.
4. Tests: projection exists after create, gone after drop.
5. Remove the P2-1 marker; update DEBT-REGISTER.
6. Run the verification commands.

## Expected outputs

- Working projection in `cwso-git-shadow`
- Tests covering create/drop projection lifecycle
- P2-1 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. After `create_shadow_workspace`, the workspace is listable at a real path inside the container
2. After `drop_shadow_workspace`, the path is gone
3. `cargo test -p cwso-git-shadow` passes
4. No `POC-DEBT (P2-1)` marker remains; DEBT-REGISTER row shows `fixed` / C021

## Verification commands

```bash
cargo test -p cwso-git-shadow
docker compose -f deploy/docker-compose.yml up -d --build git-shadow
# create a workspace via the socket, then:
docker exec cwso-git-shadow ls /var/lib/cwso/shadow/
grep -n "P2-1" services/cwso-git-shadow/src/main.rs   # = no hits
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/backend-developer/C021` from `develop`
- Commit: `feat(git-shadow): implement shadow-workspace filesystem projection`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the ADR's chosen mechanism proves unimplementable as specified, **stop** — do not
silently switch mechanisms. Report `technical` / `critical`; the ADR must be revisited.

## Execution notes

<filled during execution>
