# Task C018 — End-to-end smoke test (v1.0 definition-of-done executable)

**ID:** C018
**Owner:** qa-engineer
**Status:** done
**Priority:** P0
**Depends on:** C016, C017
**Created:** 2026-08-12
**Completed:** 2026-08-20
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

`scripts/cwso-smoke-test.sh` (348 lines) implements the 7 stages exactly as required:
health (bash, GET `/healthz` poll), then `create_shadow_workspace` /
`write_shadow_file` / `query_ast` / `commit_shadow` / `merge_concurrent_results` /
teardown (`drop_shadow_workspace`) via an embedded python3 stdlib block. Each stage
prints a `[PASS]`/`[FAIL]` line, asserts real response content (not just absence of
error — e.g. `query_ast` requires `hits >= 1`, `merge_concurrent_results` requires
`outcome=success`/`status=merged`/`reason_code=semantic_merge_success`), and fails
fast: `stage_fail()` prints the full JSON-RPC response body to stderr and exits 1
with no fallthrough to later stages. `trap cleanup EXIT`, installed before any stage
runs, unconditionally runs `docker compose down -v --remove-orphans` and preserves
the original exit code — verified live on both the pass and deliberate-failure paths.

Deviation from the brief's literal instruction to use "the exact JSON shapes from
`schemas/`": the worker discovered `schemas/create_shadow_workspace.json` and
`schemas/query_ast.json` have drifted from the real, live tool contracts in
`orchestrator/internal/tools/shadow_tools.go` (fabricated `sandbox_profile`/
`injected_memory_context` fields; missing actually-required `workspace_uuid`/`path`
on `query_ast`) and correctly built the script against the real, live-verified
contract instead — a "no mocks" e2e test that sent fabricated requests to match
stale docs would defeat the test's own purpose. Flagged the drift as a finding
rather than silently patching `schemas/*` (outside this task's file ownership).
Both the orchestrator and the Tech Lead reviewer independently re-verified this
claim directly against the Go source before accepting it; logged as its own task,
**T198** (technical-writer, P2), merged same day (MR !138).

**VERDICT: CONDITIONAL_PASS → resolved** (independent Tech Lead review; the
worker's own Tech-Lead-only routing recommendation was independently evaluated and
affirmed by the orchestrator before dispatch, not deferred to automatically — pure
test harness over already-reviewed, unmodified components, no new secret-handling
or path-confinement logic, reuses the established CI ephemeral-JWT-secret pattern).
Reviewer live-reproduced both the full-pass and deliberate-failure runs independently
(own `make up` / `docker stop cwso-git-shadow` / re-run, own `docker ps -a`/`volume
ls`/`network ls` checks afterward — not trusted from the worker's transcript),
confirmed the script's actual request payloads match the real Go `InputSchema()`s
field-for-field, confirmed zero mocks/hardcoded credentials via grep, confirmed
`glab ci lint` valid and file ownership scoped exactly to the allowed set, and found
one additional minor drift the worker hadn't flagged (`merge_concurrent_results.json`
missing the optional `rollout_session_id` field) — folded into T198 before it was
dispatched.

One blocking condition at review time: the MR's gating pipeline (`2775478431`) was
failing on `build:rollout`. Root-caused as an external, transient crates.io
supply-chain issue (`arrayref` yanked upstream, breaking `blake3`'s dependency
resolution for `cwso-rollout`) — confirmed unrelated to this task's diff (no
Rust/Cargo files touched, `build:rollout` isn't even in `e2e:smoke`'s `needs:`) and
confirmed pre-existing-green on `develop`'s own tip ~20 minutes earlier at the same
base commit. Resolved before merge: confirmed a fresh `develop` pipeline came back
fully green (including `build:rollout`, meaning the yank had resolved upstream),
then retried the MR's own gating pipeline directly (`POST
.../pipelines/2775478431/retry`) rather than substituting a new branch-triggered
pipeline (a first attempt at that was discarded — branch-pipeline `rules:` skip the
build/audit/e2e stages entirely and would not have validated the MR) — came back
14/14 green.

Merging then surfaced two further, non-review issues, both resolved directly rather
than re-opening review: (1) `docs/tasks/active-tasks.md` had a trivial adjacency
merge conflict against `develop` — T198's ledger row (a separate, unrelated
doc-logging MR) had merged minutes earlier immediately next to this task's own
status-line edit; resolved on the MR branch by keeping both rows, re-triggered CI;
(2) the resulting pipeline hit one transient CI-runner concurrency collision
(`e2e:phase2` lost a `docker compose up` container-name race against a
concurrently-running `e2e:smoke` job on the same shared docker-socket-binding
runner: `Conflict. The container name "/cwso-git-shadow" is already in use`) —
retried that single job, `e2e:phase4-swarm` auto-reran alongside it, full 14/14
green. Merged: MR !137 (`agent/qa-engineer/C018`), merge commit `f3cefd28`.

This script is the v1.0 release-gate executable — `docs/tasks/task-C062.md`
("Release v1.0.0") re-runs it before any release can be cut.
