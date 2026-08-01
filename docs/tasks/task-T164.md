# Task T164 - Fix flaky TestNodeRegistry_AssignTask (round-robin non-determinism)

- **Status:** done
- **Completed:** 2026-07-27
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** —
- **Based on:** `docs/plans/plan-v0.5.0-release.md`, CI pipeline #2708251138 job #15548635752

## Context

`go:test` fails intermittently on `develop`. This blocks the v0.5.0 release.

Observed CI failure:
```
--- FAIL: TestNodeRegistry_AssignTask (0.00s)
    node_registry_test.go:39: assignments: map[executor-2:4]
    node_registry_test.go:40: expected tasks assigned to at least 2 nodes
FAIL  github.com/emage/cwso/orchestrator/internal/rollout
```

## Root cause (already diagnosed — do not re-investigate)

`AssignTask` calls `getActiveNodesLocked()`, which builds its slice by ranging over the
`nr.nodes` **map**. Go randomises map iteration order, so the slice order differs on every
call. `activeNodes[nr.lastAssignedIdx%len(activeNodes)]` therefore indexes into a shuffled
slice and can return the same node repeatedly. Round-robin is only correct if the slice
order is stable.

## Objective

Make node ordering deterministic so round-robin actually rotates across nodes.

## Scope — files you MAY edit

- `orchestrator/internal/rollout/node_registry.go`

## Files you MUST NOT edit

- `orchestrator/internal/rollout/node_registry_test.go` (the test is correct; do not weaken it)
- Any file outside `orchestrator/internal/rollout/`
- `.gitlab-ci.yml`, `go.mod`, `Cargo.toml`, any `docs/**`

## Required change

In `orchestrator/internal/rollout/node_registry.go`:

1. Add `"sort"` to the import block (keep imports gofmt-grouped).
2. In `getActiveNodesLocked()` (around line 95-104), after the loop that appends active
   nodes and **before** `return active`, sort the slice by `NodeID`:

```go
	sort.Slice(active, func(i, j int) bool { return active[i].NodeID < active[j].NodeID })
	return active
```

Do not change `AssignTask`, `lastAssignedIdx`, locking, or any struct definition.

## Verification — run exactly these, all must pass

```bash
cd orchestrator
gofmt -l ./internal/rollout/                      # must print nothing
go vet ./internal/rollout/                        # must exit 0
go test -race -count=20 ./internal/rollout/ -run TestNodeRegistry
go test -race ./...                               # full suite, must exit 0
```

`-count=20` is mandatory: a single pass does not prove the flake is gone.

## Git workflow

```bash
git checkout develop && git pull origin develop
git checkout -b bugfix/T164-flaky-node-registry-assign
# ... make the edit, run verification ...
git add orchestrator/internal/rollout/node_registry.go
git commit -m "fix(rollout): sort active nodes for deterministic round-robin assignment

Map iteration order randomised the active-node slice, so round-robin
could assign every task to the same node. Sort by NodeID.

Refs T164"
git push origin bugfix/T164-flaky-node-registry-assign
glab mr create --source-branch bugfix/T164-flaky-node-registry-assign \
  --target-branch develop --title "fix(rollout): deterministic round-robin node assignment (T164)" \
  --description "Fixes flaky TestNodeRegistry_AssignTask. Refs T164." --yes
```

Do **not** push to `develop` or `main` directly — both are protected.

## Acceptance Criteria

- [ ] `sort.Slice` by `NodeID` added to `getActiveNodesLocked()`
- [ ] Test file unchanged (`git diff --name-only` shows only `node_registry.go`)
- [ ] `go test -race -count=20 ./internal/rollout/ -run TestNodeRegistry` passes
- [ ] `go test -race ./...` passes
- [ ] `gofmt -l ./internal/rollout/` prints nothing
- [ ] MR opened to `develop`; CI pipeline green on all 11 jobs
- [ ] MR merged; a fresh `develop` pipeline is green

## STOP conditions — halt and report, do not improvise

- `-count=20` still shows a failure → report the full output; do not relax the assertion.
- Full suite fails in a package other than `rollout` → report; that is out of scope.
- CI fails on a job other than `go:test` → report; do not touch CI config.
