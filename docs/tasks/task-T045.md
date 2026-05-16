# Task T045 — `cwso-merge-engine` Rust crate

- Phase: **4 (Production)** · Owner: **backend-developer (Rust)** · Priority: **P0**
- Depends on: T044 · Blocks: T046, T047, T048
- Status: done

## Objective
Stand up the Rust sidecar that performs AST-aware semantic merge of multiple shadow workspaces. T045 establishes the crate, IPC protocol, tree-sitter integration, and a baseline three-way merge that handles trivially disjoint edits. Real auto-resolution algorithm and conflict-matrix output land in T046–T048.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-6
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3, §6
- [ADR-006](../decisions/ADR-006-semantic-ast-merge.md)
- `cwso-git-shadow` IPC protocol (T020 outputs)

## Constraints
- Crate at `services/cwso-merge-engine/`; member of [services/Cargo.toml](../../services/Cargo.toml).
- Languages supported in T045: same four as T023 (Go, Rust, Python, TypeScript).
- IPC: framed JSON over UDS at `/run/cwso/merge-engine.sock`, same envelope shape as T020.
- Pull base/ours/theirs blob bytes from `cwso-git-shadow` via UDS — no direct repo access from this crate.
- Strict immutability: never write to a shadow workspace from inside the merge engine; output is a new tree OID handed back to `cwso-git-shadow` for commit.
- Determinism: identical inputs MUST produce identical outputs across runs (test with stable seed).

## Expected outputs
- `services/cwso-merge-engine/Cargo.toml`, `src/main.rs`, `src/proto.rs`, `src/parse.rs`, `src/merge.rs`, `src/ipc.rs`
- Updated [deploy/Dockerfile.merge-engine](../../deploy/Dockerfile.merge-engine) producing a runnable image
- Compose profile `phase4` enables the sidecar
- Baseline merge: byte-identical files → no-op; one side modified → take that side; both sides identical change → take it; **otherwise return `unimplemented_conflict` error** (real algorithm in T046)
- Tests: per-language fixtures for the four trivial cases above

## Wire protocol (v0)
```json
{ "id": "uuid", "op": "merge_three_way",
  "params": {
    "language": "go|rust|python|typescript",
    "base_b64":   "...",
    "ours_b64":   "...",
    "theirs_b64": "..."
  }
}
```
Response (success):
```json
{ "id": "uuid", "ok": true, "result": { "merged_b64": "..." } }
```
Response (deferred conflict):
```json
{ "id": "uuid", "ok": false,
  "error": { "code": "unimplemented_conflict", "message": "AST node collision; T046 will resolve" } }
```

## Acceptance criteria
1. Sidecar starts, binds UDS, responds to `stat`.
2. Trivial three-way merges succeed for all four languages (4 × 3 cases = 12 fixtures).
3. Non-trivial collision returns `unimplemented_conflict` cleanly — never panics, never produces corrupt output.
4. Determinism test: 100 repeats of the same `merge_three_way` produce identical bytes.
5. `cargo test -p cwso-merge-engine` PASS in Docker.
6. Image scan: zero HIGH/CRITICAL CVEs.

## Blocker protocol
Same as T020.

## Completion notes (2026-05-15)
- Implemented baseline `cwso-merge-engine` Rust crate with framed UDS IPC and `stat`/`merge_three_way` ops.
- Added language-aware parse validation (Go, Rust, Python, TypeScript) and deterministic trivial three-way merge handling.
- Non-trivial collisions return structured `unimplemented_conflict` errors as required for T046 follow-on.
- Wired compose and merge-engine Docker image build path for Phase 4 runtime profile.

Validation summary:
- `cargo test -p cwso-merge-engine` in Docker: PASS (5 tests).
- `docker build -f deploy/Dockerfile.merge-engine -t cwso/merge-engine:test .`: PASS.
- Trivy image scan (`HIGH`,`CRITICAL`) on `cwso/merge-engine:test`: 0 vulnerabilities.
