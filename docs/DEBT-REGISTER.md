# CWSO Debt Register

**Status:** live — this is the single place to look for outstanding proof-of-concept debt.
**Created:** 2026-08-12 (task C003)
**Supersedes (as history, not content):** `docs/archive/debt/POC-DEBT-SCORECARD-phase1.md`, `docs/archive/debt/POC-DEBT-SCORECARD-phase2.md`

## Purpose

Until this file existed, CWSO's debt was scattered across two archived phase
scorecards (`docs/archive/debt/`) and inline `POC-DEBT` code comments, with no
single view of what is closed, what blocks v1.0, and what is deferred. This
register consolidates all three sources into one live table. Every row carries a
disposition:

| Disposition | Meaning |
|---|---|
| `v1.0-blocker` | Must be closed (or explicitly documented as a limitation) before v1.0 is declared. The closing C-task is named in the row. |
| `v1.1` | Real debt, deliberately deferred past v1.0. |
| `wontfix` | Reviewed and consciously accepted as-is. |
| `fixed` | Closed in code; the evidence column quotes the verifying file/symbol. |
| `unclear` | Source row unintelligible or contradicts the code — flagged, never guessed. |

Rows are identified by their **original** IDs where one exists (roadmap blockers
`B1`–`B13`, phase-1 scorecard `D1`–`D8`, phase-2 scorecard `P2-1`–`P2-8`).
Scorecard-only items with no prior ID are numbered `R-x` here. Historical
scorecard sections at the bottom record where each row came from and whether it
was carried forward or closed.

## Live register

| ID | Source `file:line` | Category | Description | Status | Disposition | Closing task |
|---|---|---|---|---|---|---|
| B1 (= D1, P1-1) | `orchestrator/internal/mcp/protocol.go:10` | Maintainability / spec compliance | Hand-rolled MCP protocol subset instead of the official `go-sdk`; only a partial method set is implemented | closed | fixed | C030–C032 |
| B2 (= P2-1) | `services/cwso-git-shadow/src/main.rs:11` | Architecture | OverlayFS bind-mount layer deferred; shadow files are reachable only via orchestrator→sidecar IPC, so sub-agents that expect a real filesystem path cannot work | open | v1.0-blocker | C020–C025 |
| B6 (= P2-7) | scorecard P2-7 (`services/cwso-git-shadow/src/repo.rs`, `query_ast`) | Correctness | `find_references` matches identifier text only — no scope/binding analysis; false positives across shadowed names | open | v1.0-blocker | C040 |
| B7 (= P2-4) | `services/cwso-git-shadow/src/repo.rs:180` | Correctness | Every shadow commit is an orphan (no parent); workspaces never form a history chain, so per-workspace history and three-way merges are unavailable | open | v1.0-blocker | C041 |
| B12 (= P2-5) | scorecard P2-5 (`services/cwso-git-shadow/src/main.rs`, socket perms) | Security | UDS permissions are `0o666` because orchestrator and sidecar containers run under different UIDs; acceptable on a private compose-managed bind volume, not acceptable for prod | open | v1.0-blocker | C044 |
| B13 (= P2-6) | `orchestrator/internal/shadow/client.go:5` | Performance | One TCP-style request-per-connection model — no pooling, no pipelining; will throttle under concurrent dispatch | open | v1.0-blocker | C043 |
| B11 | `orchestrator/internal/rollout/evaluator_swebench.go:64` | Functionality | SWE-bench/SWE-Gym evaluator is a stub — harness launch deferred; returns neutral reward | open | v1.1 | — |
| P2-2 | scorecard P2-2 (planning item, no code marker) | Performance | Merkle-hash incremental indexer not implemented; every AST query re-parses the file. Fine at PoC sizes (<1k LOC), will not scale | open | v1.1 | — |
| R-1 (= P1-5) | `deploy/docker-compose.yml:6` | Security | File-based JWT secret (`../.env.jwt.dev`) acceptable for dev/compose; production needs external secret management (Vault/SOPS). v1.0 is local-only, so this is acceptable **if documented** | open | v1.0-blocker (document) | C063 |
| R-2 (= P1-5, prod half / T029) | `deploy/docker-compose.yml:5` | Security / operations | Vault/SOPS external secret management (T029) not started — the production half of the compose-secret debt | open | v1.1 | — |
| P2-3 | `services/cwso-git-shadow/Cargo.toml:20` | Language coverage | Only Go + Python tree-sitter grammars wired at Phase 2; Rust and TypeScript required by FR-3 | closed | fixed | — |
| D6 (= P1-7) | `orchestrator/internal/transport/http.go` (rate limiting) | Security | No per-IP rate limit on `/mcp` POST at Phase 1; relied on JWT to gate | closed | fixed | — |
| D2 (= P1-2) | `orchestrator/internal/transport/http.go` (`verifyHS256`) | Security | Hand-rolled HS256 JWT verifier; production must use `golang-jwt/jwt/v5` with RS256, key rotation, full claims validation (iss, aud, nbf, exp leeway) | open | v1.1 | — |
| D3 (= P1-3) | `orchestrator/internal/transport/http.go` (`handleSSE`) | Functionality | SSE GET endpoint emits heartbeats only; real notifications deferred to Phase 3 EventBus integration (T030) | open | v1.1 | — |
| D4 (= P1-4) | `orchestrator/internal/logging/logger.go` (package doc) | Observability | Stdlib-only logger; production should adopt `zerolog` + OTEL integration | open | v1.1 | — |
| D5 (= P1-6) | `orchestrator/internal/server/server.go` (`handleInitialize`) | Spec compliance | Capability negotiation declares `tools.listChanged: false`; full capability set (resources, prompts, sampling) deferred | open | v1.1 | — |
| D8 (= P1-8) | `orchestrator/internal/tools/fs_tools.go` (1 MiB cap) | Robustness | Read cap is hard-coded at 1 MiB; production should expose it via config | open | v1.1 | — |
| P2-8 | `services/cwso-git-shadow/src/ast.rs` (`Workspace.base_tree`) | Cleanliness | `base_tree` is stored but never read after seeding the file map; dead state pending T029 | open | v1.1 | — |

### Notes on the `fixed` rows (verification evidence)

- **P2-3 — grammar coverage: `fixed`.** `services/Cargo.toml` does not wire
  grammars itself (it is the workspace manifest); the scorecard's reference is
  to the crate manifest. `services/cwso-git-shadow/Cargo.toml` declares
  `tree-sitter-go = "0.21"`, `tree-sitter-python = "0.21"`,
  `tree-sitter-rust = "0.21"`, `tree-sitter-typescript = "0.21"`, and
  `services/cwso-git-shadow/src/ast.rs` uses all four
  (`tree_sitter_go::language()`, `tree_sitter_python::language()`,
  `tree_sitter_rust::language()`,
  `tree_sitter_typescript::language_typescript()`), with Rust and TypeScript
  unit tests present. The stale `POC-DEBT P2-3` comment on
  `services/cwso-git-shadow/Cargo.toml:20` is removed by the debt-closing work;
  this register does not touch code.
- **D6 — rate limiting: `fixed`.** `orchestrator/internal/transport/http.go`
  implements per-IP token-bucket rate limiting: import of
  `golang.org/x/time/rate` (line 21), `newRateLimiterStore(ctx)` (line 183),
  `rateLimitMiddleware(...)` wired into the handler chain (line 190), SSE
  connection limiting (lines 210–214), and the section comment
  `// --- Rate limiting middleware (T029 remediation #7) ---` (line 665)
  documenting the default of 60 requests/minute.

### In-code `POC-DEBT` marker cross-check

Per the C003 brief, code markers are the hits of
`grep -rn "POC-DEBT" . --exclude-dir=.git --exclude-dir=docs` in **project code**
(deployable services and deploy configuration) — harness/skill documentation
templates under `.gemini/`, `.cline/`, `.claude/`, `.cursor/`, `.github/`,
`.opencode/`, `.pi/`, and `.gitlab/` are examples of the tagging convention, not
CWSO product debt, and are intentionally not register rows.

| Marker location | Register row |
|---|---|
| `deploy/docker-compose.yml:6` | R-1 |
| `services/cwso-git-shadow/Cargo.toml:20` | P2-3 (fixed) |
| `services/cwso-git-shadow/src/main.rs:11` | B2 |
| `services/cwso-git-shadow/src/repo.rs:180` | B7 |
| `orchestrator/internal/mcp/protocol.go:10` | B1 (fixed) |
| `orchestrator/internal/shadow/client.go:5` | B13 |
| `orchestrator/internal/rollout/evaluator_swebench.go:64` | B11 |

7/7 code markers are represented above. (`orchestrator/internal/mcp/protocol.go:12`
is the continuation line of the B1 comment, not a separate marker.)

---

## Historical scorecard — Phase 1 (archived)

Source: `docs/archive/debt/POC-DEBT-SCORECARD-phase1.md` (Phase 1 MCP core,
2026-05; hypothesis **VALIDATED**). Rows keep their original numbering; the
scorecard's `#n` is aliased `P1-n` here for unambiguous reference.

| Scorecard row | Description (abridged) | Register ID | Carried-forward or closed |
|---|---|---|---|
| P1-1 (#1) | Hand-rolled MCP subset; adopt official `go-sdk` | B1 | carried-forward → v1.0-blocker (C030–C032) |
| P1-2 (#2) | Hand-rolled HS256 JWT verifier; needs RS256/JWKS/claims validation | D2 | carried-forward → v1.1 |
| P1-3 (#3) | SSE endpoint emits heartbeats only | D3 | carried-forward → v1.1 |
| P1-4 (#4) | Stdlib-only logger; adopt `zerolog` + OTEL | D4 | carried-forward → v1.1 |
| P1-5 (#5) | Dev JWT secret via env/file; prod needs vault | R-1 / R-2 | carried-forward → v1.0-blocker, documented (C063); prod Vault/SOPS half → v1.1 |
| P1-6 (#6) | Capability negotiation minimal (`tools.listChanged: false`) | D5 | carried-forward → v1.1 |
| P1-7 (#7) | No per-IP rate limit on `/mcp` POST | D6 | **closed** — rate limiting implemented in `orchestrator/internal/transport/http.go` (evidence above) |
| P1-8 (#8) | 1 MiB read cap hard-coded | D8 | carried-forward → v1.1 |

## Historical scorecard — Phase 2 (archived)

Source: `docs/archive/debt/POC-DEBT-SCORECARD-phase2.md` (Phase 2 shadow
workspaces + AST, 2026-05; hypothesis **VALIDATED**).

| Scorecard row | Description (abridged) | Register ID | Carried-forward or closed |
|---|---|---|---|
| P2-1 | OverlayFS bind-mount deferred; IPC-only shadow files | B2 | carried-forward → v1.0-blocker (C020–C025) |
| P2-2 | No Merkle incremental indexer; every query re-parses | P2-2 | carried-forward → v1.1 |
| P2-3 | Only Go + Python grammars wired; Rust/TS required | P2-3 | **closed** — four grammars wired in `services/cwso-git-shadow/Cargo.toml` (evidence above) |
| P2-4 | Orphan commits; no history chain | B7 | carried-forward → v1.0-blocker (C041) |
| P2-5 | UDS perms `0o666` across differing UIDs | B12 | carried-forward → v1.0-blocker (C044) |
| P2-6 | One connection per RPC; no pooling | B13 | carried-forward → v1.0-blocker (C043) |
| P2-7 | `find_references` is text matching, not scope resolution | B6 | carried-forward → v1.0-blocker (C040) |
| P2-8 | `base_tree` stored but never read | P2-8 | carried-forward → v1.1 |

The phase-2 scorecard also notes that phase-1 items D1, D2, and D6 "touched
Phase-2 paths". Those cross-references remain valid: D1 → B1, D2 → D2 (v1.1),
D6 → closed.

---

*This register is kept current by every debt-closing task and is fully
re-classified by **C060** at release time (`fixed` / `documented-limitation` /
`v1.1` — no unclassified rows); its `documented-limitation` rows feed
`docs/LIMITATIONS.md` (C063). New `POC-DEBT` tags must be added here when
introduced.*
