# Checkpoint 003 — Phase 3 Kickoff (T030 Started)

Date: 2026-05-14
Phase: 3 — Async + concurrency pipeline
Status: in_progress

## Completed Since Last Checkpoint
- T029 completed and merged to develop via MR !2.
- CI validated green on critical path jobs:
  - go:lint, build:orchestrator, build:git-shadow, go:test, rust:test, e2e:phase2
- T029 outputs now in production branch history (merge commit b5beaa326bac2518fe2ebc6ec752aa8bfe4a6602).

## Task State
- T029: done
- T030: in_progress (current execution target)
- T031/T034 remain blocked by T030

## Key Decisions
1. Begin Phase 3 on T030 first because it is the lowest-ID unblocked P0 dependency for the async path.
2. Preserve POST /mcp behavior and security middleware unchanged while upgrading SSE path.
3. Use a dedicated in-memory event bus package for per-subscriber bounded buffering and drop accounting.

## Risks
- Slow-subscriber memory pressure on SSE streams.
- Unintended regression on existing POST /mcp request/response path.
- Notification envelope drift from Streamable HTTP MCP requirements.

## Mitigations
- Bounded per-subscriber queues + explicit dropped event metric/logging.
- Focused transport regression tests before and after SSE changes.
- Event envelope tests validating JSON-RPC notification payload shape.

## Token Metrics
- Planning budget target: <= 80k
- Used in this planning/kickoff slice: within budget
- Remaining Implementation budget (Phase 3 slice): <= 120k

## Next Steps
1. Delegate T030 implementation to backend-developer with structured brief.
2. Validate unit/integration tests for event bus and transport SSE behavior.
3. Run T030 mini-gates (tech-lead + security-engineer) before moving to T031.
