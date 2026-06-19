# Phase 7 Security Checklist — Features B + C

**Based on:** ADR-008, `wasm-sparse-agent-design-v1.md` §9, `SECURITY.md`, OWASP Top 10  
**Date:** 2026-06-04  
**Sign-off:** security-engineer (T125) — complements `gate-phase7-feature-bc-2026-06-04.md`
**Merged:** MR !34 → `develop` @ `146f208` (2026-06-04)

| Control area | Requirement | Feature B (sparse) | Feature C (AST spikes) | Status |
|--------------|-------------|--------------------|-------------------------|--------|
| A01 Access control | Deny by default; server-side authZ | Orchestrator-only `create_ephemeral_sparse_agent`; UDS peer-auth on `cwso-sparse` | Spike tools/resources config-gated; subscription resolver returns 404 for unknown ids | Pass |
| A02 Cryptography | No cleartext secrets; integrity pinning | SHA-256 pins for `.cwsl` slices + optional Wasm module; TLS guidance inherited from T093 for HAL escalation | N/A (no crypto material on spike path) | Pass |
| A03 Injection | Parameterized / validated inputs | Framed JSON IPC; GEMM shape validation; tool arg validation (`skill_domain`, `quality_floor`, RAM cap) | Path glob + workspace scope validation on subscribe | Pass |
| A04 Insecure design | Threat-modeled tier (ADR-008) | wasmtime sandbox + single host-call; quality-floor → dense fallback always reachable | Event-driven monitor (no idle polling); eBPF optional with userspace fallback | Pass |
| A05 Misconfiguration | Safe defaults | `CWSO_SPARSE_*` and AST flags default false; socket required when agents enabled | Spike monitor/resources off unless explicitly enabled | Pass |
| A06 Vulnerable components | Dependency audit | `go:audit` / `rust:audit` in CI (`allow_failure` — see gate finding #1) | Same | Conditional |
| A07 Auth failures | Session/token hygiene | No new auth surface; uses existing orchestrator role model | SSE scoped to subscription id | Pass |
| A08 Integrity | Signed/pinned artifacts | Content-addressed `.cwsl` + manifest; module integrity when path configured | Broker records immutable once published | Pass |
| A09 Logging | No sensitive leakage | Structured tracing; redaction policy drops hot paths/notes/symbols when configured | Pass |
| A10 SSRF | No request-driven egress | Sidecar has no network; Wasm module has no WASI net | Spike pipeline has no outbound URL fetch | Pass |

**Overall:** All blocking controls **Pass**. A06 remains PoC-conditional (`allow_failure` audits).
