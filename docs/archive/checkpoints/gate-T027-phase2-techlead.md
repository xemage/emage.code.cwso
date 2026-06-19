# T027 — Tech Lead Review Gate (Phase 2)

**Reviewer:** tech-lead
**Date:** 2026-05-11
**Scope:** Phase 2 deliverables (T020 sidecar, T022 client+tools, T026 integration test, T028 debt scorecard)
**Verdict:** **CONDITIONAL_PASS**

---

## Method
Read-only audit per `.github/skills/code-review/SKILL.md` and `validation-gates/SKILL.md`. No code modified.

Sources reviewed:
- `services/cwso-git-shadow/src/{main,proto,repo,ast}.rs`
- `orchestrator/internal/shadow/client.go`
- `orchestrator/internal/tools/shadow_tools.go`
- `orchestrator/internal/server/server.go` (registration path)
- `orchestrator/internal/config/config.go`
- `deploy/Dockerfile.git-shadow`, `deploy/docker-compose.yml`
- `scripts/phase2-integration.py`
- `POC-DEBT-SCORECARD-phase2.md`
- Cross-checked against `docs/artifacts/requirements-v1.md` (FR-4, NFR-3, NFR-4) and `docs/artifacts/architecture-v1.md` §4 (permission tiers).

Validation evidence verified:
- `cargo test --release -p cwso-git-shadow` → 8/8 PASS (in Docker)
- `go test ./... -race` → all PASS (in `golang:1.23`, CGO_ENABLED=1)
- `python3 scripts/phase2-integration.py` → end-to-end PASS

---

## Requirement coverage

| Req | Status | Notes |
|-----|--------|-------|
| FR-4.1 (5 query types) | ✅ Met | All five implemented in `ast.rs::query` and exercised by integration test (Greet definition, Go entrypoint, Python exports). |
| FR-4.2 (Go/Rust/Python/TS) | ⚠️ Partial | Only Go + Python wired. Rust + TS missing. **Condition C1** below. (POC-DEBT P2-3) |
| FR-4.3 (Merkle incremental indexer, <400 ms) | ❌ Deferred | Every query re-parses. (POC-DEBT P2-2 → T029) — acceptable for PoC scale. |
| NFR-3 (zero host-fs writes from sub-agents) | ✅ Met | Shadow files live in the sidecar's bare repo on tmpfs. Baseline `WriteFileSync` host tool still exists but is gated by `pathGuard` to `/workspace`, mounted **read-only** in compose. |
| NFR-4 (deterministic state in Go kernel) | ✅ Met | All state transitions (workspace lifecycle, permission gate, request routing) live in Go. Sidecar is a stateless-ish blob/AST service. |
| Architecture §4 permission tiers | ✅ Met | `write_shadow_file` and `commit_shadow` are `Worker` only; `query_ast` and reads are both tiers. Verified live in test step 8 (orchestrator role correctly returned `-32002 permission_denied`). |

## Code-quality findings

### POSITIVE
1. **Lock discipline (Rust):** `workspaces` mutex is consistently dropped before acquiring `repo` mutex on the read/commit paths. No reentrant lock attempts on `parking_lot::Mutex` (which is non-reentrant). [src/repo.rs](services/cwso-git-shadow/src/repo.rs)
2. **Frame caps symmetric:** Both Rust (`FRAME_MAX = 8 MiB`) and Go (`frameMax = 8 MiB`) reject oversize frames. Header is fixed-width 4-byte big-endian on both sides.
3. **Fail-closed defaults:** Sidecar `check_path` rejects empty/absolute/`..`-containing paths; orchestrator client returns explicit errors instead of partial reads.
4. **Container hardening:** sidecar runs `read_only`, `cap_drop: ALL`, `no-new-privileges`, dedicated `cwso` user, `tini` as PID 1.
5. **Permission gate works in practice:** integration test step 8 proves the JWT-role → tool-allow-list pipeline blocks elevation.
6. **Tracing:** structured JSON logs via `tracing-subscriber` honour `RUST_LOG`.

### FINDINGS (block / condition / nit)

| ID | Severity | Where | Issue | Recommendation |
|----|----------|-------|-------|----------------|
| F1 | **HIGH** (condition) | `services/cwso-git-shadow/src/ast.rs` | Only Go + Python grammars wired; FR-4.2 requires Rust + TS by Phase 2. | Bundle into T029 before Phase 4 starts. (already POC-DEBT P2-3) |
| F2 | **HIGH** (condition) | `services/cwso-git-shadow/src/repo.rs::commit` | Orphan commits (no parent). Blocks any Phase-4 three-way merge. | Track HEAD per workspace, pass parent in `repo.commit`. Must land before T046. (POC-DEBT P2-4) |
| F3 | **HIGH** (condition) | `services/cwso-git-shadow/src/ast.rs::query` (`find_references`) | Identifier-text matching only — no scope/binding analysis. False positives across shadowed names, methods on different receivers, etc. | Adopt `tree-sitter` query DSL or a small resolver before T046. (POC-DEBT P2-7) |
| F4 | MEDIUM | `orchestrator/internal/{shadow,tools}` | No Go-side unit tests for the new package or the new tools. Integration test covers end-to-end, but unit isolation is missing. | **New task T028a** — add table-driven tests (mock UDS) for `shadow.Client` and `shadow_tools.go` argument validation. |
| F5 | MEDIUM | `services/cwso-git-shadow/src/repo.rs::write_file` | No per-blob size cap inside the sidecar; relies entirely on the 8 MiB wire frame. A future non-base64 path would bypass this. | Add an explicit `MAX_BLOB = 4 MiB` constant in `write_file`. |
| F6 | MEDIUM | `deploy/docker-compose.yml` | tmpfs `/var/lib/cwso/shadow:size=128m,mode=1777` is world-writable inside the container. Acceptable because the container has only one user, but the mode is sloppier than required. | Tighten to `mode=0700,uid=<cwso-uid>` once UID alignment lands in T029 (POC-DEBT P2-5). |
| F7 | LOW (nit) | `services/cwso-git-shadow/src/repo.rs::check_path` | `p.contains("..")` is over-broad (rejects e.g. `foo..bar.txt`). | Split on `/` and reject only `..` segments. |
| F8 | LOW (nit) | `services/cwso-git-shadow/src/ast.rs` | `Workspace.base_tree` is dead state after seeding; emits a `dead_code` warning. | Either drop or wire into `commit()` parent (overlaps with F2). |
| F9 | LOW (nit) | `orchestrator/internal/shadow/client.go` | `Client.mu` serializes every RPC. Functional but throttles concurrency — already noted as POC-DEBT P2-6. | Connection pool in T029. |
| F10 | LOW (nit) | `services/cwso-git-shadow/src/main.rs` | `_ensure_path_use(_p: &Path)` is a placeholder; remove. | Delete. |

## Security spot-check (OWASP delta from Phase 1)

| OWASP | Phase-2 impact | Status |
|-------|----------------|--------|
| A01 Broken Access Control | New tool surface; gate verified live (test step 8). | ✅ |
| A03 Injection | Sidecar consumes JSON only; libgit2 uses content-addressed OIDs. Path inputs validated. | ✅ |
| A04 Insecure Design | Sub-agents have no host-fs write path through Phase-2 tools. | ✅ |
| A05 Misconfiguration | Compose volume `cwso-runtime` shared between containers — necessary for UDS, scoped to compose project, not exposed externally. tmpfs mode 1777 is the only loose bit (F6). | ⚠️ Track |
| A08 Data Integrity | UDS frame-length is fixed-width and bounded; no length-prefix smuggling. | ✅ |
| A09 Logging | Structured JSON on both sides; no secrets logged. | ✅ |

No new CRITICAL or HIGH security findings. Security Engineer audit (T037) remains scheduled before release.

## Verdict & conditions

**CONDITIONAL_PASS** — Phase 2 advances. The following conditions are added to the active task list:

- **C1** → folded into **T029**: complete Rust + TypeScript grammars (F1 / P2-3) before T040 begins.
- **C2** → folded into **T029** with a hard gate: chained per-workspace commit history (F2 / P2-4) must land before T046.
- **C3** → folded into **T029** with a hard gate: scope-aware `find_references` (F3 / P2-7) must land before T046.
- **C4** → new task **T028a** (P0, owner backend-developer, depends on T028): Go unit tests for `internal/shadow` and `internal/tools/shadow_tools.go`.
- **C5** → folded into **T029**: tighten tmpfs mode and align UIDs (F6 / P2-5).
- **C6** → folded into **T029**: add explicit `MAX_BLOB` cap (F5).
- **C7** → housekeeping in T029: F7, F8, F10 (nits).

Phase 3 (T030+) is **unblocked**. T029 may run in parallel with T030–T033.
