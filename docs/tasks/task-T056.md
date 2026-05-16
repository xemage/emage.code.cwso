# Task T056 — ADR-006 node-level conflict detail reconciliation

- Phase: **4 (Architecture Follow-up)** · Owner: **solution-architect** · Priority: **P1**
- Depends on: T050 · Blocks: none
- Status: pending

## Objective
Reconcile current conflict-matrix implementation with ADR-006 expectation for node-level conflict detail payloads, by either defining an approved staged deferral or specifying implementation changes.

## Inputs
- [task-T050.md](./task-T050.md)
- [docs/decisions/ADR-006-semantic-ast-merge.md](../decisions/ADR-006-semantic-ast-merge.md)
- [docs/artifacts/architecture-v1.md](../artifacts/architecture-v1.md)
- [orchestrator/internal/tools/merge_tools.go](../../orchestrator/internal/tools/merge_tools.go)
- [services/cwso-merge-engine/src/proto.rs](../../services/cwso-merge-engine/src/proto.rs)

## Constraints
- Keep decision explicit and auditable.
- If deferring, include rationale, scope boundary, and target tasking.
- If implementing now, define API contract impacts clearly.

## Expected outputs
- Decision artifact update (ADR addendum/new ADR) or implementation-ready architecture note.
- Clear downstream task implications.

## Acceptance criteria
1. Architecture decision explicitly addresses node-level conflict payload expectation.
2. Decision includes actionable next steps (implement now vs defer with target).
3. No ambiguity remains for release/readiness evaluation.

## Blocker protocol
If required architectural context is missing, report exact missing artifact and minimum decision data needed.
