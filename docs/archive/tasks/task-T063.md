# Task T063 — Hardware dispatch architecture and provider contracts

- Phase: **5 (Architecture)** · Owner: **solution-architect** · Priority: **P0**
- Depends on: T062 · Blocks: T064, T065
- Status: done

## Objective
Design the hardware-aware dispatch architecture and provider contracts for heterogeneous backend selection and deterministic fallback.

## Inputs
- [docs/tasks/task-T062.md](task-T062.md)
- `docs/artifacts/requirements-phase5-hardware-v1.md` (from T062)
- [docs/decisions/ADR-006-semantic-ast-merge.md](../decisions/ADR-006-semantic-ast-merge.md)

## Constraints
- Preserve current orchestrator and sidecar boundaries.
- Provider contract must be versionable and backward compatible.

## Expected outputs
- `docs/artifacts/architecture-phase5-hhd-v1.md`
- `docs/decisions/ADR-007-hardware-dispatch-provider-contract.md`

## Acceptance criteria
1. Architecture defines routing pipeline, scoring model, and fallback semantics.
2. Provider contract specifies capability schema, error model, timeout and retry policy.
3. Security and observability implications are documented for gate review.

## Blocker protocol
If contract boundaries conflict with current APIs, report blocker type `technical` with options and tradeoffs.

## Completion notes (2026-05-22)
- Produced hardware-aware architecture artifact:
	- `docs/artifacts/architecture-phase5-hhd-v1.md`
- Produced architecture decision record:
	- `docs/decisions/ADR-007-hardware-dispatch-provider-contract.md`
- Captured provider contract model with versioning rules, capability schema, normalized error taxonomy, timeout/retry semantics, and deterministic fallback to CPU baseline.
- Documented security and observability implications aligned to the security baseline, plus compatibility and migration strategy for adapter-first rollout.

Validation evidence:
- Architecture defines routing pipeline, scoring model, and fallback semantics with deterministic tie-breaking and explicit state machine.
- Provider contract specifies capability schema, error model, timeout and retry/fallback policy.
- Cross-references to Phase 5 requirements and security baseline are included and resolve to existing artifacts.
