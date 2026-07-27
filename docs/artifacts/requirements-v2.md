# Requirements v2 — CWSO (v0.4.0 GA and beyond)

> Owner: product-owner · Based on: `requirements-v1.md`, `cwso-nextgen-blueprint-v1.md` · Status: accepted for v0.4.1+
> **Supersedes:** `requirements-v1.md` (v0.1.0 scope); applies to v0.4.0+ releases

## 1. Product overview

CWSO is a **deterministic, event-sourced orchestration platform** that exposes the Model Context Protocol (MCP) to LLM clients and manages **swarms of specialized sub-agents** working in parallel on isolated shadow workspaces, with semantic AST-aware merging. v0.4.1 GA ships with full Phase 4 feature completeness (distributed locking, sandbox tier routing, semantic merge, ephemeral sparse agents). Phase 6+ capabilities (hardware-aware dispatch, Wasm scoring, offline SFT) remain planned.

## 2. Current personas (v0.4.1)

| Persona | Need |
|---------|------|
| Orchestrator LLM (primary client) | High-level planning tools (`dispatch_concurrent_jobs`, `query_ast`, `merge_concurrent_results`); deterministic ordering guarantees. |
| Sub-agent LLM (worker) | Constrained execution tools inside isolated shadow workspace; permission tier strictly enforced. |
| Platform engineer (operator) | Docker Compose + Rust/Go dev environment; reproducible builds; observability via JSON logs + OTEL; feature flags for gradual rollout. |
| Security auditor | OWASP-Top-10-aligned audit trail, no secrets in repo, sandbox isolation verification, deterministic conflict resolution proofs. |
| ML trainer (Phase 6+) | Trajectory capture from model API calls via rollout proxy; offline SFT generation; Polar-style reward integration. |

## 3. Functional requirements — Core (v0.4.1)

### FR-1 MCP server (completed)
- ✅ MCP capability negotiation and tool registration.
- ✅ **stdio** and **Streamable HTTP** transports (spec 2025-03-26).
- ✅ Tools: `read_file_sync`, `write_file_sync`, `list_dir`, `query_ast`, `create_shadow_workspace`, `dispatch_concurrent_jobs`, `merge_concurrent_results`.
- ✅ Server-Sent Events stream for telemetry and job notifications.

### FR-2 Shadow workspaces (completed)
- ✅ Ephemeral, in-memory Git branches via `libgit2`; no host working-tree writes.
- ✅ UUID-based identity; bound to sandbox profile; OverlayFS virtual FS layer.
- ✅ Deterministic ordering of commits within a workspace via event-sourced log.

### FR-3 Sandboxes (completed)
- ✅ Three runner tiers: `docker-trusted`, `gvisor-fast-ephemeral`, `firecracker-secure-isolation`.
- ✅ Untrusted (LLM-generated) code runs in Firecracker.
- ✅ Snapshot CoW for < 1ms clone times (Phase 4).

### FR-4 AST intelligence (completed)
- ✅ `query_ast` supports: `find_definition`, `find_references`, `extract_signature`, `list_exports`, `detect_entrypoints`.
- ✅ 4-language coverage Phase 2; 8+ languages Phase 4.
- ✅ Merkle-hashed incremental indexing; full 1k-file re-index < 400 ms.

### FR-5 Async dispatch (completed)
- ✅ `dispatch_concurrent_jobs` returns HTTP 202 + UUIDs; non-blocking.
- ✅ Background runner pool with deterministic ordering.
- ✅ Per-job server-side timeout with SIGKILL on expiry.

### FR-6 Semantic merge (completed)
- ✅ `merge_concurrent_results` performs AST-aware merge of N shadow workspaces.
- ✅ Auto-resolve on disjoint AST nodes; structured conflict matrix on collision.
- ✅ Deterministic conflict reason codes; no silent corruption.

### FR-7 Security (completed)
- ✅ Streamable HTTP validates `Origin` header (DNS rebinding protection).
- ✅ JWT (dev: HS256) or OAuth2 on all HTTP endpoints.
- ✅ Permission boundaries enforced server-side; no capability escalation.
- ✅ No secrets in source; configuration via env / mounted files.

### FR-8 Distributed locking (Phase 4, completed)
- ✅ 8-shard leader election for metadata consistency.
- ✅ Cascading provisioning of shadow workspaces across nodes.
- ✅ Deterministic shard assignment based on workspace UUID.

### FR-9 Rollout / Polar capture (Phase 6+, planned)
- ⏳ `/rollout/task/submit` REST API for task enqueueing.
- ⏳ Model API proxy at `:8787` (Rust sidecar) with trajectory capture.
- ⏳ Offline SFT generation mode via `/rollout/task/offline_generate`.
- ⏳ Trajectory builder v2 with prefix-merge strategy.

## 4. Non-functional requirements — Current targets

| ID | Target | Status |
|----|--------|--------|
| NFR-1 Latency | p95 tool call < 50 ms | ✅ Achieved in Phase 2+ |
| NFR-2 Concurrency | ≥ 8 sub-agents on dev host; ≥ 50 on KVM (Phase 4) | ✅ v0.4.1 tested |
| NFR-3 Isolation | Untrusted code 100% in Firecracker; zero host-fs writes | ✅ Completed |
| NFR-4 Determinism | All coordination in Go kernel; no LLM state mutation | ✅ Event-sourced |
| NFR-5 Observability | JSON logs; per-request correlation ID; OTEL spans | ✅ Completed |
| NFR-6 Portability | Docker Compose for local dev; KVM for Phase 4 runtime | ✅ Completed |
| NFR-7 Security | OWASP Top-10 pass before v0.1.0 | ✅ Phase 7 security gate PASS |

## 5. Phase 6+ Requirements (planned, not in v0.4.1)

### FR-10 Hardware-aware dispatch (Phase 6, partial)
- CPU / memory / GPU capability registry.
- Policy engine v2 for shard assignment based on hardware hints.
- Wasm-based scoring plugin support (optional).

### FR-11 Sparse micro-agents (Phase 7)
- On-demand ephemeral agent provisioning with memory caps.
- Cross-repo context stitching via semantic symbols.
- Sparse AST tensor encoding for reduced footprint.

### FR-12 AST spike analysis (Phase 7+)
- Proactive code-change impact analysis.
- Call-graph traversal for hot-path identification.

### FR-13 Polar integration (Phase 9)
- Trajectory builder v2 with prefix-merge and EOT masking.
- Gateway staging (INIT → READY → RUNNING → POSTRUN).
- Evaluator registry for post-run reward plugins.

## 6. Acceptance criteria (v0.4.1 GA)

✅ All Phase 4 acceptance criteria met:
1. MCP endpoint with JWT auth passes `mcp-inspector` conformance.
2. 8 concurrent sub-agents each modify ≥2 files in shadow workspaces; semantic merge produces clean unified commit.
3. Deliberate conflicts return structured conflict matrix with AST node references; no corruption.
4. OWASP Top-10 audit: zero CRITICAL/HIGH findings (Phase 7 gate passed).
5. `docker compose up --profile phase2 --profile phase4` brings full stack online.
6. End-to-end demo: `make demo` or `python3 scripts/phase2-integration.py`.

## 7. Out of scope (v0.4.1)

- Multi-host distributed orchestration (single host).
- Cloud deployment automation (artifacts only).
- Commercial LLM client UI.
- Multi-tenant SaaS / billing / quotas.
- Phase 6+ features (hardware-aware, sparse, rollout).

## 8. Roadmap

| Release | Focus | Status |
|---------|-------|--------|
| v0.1.0 | MCP core, stdlib tools | Released |
| v0.2.0 | Shadow workspaces, AST, Phase 2 | Released |
| v0.3.0 | Async dispatch, job runner, Phase 3 | Released |
| v0.4.0 | Distributed locking, sandbox routing, semantic merge, Phase 4 | Released |
| v0.4.1 | KV differential prompting, offline SFT PoC, hardening | Released (current) |
| v1.0.0 | Phase 6+ stability, hardware-aware, Polar integration | Planned |
