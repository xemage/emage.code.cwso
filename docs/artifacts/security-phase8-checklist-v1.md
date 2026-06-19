# Phase 8 Security Checklist — Feature D (Sparse Merge Pre-Filter)

**Based on:** ADR-009, `sparse-ast-tensor-encoding-v1.md` §6, `SECURITY.md`, OWASP Top 10  
**Date:** 2026-06-04  
**Sign-off:** security-engineer (T130) — complements `gate-phase8-feature-d-2026-06-04.md`  
**Scope:** `services/cwso-merge-engine` sparse modules (T127–T129 on `develop` @ `0977483`)

| Control area | Requirement | Feature D (merge sparse) | Status |
|--------------|-------------|--------------------------|--------|
| A01 Access control | Deny by default; server-side authZ | No new MCP tools; merge IPC unchanged from ADR-006 | Pass |
| A02 Cryptography | Integrity / hashing | BLAKE3 payload digests in `sparse_diff`; no secrets on path | Pass |
| A03 Injection | Validated inputs | Merge inputs still tree-sitter parsed; sparse tensors built from parsed units only | Pass |
| A04 Insecure design | Advisory pre-filter | Dense merge authoritative; sparse cannot weaken conflict detection (T129) | Pass |
| A05 Misconfiguration | Safe defaults | Sparse path always on in production merge; equivalence proven in CI | Pass |
| A06 Vulnerable components | Dependency audit | `go:audit` / `rust:audit` CI `allow_failure` (inherited) | Conditional |
| A07 Auth failures | Session hygiene | N/A — no new auth surface | Pass |
| A08 Integrity | Deterministic output | Fixed key order; conformance suite guards regression | Pass |
| A09 Logging | No sensitive leakage | No PII in sparse tensor payloads (source bytes only in-process) | Pass |
| A10 SSRF | No egress | Pure in-process kernel | Pass |

**Overall:** All blocking controls **Pass**. A06 remains PoC-conditional (`allow_failure` audits).
