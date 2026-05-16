# Task T057 — E2E policy path for sidecar reason mapping

- Phase: **4 (Quality Follow-up)** · Owner: **qa-engineer** · Priority: **P1**
- Depends on: T050 · Blocks: none
- Status: pending

## Objective
Add one deterministic e2e scenario that exercises sidecar-origin policy conflict metadata path and validates orchestrator reason-code mapping from merge-engine responses.

## Inputs
- [task-T049.md](./task-T049.md)
- [task-T050.md](./task-T050.md)
- [scripts/phase2-integration.py](../../scripts/phase2-integration.py)
- [orchestrator/internal/tools/merge_tools.go](../../orchestrator/internal/tools/merge_tools.go)
- [services/cwso-merge-engine/src/ipc.rs](../../services/cwso-merge-engine/src/ipc.rs)

## Constraints
- Keep test deterministic and CI-friendly.
- Ensure scenario reaches merge-engine path (not only pre-validation short-circuit).
- Preserve existing baseline and matrix scenarios.

## Expected outputs
- New e2e scenario and assertions for sidecar-origin policy reason path.
- Validation evidence from local/CI-equivalent run.

## Acceptance criteria
1. E2E scenario reaches merge-engine and verifies mapped policy reason metadata.
2. Existing e2e scenarios continue to pass.
3. CI job remains stable without flaky timing assumptions.

## Blocker protocol
If deterministic reproduction of sidecar policy path is unstable, provide repro logs and proposed stabilization approach.
