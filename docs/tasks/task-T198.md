# Task T198 — Sync `schemas/*.json` with the real MCP tool contracts

**ID:** T198
**Owner:** technical-writer
**Status:** in_progress
**Priority:** P2 — doc-sync, not blocking anything
**Depends on:** —
**Created:** 2026-08-20
**Completed:** —
**Based on:** Discovered by the C018 worker (task C018, the v1.0 end-to-end smoke
test) while implementing `scripts/cwso-smoke-test.sh` against `schemas/*.json` per
its own task brief's instruction to "use the exact JSON shapes from `schemas/`". The
worker found the schema files don't match the real, live tool contracts, correctly
built the test against the real behavior instead (confirmed live against the running
stack) rather than the stale docs, and flagged this rather than silently patching the
schema files itself (out of C018's file ownership). The orchestrator independently
re-verified every claim below directly against `orchestrator/internal/tools/shadow_tools.go`
and `merge_tools.go` before writing this brief — not just transcribed from the
worker's report.

## Objective

`schemas/create_shadow_workspace.json` and `schemas/query_ast.json` have drifted
significantly from the tools' actual, live Go implementations in
`orchestrator/internal/tools/shadow_tools.go`. Both schema files also set
`"additionalProperties": false`, which means a client that strictly validated a
request against these schemas before sending it would either send an invalid
request (missing genuinely-required fields the schema doesn't know about) or have a
correct request rejected client-side (for fields the schema doesn't list). Bring the
schema files back in sync with the real contracts so they're trustworthy again as a
reference — this task's own originating task, C018, had to bypass them entirely to
build a working end-to-end test, which is exactly the failure mode a schema file
existing only to go stale is supposed to prevent.

## Confirmed drift (independently re-verified against current source, not assumed)

### `create_shadow_workspace`

| | Schema file claims | Real implementation (`shadow_tools.go` `CreateShadowWorkspace.InputSchema()`) |
|---|---|---|
| Properties | `base_commit_sha`, `sandbox_profile` (enum), `injected_memory_context` (array) | `base_commit_sha` only |
| Required | `["sandbox_profile"]` | none — everything is optional |
| `additionalProperties` | `false` | n/a (not enforced server-side by the schema) |

`sandbox_profile` and `injected_memory_context` do not exist anywhere in the real
tool's input handling — they appear to be either a stale draft of a feature that was
never implemented this way, or copied from a different, earlier design. Strict
schema validation against this file would **require** a field
(`sandbox_profile`) the real API doesn't accept as required (and arguably doesn't
use for anything), while a client that omits it (correctly, per the real API) would
fail schema-side validation before ever reaching the server.

### `query_ast`

| | Schema file claims | Real implementation (`shadow_tools.go` `QueryAST.InputSchema()`) |
|---|---|---|
| Properties | `query_type`, `target_symbol`, `language_context`, `path_filter` | `workspace_uuid`, `path`, `query_type`, `target_symbol` |
| Required | `["query_type", "target_symbol"]` | `["workspace_uuid", "path", "query_type"]` (`target_symbol` is present but optional) |
| `additionalProperties` | `false` | n/a |

The schema file is missing `workspace_uuid` and `path` as properties **entirely** —
both are actually required by the real API, and `additionalProperties: false` means
a strictly-validating client couldn't even send them. Conversely, the schema
requires `target_symbol`, which the real API treats as optional. Also note:
`language_context` and `path_filter` appear in the schema file but are not read
anywhere in the real tool's `Execute()` — confirm during this task whether they're
genuinely unused (in which case flag for removal or confirm they're planned/reserved)
or whether `Execute()` needs to be checked more carefully by someone with
`orchestrator/*` write access (out of this task's own scope either way — this task
only touches `schemas/*`, see File ownership).

### `merge_concurrent_results` (minor, lower severity)

Mostly accurate — all properties and the `required` list match the real
implementation (`merge_tools.go` `MergeConcurrentResults.InputSchema()`). One gap:
the schema is missing the real API's optional `rollout_session_id` string field
("Optional rollout session id for programmatic reward attachment (T136)"). Add it;
this one is a completeness gap, not a correctness-breaking drift like the other two.

## Inputs

- `schemas/create_shadow_workspace.json`, `schemas/query_ast.json`,
  `schemas/merge_concurrent_results.json` (the files to fix)
- `orchestrator/internal/tools/shadow_tools.go` (`CreateShadowWorkspace.InputSchema()`,
  `QueryAST.InputSchema()` — read-only, the source of truth)
- `orchestrator/internal/tools/merge_tools.go` (`MergeConcurrentResults.InputSchema()`
  — read-only, the source of truth)
- `scripts/cwso-smoke-test.sh` (C018 — the payload shapes it actually sends and gets
  accepted are a second, live-verified source of truth; cross-check against it)
- Every other `schemas/*.json` file — check each one against its corresponding real
  `InputSchema()` in `orchestrator/internal/tools/*.go`, not just the three C018
  happened to touch; this drift pattern may not be limited to these three

## Rails (read before starting)

### You MUST
- For every `schemas/*.json` file, confirm its properties/required/additionalProperties
  against the real tool's `InputSchema()` in `orchestrator/internal/tools/*.go` —
  don't assume only the three flagged files are affected
- Fix `create_shadow_workspace.json` and `query_ast.json` to match the real contracts
  exactly (properties, required fields, and reconsider whether `additionalProperties: false`
  is even the right constraint given how easily it's drifted — a stricter schema that
  goes stale is worse than a looser one that stays roughly accurate; use judgment, but
  justify your choice in the MR either way)
- Add the missing `rollout_session_id` field to `merge_concurrent_results.json`
- For `query_ast`'s `language_context`/`path_filter` fields (present in the schema,
  unused in `Execute()`): investigate and report your finding in the MR (are they
  genuinely dead, reserved-for-future, or is there a code path you're missing?) — do
  not silently delete or silently keep them without checking
- Cross-check your corrected schemas against `scripts/cwso-smoke-test.sh` (C018) —
  the payloads that script actually sends and gets 200s for should validate cleanly
  against your fixed schema files
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Touch `orchestrator/internal/tools/*.go` or any application code — the Go
  `InputSchema()` methods are the source of truth this task syncs *to*, not code to
  change; if you find the Go code itself looks wrong (not just the schema), report
  that as a separate finding, don't fix it here
- Touch `scripts/cwso-smoke-test.sh` or any other script
- Guess at a field's purpose — if unclear, cross-reference the tool's `Execute()`
  method and/or its test file before writing a schema constraint

## File ownership

- **May create/modify:** `schemas/*.json` (any file needing a fix, not just the
  three identified above), `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** `orchestrator/*`, `services/*`, `scripts/*`, `deploy/*`

## Acceptance criteria

1. `create_shadow_workspace.json` and `query_ast.json` match their real
   `InputSchema()` implementations exactly (properties, required fields)
2. `merge_concurrent_results.json` includes `rollout_session_id`
3. Every other `schemas/*.json` file has been checked against its real
   implementation; any additional drift found is either fixed or explicitly reported
   if fixing is out of proportion for this task
4. The `language_context`/`path_filter` investigation finding is reported in the MR
5. `scripts/cwso-smoke-test.sh`'s actual request payloads validate cleanly against
   the corrected schemas
6. `git diff --stat` touches only `schemas/*.json` and `CHANGELOG.md`

## Verification commands

```bash
# For each schemas/*.json file, diff its properties/required against the
# corresponding Go InputSchema() — no single command covers this, it's a manual
# cross-check per file; at minimum validate the JSON itself is well-formed:
for f in schemas/*.json; do python3 -m json.tool "$f" > /dev/null && echo "OK: $f"; done
```

## Git rails

- Branch: `agent/technical-writer/T198` from `develop`
- Commit: `docs(schemas): sync schemas/*.json with real MCP tool contracts`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
This is a doc-sync task, not expected to be blocking — if you find something that
looks like it needs an actual code change (not just a schema fix) to be internally
consistent, report it rather than trying to resolve it within this task's scope.

## Execution notes

<filled during execution>
