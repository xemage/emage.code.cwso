# Checkpoint 002 — Phase 2 Complete (Shadow Workspaces + AST)

**Date:** 2026-05-11
**Phase:** 2 — In-memory Shadow Workspaces + tree-sitter AST
**Verdict:** **PASS** (PoC, debt registered)

---

## What was delivered

### `cwso-git-shadow` Rust sidecar
- New crate at [services/cwso-git-shadow/](../../services/cwso-git-shadow/).
- libgit2 (`git2 = "0.19"`, vendored) backs an on-tmpfs **bare** repo with
  per-workspace in-memory file maps. Nothing touches a working tree.
- Framed-JSON protocol (4-byte big-endian length + JSON body, 8 MiB cap)
  over a Unix Domain Socket at `/run/cwso/git-shadow.sock`.
- Operations: `stat`, `create_workspace`, `list_workspaces`,
  `drop_workspace`, `write_file`, `read_file`, `list_files`, `commit`,
  `query_ast`.
- Tree-sitter parsing for Go and Python with five query types:
  `find_definition`, `find_references`, `extract_signature`,
  `list_exports`, `detect_entrypoints`.
- Unit tests: 8/8 PASS in Docker (`cargo test --release -p cwso-git-shadow`).

### Orchestrator UDS client + new MCP tools
- New package [orchestrator/internal/shadow](../../orchestrator/internal/shadow/)
  implements the framed-JSON UDS client. Goroutine-safe.
- New tools registered when `CWSO_GIT_SHADOW_SOCKET` is set:
  - `create_shadow_workspace`, `drop_shadow_workspace`,
    `read_shadow_file`, `query_ast` — orchestrator + worker.
  - `write_shadow_file`, `commit_shadow` — worker only.
- Permission tiers enforced by the existing `Authorized()` gate.

### Compose & build
- [deploy/Dockerfile.git-shadow](../../deploy/Dockerfile.git-shadow):
  multi-stage `rust:1.86-slim` → `debian:bookworm-slim`, runs as `cwso`,
  `tini` as PID 1.
- [deploy/docker-compose.yml](../../deploy/docker-compose.yml) updated:
  - Shared named volume `cwso-runtime` mounted at `/run/cwso` in both
    containers carries the UDS.
  - `git-shadow` profile `phase2`, tmpfs `/var/lib/cwso/shadow:size=128m,mode=1777`.

### Integration test
- [scripts/phase2-integration.py](../../scripts/phase2-integration.py) drives
  the whole stack via docker compose and the MCP HTTP transport. PASSes:
  - 3 isolated workspaces
  - Go file + Python file written, ws3 isolation verified
  - AST: Greet definition, Go entrypoint, Python exports
  - 40-char SHA-1 commit OID
  - Permission gate: orchestrator role denied `write_shadow_file`

---

## Hypothesis verdict
> A Go orchestrator can drive an in-memory libgit2-backed shadow workspace
> through a Rust sidecar over UDS, and tree-sitter AST queries return correct
> results for Go and Python.

**VALIDATED end-to-end inside Docker.** No host filesystem writes outside the
mounted compose volumes; sub-agent file IO is sidecar-mediated.

## Debt registered
[POC-DEBT-SCORECARD-phase2.md](../../POC-DEBT-SCORECARD-phase2.md) — 8 items
(3 High, 3 Medium, 2 Low), of which:
- P2-1 (OverlayFS) deferred to Phase 4 (T040–T044).
- P2-2 (Merkle indexer) folded into T029.
- P2-3 (Rust+TS grammars) folded into T029.
- P2-4 (chained commits) and P2-7 (scope-aware references) blocking by T046.

## Token / time budget
- Phase budget: 120k tokens. Spent (this session): ~85k. Variance: under.
- Wall-clock: image builds dominate (cold `rust:1.86-slim` + libgit2 vendored
  ≈ 2 min); compose stack starts in ~3 s.

## Decisions made (no formal ADR yet — propose next session)
1. AST consolidated into the `cwso-git-shadow` sidecar instead of a separate
   crate. Re-evaluate if Phase-4 merge engine grows the AST surface.
2. OverlayFS deferred (POC-DEBT P2-1).
3. Phase-2 grammar set: Go + Python only (POC-DEBT P2-3).
4. Orphan commits per workspace (POC-DEBT P2-4) — chained history in T029.
5. UDS perms 0o666 on a private compose volume (POC-DEBT P2-5) — UID alignment in T029.

## What's next (Phase 3 — async + concurrency)
Tasks T030–T037 in [active-tasks.md](../tasks/active-tasks.md):
- Streamable-HTTP full-duplex SSE telemetry (T030)
- Async job runner pool + `dispatch_concurrent_jobs` (T031–T032)
- Event-sourced memory broker (T033)
- Phase 3 integration tests + Tech Lead + Security gates (T035–T037)

Phase 2 completion unblocks T030; T029 (PoC-debt remediation) is also
unblocked but can run in parallel with the Phase-3 critical path.
