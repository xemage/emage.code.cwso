# Task T195 — Portable (`!linux`) fallback for `fs_tools.go`'s fd-anchored path guard

**ID:** T195
**Owner:** backend-developer
**Status:** pending
**Priority:** P1 — not blocking C015 or T194
**Depends on:** T194 (merged — this task adds a counterpart, does not modify T194's Linux path)
**Created:** 2026-08-16
**Completed:** —
**Based on:** Fast-follow from T194 (MR !127), per the security-engineer reviewer's
platform-trade-off recommendation on that MR (accepted by the human: ship the
Linux-only fd-anchored fix for T194 itself, since CI/production are unaffected, and
track the non-Linux fallback as a separate, non-blocking task — this one).

## Objective

`orchestrator/internal/tools/fs_tools.go` (T194) is now gated `//go:build linux`
for its entire contents — not just the fd-anchored TOCTOU-closing logic, but the
whole file: `pathGuard`, `ReadFileSync`, `WriteFileSync`, `ListDir`, and every helper.
This is because the fd-anchoring approach needs `syscall.Openat`/`syscall.Mkdirat`,
which Go's standard library only exposes on `GOOS=linux`.

**Verified independently (not just per T194's own doc comment):**
`orchestrator/internal/server/server.go` references `tools.ReadFileSync`,
`tools.WriteFileSync`, and `tools.ListDir` **unconditionally, with no build tag**. So
on any `GOOS != linux` target, `go build ./...` for the entire `orchestrator` module
fails outright (`undefined: tools.WriteFileSync` etc.) — this isn't a narrower
"one file doesn't compile" problem, it's a whole-module build break. Today this is
scoped correctly (CI and `deploy/Dockerfile.orchestrator` both build exclusively on
Linux), but it means there is currently **no way to build or run the orchestrator
module natively on macOS or Windows at all**, and — the sharper finding from T194's
review — **the entire T193+T194 regression test suite silently disappears from
`go test ./...` on non-Linux machines**, since the test file carries the same build
tag. A developer running the full suite on a non-Linux laptop gets a clean "PASS"
with zero visibility that an entire security-critical test file was skipped.

Add a `//go:build !linux` counterpart implementing the same tool surface using the
**portable fallback approach** T194's brief originally offered as an alternative:
re-verify path containment/symlink-safety immediately before each filesystem
operation (a narrower TOCTOU guarantee than T194's fd-anchoring — the window is
minimized, not closed by kernel-enforced atomicity), with its own regression test
suite so `go test ./...` on non-Linux actually exercises equivalent coverage instead
of silently dropping it.

## Rails (read before starting)

### You MUST
- Create a new file (suggested: `orchestrator/internal/tools/fs_tools_portable.go`,
  or `fs_tools_other.go` — your call on naming, follow existing Go convention for
  build-tag file pairs in this codebase if one exists) tagged `//go:build !linux`
  that defines the **same exported surface** as `fs_tools.go`'s Linux build:
  `pathGuard` (or an equivalent internal function with the same effective contract),
  `ReadFileSync`, `WriteFileSync`, `ListDir` — same struct shape (`Workspace string`
  field), same `Name()`/`Description()`/`InputSchema()`/`AllowedRoles()`/`Execute()`
  methods, same tool names (`read_file_sync`, `write_file_sync`,
  `list_dir`/whatever `ListDir.Name()` actually returns — check it), so
  `orchestrator/internal/server/server.go` compiles unmodified against either build
- Reuse T193's symlink-resolution logic (`resolveNearestExistingAncestor`,
  `withinWorkspace`, or portable equivalents) — do not regress the T193 fix on
  non-Linux, only the TOCTOU-closure mechanism differs between the two builds
- Implement the "re-verify immediately before use" pattern: after `pathGuard`
  returns a resolved-safe path, re-run the symlink/containment check as close as
  practically possible to the actual `os.Open`/`os.ReadFile`/`os.WriteFile`/
  `os.MkdirAll`/`os.ReadDir` call, minimizing (not eliminating — be honest about
  this) the check-then-use window
- Add an explicit doc comment on the portable file stating plainly that this
  build's TOCTOU guarantee is **narrower** than the Linux fd-anchored build's — a
  symlink swap landing exactly inside the shortened re-verify-to-use window is still
  theoretically possible here, whereas T194's Linux build closes it via kernel-level
  atomicity. Do not let this comment be buried or softened
- Add a full regression test suite for the portable build (new or shared test file,
  your call, but it must actually run under `!linux` — the whole point is closing
  the "test suite silently disappears" gap), covering at minimum the same symlink-
  at-component and symlink-at-leaf rejection cases T194 added for Linux, run against
  the portable implementation
- Add a CHANGELOG `## Unreleased` entry noting both the new portable build and the
  explicit, narrower guarantee it provides
- Verify actual cross-compilation: `GOOS=darwin go build ./...` (or whatever
  non-Linux target is practical in this environment) succeeds where it previously
  failed, and `GOOS=darwin go vet ./...` is clean

### You MUST NOT
- Modify anything in the existing `fs_tools.go` (T194's Linux build) — this is a
  new, additive counterpart file, not a rework of the existing one
- Claim the portable build provides the same guarantee as the Linux build — it does
  not, and the doc comment must say so plainly
- Touch `sandbox/**`, `deploy/docker-compose.yml`, or any C015 file

## File ownership

- **May create/modify:** `orchestrator/internal/tools/fs_tools_portable.go` (or your
  chosen filename, new), a corresponding test file (new), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/internal/tools/fs_tools.go`,
  `orchestrator/internal/tools/fs_tools_test.go` (T194's Linux build and its tests —
  read-only reference, do not modify), `sandbox/**`, `deploy/docker-compose.yml`,
  any C015 file

## Acceptance criteria

1. `GOOS=linux go build ./...` (the existing, CI/production path) is completely
   unaffected — same behavior as before this task
2. `GOOS=darwin go build ./...` (or an equivalent non-Linux target actually
   achievable in this environment) succeeds, where it previously failed
3. A non-Linux `go test ./internal/tools/...` run actually exercises symlink-
   rejection coverage (not zero tests silently skipped)
4. The portable build's doc comment plainly states its narrower TOCTOU guarantee
5. `git diff --stat` touches only new files plus `CHANGELOG.md`

## Verification commands

```bash
cd orchestrator
GOOS=linux go build ./...    # unaffected, must still succeed
GOOS=linux go vet ./...
GOOS=darwin go build ./...   # must now succeed (previously failed)
GOOS=darwin go vet ./...
# Run the portable test suite under an actual non-Linux GOOS if your environment
# allows cross-execution (e.g. via an emulator/CI matrix); at minimum confirm the
# portable file's tests compile under GOOS=darwin build and, if you can't execute
# them cross-platform in this environment, say so explicitly in the MR and explain
# how they were validated (e.g. temporarily removing the linux build tag locally to
# run them, or another concrete method — not just "should work")
```

## Git rails

- Branch: `agent/backend-developer/T195` from `develop`
- Commit: `feat(tools): add portable !linux fallback for fs_tools path guard`
- MR target: `develop`, squash and merge, delete source branch
- Security-engineer review is **not strictly required** for this task (narrower/
  weaker mitigation only, does not touch the Linux path T194 already had reviewed) —
  use your own judgment on whether Tech Lead review alone is sufficient, or whether
  the portable implementation's own correctness warrants a security pass; state your
  reasoning in the MR if you think it needs one

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If cross-compilation or cross-platform test execution isn't achievable at all in
this environment, report `technical` / `minor` (this is a portability nice-to-have,
not blocking anything) with what you could verify and what you couldn't, rather than
claiming untested coverage works.

## Execution notes

<filled during execution>
