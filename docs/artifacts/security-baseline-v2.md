# Security Baseline v2 — CWSO (v0.4.1+ with Phase 5+ considerations)

> Owner: security-engineer · Status: accepted · Applies to: v0.4.1 GA and Phase 6+ planning
> **Supersedes:** `security-baseline-v1.md` (phases 0-4 only); includes Phase 5 HW-aware and Wasm threat analysis

## 1. Threat model (priority risks)

| ID | Threat | Mitigation | v0.4.1 | Phase 5+ |
|----|--------|-----------|--------|----------|
| T-1 | Prompt injection → malicious shell payloads | Firecracker sandbox + tier gate | ✅ | ✅ |
| T-2 | DNS rebinding on Streamable HTTP | Origin allow-list validation | ✅ | ✅ |
| T-3 | Sub-agent escalation / agent spawn | Permission tier in Router (deny Worker dispatch) | ✅ | ✅ |
| T-4 | Memory context overwrite / race | Event-sourced append-only broker per workspace | ✅ | ✅ |
| T-5 | Sub-agent writes outside shadow workspace | OverlayFS + sandbox FS namespace | ✅ | ✅ |
| T-6 | Secrets leakage (logs, commits) | Structured log scrubber; pre-commit secret scan | ✅ | ✅ |
| T-7 | Unauthorized session reuse | JWT (HS256 dev / RS256 prod) short TTL | ✅ | ✅ |
| T-8 | Resource exhaustion (runaway agents) | Server-side per-job timeout + cgroup caps | ✅ | ✅ |
| T-9 | **[Phase 5]** Wasm module tampering | SHA-256 digest pin + trusted-dir enforcement | N/A | ✅ |
| T-10 | **[Phase 5]** Wasm memory/time exhaustion | Runtime page limit + per-call timeout | N/A | ✅ |
| T-11 | **[Phase 5]** Policy engine escape via Wasm | Deny-by-default host call allowlist | N/A | ✅ |
| T-12 | **[Phase 5]** Hardware fingerprinting side-channel | Capability registry sanitization on export | N/A | ⏳ |
| T-13 | **[Phase 6+]** Trajectory poisoning (Polar) | Authenticated callback validation + integrity checks | N/A | ⏳ |

## 2. OWASP Top-10 mapping (v0.4.1 + Phase 5)

| OWASP | Control | v0.4.1 | Phase 5+ |
|-------|---------|--------|----------|
| A01 Broken Access Control | Tier-based permission gate; deny by default | ✅ | ✅ |
| A02 Cryptographic Failures | TLS-only HTTP; JWT signing via env/mount; no plaintext transport | ✅ | ✅ |
| A03 Injection | Sandbox exec uses arg arrays; input schema validation; Wasm host-call allowlist | ✅ | ✅ (Wasm) |
| A04 Insecure Design | Threat model documented; misuse cases per phase; Wasm plugin isolation | ✅ | ✅ (Wasm) |
| A05 Security Misconfiguration | Minimal images; no privileged containers; secure defaults via env | ✅ | ✅ |
| A06 Vulnerable Components | Renovate + govulncheck (Go) + cargo audit (Rust); Trivy image scan | ✅ | ✅ |
| A07 Auth Failures | JWT TTL ≤ 15 min; rate-limit on auth; session ID rotation | ✅ | ✅ |
| A08 Data Integrity Failures | Conflict matrix determinism; signed artifacts (Phase 4+); Wasm digest pin | ✅ | ✅ (Wasm) |
| A09 Logging & Monitoring | OTEL traces; auth failures at WARN; Wasm plugin calls logged | ✅ | ✅ (Wasm) |
| A10 SSRF | Allow-list on HTTP fetch; no internal-network access from sandboxes | ✅ | ✅ |

## 3. Immutable constraints (cannot be overridden at any phase)

1. **No secrets in source control** — not in v0.4.1, not in Phase 5+, not ever.
2. **No real PII in test data** — all test/demo data synthetic.
3. **No bypass of Origin validation, JWT, or tier gate** — even in dev mode.
4. **No untrusted code outside Firecracker** — Phase 4+ only exception is trusted Docker containers.
5. **No `--privileged` Docker except Firecracker host runner** — never for general orchestrator.
6. **No external network from worker sandboxes by default** — require explicit policy flag.
7. **Wasm plugins cannot mutate application state** — scoring plugin only; read-only access to policy context.
8. **Wasm module integrity verified before load** — SHA-256 digest match mandatory.

## 4. Phase 5 Hardware-Aware Security Additions

### 4.1 Policy Engine v2 (HW-aware dispatch)
- **Threat**: Policy engine bug → mis-assigned jobs to incompatible hardware
- **Mitigation**: Policy engine unit tests; deterministic scoring; audit trail of shard assignments
- **Telemetry**: Policy decision logs include hardware hint → shard decision for debugging

### 4.2 Wasm Scoring Plugin
- **Module Loading**:
  - Deny-by-default plugin model
  - SHA-256 digest pin required and validated before load
  - Trusted directory enforcement (plugin path must be inside allowlist)
  - Load failure → fallback to built-in scoring (fail-open)
- **Runtime Isolation**:
  - Explicit memory page limits (e.g., 1024 pages ≈ 64 MB)
  - Per-call timeout (e.g., 10 ms)
  - Deny-by-default host-call allowlist; unknown calls rejected
- **Scoring Interface**:
  - Single export: `adjust_score(provider_hash i64, current_score_milli i64) → i64`
  - Return values clamped to `[0, 1000]` (milli-points) by orchestrator
  - No read access to workspace state, auth context, or secrets
- **Deployment**:
  - Env-controlled feature flag: `CWSO_HHD_WASM_SCORING_ENABLED` (default `false`)
  - Operator must explicitly enable and configure digest pin
  - Plugin module verified on every orchestrator start

### 4.3 Hardware Capability Registry
- **Data Sensitivity**: Capability registry includes CPU model, GPU type, memory size, network affinity
- **Mitigation**: Capability export sanitized (no hypervisor version, no raw thermal data)
- **Audit Trail**: Which capabilities were used in dispatch decision logged for compliance

### 4.4 Telemetry Redaction (Phase 5 hardening)
- **Opt-in**: `CWSO_HHD_TELEMETRY_REDACTION_ENABLED`
- **Redaction**: Request ID obfuscation (HMAC-SHA256 with `CWSO_HHD_TELEMETRY_REDACTION_SALT`), anomaly notes anonymization
- **Purpose**: GDPR/compliance data minimization

## 5. Required CI checks

### Go security
- `gosec` (Go static analyzer)
- `govulncheck` (Go vulnerability scanner)

### Rust security
- `cargo audit` (Rust dependency audit)
- `cargo deny` (Rust policy enforcement)

### Container security
- `trivy` image scan on every container build

### Code leakage
- `gitleaks` / `trufflehog` secret scan on every commit

### Supply chain
- License-allowlist enforcement (MIT / Apache-2 / BSD only)
- Signed commits encouraged (GPG verification in release CI)

### Phase 5+ additions
- Wasm module digest validation in release CI (if Wasm plugin shipped)
- Hardware capability registry validation (if HW-aware dispatch enabled)

## 6. Security review gates (by phase)

| Phase | Gate | Reviewer | Criteria |
|-------|------|----------|----------|
| Phase 4 (v0.4.1) | Security Phase 7 Gate | security-engineer | OWASP A01-A10 audit, zero CRITICAL, zero HIGH |
| Phase 5 (planned) | Wasm Plugin Review | security-engineer + backend-developer | Digest pin tested, host-call allowlist validated, resource limits verified |
| Phase 6+ (planned) | HW-aware Policy Review | security-engineer + architect | Policy engine escape testing, capability registry leak analysis |
| Phase 9 (planned) | Polar Callback Security | security-engineer | Trajectory integrity checks, callback authentication, reward injection testing |

## 7. Deployment checklist

- [ ] All secrets stored outside repo (use GitHub Secrets / GitLab CI vars / vaults only)
- [ ] TLS enabled on Streamable HTTP (production)
- [ ] JWT secret stored in secure mount / env (not in Dockerfile)
- [ ] Origin allow-list configured correctly
- [ ] Rate limiting enabled on auth endpoints
- [ ] Pre-commit secret scan hooks installed locally
- [ ] Trivy container scan passing
- [ ] OWASP checklist completed and documented
- [ ] Incident response contacts defined
