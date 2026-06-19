# Phase 9 Security Checklist — Features E + F + G (Rollout)

**Based on:** ADR-010, `rollout-architecture-v1.md` §11, `SECURITY.md`, OWASP Top 10  
**Date:** 2026-06-05  
**Sign-off:** security-engineer (T138) — complements `gate-phase9-feature-efg-2026-06-05.md`  
**Scope:** `services/cwso-rollout`, `orchestrator/internal/rollout`, merge reward hook (T132–T137 on `develop` @ `c1c56d6`)

| Control area | Requirement | Rollout stack | Status |
|--------------|-------------|---------------|--------|
| A01 Access control | Deny by default | JWT on `/rollout/*`, `/callbacks/*`, `/nodes/*`; UDS peer cred on sidecar | Pass |
| A02 Cryptography | TLS for egress | Upstream TLS validation in proxy (HAL pattern) | Pass |
| A03 Injection | Validated inputs | JSON schema for submit/status; bounded request bodies | Pass |
| A04 Insecure design | Non-blocking capture | Bounded queue + drop counter; store on dedicated thread | Pass |
| A05 Misconfiguration | Safe defaults | All rollout flags default off | Pass |
| A06 Vulnerable components | Dependency audit | `go:audit` / `rust:audit` `allow_failure` (inherited) | Conditional |
| A07 Auth failures | Token hygiene | Bearer JWT required on REST routes | Pass |
| A08 Integrity | Deterministic rewards | Fixed merge SM table (+1/−1); no judge LLM | Pass |
| A09 Logging | Redact secrets | Authorization redacted at capture | Pass |
| A10 SSRF | Callback URL | Trainer callback URL stored but not fetched by orchestrator v1 | Pass |

**Overall:** All blocking controls **Pass**. A06 remains PoC-conditional.
