# Security Baseline v1 — CWSO

> Owner: security-engineer · Status: accepted · Applies to: all phases

## 1. Threat model (top risks)
| ID | Threat | Mitigation |
|----|--------|-----------|
| T-1 | Prompt-injection driving LLM to execute malicious shell payloads | Untrusted code only in Firecracker microVM; tier router enforced server-side |
| T-2 | DNS rebinding against local Streamable HTTP | Mandatory `Origin` allow-list validation on every request |
| T-3 | Sub-agent escalates and dispatches more agents (capability cascade) | Permission tier enforced in Router; Worker role lacks `dispatch_*` and `merge_*` tools |
| T-4 | Race condition / context overwrite in shared memory | Event-sourced append-only memory broker; per-workspace pointer index |
| T-5 | Sub-agent writes outside its shadow workspace | OverlayFS mount + sandbox FS namespace; no writable host bind mounts |
| T-6 | Secrets leakage in logs / commits | Structured log scrubber; pre-commit secret scanner in CI; no env dump in errors |
| T-7 | Unauthorized session use | JWT (HS256 dev / RS256 prod) with short TTL; required on every HTTP request |
| T-8 | Resource exhaustion via runaway sub-agents | Per-job timeout enforced server-side with SIGKILL; cgroup CPU/mem caps per sandbox |

## 2. OWASP Top-10 mapping
| OWASP | Control |
|-------|---------|
| A01 Broken Access Control | Tier-based permission gate in Router; deny by default |
| A02 Cryptographic Failures | TLS-only HTTP in Phase 3+; JWT signing keys via env/secret mount |
| A03 Injection | All sandboxed shell exec uses arg arrays, never `sh -c`; tool inputs schema-validated |
| A04 Insecure Design | Threat model documented; misuse cases tracked per phase |
| A05 Security Misconfiguration | Minimal Docker images; no privileged containers except Firecracker (KVM-only) |
| A06 Vulnerable Components | Renovate / `govulncheck` / `cargo audit` in CI |
| A07 Auth Failures | JWT TTL ≤ 15 min; rate-limit on auth endpoints |
| A08 Data Integrity Failures | All artifacts signed; `cosign` verification in release pipeline (Phase 4) |
| A09 Logging & Monitoring | OTEL traces; auth + tier-violation events logged at WARN |
| A10 SSRF | HTTP fetcher (if any) uses allow-list; no internal-network fetches by default |

## 3. Immutable constraints (cannot be overridden)
1. No secrets in source control.
2. No real PII in test data.
3. No bypass of Origin validation, JWT, or tier gate — even in dev.
4. No untrusted code outside Firecracker.
5. No `--privileged` Docker except the Firecracker host runner.
6. No external network from worker sandboxes by default.

## 4. Required CI checks
- `gosec`, `govulncheck` (Go)
- `cargo audit`, `cargo deny` (Rust)
- `trivy` image scan on every container build
- `gitleaks` / `trufflehog` secret scan on every commit
- License-allowlist enforcement (MIT/Apache-2/BSD only)
