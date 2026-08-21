# Task C024 — Prove it: sub-agent E2E against a real path, in CI

**ID:** C024
**Owner:** qa-engineer
**Status:** in_progress
**Priority:** P0
**Depends on:** C022, C023
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C024 row, §1.5); docs/plans/plan-cwso-v1.0-phase2-real-filesystem-v1.md

## Objective

Prove the projection end-to-end, in CI: a sub-agent runs `ls`, `cat`, and a real test
command against a shadow workspace at a real path, edits a file with an ordinary
editor, and `commit_shadow` captures the change. This is the evidence that CG2 closed.

## Inputs

- C021–C023 implementation
- `scripts/cwso-smoke-test.sh` (C018 — extend, don't duplicate)
- `schemas/` tool shapes

## Rails (read before starting)

### You MUST
- Add a CI job that: creates a shadow workspace → asserts a shell can `cd` into the projected path and run `ls`/`cat` → runs a real test command (e.g., `python3 -m pytest` on a fixture, or `go test` on a Go fixture) → edits a file via a non-CWSO editor (`sed`/`tee` from the shell) → calls `commit_shadow` → asserts the commit contains the edit
- Use fixtures covering at least one of the four wired grammars (Go, Python, Rust, TypeScript)
- Keep the test hermetic: fixtures under the service's test directory, no network
- Fail loudly with the shell transcript on any assertion failure

### You MUST NOT
- Mock the filesystem or the commit — real path, real edit, real commit
- Weaken an assertion to make CI green — a failure here is a Phase-2 product bug
- Duplicate C018's smoke test — extend it or add a sibling script/job
- Touch application code

## File ownership

- **May create/modify:** `scripts/` (new or extended test script), test fixtures under `services/cwso-git-shadow/tests/` or `scripts/fixtures/`, `.gitlab-ci.yml` (add one job)
- **Must NOT touch:** `orchestrator/*`, `services/*/src/*`, `deploy/*`

## Steps (execute in order)

1. Read C018's smoke test and the C021–C023 implementation surface.
2. Write the E2E script/fixtures.
3. Run locally against the real stack until green.
4. Add the CI job; confirm it runs in the pipeline.

## Expected outputs

- E2E test script + fixtures
- CI job proving the projection

## Acceptance criteria

1. CI job green: shell `cd` + `ls`/`cat` + real test command + editor edit + `commit_shadow` captures the edit
2. Runs in CI, not only by hand
3. No mocks

## Verification commands

```bash
make up
bash scripts/cwso-projection-e2e.sh; echo "exit=$?"
make down
```

## Git rails

- Branch: `agent/qa-engineer/C024` from `develop` (rebased on merged C022/C023)
- Commit: `test: prove shadow-workspace filesystem projection end-to-end`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A failing assertion here means Phase 2 is not done — report `technical` / `critical`
with the transcript; do not annotate it away.

## Execution notes

<filled during execution>
