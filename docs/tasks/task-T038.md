# Task T038 — Phase 3 Coverage Boost

**Status:** in_progress  
**Owner:** backend-developer  
**Priority:** P0  
**Depends on:** T037  
**Created:** 2026-05-15

---

## Objective

Increase orchestrator Go test coverage before Phase 4 by adding focused unit tests in low-coverage, high-impact packages, while preserving behavior and keeping the suite deterministic.

## Inputs

- `docs/plans/plan-T038-phase3-coverage-boost.md`
- Current baseline from local run:
  - total coverage ~60.5%
  - low packages: `internal/tools` (~34.4%), `internal/transport` (~58.3%), `internal/server` (~56.0%), `internal/logging` (0%), `internal/mcp` (0%)

## Scope

1. Add/extend tests in:
   - `orchestrator/internal/tools`
   - `orchestrator/internal/transport`
   - `orchestrator/internal/server`
   - `orchestrator/internal/logging`
   - `orchestrator/internal/mcp`
2. Keep tests fast and deterministic.
3. Run full regression + coverage and report delta.

## Acceptance Criteria

- [ ] `go test ./...` passes
- [ ] `go test ./... -coverprofile=coverage.out` passes
- [ ] Total Go coverage increases materially (target >=72%)
- [ ] No behavior regressions
- [ ] CI-ready commit with concise test-focused changes

## Expected Outputs

- New/updated `*_test.go` files in the target packages
- Coverage summary showing before/after
- Commit ready for MR
