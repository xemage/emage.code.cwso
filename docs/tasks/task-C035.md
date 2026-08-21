# Task C035 — fd-anchored recursive read-back walk (harden R-3)

**ID:** C035
**Owner:** backend-developer
**Status:** in_progress
**Priority:** P0
**Depends on:** C022
**Created:** 2026-08-21
**Completed:** —
**Based on:** `docs/DEBT-REGISTER.md` row R-3; independent Security Engineer review of C022's MR !153; docs/decisions/ADR-012-shadow-workspace-filesystem-projection.md

## Objective

Close `docs/DEBT-REGISTER.md` row **R-3** (reclassified from a provisional `v1.1`/
`wontfix`-pending-review disposition to **`v1.0-blocker`** by independent Security
Engineer review of C022's MR !153): C022's write-back read-side scan
(`scan_workspace_tree` in `services/cwso-git-shadow/src/repo.rs`, and the equivalent
single-file logic in `writeback.rs`'s `sync_file`) checks `symlink_metadata`
immediately before reading or recursing into an entry, but that check and the
subsequent `fs::read`/directory-recursion are two separate syscalls — not one
fd-anchored operation, unlike the write side's `openat(..., O_NOFOLLOW)`
(`materialize_write_via_fd_walk`, built under C021/T193/T194). A path component
could in principle be swapped for a symlink in the narrow window between the two.

The reviewer confirmed this is **not exploitable against today's mount topology**
(nothing outside `git-shadow` itself currently has any path to a shadow
workspace's projected directory) but found that treating it as *permanently*
acceptable rests on an assumption about sandbox-mount wiring that C024 has not
yet built, and which ADR-012 itself names as an unsolved mount-propagation
problem — accepting a TOCTOU gap permanently on a premise that depends on future
code being built a particular way was judged inconsistent with the bar this
project already held the write side to (SEC-001, HIGH, blocking, C021). This
task closes the gap for real rather than deferring it.

## Inputs

- `services/cwso-git-shadow/src/repo.rs`: `scan_workspace_tree`, `scan_dir_into`,
  and their "Residual TOCTOU"/R-3 doc comment (the precise gap description)
- `services/cwso-git-shadow/src/writeback.rs`: `sync_file`, `sync_new_subtree`
  (the single-file/single-new-directory equivalents of the same policy)
- `services/cwso-git-shadow/src/repo.rs`'s `open_root_dir`/`openat_dir_nofollow`/
  `openat_leaf_nofollow`/`materialize_write_via_fd_walk` (C021's write-side
  fd-anchored primitives — generalize these to a recursive **read** walk, do not
  reinvent the pattern)
- `docs/DEBT-REGISTER.md` row R-3 (the tracked gap; flip to `closed`/`fixed` when done)

## Rails (read before starting)

### You MUST
- Implement the read-side directory walk using fd-anchoring throughout: open
  `workspace_dir` once (the same non-attacker-controlled trust anchor C021's
  write side uses), then descend one path component at a time via `openat`
  (`O_DIRECTORY | O_NOFOLLOW`) relative to the previous hop's already-open fd —
  never a fresh, multi-component, name-based path resolution after an earlier
  check
- Use `fdopendir`(via the `nix`/`rustix` crate, or raw `libc` calls matching
  C021's existing style — `libc` is already a direct dependency) or equivalent
  to enumerate a directory's entries from an already-open directory fd, rather
  than `std::fs::read_dir` on a path string, so that listing a directory's
  contents is anchored to the same fd used to open it, not a separate
  name-based lookup
- For each entry, read file content via `openat(..., O_NOFOLLOW)` relative to
  the containing directory's fd (mirroring `openat_leaf_nofollow`, generalized
  to reads) rather than `std::fs::read(path)` on a reconstructed path string
- Preserve the exact same *policy* the current code already documents (never
  follow a symlink, skip and log instead; skip and log non-UTF-8 names,
  `docs/DEBT-REGISTER.md` row R-4) — this task closes the TOCTOU gap in *how*
  that policy is enforced, it does not change the policy itself
- Apply the fix consistently to BOTH call sites: the reconciliation pass
  (`scan_workspace_tree`) and the inotify event handler's single-file/
  single-new-subtree sync (`sync_file`/`sync_new_subtree` in `writeback.rs`)
- Add a regression test demonstrating the race is closed, following the same
  deterministic-first, race-stress-second discipline as C021's SEC-001 fix
  (`write_file_race_against_symlink_swap_never_escapes_workspace`): a
  deterministic pre-planted-symlink test is necessary but not sufficient to
  distinguish old from new (a static symlink is already caught by the current
  `symlink_metadata` check); the test that actually proves this fix closes a
  real race is a genuine concurrent race-stress test (a second thread
  repeatedly swapping a directory component for a symlink and back while the
  read-side scan runs in a loop), matching `write_file_race_against_symlink_swap_never_escapes_workspace`'s
  structure on the write side. Before/after discipline required: demonstrate
  the OLD (pre-fix) scan code genuinely fails this race test (splice the new
  test onto the pre-fix commit, same methodology the orchestrator used to
  verify SEC-001's fix), not just that the new code passes it.
- Flip `docs/DEBT-REGISTER.md` row R-3 to `closed`/`fixed`, referencing this
  task, once the fd-anchored walk is genuinely in place and tested

### You MUST NOT
- Change the symlink/non-UTF-8 *policy* itself (still skip-and-log, never
  follow/never error) — only how the check is enforced (fd-anchored vs.
  separate-syscall)
- Change write-back's read-only contract — this task still never writes to
  the real, projected filesystem; it only hardens how content is *read*
- Introduce a new dependency without explicit justification in the MR — check
  whether `libc` (already a direct dependency, used by C021's write-side
  primitives) is sufficient for `fdopendir`/`openat`-based reads before
  reaching for a new crate (`rustix`/`cap-std` are legitimate alternatives if
  you judge them meaningfully better, same as C021's fix was allowed to
  consider `cap-std` and chose not to — justify explicitly either way)
- Touch `orchestrator/*`, other services, or `deploy/*`/`schemas/*`
- Expand scope into write-back's actual sync logic, the rename-decomposition
  design (R-5, non-blocking, separate from this task), or C023's crash-safety
  work

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `docs/DEBT-REGISTER.md`
  (only the R-3 row)
- **Must NOT touch:** `orchestrator/*`, other services, deploy files, `schemas/*`

## Steps (execute in order)

1. Read C021's write-side fd-anchored primitives (`repo.rs`) in full — this is
   the pattern to generalize, not a new design.
2. Implement the fd-anchored recursive read walk (`fdopendir`/`openat` based),
   applied to both the reconciliation pass and the inotify handler's
   single-entry sync paths.
3. Add the deterministic pre-planted-symlink test (regression coverage) and
   the genuine concurrent race-stress test (the one that actually
   distinguishes old from new).
4. Verify the old code fails the race test, the new code passes it — the
   same before/after discipline as C021's SEC-001 fix.
5. Flip R-3 to `closed`/`fixed`.
6. Run verification.

## Expected outputs

- Fd-anchored, TOCTOU-closed read-side walk in both `repo.rs` and `writeback.rs`
- A regression test suite proving the race is closed (deterministic +
  race-stress, with before/after evidence)
- `docs/DEBT-REGISTER.md` row R-3 flipped to `closed`/`fixed`

## Acceptance criteria

1. Every read-side directory walk (reconciliation pass and inotify single-
   entry sync) is fd-anchored end-to-end — no step re-resolves a
   multi-component path by name after an earlier check
2. The existing symlink-skip / non-UTF-8-skip policy is unchanged in
   behavior, only in enforcement mechanism
3. A genuine concurrent race-stress test demonstrably fails against the
   pre-fix code and passes against the fix (before/after evidence in the MR)
4. `cargo test -p cwso-git-shadow` passes (existing 29 tests plus new ones)
5. `docs/DEBT-REGISTER.md` row R-3 shows `closed`/`fixed`, referencing C035

## Verification commands

```bash
cd services && rustup run 1.87.0 cargo test -p cwso-git-shadow
grep -n "R-3" ../docs/DEBT-REGISTER.md   # row shows closed/fixed, not open
```

## Git rails

- Branch: `agent/backend-developer/C035` from `develop` (rebased on merged C022)
- Commit: `fix(git-shadow): fd-anchor the write-back read-side scan (close R-3)`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
This task, like C022, is new security-relevant filesystem-interaction code and
will need **both Tech Lead and Security Engineer review**, the same bar C021
and C022 were held to — note this expectation in your MR rather than routing
only to Tech Lead.

## Note on sequencing with C023

This task and C023 (projection lifecycle + crash safety) both have broad
`services/cwso-git-shadow/**` file ownership over code C021/C022 already
touch. Do not dispatch both in parallel without the orchestrator's explicit
sign-off on the split — the established practice on this roadmap (see C021→
C022→C023 sequencing rationale in `docs/tasks/active-tasks.md`'s history) has
been to sequence, not parallelize, tasks with this much file-ownership overlap.

## Execution notes

<filled during execution>
