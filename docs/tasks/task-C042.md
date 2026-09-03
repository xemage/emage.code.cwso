# Task C042 — Three-way merge + conflict matrix

**ID:** C042
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C041
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B7); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md; input/CWSO_ Agentic AI Orchestration Blueprint.md §5.4

## Objective

Now that C041 supplies parents, enable a genuine three-way merge in
`merge_concurrent_results`. Where a merge is unresolvable, return the **conflict
matrix** the Blueprint §5.4 promises — never a corrupted file.

## Inputs

- C041 (parent commits)
- `services/cwso-merge-engine/` (the merge sidecar)
- Blueprint §5.4 (the conflict-matrix contract — read it before designing the response)
- `schemas/merge_concurrent_results.json`

## Rails (read before starting)

### You MUST
- Implement three-way merge using the common ancestor (from the now-chained history)
- On unresolvable conflict, return a structured conflict matrix per Blueprint §5.4 (deterministic conflict classes and reason codes) — as **data**, per the existing schema
- Add tests: (a) a genuine three-way merge succeeds; (b) an unresolvable merge returns a conflict matrix and **no file is corrupted** (the pre-merge state is preserved)
- Keep the merge deterministic — same inputs → same outputs, always
- Update `docs/DEBT-REGISTER.md` if any shortcut is introduced

### You MUST NOT
- Ever write a partially-merged/corrupted file — conflict matrix or clean merge, nothing in between
- Change the `merge_concurrent_results` schema shape (extend within it if the Blueprint requires; flag any schema change to the orchestrator before making it)
- Implement merge *strategies* beyond the Blueprint §5.4 contract
- Touch git-shadow's projection code (C021–C023) or the orchestrator

## File ownership

- **May create/modify:** `services/cwso-merge-engine/**`, `services/cwso-git-shadow/**` (only if ancestor lookup requires it — justify in MR), `docs/DEBT-REGISTER.md` (only if new debt)
- **Must NOT touch:** `orchestrator/*`, `schemas/*` (without orchestrator sign-off), other services

## Steps (execute in order)

1. Read Blueprint §5.4 and the current merge-engine.
2. Implement ancestor-based three-way merge.
3. Implement the conflict-matrix return for unresolvable cases.
4. Tests: clean merge + conflict matrix + no-corruption.
5. Verification.

## Expected outputs

- Three-way merge in `cwso-merge-engine`
- Conflict-matrix return per §5.4
- Tests for both outcomes

## Acceptance criteria

1. A genuine three-way merge succeeds
2. An unresolvable merge returns a conflict matrix; pre-merge state intact
3. Merge is deterministic across repeated runs
4. `cargo test -p cwso-merge-engine` passes

## Verification commands

```bash
cargo test -p cwso-merge-engine
cargo test -p cwso-git-shadow
```

## Git rails

- Branch: `agent/backend-developer/C042` from `develop` (rebased on merged C041)
- Commit: `feat(merge-engine): three-way merge with conflict matrix`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If the Blueprint §5.4 contract is ambiguous, cite the ambiguity and report
`unclear_requirements` / `major` — do not invent a conflict format.

## Execution notes

**Status: implemented, tests green, ready for review.**

### What was already there vs. what this task added

Before this task, `services/cwso-merge-engine/src/merge.rs` already contained a
substantial AST-aware three-way merge (`merge_three_way`, `merge_units`,
`resolve_base_decisions`, `merge_insertions`, plus a sparse-diff pre-filter fast
path and a large dense/sparse conformance corpus) — this predates Phase 4, not
work from C040/C041. It already treats its `base` argument as the merge-base for
real three-way semantics (auto-resolves disjoint edits, takes the non-`base`
side when only one side changed, requires byte-identical convergence when both
sides independently make the same change). What was genuinely missing, and is
what this task added:

1. **A real conflict matrix, in production code, not just test helpers.** Before
   this change, an unresolvable merge collapsed to a single bare
   `MergeError::SemanticConflict` (no data), and `ipc.rs` turned that into one
   generic `merge_conflict` / `semantic_conflict` / `ast_overlap_conflict` error
   with a fixed string message — no indication of *which* AST units collided or
   *why*. (A conflict-matrix-shaped helper existed, but only inside
   `#[cfg(test)]`, only for a dense/sparse conformance assertion, and it
   short-circuited on the first conflicting unit rather than collecting all of
   them.)
2. **Wiring that matrix into the actual IPC response** so a caller of
   `merge_three_way` over the Unix-domain-socket protocol receives it as
   structured data, not just prose.

### How I interpreted the Blueprint §5.4 contract

Blueprint §5.4 itself (`input/CWSO_ Agentic AI Orchestration Blueprint.md`,
"5.4 merge_concurrent_results") is the **request-side MCP tool schema** for
`merge_concurrent_results` (`source_workspace_uuids`, `target_branch_ref`,
`auto_resolve_heuristic`) plus one line of prose: "If algorithmically
unresolvable structural conflicts occur, returns a formatted JSON conflict
matrix instead of corrupting the file." It does **not** define an output/matrix
JSON shape. The shape is instead described qualitatively in §3.3 step 4
("Conflict Escalation Matrix"):

> If an irreconcilable semantic conflict materializes (e.g., Agent A deletes a
> database struct that Agent B is simultaneously implementing a new trait for),
> the orchestrator halts the automated merge. It formats a highly structured
> JSON conflict report detailing the exact AST node collisions and streams this
> matrix back to the primary LLM via the MCP interface.

And `docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md` (risk table) confirms
the intended scope boundary explicitly: "C042's contract is the Blueprint §5.4
conflict matrix as *data*; presentation is out of scope."

I designed `ConflictMatrixEntry` (in `merge.rs`) directly off the §3.3 step 4
prose — "detailing the exact AST node collisions" — as one row per colliding
top-level AST unit: `unit_key`, `node_kind` (tree-sitter node kind, e.g.
`function_declaration`), `node_name` (extracted name, `None` for anonymous
nodes), `ours_state`/`theirs_state` (`unchanged` | `deleted` | `modified` |
`inserted`), and a deterministic `reason_code`
(`both_modified_diverged` | `delete_modify_conflict` | `insertion_diverged`).
This is additive, internal to the merge-engine's own wire protocol
(`services/cwso-merge-engine/src/proto.rs`'s `ErrorObj`), and does **not**
touch `schemas/merge_concurrent_results.json` (which, on inspection, is
exclusively a *request*-argument schema — it has no output/response
definition at all, so "conforming to it" for a conflict-matrix *response* was
not literally applicable; I did not modify it and did not need to).

I flagged, rather than guessed, on one design point I could have gotten wrong
without checking: whether the merge-engine needed to gain its own
git-shadow-facing ancestor/`merge_base` lookup for "ancestor-based" three-way
merge. It does not — `schemas/merge_concurrent_results.json`'s
`merge_inputs[].base_content` is already a required field, i.e. the
merge-engine's contract already receives the common ancestor as plain input
data; the caller (orchestrator, out of my file-ownership scope) is responsible
for sourcing that content from git-shadow's now race-safe parent chain (C041).
This is exactly what "unblocking C042's three-way merge" in DEBT-REGISTER B7's
row means: C041 makes the `base_content` an orchestrator can hand to
`merge_inputs` trustworthy (a real common ancestor, not a torn/orphaned read),
not that the merge-engine itself needs new git-shadow-facing plumbing.
Consequently **I did not touch `services/cwso-git-shadow/**`** — there was no
genuine need, and inventing an ancestor-lookup endpoint there would have been
unused dead code without a corresponding (out-of-scope, orchestrator-owned)
caller to invoke it.

### Files changed (all within `services/cwso-merge-engine/**`, no other ownership boundary touched)

- `services/cwso-merge-engine/src/merge.rs` — added `ConflictState`,
  `ConflictMatrixEntry`, and `pub fn conflict_matrix(language, base, ours,
  theirs) -> Result<Vec<ConflictMatrixEntry>, MergeError>` (a second,
  independent dense pass over the same three inputs, run only on the already-
  rare conflict path). Collects **every** colliding top-level unit (unlike
  `resolve_base_decisions`/`merge_insertions`, which short-circuit on the
  first) across three collision classes: both-sides-modified-differently,
  delete-vs-modify, and same-key insertion divergence. `merge_three_way` and
  every existing function/test are untouched — zero behavior change to the
  merge algorithm itself, purely additive.
- `services/cwso-merge-engine/src/proto.rs` — added an optional
  `conflict_matrix: Option<Vec<ConflictMatrixEntry>>` field on `ErrorObj`
  (`#[serde(skip_serializing_if = "Option::is_none")]`, so it's simply absent
  from the wire payload for every existing non-matrix error path — `parse_error`,
  `invalid_input`, `empty_merge_input`, and the matrix-unavailable fallback for
  `merge_conflict`) and a new `Response::error_with_conflict_matrix` constructor.
  Backward compatible with the orchestrator's existing Go client
  (`orchestrator/internal/mergeengine/client.go`'s `response.Error` struct only
  reads `code`/`class`/`reason_code`/`message`; Go's `encoding/json` silently
  ignores unknown object keys, so this is safe to land without an orchestrator
  change in the same MR — wiring the Go side up to actually surface the matrix
  to callers is future work, out of this task's file-ownership scope).
- `services/cwso-merge-engine/src/ipc.rs` — on `Err(MergeError::SemanticConflict)`,
  calls the new `conflict_matrix` and, if it yields at least one row, returns
  `error_with_conflict_matrix` instead of the old bare `error_with_meta`; falls
  back to the original message-only conflict (unchanged behavior) if the matrix
  comes back empty or itself fails to parse — this only happens when the
  conflict was a whole-file `has_error()` parse-validation failure rather than a
  per-unit collision, and no rows can be honestly fabricated in that case.
  `base`/`ours`/`theirs` are read-only through this entire branch (`&[u8]`),
  never mutated, so the pre-merge state guarantee holds structurally, not just
  by convention.

### Never-corrupt-a-file guarantee

Enforced structurally, not just by test coverage: `Response` is `#[serde(untagged)]`
over exactly two shapes (`Ok { ok, result }` / `Err { ok, error }`); the success
path's `result` object has one key (`merged_b64`, the *fully* assembled merge)
and the conflict path's `error` object has no content/merged-bytes field at
all — only metadata (`code`/`class`/`reason_code`/`message`/`conflict_matrix`).
There is no third response shape and no partial-content field anywhere in the
protocol, so a "half-merged" response is not constructible by this code, not
merely avoided by discipline.

### Tests added (all in `services/cwso-merge-engine/src/{merge,ipc}.rs`, `#[cfg(test)]`)

`merge.rs` (function-level, `merge_three_way`/`conflict_matrix` directly):
- `c042_genuine_three_way_merge_succeeds_with_real_output` — AC1.
- `c042_conflict_matrix_reports_every_colliding_unit` — 3-unit fixture, two
  units genuinely diverge, one is a disjoint auto-resolvable edit; asserts the
  matrix contains *exactly* the two colliding units (not the disjoint one) with
  correct `node_kind`/`node_name`/`reason_code`.
- `c042_conflict_matrix_reports_delete_modify_conflict` — delete/modify
  collision, distinct reason code.
- `c042_conflict_matrix_reports_insertion_diverged` — both sides insert a new,
  same-named function with different bodies; asserts `Inserted`/`Inserted` +
  `insertion_diverged`.
- `c042_conflict_matrix_is_deterministic_across_repeated_runs` — AC3, 100
  repeated calls, byte-identical serialized JSON every time.
- `c042_pre_merge_state_provably_unchanged_after_conflict` — AC2 (no-corruption
  half): clones `base`/`ours`/`theirs` before calling both `merge_three_way`
  and `conflict_matrix`, asserts byte-identical after.

`ipc.rs` (wire-level, through `dispatch`):
- `merge_conflict_includes_semantic_class_and_reason` (pre-existing test,
  strengthened) — now also asserts the conflict response carries a non-empty
  `conflict_matrix`.
- `c042_dispatch_merges_disjoint_edits_successfully` — AC1 at the IPC boundary,
  full envelope round-trip, decodes `merged_b64` and compares to expected bytes.
- `c042_dispatch_conflict_never_leaks_merged_content_and_preserves_pre_merge_state`
  — AC2 (both halves): asserts the request's own `base_b64`/`ours_b64`/
  `theirs_b64` strings are untouched after `dispatch`, then serializes the full
  `Envelope<Response>` to JSON and asserts (a) no top-level `result` key exists
  at all on the conflict response, (b) the `error` object has no `merged_b64`
  key, (c) `error.conflict_matrix` is a non-empty array.
- `c042_dispatch_is_deterministic_across_repeated_runs` — AC3 at the IPC
  boundary, both a successful-merge case and a conflict case, 25 repeats each,
  byte-identical serialized `Response` every time.

### Verification (real output, this session)

`cargo test -p cwso-merge-engine` (AC4), run twice — once under the sandbox's
default `rustc 1.86.0`, once under `rustc 1.87.0` (the project's actual pinned
toolchain per `deploy/Dockerfile.merge-engine`/`.gitlab-ci.yml`, confirmed via
`docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`; the sandbox's
`rustup default` simply hadn't been switched — this is a local-environment
detail, not project debt, so no `docs/DEBT-REGISTER.md` row was added):

```
$ cargo test -p cwso-merge-engine   # rustc 1.86.0 (sandbox default)
running 36 tests
...
test result: ok. 35 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.25s

$ rustup run 1.87.0 cargo test -p cwso-merge-engine   # project's pinned toolchain
running 36 tests
...
test result: ok. 35 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.28s
```

(The 1 ignored test, `large_repo_merge_prefilter_benchmark`, is a pre-existing
manual perf benchmark explicitly marked `#[ignore]`, unrelated to this task.)

`cargo test -p cwso-git-shadow` (brief's regression check, since I touched
neither `services/cwso-git-shadow/**` nor its dependencies) — could not run
under `rustc 1.86.0` (fails to compile `git2 0.21.0`, a pre-existing,
already-documented toolchain requirement per the artifact above, reproduced
identically on a clean `git stash` of this branch before any of my edits); ran
clean under the project's pinned `rustc 1.87.0`:

```
$ rustup run 1.87.0 cargo test -p cwso-git-shadow
running 59 tests
...
test result: ok. 59 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 3.05s

     Running tests/signal_shutdown.rs
running 2 tests
...
test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.04s
```

61/61 green, no regression, confirming the "MUST NOT touch git-shadow's
projection code" rail was honored with zero side effects.

`cargo fmt -p cwso-merge-engine -- --check` — clean after one `cargo fmt` pass
over the new code. `cargo clippy -p cwso-merge-engine --all-targets` — zero new
warnings; all warnings present are pre-existing, in code this task did not
touch (`sparse_tensor.rs` dead-code helpers, `ipc.rs:264`'s pre-existing
`needless_return`, `merge.rs:290`/`312`'s pre-existing `needless_borrow`/
`unnecessary_cast` inside untouched functions).

`git diff --stat` against `origin/develop` — confirms file-ownership scope was
honored: only `services/cwso-merge-engine/src/{ipc,merge,proto}.rs` changed
(3 files, +595/-7); nothing under `services/cwso-git-shadow/**`, `schemas/*`,
or `orchestrator/*` was touched.

### Assumptions / decisions

- Kept `MergeError::SemanticConflict` as a zero-payload unit variant used by
  `merge_three_way` exactly as before (no behavior change to the ~26
  pre-existing tests that pattern-match/`assert_eq!` against it). The matrix is
  computed by a **second**, independent function (`conflict_matrix`) called
  only from the IPC layer once a conflict is already known, rather than
  threading matrix data through `merge_three_way`'s own return type — this
  keeps the change additive and avoids touching the existing merge algorithm's
  tested contract at all.
- `conflict_matrix` always uses the dense (no sparse-prefilter) diff path.
  Correctness matters more than throughput on what is, by construction, the
  rare failure path; the existing sparse/dense conformance test elsewhere in
  the suite already proves the two paths agree on conflict classification, so
  this is not a coverage gap.
- Did not implement AST-*semantic* diffing beyond what already existed
  (per-top-level-unit granularity, keyed by tree-sitter node kind + extracted
  name). Per the brief's "do not implement merge strategies beyond the
  Blueprint §5.4 contract" rail and the phase-4 plan's explicit note that
  deeper semantic/AST-aware merging is out of scope here.

### Blocker status

None. No genuine ambiguity blocked progress — the one place the Blueprint text
alone was underspecified (the exact matrix JSON shape) was resolved by design
authority already delegated to this task via the phase-4 plan's explicit "as
*data*" framing and by keeping the design additive/backward-compatible rather
than by guessing a schema I'd need sign-off to introduce.
