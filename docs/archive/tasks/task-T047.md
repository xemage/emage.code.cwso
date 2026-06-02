# Task T047 — merge_concurrent_results tool

- Phase: **4 (Production)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T046 · Blocks: T048
- Status: done

## Objective
Implement the orchestrator `merge_concurrent_results` tool by integrating with `cwso-merge-engine`, so concurrent shadow workspace outputs can be merged deterministically with explicit success or conflict responses.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-6
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [task-T045.md](./task-T045.md)
- [task-T046.md](./task-T046.md)
- [plan-T047-phase4-merge-concurrent-results-tool.md](../plans/plan-T047-phase4-merge-concurrent-results-tool.md)

## Constraints
- Preserve existing orchestrator tool contracts; only additive schema changes if needed.
- Integrate over merge-engine IPC without direct filesystem writes.
- Map semantic merge outcomes clearly: success vs explicit conflict.
- Keep deterministic behavior and robust error handling.
- Ensure tool behavior is observable with structured logs/reason codes.

## Expected outputs
- Tool implementation/wiring for `merge_concurrent_results`
- Any required schema updates for tool request/response
- Tests covering success, conflict, and failure-path behavior
- Documentation updates if tool contract changes

## Acceptance criteria
1. Tool invokes merge-engine and returns merged result on semantic success.
2. Conflict path returns explicit structured conflict outcome (compatible with T048 follow-on).
3. Runtime/IPC failures return stable, non-crashing error responses.
4. Existing tooling remains backward-compatible.
5. Relevant orchestrator and merge-engine tests pass.
6. Task output unblocks T048.

## Blocker protocol
Same as T020. Escalate technical blocker if merge-engine/orchestrator contract mismatch cannot be resolved without breaking changes.

## Completion notes (2026-05-16)
- Implemented orchestrator `merge_concurrent_results` tool in `orchestrator/internal/tools/merge_tools.go` with deterministic per-file processing and stable outcome mapping.
- Integrated tool with `cwso-merge-engine` over framed UDS IPC through a dedicated client package `orchestrator/internal/mergeengine`.
- Added explicit structured mapping for semantic success (`status=merged`), conflict (`status=conflict`, `reason_code=semantic_conflict`), and runtime/IPC failures (`status=error`, stable reason codes).
- Preserved backward compatibility by keeping existing request fields and adding `merge_inputs` as an additive argument for explicit merge payloads.
- Wired server registration behind `CWSO_MERGE_ENGINE_SOCKET`, updated config/env handling, and aligned schema in `schemas/merge_concurrent_results.json`.

Validation summary:
- `go test ./internal/mergeengine ./internal/tools ./internal/server ./internal/config`: PASS.
- `go test ./...` (orchestrator): PASS.
- `docker run --rm -v "$PWD":/workspace -w /workspace/services rust:1.83-bookworm bash -lc 'set -euo pipefail; export PATH=/usr/local/cargo/bin:$PATH; cargo test -p cwso-merge-engine'`: PASS (7 tests).

Unblock status:
- T048 unblocked.
