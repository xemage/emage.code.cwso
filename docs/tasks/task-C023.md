# Task C023 — Projection lifecycle + crash safety

**ID:** C023
**Owner:** backend-developer
**Status:** in_progress
**Priority:** P0
**Depends on:** C021, C022
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C023 row); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md; docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md

> **2026-08-21 correction (orchestrator):** this brief was originally written
> before ADR-012 was decided and assumed a *mount-based* projection mechanism
> (OverlayFS/FUSE) — its acceptance criteria and verification commands were
> phrased around "leaked mounts" and `mount | grep`. ADR-012's actual GO
> decision (materialise-to-tmpfs, implemented by C021/C022) creates **no
> per-workspace OS mount of any kind** — every shadow workspace is a plain
> directory (`<storage_root>/<workspace-uuid>/`) under the *one*,
> already-existing `tmpfs` mount Compose sets up for the whole `git-shadow`
> service (`deploy/docker-compose.yml`). The crash-safety intent behind this
> task is unchanged and still fully necessary — a crash must not leave stale
> state behind, and that must be tested explicitly, not assumed — but the
> concrete failure mode is **stale per-workspace *directories*** left behind
> inside the one persistent tmpfs, not leaked *mounts*. The rails below have
> been rewritten to reflect this; do not follow the original "mount"-based
> framing if you find an older cached copy of this brief anywhere.
>
> **A second, load-bearing fact C022 surfaced that changes this task's scope**
> (see `services/cwso-git-shadow/src/writeback.rs`'s "Durability" doc-comment
> section): `ShadowStore::new` always starts from an **empty** `workspaces`
> map — there is no on-disk/ODB-ref record today of *which* per-workspace
> directories under `storage_root` correspond to a still-"open" workspace.
> This means **every** restart (graceful or crash) already loses all
> in-flight (uncommitted) workspace *state*, regardless of this task —
> that larger gap (should workspace state survive a restart at all?) is
> explicitly **out of scope for C023** (nothing in this task's rails asks you
> to add restart persistence for open workspaces) and should not be
> conflated with the narrower thing C023 *does* need to guarantee: that a
> restart, however it happens, leaves **zero stale per-workspace
> directories** on disk once the reconciliation sweep below has run — not
> that it leaves workspace *state* intact. A concrete, useful consequence:
> because the in-memory map is unconditionally empty immediately after any
> restart, the startup sweep does not need to diff against "workspaces the
> new process currently thinks are open" (there are none, ever, at that
> point) — it only needs to remove every subdirectory of `storage_root`
> except the persistent `bare.git` ODB directory itself. Confirm this
> reasoning yourself against the real code before implementing; if you find
> it's wrong (e.g. a later change made `workspaces` non-empty at startup),
> that's a `technical`/`major` blocker — stop and report, the correct sweep
> semantics would need to change.

## Objective

The projection is created with the workspace and torn down with it — and a crash
must not leave stale per-workspace projection directories behind. Test the crash
path explicitly, not just the graceful-shutdown path (`Drop` always running is
exactly what a `kill -9` denies you).

## Inputs

- C021's projection implementation (`ShadowStore::create`/`drop_workspace`,
  `workspace_dir`) and C022's write-back engine (`WriteBackEngine::spawn`/
  `register_workspace`/`unregister_workspace`, `services/cwso-git-shadow/src/writeback.rs`)
- `services/cwso-git-shadow/src/main.rs` (service lifecycle, signal handling)
- `deploy/docker-compose.yml` (the `git-shadow` service's `tmpfs` config — read
  this to confirm your understanding of what is and isn't host-visible/
  container-lifetime-scoped before designing your test methodology)

## Rails (read before starting)

### You MUST
- Register projection teardown on: workspace drop (already exists via C021/C022;
  confirm and don't duplicate), service shutdown (SIGTERM/SIGINT — confirm
  in-flight write-back work quiesces cleanly, and the `WriteBackEngine`'s two
  background threads don't need to be force-killed), and service **startup**
  (a sweep that removes every stale per-workspace directory left behind by a
  previous instance that didn't get to run its own graceful teardown)
- Implement the startup reconciliation sweep as reasoned above: on boot, after
  `ShadowStore::new` opens/creates `bare.git`, remove every OTHER entry directly
  under `storage_root` (every per-workspace directory, by construction — there
  is no in-memory workspace this fresh process could believe still legitimately
  owns any of them). Log what was swept (path + count) at `info` for
  operability; this is expected, routine behavior, not a warning-worthy anomaly.
- Add a **deterministic, library-level crash test** that does not depend on
  process-kill/container-restart timing: construct a real `ShadowStore` against
  a real temp `storage_root`, create a workspace (materializing its real
  directory), then **simulate an unclean crash** by simply dropping/discarding
  that `ShadowStore` instance without calling `drop_workspace` on it first (a
  `kill -9` gives zero opportunity for any in-process cleanup — including any
  `Drop` impl — to run at all; do not use a code path that relies on `Drop`
  running, since that's precisely what this test must NOT assume) — then
  construct a **fresh** `ShadowStore::new` against the *same* `storage_root`
  (representing a process restart against the same, still-mounted tmpfs
  directory) and assert the previously-materialized workspace directory is
  gone once the fresh instance's startup sweep has run.
- Additionally attempt a **live, container-level confirmation** if you can
  construct one that's actually meaningful (see the "You MUST NOT" note below
  on why a naive version of this is not meaningful) — e.g., if there's a
  practical way to kill and restart the `cwso-git-shadow` *process* while the
  *container* (and therefore its tmpfs mount) stays alive, that's real
  additional evidence on top of the library-level test, not a replacement for
  it.

### You MUST NOT
- Rely on "drop always runs" — the crash path is the entire point of this task;
  your primary test must not be structured in a way that only exercises the
  graceful-shutdown code path
- Leave cleanup to the user or to a manual script
- Change the projection mechanism (C021) or the write-back engine's actual
  sync logic (C022) — this task adds lifecycle/teardown/startup-sweep code
  around them, it does not alter how materialisation or write-back work
- Touch other services or orchestrator code
- **Do not build your crash test around `docker compose down` "leaving no
  orphaned mounts on the host."** This was the original brief's framing and
  is not a meaningful test for this mechanism: a Compose `tmpfs:` mount is
  scoped entirely to the *container's own* mount namespace, not host-visible
  at all, and is unconditionally reclaimed by the kernel the moment the
  container itself is removed — `docker compose down` will trivially report
  zero mounts regardless of whether this task's application-level cleanup
  logic works correctly or is completely broken. It tests container
  lifecycle, not this service's crash-safety code. If you want a container-
  level confirmation, it must kill and restart the *process*, not the
  *container* — a fresh container/fresh tmpfs proves nothing about
  reconciliation, since a brand-new tmpfs is empty by definition, with or
  without any sweep logic existing at all.

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`
- **Must NOT touch:** `orchestrator/*`, other services, deploy files

## Steps (execute in order)

1. Confirm this brief's two corrected premises against the real, current code
   yourself (no per-workspace mounts exist; `ShadowStore::new`'s `workspaces`
   map starts empty on every construction) before designing anything.
2. Implement the startup reconciliation sweep in `ShadowStore::new` (or
   immediately after it, wherever `main.rs` currently calls it).
3. Confirm/harden SIGTERM/SIGINT shutdown: does the `WriteBackEngine`'s
   `event_loop`/`reconcile_loop` threads need an explicit stop signal, or is
   process exit sufficient given they hold no state that must be flushed
   (see C022's own "Durability" doc comment — write-back has no in-process
   buffer to flush)? Document your conclusion either way.
4. Write the deterministic library-level crash test (see "You MUST" above).
5. Attempt a live container-level confirmation if you can construct a
   meaningful one (process-kill, not container-kill); if you conclude it
   isn't practically constructible without trivially recreating the tmpfs,
   say so explicitly in your MR rather than shipping a test that doesn't
   actually exercise this logic.
6. Run verification.

## Expected outputs

- Startup reconciliation sweep removing stale per-workspace directories
- Confirmed/hardened graceful-shutdown behavior for the write-back engine's
  background threads
- A deterministic crash test proving the sweep works, independent of any
  process-kill timing

## Acceptance criteria

1. Drop (graceful) → projection directory gone (already true via C021/C022;
   confirm, don't re-implement)
2. Simulated crash (constructed per the deterministic test above, not a
   `Drop`-reliant code path) → a fresh `ShadowStore` against the same
   `storage_root` sweeps the stale directory on construction
3. SIGTERM/SIGINT shutdown does not hang or panic the write-back engine's
   background threads
4. `cargo test -p cwso-git-shadow` passes, including the new deterministic
   crash test

## Verification commands

```bash
cd services && rustup run 1.87.0 cargo test -p cwso-git-shadow
# Deterministic crash-path test is part of the above; no Docker required for
# the primary evidence. If you also built a live container-level
# confirmation:
docker compose -f deploy/docker-compose.yml up -d --build git-shadow
# create a workspace, note its real directory, then kill (and restart) the
# cwso-git-shadow *process* specifically -- not the container -- and confirm
# the directory is swept; document exactly how you did this in the MR.
docker compose -f deploy/docker-compose.yml down
```

## Git rails

- Branch: `agent/backend-developer/C023` from `develop` (rebased on merged C021/C022)
- Commit: `feat(git-shadow): projection lifecycle and crash-safe startup reconciliation`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If you find the two corrected premises above (no per-workspace mounts;
`workspaces` map always starts empty) do not actually hold against the current
code, that is `technical` / `major` — stop and report rather than implementing
against a premise you've found to be false.

## Execution notes

<filled during execution>
