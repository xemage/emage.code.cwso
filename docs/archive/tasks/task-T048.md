# Task T048 — Conflict matrix escalation

- Phase: **4 (Production)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T047 · Blocks: T049
- Status: done

## Objective
Implement deterministic conflict-matrix escalation for concurrent merge results so the orchestrator can classify outcomes into stable conflict classes and route each class to the correct escalation behavior without regressions to the successful merge path.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-6
- [architecture-v1.md](../artifacts/architecture-v1.md) §3, §6
- [task-T047.md](./task-T047.md)
- [plan-T048-phase4-conflict-matrix-escalation.md](../plans/plan-T048-phase4-conflict-matrix-escalation.md)
- [merge_concurrent_results.json](../../schemas/merge_concurrent_results.json)

## Constraints
- Preserve deterministic merge outcomes and current success behavior from T047.
- Conflict handling must be additive and backward compatible where feasible.
- Use explicit machine-readable class and reason fields suitable for automation.
- Keep orchestrator-to-merge-engine mapping consistent and testable.
- No privilege escalation or policy bypass in conflict handling.

## Expected outputs
- Conflict taxonomy and escalation mapping implemented in merge-engine + orchestrator tool layer.
- Stable response semantics for at least: merged, semantic_conflict, policy_conflict, runtime_error (or equivalent final taxonomy).
- Unit tests and contract tests covering matrix classes and mapping boundaries.
- Updated schema/docs only where required by additive behavior.

## Acceptance criteria
1. Every conflict outcome is classified into a deterministic escalation class.
2. Tool responses expose stable class/reason metadata for each non-merged case.
3. Existing successful merge path remains unchanged and verified by tests.
4. Orchestrator mapping is consistent with merge-engine outputs across all classes.
5. Go and Rust tests for new/changed paths pass in local CI-equivalent runs.
6. Task output unblocks T049 end-to-end suite.

## Blocker protocol
If conflict classes cannot be mapped without breaking existing contract behavior, report a technical blocker with proposed additive migration path and impact summary.

## Completion notes (2026-05-16)
- Implemented additive conflict-matrix escalation metadata for `merge_concurrent_results` responses in `orchestrator/internal/tools/merge_tools.go`.
- Added deterministic non-merged classification fields:
	- `escalation_class`: `semantic_conflict` | `policy_conflict` | `runtime_error`
	- `escalation_action`: stable automation actions per class
	- Existing `reason_code` retained and now filled from merge-engine metadata when present.
- Preserved merged success-path semantics (`status=merged`, `reason_code=semantic_merge_success`) and existing outcome/counter behavior for successful merges.
- Extended merge-engine protocol error envelope with additive optional metadata in `services/cwso-merge-engine/src/proto.rs`:
	- `error.class`
	- `error.reason_code`
- Updated merge-engine dispatch in `services/cwso-merge-engine/src/ipc.rs` to emit deterministic codes/classes/reasons:
	- semantic merge conflict: `code=merge_conflict`, `class=semantic_conflict`, `reason_code=ast_overlap_conflict`
	- invalid payload input: `code=invalid_input`, `class=policy_conflict`, `reason_code=invalid_payload_encoding`
- Updated orchestrator merge-engine client in `orchestrator/internal/mergeengine/client.go` to deserialize and carry `class`/`reason_code` metadata.
- Added legacy compatibility mapping so old sidecar code `unimplemented_conflict` still maps deterministically to `semantic_conflict`.

Validation summary:
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/mergeengine ./internal/tools`: PASS
- `cd /home/emage/Code/emage/CWSO/orchestrator && go test ./internal/server ./internal/config`: PASS
- `cd /home/emage/Code/emage/CWSO/services && docker run --rm -v "$PWD":/workspace -w /workspace rust:1.83-bookworm bash -lc 'set -euo pipefail; export PATH=/usr/local/cargo/bin:$PATH; cargo test -p cwso-merge-engine'`: PASS (9 tests)

Notes:
- Rust formatting via `cargo fmt` was not run in the container because `cargo-fmt`/`rustfmt` is not installed in the image toolchain.

Unblock status:
- T049 unblocked by deterministic conflict matrix classification and passing Go/Rust validation.
