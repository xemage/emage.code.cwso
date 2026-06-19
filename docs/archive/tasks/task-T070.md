# Task T070 — Phase 5 integration and reliability QA

- Phase: **5 (QA)** · Owner: **qa-engineer** · Priority: **P0**
- Depends on: T066, T067, T068, T069 · Blocks: T071
- Status: done (2026-05-23)

## Objective
Validate integrated behavior across HHD, Wasm micro-agents, and experimental paths while ensuring no regression in reliability.

## Inputs
- [docs/tasks/task-T066.md](task-T066.md)
- [docs/tasks/task-T067.md](task-T067.md)
- [docs/tasks/task-T068.md](task-T068.md)
- [docs/tasks/task-T069.md](task-T069.md)

## Constraints
- Cover mixed-backend scenarios plus forced-fallback cases.
- Include soak and fault-injection coverage for reliability claims.

## Expected outputs
- `docs/artifacts/qa-phase5-report-v1.md`
- Updated automated integration/e2e suites.

## Acceptance criteria
1. Regression suite passes for baseline mode and enabled feature paths.
2. Reliability KPIs are equal to or better than pre-phase baseline.
3. Failure and fallback behavior is documented with trace evidence.

## Blocker protocol
If flaky infrastructure prevents stable verdicts, report blocker type `dependency` and provide reproducible failure matrix.

## Completion notes (2026-05-23)
- Produced QA artifact `docs/artifacts/qa-phase5-report-v1.md` covering integrated scope, prerequisites, matrix coverage, reliability KPI evidence, fallback trace references, and final verdict.
- Extended policy-engine automated QA coverage in `orchestrator/internal/dispatch/policy_engine_v2_test.go`:
	- `TestPolicyEngineV2MixedBackendForcedFallbackWalksDeterministicChain`
	- `TestPolicyEngineV2FaultInjectedScoreAdjusterRemainsDeterministicAcrossRepeats`
- Reused existing fallback evidence tests for sparse/quantized, SSM, Wasm score-adjuster failure, and event-monitor eBPF/userspace fallback.

### Validation run
- `cd orchestrator && go test ./internal/dispatch ./internal/tools ./internal/server ./internal/config` -> PASS
