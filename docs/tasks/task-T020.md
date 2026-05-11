# Task T020 — `cwso-git-shadow` Rust crate

- Phase: **2 (PoC)** · Owner: **backend-developer (Rust)** · Priority: **P0**
- Depends on: T011 · Blocks: T021, T022, T023 (indirectly)
- Status: pending

## Objective
Implement the Rust sidecar that owns all in-memory Git Object Database operations for shadow workspaces. The sidecar exposes a small, framed-JSON request/response protocol over a Unix domain socket; the Go orchestrator calls it to (a) initialize a bare repo on tmpfs from a base commit, (b) create lightweight workspace handles, (c) write blobs and trees without touching the host working tree, and (d) tear workspaces down cleanly.

## Inputs (read these first)
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-2
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §3
- [ADR-004](../decisions/ADR-004-in-memory-git-odb.md)

## Constraints
- Crate lives at `services/cwso-git-shadow/`; integrates into existing workspace [services/Cargo.toml](../../services/Cargo.toml).
- Use `git2 = "0.19"` (libgit2 bindings) only; no shell-out to `git`.
- Build inside `rust:1.80-slim` Docker image (system `libgit2-dev`, `pkg-config`).
- All workspace storage is **tmpfs-backed** (mounted at `/var/lib/cwso/shadow`); zero writes to repo working tree.
- IPC: framed JSON over UDS at `/run/cwso/git-shadow.sock`. Frame = 4-byte big-endian length + JSON body. No HTTP.
- Strict per-workspace memory ceiling: 256 MiB; reject creation when exceeded.
- All paths traversed inside the bare repo MUST be validated against the workspace's allowed scope.
- POC-DEBT tags allowed and required for any shortcut; logged in T028 scorecard.

## Expected outputs
- `services/cwso-git-shadow/Cargo.toml`, `src/main.rs`, `src/proto.rs`, `src/repo.rs`, `src/ipc.rs`
- Updated [deploy/Dockerfile.git-shadow](../../deploy/Dockerfile.git-shadow) producing a runnable image (replace placeholder)
- Compose profile `phase2` enables the sidecar; UDS shared via named volume
- Unit tests for: open base repo, create workspace, hash-write blob, build tree, list entries, tear down
- Integration test (Rust): create 5 concurrent workspaces from a fixture repo, mutate disjoint paths, verify zero working-tree writes

## Wire protocol (v0)
Request envelope:
```json
{ "id": "uuid", "op": "create_workspace|write_blob|build_tree|read_blob|drop_workspace|stat", "params": { ... } }
```
Response envelope:
```json
{ "id": "uuid", "ok": true, "result": { ... } }      // success
{ "id": "uuid", "ok": false, "error": { "code": "...", "message": "..." } }
```
Operations (minimum):
| op | params | result |
|----|--------|--------|
| `create_workspace` | `{ base_commit_sha?: string }` | `{ workspace_uuid, base_tree_oid }` |
| `write_blob` | `{ workspace_uuid, path, content_b64 }` | `{ blob_oid }` |
| `build_tree` | `{ workspace_uuid }` | `{ tree_oid, commit_oid }` |
| `read_blob` | `{ workspace_uuid, path }` | `{ content_b64, oid }` |
| `drop_workspace` | `{ workspace_uuid }` | `{ dropped: true }` |
| `stat` | `{}` | `{ workspaces: int, mem_bytes: int }` |

## Acceptance criteria
1. Sidecar starts in container, binds UDS, accepts ping/`stat`.
2. `create_workspace` against a fixture bare repo returns a UUID and base tree OID; **no** files appear in any host working tree.
3. 5 concurrent `create_workspace` calls succeed; total resident memory < 128 MiB on a 1k-file fixture.
4. `write_blob` + `build_tree` produces a new tree OID distinct from the base; `read_blob` round-trips identical bytes (including binary content).
5. `drop_workspace` releases memory measurably (validated via `stat` before/after).
6. `cargo test -p cwso-git-shadow` PASS in Docker.
7. POC-DEBT tags present for any shortcut; ready for T028 inventory.

## Blocker protocol
Report blockers to the orchestrator with `type` ∈ `{technical, dependency, unclear_requirements, external}` and `severity` ∈ `{critical, major, minor}`. Do not silently retry beyond two attempts.
