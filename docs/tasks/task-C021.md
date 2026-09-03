# Task C021 — Implement the filesystem projection

**ID:** C021
**Owner:** backend-developer
**Status:** done
**Priority:** P0
**Depends on:** C020 (ADR-012 approved: GO)
**Created:** 2026-08-12
**Completed:** 2026-08-21
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

Implemented ADR-012's materialise-to-tmpfs mechanism in `services/cwso-git-shadow/src/repo.rs`:
`ShadowStore::create()` eagerly materializes every seeded base-tree file to a
real path under `<storage_root>/<workspace-uuid>/`; `write_file()` keeps that
real path in sync on every subsequent write; `drop_workspace()` removes the
real directory alongside the in-memory teardown. Read-side projection +
lifecycle wiring only, per scope — write-back into the git object store is
explicitly C022's job, not implemented here. No new container capability
needed: reuses the `git-shadow` service's existing tmpfs mount under
`cap_drop: ["ALL"]`, exactly the pattern ADR-012 cited as its load-bearing
proof this mechanism needs no privilege grant. Base-tree symlink-mode
(`120000`) blobs deliberately materialize as inert plain files, never a real
OS symlink, closing that attack class entirely.

**First-round review: split verdict.** Tech Lead PASS (scope, no-new-
capability claim, symlink handling, file ownership all independently
verified). Security Engineer **FAIL — SEC-001 (HIGH)**: a genuine TOCTOU gap
in the original `materialize_write()` — it canonicalized and validated
containment of the parent directory as a path string, then performed a
separate, later `open()` that re-resolved the path fresh from the
filesystem; `O_NOFOLLOW` only guarded the leaf, so a symlink swapped into an
intermediate directory component between the check and the open would be
silently followed, escaping the workspace root. Structurally the same bug
class T194 closed in the Go orchestrator, unfixed here in Rust. Per policy,
the HIGH finding blocked merge regardless of the Tech Lead PASS.

**Fix (commit `4f0cc28`)**: replaced the canonicalize-then-reopen pattern
with an fd-anchored walk mirroring `fs_tools.go`'s `secureResolveDirs`/
`secureOpenLeaf` (T193/T194): `open_root_dir` (the one name-based lookup,
justified as the trust anchor since `workspace_dir`'s UUID component is
server-generated, never client-influenced) → `openat_dir_nofollow` for
every intermediate component, each anchored to the immediately-preceding
hop's already-open fd with `O_DIRECTORY|O_NOFOLLOW` → `openat_leaf_nofollow`
for the final component, same anchoring → write via the resulting fd
itself, never a reconstructed path string. No new dependency (`libc`, not
`cap-std`). Two new tests: a deterministic pre-planted-intermediate-symlink
test (honestly documented in-code as not distinguishing old from new — the
old canonicalize check already caught this static case — new regression
coverage regardless) and a genuine concurrent race-stress test (2000
iterations). The orchestrator independently reproduced the before/after
distinction from scratch before dispatching re-review: spliced only the
two new test functions onto the untouched pre-fix commit in a throwaway
worktree — the race test genuinely panicked on the old code and passed
cleanly on the fix.

**VERDICT: PASS** (independent Security Engineer re-review, MR !149) —
reproduced the same before/after distinction independently from scratch
(not trusting either the orchestrator's or the worker's reproduction),
confirmed fd-anchoring is genuinely end-to-end with no path-string fallback
at any hop, confirmed the UUID trust-anchor reasoning is sound, confirmed a
necessary NUL-byte `CString` guard the raw `libc` calls need that the old
`std::fs` calls got for free, reproduced the full 22/22 test suite,
confirmed zero scope creep into C022 and zero new dependency. CI green
except one non-blocking `rust:lint` fmt nit already configured
`allow_failure: true` in this project.

MR !149 (`agent/backend-developer/C021`), merged to `develop` via merge
commit `94459c76`. Closes C021 — the first implementation task under
ADR-012's GO decision — unblocking C022 (write-back into the git object
store) and C023 (projection lifecycle + crash safety).
