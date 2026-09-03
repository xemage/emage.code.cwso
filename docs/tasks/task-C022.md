# Task C022 — Write-back: filesystem mutations flow into the git ODB

**ID:** C022
**Owner:** backend-developer
**Status:** done
**Priority:** P0
**Depends on:** C021
**Created:** 2026-08-12
**Completed:** 2026-08-21
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

Added `services/cwso-git-shadow/src/writeback.rs` (`WriteBackEngine`, ~665
lines): `inotify`-driven write-back as the primary mechanism per ADR-012,
with a periodic hash-based reconciliation pass (`scan_workspace_tree`,
`repo.rs`) as the correctness backstop for `inotify`'s documented failure
modes (`IN_Q_OVERFLOW`; non-recursive watching). All four mutation types
made directly at the real projected path (bypassing this service's own IPC
entirely) now flow back into `Workspace.files`: create/modify via
`IN_CLOSE_WRITE`, delete via `IN_DELETE`, rename via independent
`IN_MOVED_FROM`+`IN_MOVED_TO` (no cookie correlation — see below). New
dependency: `inotify = { version = "0.11", default-features = false }`
(blocking API only, avoiding the crate's `tokio`-based async feature this
synchronous, thread-per-loop service doesn't need) — confirmed the only new
dependency. Found and fixed a real deadlock during development (not
shipped): an earlier draft shared one mutex between the blocking
event-reader thread and the watch-add/remove path, so any `create_workspace`
call would hang forever waiting on a lock the reader thread held
indefinitely inside its blocking read call; fixed by giving the reader
exclusive, unlocked ownership of the raw `Inotify` value and routing watch
add/remove through a separate `Mutex<inotify::Watches>` handle. Read-back
path safety: every entry's type determined via `symlink_metadata` (never
followed, skipped and logged, both in the reconciliation scan and the
single-file event handler); paths built from kernel-supplied `DirEntry`
components only, never a string-based traversal.

Three judgment calls explicitly flagged for reviewer disposition, not
decided unilaterally:

1. **Read-side TOCTOU (`R-3`)**: the `symlink_metadata` check and the
   subsequent read/recursion are two separate syscalls, not fd-anchored
   like the write side's `openat(..., O_NOFOLLOW)`.
2. **Rename handling**: independent delete+create rather than
   `inotify`-cookie-correlated pairs.
3. **Non-UTF-8 filenames (`R-4`)**: silently skipped — a pre-existing
   constraint (the `HashMap<String, Oid>` + JSON-string IPC protocol
   already couldn't represent such a path via `write_file` either),
   surfaced by this task, not introduced by it.

**VERDICT: CONDITIONAL_PASS** from both independent reviewers (Tech Lead +
Security Engineer, MR !153) — both conditions documentation/tracking-only,
no code-behavior change required.

Tech Lead endorsed the no-cookie-correlation rename design as sound, but
identified a real, narrow race the worker's own framing hadn't surfaced:
the delete-half and create-half are two separate critical sections, and a
`commit()` landing in the gap between them could observe the affected file
missing under *both* its old and new path — a transient full disappearance,
self-resolving on the next event or reconciliation tick. Condition:
document it. Applied directly: `writeback.rs`'s module doc comment gained a
new "Rename handling" section (tagged `POC-DEBT R-5`), and
`docs/DEBT-REGISTER.md` gained a new, non-blocking `v1.1` row (R-5) noting
a future hardening — batching same-tick, same-cookie `MOVED_FROM`/
`MOVED_TO` pairs into one atomic operation — as not required for v1.0.

Security Engineer confirmed R-3 is not exploitable against *today's* mount
topology (traced the actual topology directly: nothing outside `git-shadow`
itself currently has any path to a shadow workspace's projected directory)
but found the "same actor already has access" justification for accepting
it *permanently* rests on an assumption about C024's not-yet-built
sandbox-mount wiring — which ADR-012 itself names as an unsolved
mount-propagation problem — judged inconsistent with the blocking bar this
project already held the equivalent write-side gap to (SEC-001, HIGH, C021).
Condition: reclassify `docs/DEBT-REGISTER.md` row R-3 from `v1.1`/`wontfix`-
pending-review to **`v1.0-blocker`**, with a named follow-up task. Applied
directly: R-3's disposition text rewritten, and a new task brief written,
**`docs/tasks/task-C035.md`** — "fd-anchored recursive read-back walk
(harden R-3)" — generalizing C021's `openat`/`mkdirat` write-side primitives
to a tree-walking read via `openat`+`fdopendir`, which the reviewer
confirmed is tractable, not a novel problem, just more code than the linear
write-side walk. If the fix is deferred rather than implemented, the "same
actor" premise must be explicitly re-confirmed against C024's actual
mount-scoping implementation before v1.0 sign-off — recorded as a hard
requirement in R-3's row, not left implicit.

Both conditions applied in a single follow-up commit (doc-comment +
DEBT-REGISTER edits only, zero logic change) — verified with a clean
`cargo build` and the full 29/29 test suite still green before pushing, no
third review round needed since neither condition changed any code
behavior. CI green throughout except the same non-blocking `rust:lint`
fmt-nit class C021 hit (`allow_failure: true`, confirmed on the job itself;
root-caused via trace inspection as a pure whitespace diff — three
single-line match arms/one method chain rustfmt wants reformatted, zero
logic difference) and two further instances of this session's now-familiar
transient shared-runner container-name collisions (`cwso-git-shadow`,
`cwso-merge-engine` name conflicts under concurrent CI load), each
independently confirmed unrelated to this diff via trace inspection and
resolved by retrying the single affected job.

MR !153 (`agent/backend-developer/C022`), merged to `develop` via merge
commit `70ec89fd`. This is a good example of the security bar staying
consistent across the whole roadmap: R-3 could easily have been quietly
endorsed as an acceptable trade-off on the worker's own reasoning; it
wasn't, because that reasoning rested on a premise (future code C024
hasn't built yet) rather than a closed fact — the same standard already
applied to SEC-001 on the write side (C021). Unblocks **C023** (lifecycle +
crash safety — brief already corrected for the real materialise-to-tmpfs
mechanism, ready to dispatch) and **C035** (fd-anchored read-back
hardening, `v1.0-blocker`) — both P0, both sharing broad file ownership
with C021/C022's existing code, so sequence rather than parallelize per
this roadmap's established practice.
