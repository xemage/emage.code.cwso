# PoC Debt Scorecard — Phase 2 (Shadow Workspaces + AST)

## Hypothesis
A Go orchestrator can drive an in-memory libgit2-backed shadow workspace
through a Rust sidecar over a Unix Domain Socket, and tree-sitter AST queries
return correct results for Go and Python — all without touching the host
filesystem.

## Result
**VALIDATED.** End-to-end integration test (`scripts/phase2-integration.py`)
PASSes: 3 isolated workspaces, Go+Python writes, AST queries
(`find_definition`, `detect_entrypoints`, `list_exports`), commit producing
SHA-1, and permission gate enforcement (orchestrator role denied
`write_shadow_file`) all green.

## Debt Inventory

| # | File | Location | Category | Description | Production Effort |
|---|------|----------|----------|-------------|-------------------|
| P2-1 | `services/cwso-git-shadow/src/main.rs` | module doc | Architecture | OverlayFS bind-mount layer deferred. Shadow files are accessed via IPC instead of an OS mount. Sub-agents that expect a real fs path will not work until T040–T044. | L — depends on Phase-4 sandbox runners |
| P2-2 | n/a (planning) | — | Performance | Merkle-hash incremental indexer (T025) not implemented. Every AST query re-parses the file. Latency is fine for PoC sizes (<1k LOC) but won't scale. | M — implement content-addressed parse cache keyed on blob OID |
| P2-3 | `services/cwso-git-shadow/Cargo.toml` and `src/ast.rs` | grammars | Language coverage | Only `tree-sitter-go` and `tree-sitter-python` are wired. Rust and TypeScript grammars are required by FR-3. | S — add 2 grammar crates and extend `Lang` enum |
| P2-4 | `services/cwso-git-shadow/src/repo.rs` | `commit()` | Correctness | Each commit is an orphan (no parent). Workspaces never form a chain, so per-workspace history and three-way merges are unavailable. | M — track HEAD per workspace and pass parent commit |
| P2-5 | `services/cwso-git-shadow/src/main.rs` | socket perms | Security | UDS perms are 0o666 because orchestrator and sidecar containers run under different UIDs. Acceptable on a private compose-managed bind volume; not acceptable for prod. | S — align UIDs in images and use 0o660 with a shared GID |
| P2-6 | `orchestrator/internal/shadow/client.go` | connection model | Performance | One-shot connection per RPC; no pooling, no pipelining. Sufficient for PoC throughput; will throttle under Phase-3 concurrent dispatch. | M — connection pool + request multiplexer |
| P2-7 | `services/cwso-git-shadow/src/repo.rs` | `query_ast` | Correctness | `find_references` matches identifier text only — no scope/binding analysis. False positives across shadowed names. | M — use tree-sitter queries with scope resolution or a lightweight resolver |
| P2-8 | `services/cwso-git-shadow/src/ast.rs` | `Workspace.base_tree` | Cleanliness | `base_tree` is stored but never read after seeding the file map. Dead state until T029. | S — drop or wire into commit parent |

## Phase-1 debts that touched Phase-2 paths
- D1 (hand-rolled MCP) — still present in the shadow tools' input-schema
  shape; will be reconciled with the official go-sdk in T029.
- D2 (HS256 JWT) — still gates the new tools; same remediation in T029.
- D6 (no rate limiting) — Phase-2 endpoints inherit this gap.

These are tracked in the canonical
[POC-DEBT-SCORECARD-phase1.md](POC-DEBT-SCORECARD-phase1.md) and remain open.

## Summary
- New Phase-2 debt items: 8
- Critical (must fix before production): 3 (P2-1, P2-3, P2-7)
- Medium (should fix before production): 3 (P2-2, P2-4, P2-6)
- Low / cleanup: 2 (P2-5, P2-8)

## Recommendation
**Go for Phase 3** with the following conditions:
1. P2-3 (Rust+TS grammars) is bundled into T029 (PoC-debt remediation).
2. P2-4 (chained commits) and P2-7 (scope-aware references) are required
   before the Phase-4 semantic-merge work; they should not be deferred past
   T046.
3. P2-1 (OverlayFS) blocks any tool that expects a host file path inside the
   sandbox; the Phase-3 async runner must continue to call shadow tools via
   IPC until T044 lands.
