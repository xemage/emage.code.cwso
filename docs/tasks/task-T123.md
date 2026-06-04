# Task T123 — Quality-floor guardrail → dense GPU escalation

**Status:** done  
**Owner:** backend-developer  
**Priority:** P0  
**Depends on:** T122  
**Roadmap mapping:** Feature B placeholder T094 → active T123

## Objective

When a sparse micro-agent `quality_floor` breaches the shared guardrail minimum, skip sparse
instantiation and escalate to a dense GPU backend via the Phase 6 HAL, reusing the existing
`quality_guardrail_autodisable` reason path.

## Inputs

- `docs/artifacts/wasm-sparse-agent-design-v1.md` §8
- `docs/decisions/ADR-008-wasm-sparse-agent-tier.md`
- `orchestrator/internal/dispatch/policy_engine_v2.go` (guardrail threshold)

## Outputs

- `orchestrator/internal/dispatch/sparse_escalation.go` + tests
- `orchestrator/internal/tools/sparse_agent_guardrail.go` + tests
- `create_ephemeral_sparse_agent` optional `quality_floor` / `task_description`
- Config: `CWSO_SPARSE_QUALITY_GUARDRAIL_ENABLED` (default off), reuses `CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE`

## Acceptance criteria

- [x] Quality-floor breach detected via shared `QualityGuardrailBreached` helper
- [x] Escalation returns `reason_code: quality_guardrail_autodisable` and dense provider selection
- [x] HAL job enqueued when jobs + HAL socket are configured
- [x] Feature-flagged off by default
- [x] Unit tests for breach detection, escalation decision, and tool path
