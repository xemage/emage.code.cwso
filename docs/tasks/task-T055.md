# Task T055 — Align merge_inputs schema/runtime contract

- Phase: **4 (Quality Follow-up)** · Owner: **backend-developer** · Priority: **P1**
- Depends on: T050 · Blocks: none
- Status: pending

## Objective
Resolve mismatch between tool JSON schema and runtime validation for `merge_concurrent_results` input requirements, ensuring schema-valid requests do not fail unexpectedly at runtime.

## Inputs
- [task-T050.md](./task-T050.md)
- [orchestrator/internal/tools/merge_tools.go](../../orchestrator/internal/tools/merge_tools.go)
- [schemas/merge_concurrent_results.json](../../schemas/merge_concurrent_results.json)

## Constraints
- Preserve backward-compatible behavior where possible.
- Keep schema and runtime required fields semantically identical.
- Update tests accordingly.

## Expected outputs
- Code/schema alignment for `merge_inputs` requirement handling.
- Unit tests proving aligned behavior.

## Acceptance criteria
1. Required fields match between schema and runtime checks.
2. Existing tool tests pass and include a regression case for this mismatch.
3. No new validation regressions introduced.

## Blocker protocol
If backward compatibility constraints prevent strict alignment, document trade-off and propose phased migration with explicit deprecation note.
