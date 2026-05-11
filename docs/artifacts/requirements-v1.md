# Requirements v1 — CWSO

> Owner: product-owner · Based on: `CWSO_ Agentic AI Orchestration Blueprint.md` · Status: accepted

## 1. Product overview
The Concurrent Workspace & Swarm Orchestrator (CWSO) is a **deterministic backend kernel** that exposes the Model Context Protocol (MCP) to LLM clients (Claude Desktop, custom terminals, `mcp-inspector`) and orchestrates **swarms of specialized LLM sub-agents** working in parallel on a shared codebase. Coordination, state, security boundaries, and merge logic are owned by the backend — never by the LLM.

## 2. Personas
| Persona | Need |
|---------|------|
| Orchestrator LLM (primary client) | High-level planning tools (`dispatch_concurrent_jobs`, `query_ast`, `merge_concurrent_results`); never blocked on tool calls. |
| Sub-agent LLM (worker) | Constrained execution tools inside an isolated shadow workspace; cannot escalate or spawn further agents. |
| Platform engineer (operator) | Reproducible Docker dev environment, observability, secure defaults, easy local run. |
| Security auditor | OWASP-Top-10-aligned audit trail, no secrets in repo, sandbox isolation guarantees. |

## 3. Functional requirements

### FR-1 MCP server
- FR-1.1 Implement MCP capability negotiation, tool registration, and JSON-RPC 2.0 message handling.
- FR-1.2 Support both **stdio** and **Streamable HTTP** transports (spec 2025-03-26).
- FR-1.3 Expose tools: `read_file_sync`, `write_file_sync`, `list_dir`, `query_ast`, `create_shadow_workspace`, `dispatch_concurrent_jobs`, `merge_concurrent_results`.
- FR-1.4 Server-Sent Events stream for unidirectional telemetry pushed to client.

### FR-2 Shadow workspaces
- FR-2.1 Create ephemeral, in-memory Git branches via `libgit2` ODB manipulation; **no host working-tree writes**.
- FR-2.2 Each shadow workspace is uniquely identified by UUID and bound to a sandbox profile.
- FR-2.3 Sub-agents see a virtual filesystem (OverlayFS) layered over a read-only base commit projection.

### FR-3 Sandboxes
- FR-3.1 Three runner tiers selectable per dispatch: `docker-trusted`, `gvisor-fast-ephemeral`, `firecracker-secure-isolation`.
- FR-3.2 Untrusted (LLM-generated) code MUST run in Firecracker.
- FR-3.3 Firecracker uses snapshot CoW for sub-millisecond clone.

### FR-4 AST intelligence
- FR-4.1 `query_ast` supports queries: `find_definition`, `find_references`, `extract_signature`, `list_exports`, `detect_entrypoints`.
- FR-4.2 Initial language coverage: Go, Rust, Python, TypeScript (Phase 2). Extended grammars (≥10 languages) by Phase 4.
- FR-4.3 Indexing is incremental via Merkle hashing; full re-index of a 1k-file repo < 400 ms.

### FR-5 Async dispatch
- FR-5.1 `dispatch_concurrent_jobs` returns HTTP 202 + UUIDs immediately; never blocks the caller.
- FR-5.2 Background runner pool executes jobs; lifecycle events stream over SSE as JSON-RPC notifications.
- FR-5.3 Per-job timeout enforced server-side; SIGKILL on expiry.

### FR-6 Semantic merge
- FR-6.1 `merge_concurrent_results` performs AST-aware merge of N shadow workspaces into a target ref.
- FR-6.2 Auto-resolve when changes are on disjoint AST nodes.
- FR-6.3 On unresolvable collision, return structured conflict matrix (no silent corruption).

### FR-7 Security
- FR-7.1 Streamable HTTP endpoints validate `Origin` header (DNS-rebinding protection).
- FR-7.2 JWT or OAuth2 session enforced on all HTTP endpoints.
- FR-7.3 Permission boundaries: Orchestrator role cannot invoke worker-tier tools; Worker role cannot dispatch.
- FR-7.4 No secrets in source; configuration via env vars / mounted secret files.

## 4. Non-functional requirements
| ID | Target |
|----|--------|
| NFR-1 Latency | p95 tool-call < 50 ms (Phase 1); p95 workspace-create < 200 ms (Phase 2); SSE end-to-end push < 100 ms (Phase 3) |
| NFR-2 Concurrency | ≥ 8 concurrent sub-agents on commodity dev host; ≥ 50 on KVM-equipped host (Phase 4) |
| NFR-3 Isolation | Untrusted code 100% executed in Firecracker; zero host-fs writes from sub-agents |
| NFR-4 Determinism | Coordination/state transitions live in Go kernel; no LLM-driven state mutation |
| NFR-5 Observability | Structured JSON logs; per-request correlation ID; OpenTelemetry spans on all tool calls |
| NFR-6 Portability | All components run in Docker Compose for local dev; KVM only required at Phase 4 runtime |
| NFR-7 Security | Pass OWASP Top-10 audit before v0.1.0; no CRITICAL/HIGH findings |

## 5. Acceptance criteria (program-level)
1. `mcp-inspector` capability conformance passes against the running server.
2. A demo script dispatches 8 sub-agents that each modify ≥2 files in disjoint shadow workspaces; semantic merge produces a clean unified commit on `main`.
3. A deliberately-conflicting two-agent scenario returns a conflict matrix with correct AST node references; no file corruption.
4. OWASP Top-10 audit report shows no CRITICAL or HIGH findings.
5. `docker compose up` brings the stack online; `make demo` runs the end-to-end demo.

## 6. Out of scope (v0.1.0)
- Multi-host distributed orchestration (single host only).
- Cloud deployment automation (artifacts only; not Kubernetes).
- Commercial LLM client UI.
- Multi-tenant SaaS / billing / quotas.
