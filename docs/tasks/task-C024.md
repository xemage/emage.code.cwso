# Task C024 — Prove it: sub-agent E2E against a real path, in CI

**ID:** C024
**Owner:** qa-engineer
**Status:** done
**Priority:** P0
**Depends on:** C022, C023
**Created:** 2026-08-12
**Completed:** 2026-08-21
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

Added `scripts/cwso-mcp-call.py` (real MCP HTTP/token-minting helper,
reusing `scripts/cwso-token.sh`'s pattern) and `scripts/cwso-projection-e2e.sh`
(~461 lines), plus a Go fixture (`scripts/fixtures/go/greet/`) and a new
`e2e:projection` CI job. Transport split: `create_shadow_workspace`/
`commit_shadow` over the real MCP HTTP transport; the actual proof steps
(`ls`/`cat`/a real compiled test command/a `sed`-based editor edit) via
`docker exec` into the live `cwso-git-shadow` container, the only place the
real projected path is reachable from. Distinct from and non-duplicative
of C018's `cwso-smoke-test.sh`.

Two judgment calls resolved within scope: (1) the shadow tmpfs is `noexec`
and the `git-shadow` runtime image has no compiler toolchain — both the
intended consequence of C019's hardening — worked around by pre-compiling
a static Go test binary outside the container and running it from the
exec-allowed `/run/cwso` volume, still validating real content at the real
materialized path. Documented as `docs/DEBT-REGISTER.md` row **R-6**
(`wontfix`, deferred per the reviewer's explicit recommendation — loosening
would trade away already-reviewed security posture for marginal
convenience with no current consumer). (2) found and fixed a CI-only
bind-mount bug (same class as `CWSO_WORKSPACE_HOST`) by building the Go
fixture via `docker create`+`docker cp`+`docker start -a` instead of a
host bind-mount, verified in both local dev and live CI.

**VERDICT: CONDITIONAL_PASS** (independent Tech Lead review, MR !163) —
content fully approved as-is, zero code changes required: every claim
independently reproduced (built the real image, ran the real stack, forced
the deliberate-failure path, verified idempotent cleanup by directly
inspecting container/tmpfs state); both judgment calls confirmed
legitimate; confirmed the test genuinely asserts against real content at
the real materialized path, not a decoy copy; file ownership confirmed
clean. One procedural condition: the MR's pipeline was red at review time
from `e2e:smoke`/`e2e:phase2` (the same "Errno 111 Connection refused"
transient pattern seen repeatedly this session, confirmed unrelated —
`e2e:projection` itself was green, all 8 stages, 65s) — resolved by
retrying both jobs sequentially, each succeeding on the first retry with
the same signature, full pipeline confirmed genuinely green before merge.

MR !163 (`agent/qa-engineer/C024`), merged to `develop` via merge commit
`5ffeec60`. This closes gate CG2 ("real filesystem") — the entire
C020→C035→C024 chain is complete, reviewed, and merged.
