# Task T049 — Phase 4 swarm e2e suite

- Phase: **4 (Production)** · Owner: **qa-engineer** · Priority: **P0**
- Depends on: T048 · Blocks: T050
- Status: done

## Objective
Implement and execute a Phase 4 end-to-end swarm suite that validates deterministic conflict-matrix escalation behavior across merge paths and ensures T048 behavior is preserved under integration conditions.

## Inputs
- [task-T048.md](./task-T048.md)
- [checkpoint-016-phase4-t048-complete.md](../checkpoints/checkpoint-016-phase4-t048-complete.md)
- [plan-T049-phase4-swarm-e2e-suite.md](../plans/plan-T049-phase4-swarm-e2e-suite.md)
- Existing integration scripts and CI jobs

## Constraints
- Validate merged success and non-merged classes with deterministic assertions.
- Avoid introducing flaky timing assumptions; use explicit readiness checks.
- Keep suite CI-compatible with current pipeline environment.
- Preserve existing e2e coverage while adding matrix-focused scenarios.

## Expected outputs
- E2E scenarios for:
  - merged success
  - semantic conflict escalation
  - policy conflict escalation
  - runtime error escalation
- Script/test updates integrated into CI-e2e path.
- Validation evidence (pass logs + scenario outcomes) in completion notes.

## Acceptance criteria
1. E2E suite asserts escalation class/action/reason fields for each scenario.
2. Existing baseline phase2/phase3 flows still pass.
3. CI-equivalent local run passes with deterministic outputs.
4. Task evidence is sufficient to start T050 Tech Lead gate.

## Blocker protocol
If deterministic assertions cannot be achieved due to unstable environment behavior, report blocker with flaky case details, repro steps, and proposed stabilization approach.

## Completion notes (2026-05-16)
- Extended existing CI integration harness in `scripts/phase2-integration.py` to support profile-driven execution and optional Phase 4 conflict-matrix assertions via `merge_concurrent_results`.
- Added deterministic assertions for matrix scenarios:
  - merged success (`status=merged`, `reason_code=semantic_merge_success`, no escalation fields)
  - semantic conflict (`status=conflict`, `reason_code=ast_overlap_conflict`, `escalation_class=semantic_conflict`, `escalation_action=manual_merge_review`)
  - policy conflict (`status=error`, `reason_code=invalid_input`, `escalation_class=policy_conflict`, `escalation_action=fix_input_and_retry`)
  - runtime error (`status=error`, `reason_code=merge_engine_unavailable`, `escalation_class=runtime_error`, `escalation_action=retry_or_investigate_runtime`)
- Kept baseline phase2 behavior unchanged when `CWSO_PHASE4_MATRIX` is not enabled.
- Integrated CI-compatible Phase 4 execution path in `.gitlab-ci.yml`:
  - Added `build:merge-engine`
  - Added `e2e:phase4-swarm` with `CWSO_COMPOSE_PROFILES=phase2,phase4` and `CWSO_PHASE4_MATRIX=1`

Validation evidence:
- `cd /home/emage/Code/emage/CWSO && python3 -m py_compile scripts/phase2-integration.py`: PASS
- `cd /home/emage/Code/emage/CWSO && echo 'ci-ephemeral-secret-not-used-in-prod-ci-only' > .env.jwt.dev && CWSO_JWT_SECRET='ci-ephemeral-secret-not-used-in-prod-ci-only' python3 scripts/phase2-integration.py`: PASS (baseline phase2 flow)
- `cd /home/emage/Code/emage/CWSO && echo 'ci-ephemeral-secret-not-used-in-prod-ci-only' > .env.jwt.dev && CWSO_JWT_SECRET='ci-ephemeral-secret-not-used-in-prod-ci-only' CWSO_COMPOSE_PROFILES='phase2,phase4' CWSO_PHASE4_MATRIX='1' python3 scripts/phase2-integration.py`: PASS (phase2 + phase4 matrix flow)
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/integration ./internal/server`: PASS

Unblock status:
- T049 complete; acceptance criteria met.
- T050 is unblocked.
