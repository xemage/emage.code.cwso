# Task T046 — AST diff + semantic merge algorithm

- Phase: **4 (Production)** · Owner: **backend-developer (Rust)** · Priority: **P0**
- Depends on: T045 · Blocks: T047
- Status: done

## Objective
Implement AST-aware semantic merge logic in `cwso-merge-engine` to auto-resolve disjoint structural edits safely and deterministically. This task upgrades merge behavior beyond trivial cases while preserving explicit conflict outcomes for unresolved collisions.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-6
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [ADR-006](../decisions/ADR-006-semantic-ast-merge.md)
- [task-T045.md](./task-T045.md)
- [plan-T046-phase4-semantic-merge-algorithm.md](../plans/plan-T046-phase4-semantic-merge-algorithm.md)

## Constraints
- Keep merge engine crate path and IPC contract stable from T045.
- Supported languages remain: Go, Rust, Python, TypeScript.
- Deterministic behavior required: identical inputs -> identical outputs.
- Never silently corrupt output; unresolved ambiguity must return conflict response.
- Avoid direct workspace writes; maintain immutable merge-engine behavior.
- Keep T045 trivial cases green while extending semantic capability.

## Expected outputs
- Enhanced AST diff + semantic merge logic in `services/cwso-merge-engine/src/merge.rs` (and supporting modules)
- Additional parsing/normalization helpers as needed
- Fixture/test suite covering disjoint auto-resolution and collision scenarios
- Determinism evidence for new semantic paths

## Acceptance criteria
1. Disjoint AST-node edits from `ours` and `theirs` auto-resolve to a valid merged output for all four languages.
2. Overlapping/colliding edits return explicit conflict error (still no corruption).
3. Existing trivial-case behavior from T045 remains passing.
4. Determinism tests pass for semantic-resolution paths.
5. `cargo test -p cwso-merge-engine` passes.
6. Merge output remains parse-valid for selected language when auto-resolution succeeds.
7. Task output unblocks T047.

## Blocker protocol
Same as T020. Escalate technical blocker if language-normalization gaps prevent deterministic, safe resolution without over-broad conflicting.

## Completion notes (2026-05-16)
- Implemented deterministic AST-aware semantic merge in `services/cwso-merge-engine/src/merge.rs` with top-level AST unit normalization, per-side node diffing, disjoint auto-resolution, and explicit overlap conflict detection.
- Preserved T045 trivial merge behavior (`ours == theirs`, one-side-changed, identical changes) while extending non-trivial semantic resolution.
- Kept IPC envelope/operation contract stable from T045; overlap failures continue returning the existing error code with updated semantic conflict messaging.
- Added semantic tests across Go/Rust/Python/TypeScript for disjoint resolution, overlap conflicts, determinism (100 repeats), and parse-valid merged output assertions.

Validation summary:
- `cargo test -p cwso-merge-engine` (via Rust Docker image): PASS (7 tests).
- Confirms T046 outputs unblock T047.
