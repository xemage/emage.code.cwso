# Task T193 — Fix `pathGuard()` symlink-escape gap for new-file writes

**ID:** T193
**Owner:** backend-developer
**Status:** done
**Priority:** P0 — blocking
**Depends on:** —
**Created:** 2026-08-16
**Completed:** 2026-08-16
**Based on:** Discovered by the C015 worker (task C015, dispatched to implement the
read-write workspace mount) while addressing security condition SEC-C019-01. The
worker treated this as a hard-stop per its brief's rail ("if a required property
cannot be met... STOP... report critical") rather than shipping a softened claim or
working around it, and reported a blocker instead of committing. C015 is paused
pending this fix. This finding, and the fix approach below, originate from that
worker's investigation; the orchestrator independently re-verified the vulnerable
code path against current source before writing this brief (see "Independent
verification" below).

## Objective

`orchestrator/internal/tools/fs_tools.go`'s `pathGuard()` is supposed to reject any
path that resolves (through symlinks) to somewhere outside the configured workspace
root — this is the in-process trust boundary that matters once C015 makes the
workspace mount read-write instead of read-only. It correctly does this for paths
that **already exist** on disk. It does **not** do this for paths that **do not yet
exist** — i.e. exactly the case of writing a *new* file — because of how it handles
`filepath.EvalSymlinks`' error return. Close this gap without touching the
already-correct existing-file behavior, and add a regression test for the specific
case that was missing one.

## The bug (verified against current source, `orchestrator/internal/tools/fs_tools.go`)

```go
// lines 34–47
clean := filepath.Clean(candidate)
rel, err := filepath.Rel(absRoot, clean)
if err != nil || strings.HasPrefix(rel, "..") {
    return "", fmt.Errorf("path %q escapes workspace root", targetPath)
}
// Resolve symlinks if target exists; reject if it escapes.
if resolved, err := filepath.EvalSymlinks(clean); err == nil {
    relResolved, err := filepath.Rel(absRoot, resolved)
    if err != nil || strings.HasPrefix(relResolved, "..") {
        return "", fmt.Errorf("path %q symlinks outside workspace root", targetPath)
    }
    return resolved, nil
}
return clean, nil
```

The `filepath.Rel`/`..`-prefix check (lines 35–38) is a **purely lexical** check on
the *unresolved* input string — it cannot detect a symlink. The actual symlink
defense is the `EvalSymlinks` block (lines 40–46) — but `EvalSymlinks` requires every
path component, including the leaf, to exist; if the leaf doesn't exist yet (a new
file), it returns an error, `err == nil` is false, the `if` body never runs, and
execution falls through to **line 47: `return clean, nil`** — handing back the
unresolved path with **no symlink check performed at all**.

`WriteFileSync.Execute` (`orchestrator/internal/tools/fs_tools.go`, tool name
`write_file_sync`, the only tool with write access — `AllowedRoles() []Role{RoleWorker}`)
calls `pathGuard`, then unconditionally `os.MkdirAll(filepath.Dir(safe), 0o755)` and
`os.WriteFile(safe, ...)` on whatever path `pathGuard` returned. If an intermediate
path component is a symlink pointing outside the workspace root (e.g.
`/workspace/escape -> /etc`) and the request targets a **new** file under it (e.g.
`path: "escape/pwned.txt"`), `pathGuard` returns the unresolved
`/workspace/escape/pwned.txt` unchecked, and `os.WriteFile` follows the OS-level
symlink resolution straight through it — writing to `/etc/pwned.txt`, fully outside
the workspace.

### Scope table (from the originating investigation, independently re-derivable from the code above)

| Scenario | Tool | Result today |
|---|---|---|
| Write a **new** file through a symlinked intermediate directory | `write_file_sync` | **Escapes** — the bug |
| **Overwrite an existing** file reached through a symlinked path | `write_file_sync` | Correctly rejected (leaf exists → `EvalSymlinks` succeeds → resolved path checked) |
| **Read** an existing file through a symlinked path | `read_file_sync` | Correctly rejected (same reason) |

Only the first row is broken. `read_file_sync` has no write side effect and is not
independently vulnerable, but T193's fix (below) must not weaken its existing-file
behavior either.

## Required fix

Resolve symlinks on the **nearest existing ancestor directory**, then re-join the
non-existing tail components, before applying the workspace-boundary check — instead
of giving up entirely when `EvalSymlinks(clean)` fails on a fully-nonexistent path.
Concretely: walk `clean` upward (`filepath.Dir`) until `EvalSymlinks` succeeds on an
ancestor (this will always terminate at `absRoot` itself, or fail entirely if
`absRoot` doesn't exist — treat that as an error, don't silently permit it), resolve
that ancestor, verify the resolved ancestor is inside `absRoot` (same check as the
existing-file branch), then rejoin the tail path components onto the resolved
ancestor and re-verify the final joined path is still inside `absRoot` (a resolved
ancestor could theoretically be a workspace-internal symlink that itself is fine, but
the rejoined tail should still be sanity-checked).

This is a rail, not a full implementation spec — implement it correctly and defend
the approach in the MR; the above is the shape the originating investigation
converged on, not a mandate to copy verbatim if you find a cleaner approach that
achieves the same guarantee (every new-file write path is symlink-resolved against
its nearest existing ancestor before being trusted).

## Rails (read before starting)

### You MUST
- Fix `pathGuard()` so a new-file write through a symlinked intermediate directory
  that points outside the workspace root is **rejected**, matching the existing
  behavior for reads/overwrites of existing files
- Add a regression test for exactly this case: attempt `write_file_sync` on a
  new (non-existent) path whose intermediate directory is a symlink pointing outside
  the configured workspace root; assert the call is rejected with a clear error, and
  assert no file was written outside the workspace
- Add/confirm a regression test that a **legitimate** new-file write (no symlinks
  involved, or a symlink that stays inside the workspace) still succeeds — do not
  fix the escape by breaking the common case
- Keep the existing-file behavior (both read and overwrite) unchanged — do not modify
  the `if resolved, err := filepath.EvalSymlinks(clean); err == nil { ... }` branch's
  logic beyond what's strictly needed to share code with the new ancestor-resolution
  path, if you choose to share code
- Add a CHANGELOG `## Unreleased` entry
- Run the full existing `fs_tools` test suite and confirm nothing regresses

### You MUST NOT
- Weaken the workspace-boundary check for existing files to fix this
- Add a workaround at the caller level (`WriteFileSync.Execute`) instead of fixing
  `pathGuard()` itself — other current or future callers of `pathGuard()` need the
  same guarantee, not just this one call site
- Touch `sandbox/**`, `deploy/docker-compose.yml`, or any of C015's in-flight files —
  this is `orchestrator/internal/tools/fs_tools.go` application code, out of C015's
  (devops-engineer) file-ownership lane entirely, which is exactly why this is a
  separate task with a different owner

## File ownership

- **May create/modify:** `orchestrator/internal/tools/fs_tools.go`,
  `orchestrator/internal/tools/fs_tools_test.go` (or wherever the existing test file
  for this package lives — check first), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `sandbox/**`, `deploy/docker-compose.yml`,
  `docs/artifacts/sandbox-trustworthiness-v1.md` (C019's artifact — immutable
  evidence record, do not edit it even though this finding relates to it), any C015
  file

## Acceptance criteria

1. New-file write through a workspace-external symlinked intermediate directory is
   rejected (new regression test proves this)
2. Legitimate new-file writes (no escaping symlink involved) still succeed (test
   proves this)
3. Existing-file read/overwrite behavior through symlinks is unchanged (existing
   tests still pass)
4. Full `fs_tools` package test suite passes
5. `git diff --stat` touches only the files listed under "File ownership"

## Verification commands

```bash
cd orchestrator
go test ./internal/tools/... -run TestPathGuard -v
go test ./internal/tools/... -v
go vet ./internal/tools/...
```

## Git rails

- Branch: `agent/backend-developer/T193` from `develop`
- Commit: `fix(tools): resolve symlinks on nearest existing ancestor in pathGuard for new-file writes`
- MR target: `develop`, squash and merge, delete source branch
- **This MR requires security-engineer review** (not just Tech Lead) — this is a fix
  to a path-confinement/trust-boundary security control, same review-depth pattern as
  C019. Note this requirement explicitly in the MR description.

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the ancestor-walk approach has its own edge case you can't close cleanly (e.g. a
TOCTOU race between the check and the actual write — note whether this is a realistic
concern given the single-process, single-workspace-per-orchestrator-instance model, or
a real gap), report it rather than shipping a fix with a known residual hole silently
undocumented.

## Execution notes

Implemented `resolveNearestExistingAncestor()`: walks upward via `filepath.Dir`
until an ancestor directory exists, resolves symlinks there via `EvalSymlinks`,
fails closed (returns an error, does not fall through) on any `EvalSymlinks` error
that isn't simply "the leaf doesn't exist yet" (e.g. a permissions error). The
resolved ancestor and the rejoined non-existent tail are each independently
re-verified against the workspace root via a new shared `withinWorkspace()` helper,
which the existing-file branch was refactored to use too (logic unchanged, just
de-duplicated). 6 new regression tests added, including a genuine before/after
demonstration: stashed the fix, reran the new tests against the old code (3 failed,
confirmed via a throwaway repro that a real file was written outside the workspace),
restored the fix, all pass. Full `internal/tools` package: 60/60 tests, `go vet`/
build clean.

Independent security-engineer review (MR !126) returned **CONDITIONAL_PASS with no
conditions on this task's own scope** — every claim independently re-derived rather
than trusted: reproduced the before/after test failure/pass split directly (not just
read the worker's transcript), walked the actual `resolveNearestExistingAncestor()`
code for correct termination and fail-closed behavior, confirmed the double
containment re-check on the rejoined path, confirmed existing-file behavior is
provably equivalent at the code level, and independently reran the full suite.

The review surfaced one new structural finding not part of this task's own scope: a
TOCTOU gap between `pathGuard()`'s check and each caller's later filesystem
operation. The reviewer gave a more precise reason it's not exploitable *today* than
the original "single-writer" framing this task first proposed: no tool in CWSO's
current MCP surface can create a symlink at runtime at all (independently confirmed —
no `os.Symlink` call anywhere in the package), so there's no reachable primitive to
exploit the race window with, regardless of writer concurrency. The reviewer
explicitly flagged that precondition as fragile and exactly what C015's read-write
mount removes. Tracked forward as **T194**, a new blocking task — not closed here,
and this fix is not weakened or reworked because of it (T194 builds on top).

Merged to `develop` 2026-08-16 (squash), MR !126.
