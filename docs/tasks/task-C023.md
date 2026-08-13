# Task C023 — Projection lifecycle + crash safety

**ID:** C023
**Owner:** backend-developer
**Status:** pending
**Priority:** P0
**Depends on:** C021
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C023 row); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md

## Objective

The projection is created with the workspace and torn down with it — and a crash must
not leak mounts. `docker compose down` after a forced kill leaves zero orphaned mounts
on the host. Test the crash path explicitly.

## Inputs

- C021/C022 implementation
- `services/cwso-git-shadow/src/main.rs` (service lifecycle, signal handling)
- `deploy/docker-compose.yml` (git-shadow tmpfs/volumes)

## Rails (read before starting)

### You MUST
- Register projection teardown on: workspace drop, service shutdown (SIGTERM/SIGINT), and service startup (reconcile/stale-mount sweep for mounts leaked by a previous crash)
- Implement a startup reconciliation pass: on boot, detect and clean projections from workspaces that no longer exist
- Add a crash test: create workspace + projection → `kill -9` the service → restart → assert no stale mount remains
- Assert `docker compose down` after the forced kill leaves no orphaned mounts on the host (`mount | grep` evidence)

### You MUST NOT
- Rely on "drop always runs" — the crash path is the point of this task
- Leave cleanup to the user or to a manual script
- Change the projection mechanism (C021) or write-back (C022)
- Touch other services or orchestrator code

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`
- **Must NOT touch:** `orchestrator/*`, other services, deploy files

## Steps (execute in order)

1. Implement teardown on drop + graceful shutdown.
2. Implement startup reconciliation sweep.
3. Write the kill -9 crash test.
4. Run verification, capturing `mount` before/after evidence.

## Expected outputs

- Lifecycle-safe projection teardown + startup reconciliation
- Crash test proving no leaked mounts

## Acceptance criteria

1. Drop → projection gone
2. Graceful restart → no stale mounts
3. `kill -9` → restart → reconciliation removes stale mounts
4. `docker compose down` after forced kill → zero orphaned mounts on host

## Verification commands

```bash
cargo test -p cwso-git-shadow
docker compose -f deploy/docker-compose.yml up -d --build git-shadow
# create workspace, then:
docker kill -s KILL cwso-git-shadow
docker compose -f deploy/docker-compose.yml up -d git-shadow
mount | grep -c "cwso/shadow"   # = 0 after reconciliation
docker compose -f deploy/docker-compose.yml down
mount | grep -c "cwso/shadow"   # = 0
```

## Git rails

- Branch: `agent/backend-developer/C023` from `develop` (rebased on merged C021)
- Commit: `feat(git-shadow): projection lifecycle and crash-safe teardown`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the container's mount namespace makes host-visible leaks impossible to detect from
inside, say so and test from the host — do not assert what you cannot observe.

## Execution notes

<filled during execution>
