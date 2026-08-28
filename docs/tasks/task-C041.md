# Task C041 — Parent-commit tracking per workspace

**ID:** C041
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B7, P2-4); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

Shadow workspaces currently produce **orphan commits** — no parent tracking — so there
is no history chain and no basis for a three-way merge. Track HEAD per workspace and
pass the parent into `repo.commit`, so each workspace forms a real chain. This unblocks
C042 (three-way merge).

## Inputs

- `services/cwso-git-shadow/src/repo.rs:180` (the P2-4 orphan-commit marker)
- `services/cwso-git-shadow/src/main.rs` (workspace state: where HEAD should live)

## Rails (read before starting)

### You MUST
- Track the current HEAD commit per workspace (in the workspace state)
- Pass the current HEAD as the parent when committing; update HEAD after each commit
- Handle the first commit in a workspace (no parent — a legitimate root commit, not an "orphan" by accident)
- Add tests: two sequential commits in one workspace form a chain (`git log` shows parent linkage)
- Remove the P2-4 marker and update `docs/DEBT-REGISTER.md` (B7 → `fixed`, closing task C041)

### You MUST NOT
- Change the `commit_shadow` tool signature (parent is derived from workspace state, not a new parameter)
- Implement merging — that is C042, which consumes this task's parents
- Touch the merge-engine or orchestrator
- Break existing single-commit behavior (first commit still works)

## File ownership

- **May create/modify:** `services/cwso-git-shadow/**`, `docs/DEBT-REGISTER.md` (B7 row)
- **Must NOT touch:** `orchestrator/*`, other services, `schemas/*`

## Steps (execute in order)

1. Read `repo.rs` commit path and workspace state.
2. Add HEAD tracking; thread parent into commit.
3. Tests: chain of ≥2 commits; first-commit root case.
4. Remove marker; update DEBT-REGISTER.

## Expected outputs

- Parent-tracked commits in `cwso-git-shadow`
- Chain tests
- P2-4 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. `git log` in a shadow workspace shows a chain, not orphans
2. First commit is a proper root commit
3. `cargo test -p cwso-git-shadow` passes
4. DEBT-REGISTER B7 = `fixed` / C041

## Verification commands

```bash
cargo test -p cwso-git-shadow commit
grep -n "P2-4" services/cwso-git-shadow/src/repo.rs   # = no hits
```

## Git rails

- Branch: `agent/backend-developer/C041` from `develop`
- Commit: `fix(git-shadow): track parent commits per workspace`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.

## Execution notes

**Executed by:** backend-developer, 2026-08-27, in worktree
`/home/emage/Code/emage/worktrees/agent-backend-developer-C041`, branch
`agent/backend-developer/C041` off `origin/develop` @ `0bec0f7`.

### What changed

`services/cwso-git-shadow/src/repo.rs` only:

1. `Workspace` (line ~49) gained a new field, `head: Option<Oid>` — the oid of
   this workspace's current chain tip. Doc comment explains it is seeded from
   the base commit (if any) and advances on every successful `commit()`.
2. `ShadowStore::create` (line ~109) was changed to also capture the base
   commit's own oid (`base_commit`, alongside the existing `base_tree`) and
   seed `Workspace.head = base_commit` — so a workspace created from an
   existing commit chains its own first commit onto that base rather than
   rooting itself.
3. `ShadowStore::commit` (line ~310) was changed to: read `ws.head` before
   dropping the workspace-map lock; resolve it (if `Some`) to a real
   `git2::Commit` via `repo.find_commit`; pass that as the sole parent to
   `repo.commit` (or an empty parent list if `head` is `None`, i.e. this
   workspace's first-ever commit with no base); and, only after `repo.commit`
   succeeds, re-lock the workspace map and set `ws.head = Some(commit_oid)`.
   A missing workspace at that final step (raced against `drop_workspace`) is
   a harmless no-op, matching this module's existing race-handling pattern
   elsewhere (e.g. `wb_apply_write`).
4. The `POC-DEBT P2-4` marker comment and the old `let parents: Vec<git2::Commit>
   = vec![];` orphan-commit line were removed.
5. Four new tests added to `mod tests`: `first_commit_in_fresh_workspace_is_a_root_commit`,
   `sequential_commits_in_one_workspace_form_a_parent_chain`,
   `third_commit_continues_the_same_chain`, and
   `workspace_created_from_base_commit_chains_onto_it`.

`commit_shadow`'s IPC signature (`Request::Commit { workspace_uuid, message }`
in `src/proto.rs`, and the `dispatch` match arm in `repo.rs`) was **not**
touched — the parent is derived entirely from `Workspace.head`, per the
brief's "must not" rail. No merge logic was added; C042 still owns
three-way merge. `services/cwso-git-shadow/src/main.rs` was **not** touched —
the workspace state (including the new `head` field) lives entirely in the
`Workspace` struct in `repo.rs`; `main.rs` has no workspace-state code of its
own to update (confirmed by inspection: it contains no `Workspace`/`workspace`
struct or field references beyond doc comments).

### A note on the local toolchain

Building/testing locally with the default `rustc 1.86.0` fails with
`error[E0658]: use of unstable library feature 'inherent_str_constructors'`
inside `git2 0.21.0` — this is the exact, already-documented (T172,
`docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`) toolchain
requirement: this workspace requires Rust 1.87+. `rustup` already had `1.87`
installed locally, so all commands below were run via `rustup run 1.87 cargo
...`, matching what CI's `rust:1.87`/`rust:1.87-slim` images use. This is a
pre-existing environment fact, not something introduced or worked around by
this task.

While implementing, an initial version of the fix added an explicit
`drop(repo)` right after `repo.commit(...)` (to release the mutex before
re-locking `self.workspaces`); on 1.87 this failed to compile
(`error[E0505]: cannot move out of 'repo' because it is borrowed`) because
`tree` (a `git2::Tree<'_>` borrowed from `repo`) was still alive at that
point. Fixed by explicitly dropping `parent_commit` and `tree` before `repo`,
in that order, rather than relying on end-of-scope drop order (which would
have worked too, but the explicit drops keep the mutex hold time minimal and
document *why* the ordering matters).

### Verification (real output)

```
$ rustup run 1.87 cargo test -p cwso-git-shadow
...
running 40 tests
test repo::tests::first_commit_in_fresh_workspace_is_a_root_commit ... ok
test repo::tests::sequential_commits_in_one_workspace_form_a_parent_chain ... ok
test repo::tests::third_commit_continues_the_same_chain ... ok
test repo::tests::workspace_created_from_base_commit_chains_onto_it ... ok
... (36 other pre-existing repo::tests / ast::tests / writeback::tests / tests, all ok)
test result: ok. 40 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 3.07s

     Running tests/signal_shutdown.rs (target/debug/deps/signal_shutdown-...)
running 2 tests
test sigint_shuts_down_promptly_without_hanging_or_panicking ... ok
test sigterm_shuts_down_promptly_without_hanging_or_panicking ... ok
test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.06s
```

```
$ grep -n "P2-4" services/cwso-git-shadow/src/repo.rs
(no output — zero hits)
```

Additionally verified acceptance criterion 1 directly against a real, on-disk
bare repo with the `git` CLI (not just `git2`'s own parent-linkage API), via a
throwaway temporary test that was removed before the final commit:

```
$ git --git-dir=<tmp>/bare.git log --graph --oneline --parents <second-commit-oid>
* bce012d 027dd21 second
* 027dd21 first

$ git --git-dir=<tmp>/bare.git cat-file -p <second-commit-oid>
tree f0d28a23ada3ba875eed6896633eb104367325ee
parent 027dd217877f88b8d74a45d4111902190bed9666
...

$ git --git-dir=<tmp>/bare.git cat-file -p <first-commit-oid>
tree 855dfa3b6ea84d4cc77bd84e699dece076ecaed3
...
(no "parent" line — confirmed root commit)
```

`cargo fmt --all -- --check` (rustup 1.87) reports diffs at four locations in
`repo.rs` (lines 650, 731, 874, 1076, all in `open_root_dir`/
`materialize_write_via_fd_walk`/`open_scan_root_dir`/`scan_dir_into`) and six
in `writeback.rs` — confirmed via `git diff origin/develop -- repo.rs`'s hunk
list that every one of these lines falls **outside** every hunk this task
touched, and `writeback.rs` was not touched at all, so this is pre-existing
rustfmt-version drift in the repo (unrelated to this task), not something
introduced here. My one newly-added line that also drifted
(an `assert_eq!` in `first_commit_in_fresh_workspace_is_a_root_commit`) was
reformatted to match rustfmt's expected multi-line style before the final
commit.

### Acceptance criteria — status

1. `git log` in a shadow workspace shows a chain, not orphans — **met**, both
   via `git2::Commit::parent_id`/`parent_count` assertions in the new tests
   and via a direct `git log --graph --oneline --parents` CLI check above.
2. First commit is a proper root commit — **met**
   (`first_commit_in_fresh_workspace_is_a_root_commit`, and the CLI check
   above shows `first`'s `cat-file -p` output has no `parent` line).
3. `cargo test -p cwso-git-shadow` passes — **met**, 40 + 2 = 42/42 passing
   (output above).
4. `docs/DEBT-REGISTER.md` B7 = `fixed` / C041 — **met**.

### Blocker status

None.

---

## Addendum: SEV-C041-001 concurrency fix (post-review)

**Executed by:** backend-developer, 2026-08-27, same worktree/branch, one
additional commit on top of `b490148`.

### Finding

Independent security review of the C041 commit above identified and
adversarially reproduced (5/5 runs, 8-thread probe) a real, HIGH-severity
concurrency bug, tracked as **SEV-C041-001** and `docs/DEBT-REGISTER.md` row
**R-7**: unsynchronized concurrent `commit()` calls against the SAME
workspace could silently orphan a commit from the parent chain. The
`self.workspaces` lock only protected the workspace-map lookup itself
(briefly, at the start and end of `commit()`), not the full read-head →
build-tree → `repo.commit` → advance-head span. Two racing `commit()` calls
against the same `workspace_id` could both read the same stale `ws.head`,
both commit successfully against that same parent, and have whichever one
advanced `ws.head` last silently drop the other's commit from the chain
(still present in the git object database, but unreachable by walking
`parent_id` back from `head`, invisible to `git log`/any future
ancestor-walk — including C042's planned three-way merge).

This was flagged as needing a fix *before* C043 merges, not after: C043
removes the single global mutex in `orchestrator/internal/shadow/client.go`
(the thing that currently, incidentally, masked this bug by serializing all
shadow RPCs system-wide) via bounded connection pooling — so C043 landing
first would turn this bug from latent to live on the now-honestly-concurrent
dispatch path.

### Fix

`services/cwso-git-shadow/src/repo.rs`:

1. `ShadowStore` gained a new field, `commit_locks: Mutex<HashMap<Uuid,
   Arc<Mutex<()>>>>` — a per-workspace serialization side-table. Doc comment
   on the field explains the guarantee (same-workspace commits serialize,
   different-workspace commits never block each other) and the deliberate
   choice not to clean up stale entries on `drop_workspace` (UUIDs are never
   reused via `Uuid::new_v4`, so a stale entry can never be mistakenly
   acquired for a different, later workspace — it is an unbounded-but-
   negligible leak: one small `Arc<Mutex<()>>` per workspace ever created,
   for the life of the process, which is smaller than the cost of writing
   defensively-correct cleanup code against a race that cannot actually
   cause incorrect behavior).
2. `ShadowStore::new` initializes the new field alongside the existing ones.
3. `ShadowStore::commit` now looks up (or lazily inserts) this workspace's
   own `Arc<Mutex<()>>` entry under a brief lock on `commit_locks` itself,
   clones it, and immediately holds a guard on *that* clone (`_commit_guard`)
   for the rest of the function body — covering the full read-head →
   build-tree → `repo.commit` → advance-head span, including every early
   `?` return. The guard is released by `Drop` on every exit path.
4. `commit`'s doc comment was extended (not replaced) with a new paragraph
   describing the SEV-C041-001 guarantee, referencing `docs/DEBT-REGISTER.md`
   row R-7, matching this crate's existing convention for security-fix doc
   comments (compare to `materialize_write`'s SEC-001 paragraph and
   `scan_workspace_tree`'s R-3/C035 paragraph).

No other files were modified. `commit_shadow`'s IPC signature is unchanged;
no merge logic was added; `orchestrator/*` was not touched.

### Regression tests (new, in `repo.rs`'s `mod tests`)

1. **`concurrent_commits_against_one_workspace_never_lose_a_commit`** — the
   reproduced adversarial probe, made permanent: 8 threads, synchronized via
   a `std::sync::Barrier` so all 8 `commit()` calls against the SAME
   workspace genuinely overlap, looped over 20 independent iterations (fresh
   store/workspace per iteration) rather than run once. After every
   iteration, walks `parent_id`/`parent_count` from the workspace's final
   `head` back to the root, collecting every reachable oid, and asserts (a)
   all 8 threads' commit oids are in that reachable set and (b) the
   reachable set has exactly 8 members (a single linear chain, not merely
   "all present somewhere"). This asserts zero lost commits, not just "no
   panic".
2. **`concurrent_commits_against_different_workspaces_are_not_serialized`**
   — proves the fix does not regress cross-workspace concurrency: manually
   acquires and holds workspace `a`'s `commit_locks` entry (standing in for
   an in-flight `commit()` against `a`), then spawns a thread committing to
   a *different* workspace `b` and asserts it completes within a 5-second
   `recv_timeout` — if the fix had accidentally reintroduced a single global
   commit lock, this would hang and the timeout would fail the test.

### Before/after evidence (real output)

Ran the new adversarial test 5 times against the pre-fix code (fix
temporarily bypassed in-place with a `_commit_guard = ();` no-op, then
restored — the restored file was diffed byte-for-byte identical to the
pre-bypass state before the final commit, confirmed via
`diff repo.rs repo.rs.fixed.bak`):

```
$ rustup run 1.87 cargo test -p cwso-git-shadow --bin cwso-git-shadow \
    concurrent_commits_against_one_workspace_never_lose_a_commit -- --nocapture
(repeated 5 times, pre-fix code)

thread 'repo::tests::concurrent_commits_against_one_workspace_never_lose_a_commit' panicked at
cwso-git-shadow/src/repo.rs:1616:17:
iteration 0: thread 0's commit bae8eeb29e3fa80fb7e3923486c5fa784fffa085 is NOT reachable by
walking parent_id from the workspace's final head -- this is SEV-C041-001 (a concurrent commit
silently orphaned from the chain)
test repo::tests::concurrent_commits_against_one_workspace_never_lose_a_commit ... FAILED
test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 41 filtered out; finished in 0.02s
```
Result: **FAILED 5/5 runs**, always on iteration 0 (the very first barrier-
synchronized round of 8 concurrent commits).

Same command, 8 times, against the fixed code:

```
$ rustup run 1.87 cargo test -p cwso-git-shadow --bin cwso-git-shadow \
    concurrent_commits_against_one_workspace_never_lose_a_commit -- --nocapture
(repeated 8 times, post-fix code)

test repo::tests::concurrent_commits_against_one_workspace_never_lose_a_commit ... ok
test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 41 filtered out; finished in ~0.20s
```
Result: **ok 8/8 runs** (each run internally covers 20 iterations × 8
threads = 160 barrier-synchronized concurrent-commit rounds, so 1,280 total
probe rounds passed cleanly across the 8 isolated runs).

Full suite, fixed code:

```
$ rustup run 1.87 cargo test -p cwso-git-shadow
running 42 tests
... (all repo::tests, ast::tests, writeback::tests, tests, including the
     four pre-existing C041 tests and the two new SEV-C041-001 tests)
test result: ok. 42 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 3.04-3.12s

     Running tests/signal_shutdown.rs
running 2 tests
test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.04s
```

`cargo fmt -p cwso-git-shadow -- --check`: reports the same 10 pre-existing
diffs (4 in `repo.rs` at lines 734/815/958/1160, 6 in `writeback.rs`) as the
unmodified `b490148` baseline (confirmed by running the same check against a
`git stash` of this addendum's changes) — zero new diffs introduced. One new
diff this addendum's own test code initially introduced (a `recv_timeout`
chain in `concurrent_commits_against_different_workspaces_are_not_serialized`)
was reformatted to rustfmt's preferred single-line-call style before the
final commit.

`cargo clippy -p cwso-git-shadow --all-targets`: identical warning set
before and after (same 4 distinct pre-existing warnings — deprecated
`TempDir::into_path`, `ast.rs` recursion-only parameter, `main.rs`
`needless_return`, `items_after_test_module` — confirmed via the same
`git stash` A/B comparison). Zero new warnings.

### DEBT-REGISTER row added

`docs/DEBT-REGISTER.md` row **R-7** (Category: Concurrency; Status:
`closed`; Disposition: `fixed`; Closing task: `C041`) — see that file for
the full row text and its companion evidence note in "Notes on the `fixed`
rows".

### Cross-workspace concurrency confirmation

`concurrent_commits_against_different_workspaces_are_not_serialized`
(described above) passes, confirming a commit against one workspace never
blocks on an unrelated workspace's commit lock — the per-workspace side-
table design does not reintroduce the single-global-lock throttling problem
C043's connection pooling exists to remove.

### Acceptance criteria — status (addendum)

1. Race fixed per the non-negotiable property (same-workspace commits
   always serialize; different-workspace commits never block each other) —
   **met**.
2. Adversarial 8-thread regression test added, confirmed to fail reliably
   against pre-fix code and pass reliably against the fix — **met** (5/5
   pre-fix failures, 8/8 post-fix passes, see evidence above).
3. `commit`'s doc comment corrected to describe the concurrency requirement
   and reference SEV-C041-001 — **met**.
4. `docs/DEBT-REGISTER.md` row R-7 added, `closed`/`fixed`/`C041` — **met**.
5. Full C041 acceptance criteria plus the new regression test re-verified —
   **met** (42/42 unit tests + 2/2 signal-shutdown tests passing).
6. `cargo fmt`/`cargo clippy` show no new issues — **met** (byte-identical
   diff/warning sets to the unmodified baseline).
7. Cross-workspace concurrency not regressed — **met**
   (`concurrent_commits_against_different_workspaces_are_not_serialized`).

### Blocker status (addendum)

None.
