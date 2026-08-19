# Task T194 — Close the TOCTOU gap between `pathGuard()` and its callers' filesystem ops

**ID:** T194
**Owner:** backend-developer
**Status:** done
**Priority:** P0 — blocking
**Depends on:** T193 (merged — this task builds on its fix, does not replace it)
**Created:** 2026-08-16
**Completed:** 2026-08-16
**Based on:** Discovered by the T193 worker while fixing the `pathGuard()` symlink-escape
gap (task T193, MR !126), and independently assessed by the security-engineer
reviewer of that MR. This task carries forward the reviewer's corrected framing of
the risk, not the original worker's framing — see "Why this is P0 and blocking, not
backlog" below.

## Objective

`pathGuard()` (fixed in T193) returns a filesystem path it has verified, at the
moment of the check, resolves inside the workspace root. Every caller then performs
its actual filesystem operation **afterward**, as a separate step:

- `WriteFileSync.Execute` (`orchestrator/internal/tools/fs_tools.go:208-226`): calls
  `pathGuard()` at line 219, then `os.MkdirAll(filepath.Dir(safe), 0o755)` at line
  223 and `os.WriteFile(safe, ...)` at line 226
- `ReadFileSync.Execute` (lines 149-174): calls `pathGuard()` at line 159, then
  `os.Stat(safe)` at line 164 and `os.ReadFile(safe)` at line 174
- `ListDir.Execute` (lines 257-273): calls `pathGuard()` at line 269, then
  `os.ReadDir(safe)` at line 273

This is a classic **check-then-use (TOCTOU) race**: nothing prevents the filesystem
state at `safe` from changing between `pathGuard()` returning it and the operation
that actually touches it. If a symlink is swapped into place at (or along) `safe`'s
path in that window — even a `safe` path that `pathGuard()` correctly verified
contained no symlink at check time — the subsequent `os.WriteFile`/`os.ReadFile`/
`os.ReadDir`/`os.MkdirAll` call will follow the OS's live symlink resolution at
execution time, not the resolution `pathGuard()` saw a moment earlier.

## Why this is P0 and blocking, not backlog

**Use the corrected framing below, not "single-writer, so not exploitable today."**
The T193 worker's original assessment reasoned this away as low-risk because CWSO's
current tool-call model doesn't have concurrent writers racing each other. The
security-engineer reviewer of MR !126 explicitly flagged that framing as fragile and
gave the more precise reason the gap isn't exploitable **today**: **nothing in the
current MCP tool surface can create a symlink inside the workspace at runtime** — this
task's own independent grep confirms it (`grep -rn "Symlink" orchestrator/internal/tools/*.go`
finds no `os.Symlink` call or equivalent anywhere in the package). A TOCTOU race is
inert if the attacker has no primitive to swap in the malicious symlink in the first
place, regardless of how many writers are or aren't concurrent.

**That precondition is exactly what C015 removes.** C015 (currently `blocked`,
waiting on this task) mounts the user's real repository read-write into the
workspace. Once that lands, the workspace can contain symlinks the user's own
tooling created outside of CWSO's control (a build system, a git hook, an editor, a
`node_modules` symlink, anything) — CWSO no longer has to create the symlink itself
for the race window to become reachable. This is why the fix must land **before**
C015 proceeds, the same gating relationship T193 had.

## Required fix

Close the check-then-use window using one of (pick the correct approach for Go's
actual capabilities, don't assume both are equally available — investigate and
justify your choice in the MR):

1. **Preferred, if practical in Go's standard library / available dependencies**:
   anchor the post-`pathGuard()` filesystem operation to a file descriptor captured
   at check time using `*at()`-family syscalls with `O_NOFOLLOW` (e.g.
   `unix.Openat`/`os.OpenFile` combined with `O_NOFOLLOW` on the final path
   component, or opening the containing directory and using `*at` operations
   relative to that directory's fd) so the kernel — not a second, separately-timed
   `Stat`/`Open` call — is what refuses to follow a symlink planted after the check.
   Investigate what's realistically available: Go's `os` package added `O_NOFOLLOW`-
   adjacent behavior and `syscall`/`golang.org/x/sys/unix` expose the `*at` family on
   Linux/Darwin; confirm what CWSO's actual deployment targets need (check
   `deploy/Dockerfile.orchestrator`'s base image OS) before committing to a specific
   syscall surface.
2. **Fallback, if (1) isn't practical for one or more of the three call sites**:
   re-verify containment inside the same effective critical section immediately
   before the write/read/list — i.e. re-run the symlink-resolution check on the
   already-opened file/directory (not a fresh path-based `Stat`, which reopens the
   TOCTOU window) immediately before use, minimizing but not eliminating the window,
   and document explicitly which call sites got the strong (1) fix vs. the weaker
   (2) mitigation and why.

Apply the chosen fix(es) to all three call sites (`WriteFileSync`, `ReadFileSync`,
`ListDir`) — the race exists in all three, not just the write path, even though
write is the highest-consequence one.

### Testing requirement

A true TOCTOU race is inherently hard to deterministically prove with a unit test
(it depends on timing a filesystem change to land inside a window that a correct fix
should make arbitrarily small or nonexistent). You are not required to produce a
flaky race-condition integration test to call this done. Provide **whichever of the
following your own engineering judgment says is appropriate, but not an untested
claim**:

- A regression test that directly exercises the *at-anchored / re-verified code
  path and proves it rejects a symlink that's already in place before the operation
  (this is not itself a TOCTOU test, but it proves the new code path is reachable
  and correct on its own terms — necessary but not sufficient)
- A best-effort race test if you judge one can be written without excessive flakiness
  (e.g. a goroutine that swaps a symlink in a tight loop while the main test
  repeatedly calls the tool, asserting no escape is ever observed over N iterations)
  — acceptable as supporting evidence, not as the sole proof
- A written reasoning proof in the MR description walking through why the specific
  syscall/fd-anchoring approach you chose closes the window (e.g. "the kernel
  resolves `*at()` calls atomically relative to the directory fd captured before any
  attacker-controlled window existed, so there is no second lookup for an attacker
  to race")

## Inputs

- `orchestrator/internal/tools/fs_tools.go` (current state, post-T193 — `pathGuard()`,
  `withinWorkspace()`, `resolveNearestExistingAncestor()`, and the three caller
  `Execute` methods)
- `orchestrator/internal/tools/fs_tools_test.go` (T193's existing regression tests —
  do not regress these)
- `deploy/Dockerfile.orchestrator` (confirm the actual runtime OS/kernel to scope
  which syscall family is realistically available)
- `docs/tasks/task-T193.md` (the prior fix this task builds on)

## Rails (read before starting)

### You MUST
- Close (or, for the documented fallback case, meaningfully narrow and clearly
  document) the check-then-use window for all three `pathGuard()` call sites
- Preserve T193's fix and all of its existing passing tests — this task extends the
  defense, it does not replace or weaken the symlink-resolution logic T193 added
- Investigate and justify your chosen syscall/approach against CWSO's actual
  deployment target rather than assuming a syscall family is available
- Provide test/proof evidence per the "Testing requirement" above — pick the
  appropriate one(s) and justify the choice, don't skip evidence entirely
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Weaken or remove T193's symlink-resolution fix
- Claim the race is "closed" without either a working `*at()`-anchored
  implementation or an explicit, honest statement of what residual window remains
  under the fallback approach
- Touch `sandbox/**`, `deploy/docker-compose.yml`, or any C015 file — this remains
  `orchestrator/*` application code, out of C015's devops-engineer lane

## File ownership

- **May create/modify:** `orchestrator/internal/tools/fs_tools.go`,
  `orchestrator/internal/tools/fs_tools_test.go`, `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `sandbox/**`, `deploy/docker-compose.yml`,
  `docs/artifacts/sandbox-trustworthiness-v1.md`, any C015 file

## Acceptance criteria

1. All three `pathGuard()` call sites (`WriteFileSync`, `ReadFileSync`, `ListDir`)
   have the check-then-use window closed or explicitly, honestly documented as
   narrowed-not-eliminated with a stated residual risk
2. T193's existing regression tests still pass unmodified in behavior
3. Test/proof evidence provided per the "Testing requirement" section
4. `git diff --stat` touches only the files listed under "File ownership"

## Verification commands

```bash
cd orchestrator
go test ./internal/tools/... -v
go vet ./internal/tools/...
go build ./...
```

## Git rails

- Branch: `agent/backend-developer/T194` from `develop`
- Commit: `fix(tools): close TOCTOU window between pathGuard and filesystem operations`
- MR target: `develop`, squash and merge, delete source branch
- **This MR requires security-engineer review** (not just Tech Lead) — same pattern
  as T193 and C019. Note this requirement explicitly in the MR description.

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If neither the `*at()`-anchored approach nor a meaningfully-narrowed fallback proves
achievable for a given call site within reasonable effort, report `technical` /
`critical` with your findings — do not ship a call site with an undocumented,
unaddressed race, and do not let C015 proceed past this task with a known-open gap
that this task's own investigation couldn't close.

## Execution notes

Implemented the preferred `*at()`-anchored approach from the brief: `secureResolveDirs`/
`secureOpenLeaf` (naming approximate — see final source) walk the already-verified
canonical path one directory component at a time via `openat(2)`, each hop anchored
to the file descriptor obtained by the *previous* hop, with `O_NOFOLLOW` on every
hop including the final one (`O_DIRECTORY|O_NOFOLLOW` for intermediate directories).
If a symlink is swapped into any component after `pathGuard`'s check but before this
walk runs, the corresponding `openat()` call is refused by the kernel at that exact
hop — there is no remaining independently-timed top-down name lookup for a race to
win against. Applied to all three call sites (`WriteFileSync`, `ReadFileSync`,
`ListDir`). New regression tests for symlink-at-intermediate-component and
symlink-at-leaf rejection, plus a best-effort concurrent race test. All of T193's
existing tests unmodified and passing. Full module `go test ./... -race` clean,
`gofmt`/`vet`/build clean.

Requires `//go:build linux` (confirmed: Go's stdlib exposes `syscall.Openat`/
`Mkdirat` only on Linux; no equivalent on Darwin, none at all on Windows) — the
entire `fs_tools.go` file, not just the new logic, carries this tag. Cross-compilation
confirmed this breaks native non-Linux builds of the whole `orchestrator` module
(`orchestrator/internal/server/server.go` references these types unconditionally),
though CI and `deploy/Dockerfile.orchestrator` (both `golang:1.25-alpine`, Linux-only)
are entirely unaffected.

Independent security-engineer review (MR !127) returned **CONDITIONAL_PASS, no
blocking conditions on the merge itself**: re-derived the fd-anchoring chain
line-by-line for all three call sites (confirmed no hop silently falls back to a
fresh path-based lookup, which would have reintroduced the race), independently
reran the full test/vet/build/`-race` suite, confirmed T193's fix and tests
genuinely unmodified. Per the orchestrator's process, the platform trade-off (Linux-
only fix vs. portable fallback) was explicitly held as an open decision point pending
the reviewer's recommendation rather than folded into the pass/fail verdict — the
reviewer recommended shipping the Linux-only fix as-is (CI/production unaffected)
and tracking the portable fallback separately, non-blocking; the human accepted that
recommendation. Fast-follow tracked as **T195** (P1, not blocking C015 or this task).

Merged to `develop` 2026-08-16 (squash), MR !127. **C015 is now fully unblocked**
(both T193 and T194 satisfied) and was resumed immediately after this merge.
