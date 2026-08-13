# Task C018 — End-to-end smoke test (v1.0 definition-of-done executable)

**ID:** C018
**Owner:** qa-engineer
**Status:** pending
**Priority:** P0
**Depends on:** C016, C017
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (C018 row, §1.5); docs/plans/plan-cwso-v1.0-phase1-one-command-stack-v2.md

## Objective

Create `scripts/cwso-smoke-test.sh`: from a clean checkout, `make up` → create a shadow
workspace via MCP → write a file → run an AST query → commit → merge → teardown. This
script **is** the v1.0 definition-of-done executable — Phase 6's release gate re-runs it.

## Inputs

- `Makefile` (`up`/`down` from C016)
- `scripts/cwso-token.sh` (C013)
- `schemas/create_shadow_workspace.json`, `schemas/query_ast.json`, `schemas/merge_concurrent_results.json` (exact tool call shapes — do not guess payloads)
- `scripts/phase2-integration.py` (existing integration flow to reference)

## Rails (read before starting)

### You MUST
- Drive the MCP tools over the HTTP transport with a minted token, using the exact JSON shapes from `schemas/`
- Assert each stage with a clear `PASS/FAIL` line: health → create_shadow_workspace → write_shadow_file → query_ast → commit_shadow → merge_concurrent_results → teardown
- Exit non-zero on the first failed stage, printing the failing response body
- Leave the host clean: `make down` and no stray containers/volumes on both success and failure paths (trap EXIT)
- Add a `smoke` Makefile target and a CI job that runs the script
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Mock any stage — this is an end-to-end test against real containers
- Hardcode a token or secret
- Assert on timing/performance (correctness only; benchmarks are out of scope)
- Modify application code to make the test pass — a failing stage is a finding: report it
- Touch the C016 `up`/`down`/`logs` or C017 `doctor` Makefile targets (add `smoke` only)

## File ownership

- **May create/modify:** `scripts/cwso-smoke-test.sh` (new), `Makefile` (add `smoke` target only), `.gitlab-ci.yml` (add one job), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/*`, `services/*`, `deploy/*`, `schemas/*` (read-only references), other scripts

## Steps (execute in order)

1. Read the three schema files and `scripts/phase2-integration.py` for the call sequence.
2. Implement the script with per-stage assertions and an EXIT trap for teardown.
3. Run it against a real `make up` stack until green.
4. Add the `smoke` target and CI job.
5. CHANGELOG.

## Expected outputs

- `scripts/cwso-smoke-test.sh` (executable)
- `Makefile` `smoke` target
- CI job running the smoke test
- CHANGELOG entry

## Acceptance criteria

1. Clean checkout → `make up` → `bash scripts/cwso-smoke-test.sh` → all stages PASS
2. A deliberately broken stage (e.g., stop git-shadow mid-run) → non-zero exit, failing response printed, host left clean
3. CI job exists and runs the script
4. No mocks, no hardcoded credentials

## Verification commands

```bash
make up
bash scripts/cwso-smoke-test.sh; echo "exit=$?"
docker ps --format '{{.Names}}' | grep -c cwso   # stack running during test
make down && docker ps -a --format '{{.Names}}' | grep -c cwso   # = 0 after teardown
```

## Git rails

- Branch: `agent/qa-engineer/C018` from `develop` (rebased on merged C016/C017)
- Commit: `test: add end-to-end smoke test as v1.0 definition-of-done`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
A stage that fails against the real stack is a **product bug**, not a test problem —
do not weaken the assertion; report `technical` / `critical` with the response body.

## Execution notes

<filled during execution>
