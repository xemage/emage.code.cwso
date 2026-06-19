# Task T068 — SSM sequence-assist spike

- Phase: **5 (R&D Spike)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T065 · Blocks: T070
- Status: done (2026-05-23)

## Objective
Evaluate sequence-state-model (SSM) assist modules for long-context sequence handling within dispatch-adjacent workloads.

## Inputs
- [docs/tasks/task-T065.md](task-T065.md)
- Benchmark definitions from T062

## Constraints
- Isolate as non-critical experiment; no release dependency.
- Use reproducible benchmark scripts and fixed datasets.

## Expected outputs
- `docs/artifacts/hypothesis-T068-results-v1.md`
- Spike implementation notes and integration feasibility report.

## Acceptance criteria
1. Throughput and latency comparisons are reproducible.
2. Output quality checks are documented against baseline thresholds.
3. Recommendation explicitly states production readiness level.

## Blocker protocol
If evaluation tooling is insufficient, report blocker type `dependency` and define minimum tooling additions needed.

## Completion notes (2026-05-23)
- Implemented a minimal SSM sequence-assist spike in `orchestrator/internal/dispatch/policy_engine_v2.go` behind explicit `SSM.Enabled` feature configuration (default `false`).
- Added a sequence-length sensitivity modifier driven by request labels (`sequence_length`, `sequence.length`, `context_tokens`, `context.tokens`) and restricted to long-context workloads.
- Added a configurable throughput bias control (`SSM.ThroughputBias`) and sequence sensitivity scaling (`SSM.SequenceSensitivity`).
- Added compatibility guardrail fallbacks for invalid and out-of-threshold sequence signals:
	- `ssm_signal_invalid_fallback`
	- `ssm_signal_out_of_threshold_fallback`
- Added test coverage in `orchestrator/internal/dispatch/policy_engine_v2_test.go` for:
	- disabled-feature baseline preservation,
	- enabled-feature long-context scoring/selection change,
	- safe guardrail fallback behavior.
- Added runtime config wiring and validation in:
	- `orchestrator/internal/config/config.go`
	- `orchestrator/internal/config/config_test.go`
	- `orchestrator/internal/server/server.go`
- Produced hypothesis artifact with reproducible synthetic benchmark method and readiness verdict:
	- `docs/artifacts/hypothesis-T068-results-v1.md`

### Validation run
- `cd orchestrator && go test ./internal/dispatch ./internal/config ./internal/server ./internal/tools` -> PASS
