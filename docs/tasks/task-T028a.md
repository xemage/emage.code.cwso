# Task T028a — Go unit tests for shadow client + shadow tools

- Phase: **2 (PoC hardening)** · Owner: **backend-developer** · Priority: **P0**
- Depends on: T028 · Blocks: T029
- Status: in_progress

## Objective
Add deterministic Go unit tests covering the orchestrator-side shadow integration points introduced in Phase 2: the UDS framed-JSON client and the MCP shadow tools (`create_shadow_workspace`, `drop_shadow_workspace`, `read_shadow_file`, `write_shadow_file`, `commit_shadow`, `query_ast`).

## Inputs
- [task-T023.md](task-T023.md)
- [completed-tasks.md](completed-tasks.md) (T028 gate context)
- [architecture-v1.md](../artifacts/architecture-v1.md) §4, §6
- [gate-T027-phase2-techlead.md](../checkpoints/gate-T027-phase2-techlead.md) (F4)

## Constraints
- Keep tests hermetic (no external sidecar binary dependency).
- Use temporary Unix sockets and fake sidecar handlers for protocol-level tests.
- Validate argument errors via MCP tool results (`isError=true`), not panics.
- Do not modify production behavior in this task.

## Expected outputs
- `orchestrator/internal/shadow/client_test.go`
- `orchestrator/internal/tools/shadow_tools_test.go`
- Coverage of success + error paths for sidecar call/decode and tool argument validation/role gates.

## Acceptance criteria
1. `go test ./internal/shadow ./internal/tools -race -count=1` passes in Docker.
2. Client tests cover framed I/O guardrails and sidecar error propagation.
3. Tool tests cover required argument validation and at least one successful sidecar-backed execution per major tool family.
4. No filesystem/network dependencies beyond local temp UDS sockets.
5. No production code changes required.

## Blocker protocol
Same as T020.
