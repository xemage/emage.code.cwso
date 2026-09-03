# Task T199 — Wire `ErrorObj.conflict_matrix` into `mergeengine.Client` and surface it to MCP callers

**ID:** T199
**Owner:** backend-developer
**Status:** pending
**Priority:** P2
**Depends on:** —
**Created:** 2026-08-28
**Completed:** —
**Based on:** Discovered during C042 (three-way merge + conflict matrix), logged separately
per Tech Lead review's explicit recommendation (MR !185) — non-blocking, does not gate
C042's own merge, following the same cross-boundary-gap pattern as T197/T198/C036/C037.

## Objective

C042 (`services/cwso-merge-engine`) now computes and returns a structured Blueprint §5.4
conflict matrix (`ConflictMatrixEntry[]`) as an additive, optional `conflict_matrix` field
on the sidecar's JSON-RPC `error` object when a three-way merge is unresolvable. The
orchestrator-side Go client for this sidecar
(`orchestrator/internal/mergeengine/client.go`) does not know this field exists —
`response.Error` is an anonymous struct with only `Code`/`Class`/`ReasonCode`/`Message`
(see `client.go` lines ~55-61), and Go's `encoding/json` silently drops unknown JSON
fields on unmarshal by default (confirmed: no `DisallowUnknownFields` call anywhere on the
decoder in this package). This is **safe but inert**: no MCP client can currently see the
conflict-matrix data C042's sidecar now produces — `merge_concurrent_results`' conflict
responses are exactly as uninformative to callers as they were before C042, even though
the underlying data now exists one hop away.

Wire the field through end to end so a real MCP client calling `merge_concurrent_results`
on an unresolvable merge receives the conflict matrix, not just the existing
class/reason_code/message.

## Inputs

- `services/cwso-merge-engine/src/proto.rs` (`ErrorObj`'s `conflict_matrix` field — the
  Rust-side shape to mirror on the Go side; read this first for field names/types)
- `services/cwso-merge-engine/src/merge.rs` (`ConflictMatrixEntry`/`ConflictState` — the
  row shape: `unit_key`, `node_kind`, `node_name` (nullable), `ours_state`, `theirs_state`,
  `reason_code`)
- `orchestrator/internal/mergeengine/client.go` (the Go client — `response.Error`'s
  anonymous struct needs the new field; `SidecarError` needs a corresponding field to
  carry it out to callers)
- `orchestrator/internal/tools/merge_tools.go` (`mapToolMergeError`/`classifySidecarError`
  — where a `*mergeengine.SidecarError` currently becomes an MCP tool-level escalation via
  `applyEscalation`; this is where the conflict matrix needs to actually reach the
  `merge_concurrent_results` tool's JSON-RPC response)
- `schemas/merge_concurrent_results.json` (the tool's response shape — almost certainly
  needs a new field to describe the conflict matrix in the *response*, distinct from the
  *request* shape this schema currently documents; read `schemas/README.md` first for this
  directory's descriptive-not-prescriptive convention before deciding exactly what to add)
- Blueprint §3.3 step 4 / §5.4 (`input/CWSO_ Agentic AI Orchestration Blueprint.md`) — the
  contract this is ultimately in service of: an LLM/human reviewer-facing conflict report

## Rails (read before starting)

### You MUST
- Add the missing field(s) to `mergeengine.Client`'s response/error decoding
  (`client.go`) and to `SidecarError` (or an equivalent carrier type) so the conflict
  matrix survives the IPC hop intact
- Wire it through `merge_tools.go`'s error-classification path so a `merge_concurrent_results`
  MCP response actually includes the conflict matrix data for an unresolvable merge
- Update `schemas/merge_concurrent_results.json` to describe the new response field(s) —
  this is a schema change the orchestrator (you, dispatched as backend-developer, but
  flag it explicitly in your MR per this project's schema-change discipline) is doing
  deliberately, not silently
- Add tests: a real end-to-end (or as close as file ownership allows) proof that a
  conflict response from the sidecar results in a `merge_concurrent_results` MCP response
  that includes the conflict matrix — not just a unit test of the decode step in isolation
- Confirm backward compatibility: an OLDER sidecar binary (pre-C042, no `conflict_matrix`
  field in its JSON) must still work with this newer Go client without error — the new
  field must be optional/nullable on decode

### You MUST NOT
- Change the sidecar (`services/cwso-merge-engine/**`) — C042 already implemented and
  shipped the producing side; this task is Go-side consumption only
- Change the conflict-matrix row shape (`unit_key`/`node_kind`/`node_name`/`ours_state`/
  `theirs_state`/`reason_code`) — mirror it, don't redesign it
- Break `merge_concurrent_results`' existing response shape for the non-conflict (clean
  merge) and non-matrix (message-only conflict, e.g. a whole-file parse failure) cases —
  this must be a strict additive extension

## File ownership

- **May create/modify:** `orchestrator/internal/mergeengine/**`,
  `orchestrator/internal/tools/merge_tools.go` (and its test file),
  `schemas/merge_concurrent_results.json`
- **Must NOT touch:** `services/**`, other `orchestrator/internal/*` packages, other
  schemas

## Steps (execute in order)

1. Read `services/cwso-merge-engine/src/proto.rs`'s `ErrorObj` and `src/merge.rs`'s
   `ConflictMatrixEntry`/`ConflictState` for the exact field names/types to mirror.
2. Add the corresponding Go types and decode path in `client.go`.
3. Thread the conflict matrix through `merge_tools.go`'s error-handling path into the
   actual MCP tool response.
4. Update `schemas/merge_concurrent_results.json` for the new response field(s).
5. Tests: decode-level, and as close to end-to-end as this task's file ownership allows.
6. Verify backward compatibility with a pre-C042-shaped (no `conflict_matrix`) error
   response.

## Expected outputs

- Go-side types/decoding for the conflict matrix in `mergeengine.Client`
- `merge_concurrent_results` MCP responses that include the conflict matrix on an
  unresolvable merge
- Updated `schemas/merge_concurrent_results.json`
- Tests proving the wiring end to end (within file-ownership bounds) and backward
  compatibility with an older sidecar

## Acceptance criteria

1. A real `merge_concurrent_results` MCP tool-call response, for an unresolvable merge,
   includes the conflict matrix data (not just class/reason_code/message as before)
2. `go build ./...` and `go test ./internal/mergeengine/... ./internal/tools/...` pass
3. A pre-C042-shaped sidecar error response (no `conflict_matrix` field) still decodes
   and behaves correctly — no panic, no error, graceful absence
4. `schemas/merge_concurrent_results.json` accurately documents the new response field(s)

## Verification commands

```bash
cd orchestrator
go build ./...
go test ./internal/mergeengine/... ./internal/tools/... -count=1
go vet ./...
```

## Git rails

- Branch: `agent/backend-developer/T199` from `develop`
- Commit: `feat(mergeengine): surface conflict_matrix to MCP callers`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries. If the
right response shape for `merge_concurrent_results`' schema is ambiguous (e.g. per-file
vs. per-batch conflict matrix, given `merge_inputs` is an array), cite the ambiguity and
report `unclear_requirements` / `minor` rather than inventing a shape unilaterally.

## Execution notes

<filled during execution>
