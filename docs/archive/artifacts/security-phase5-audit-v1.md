# Security Phase 5 Audit v1

Date: 2026-05-23  
Owner: security-engineer  
Task: T071  
Based on: docs/tasks/task-T071.md, docs/tasks/task-T070.md, docs/artifacts/qa-phase5-report-v1.md, docs/artifacts/hypothesis-T067-results-v1.md, docs/artifacts/hypothesis-T068-results-v1.md, docs/artifacts/hypothesis-T069-results-v1.md, docs/artifacts/wasm-scoring-runtime-ops-v1.md

## Scope

In-scope code surfaces:
- `orchestrator/internal/dispatch/*` for policy v2, sparse/quantized assist, SSM assist, telemetry emitter/monitor, and Wasm scoring runtime.
- `orchestrator/internal/config/config.go` and `orchestrator/internal/config/config_test.go` for feature gating and guardrail validation.
- `orchestrator/internal/server/server.go` for runtime wiring and fail-safe behavior.

In-scope artifact evidence:
- `docs/artifacts/hypothesis-T067-results-v1.md`
- `docs/artifacts/hypothesis-T068-results-v1.md`
- `docs/artifacts/hypothesis-T069-results-v1.md`
- `docs/artifacts/qa-phase5-report-v1.md`

Out of scope:
- Non-Phase-5 tool surfaces and previously accepted findings from earlier security gates.

## Methodology

1. Static code review focused on trust boundaries, input validation, privilege checks, and fallback behavior.
2. Configuration hardening review for all new environment-controlled surfaces.
3. Test-evidence correlation against unit/integration tests in dispatch/config/server packages.
4. OWASP Top 10 mapping for relevant attack classes in this phase.

### OWASP Top 10 relevance mapping

| OWASP | Relevance in Phase 5 | Audit focus |
|---|---|---|
| A01 Broken Access Control | Medium | Verify privileged paths (eBPF) are optional and capability-gated. |
| A03 Injection | Medium | Validate external labels and feature flags are parsed safely and fail to baseline on invalid inputs. |
| A04 Insecure Design | High | Confirm default-off feature flags and deterministic baseline fallback behavior. |
| A05 Security Misconfiguration | High | Validate environment guardrails for policy/telemetry/Wasm settings and ranges. |
| A08 Software and Data Integrity Failures | High | Assess integrity controls for loading operator-supplied Wasm modules. |
| A09 Security Logging and Monitoring Failures | Medium | Validate anomaly and decision telemetry coverage and fallback observability. |

## Findings

| ID | Severity | Status | Area | Evidence | Finding | Remediation notes |
|---|---|---|---|---|---|---|
| F-071-01 | MEDIUM | OPEN | Wasm runtime integrity (A08) | `orchestrator/internal/dispatch/wasm_scoring_plugin.go`, `orchestrator/internal/server/server.go` | Wasm module loading is path-based only; there is no signature/hash verification for module provenance before compile/instantiate. | Add a mandatory module-integrity control (SHA-256 pin or signature verification) before `CompileModule`. Restrict load path to trusted directory and enforce read-only mount for production modules. |
| F-071-02 | LOW | OPEN | Telemetry data minimization (A09) | `orchestrator/internal/dispatch/telemetry.go`, `orchestrator/internal/dispatch/anomaly_monitor.go` | Decision/anomaly telemetry can carry request metadata and environment-derived notes. Current implementation does not enforce redaction/minimization policy for these fields. | Define and enforce a telemetry field classification/redaction policy (for example: hash or drop request identifiers at publish time when privacy mode is enabled). |
| F-071-03 | LOW | OPEN | eBPF signal trust semantics (A04/A09) | `orchestrator/internal/dispatch/anomaly_monitor.go` | `ebpf-hook` detection latency is currently estimated (constant value) rather than measured from probe events, which may overstate confidence in anomaly timings. | Keep eBPF path default-off and document this as non-authoritative telemetry until real probe timestamps are integrated and validated. |

No CRITICAL or HIGH findings were identified in this audit scope.

## Explicit validation: Wasm host-call allowlist controls

Validated controls:
1. Deny-by-default host-call model is implemented. When the allowlist is empty, no host module functions are instantiated.
2. Unknown host-call names are rejected explicitly during runtime setup, preventing accidental privilege expansion.
3. Current recognized host call set is intentionally minimal (`time.now_unix_ms` only).
4. Runtime constraints exist for Wasm execution (`CallTimeout`, memory page limit, score clamp range), and plugin is feature-gated by default.

Evidence:
- Host-call allowlist enforcement and unknown-call rejection in `instantiateHostAllowlist(...)` within `orchestrator/internal/dispatch/wasm_scoring_plugin.go`.
- Unknown host-call rejection test: `TestNewWasmScoreAdjusterRejectsUnknownHostCall` in `orchestrator/internal/dispatch/wasm_scoring_plugin_test.go`.
- Config guards for Wasm enablement and runtime bounds in `orchestrator/internal/config/config.go` with coverage in `orchestrator/internal/config/config_test.go`.
- Server wiring keeps plugin optional and falls back to baseline scoring on plugin initialization failure in `orchestrator/internal/server/server.go`.

Conclusion on allowlist control: PASS for host-call exposure minimization and explicit allowlist enforcement.

## Gate verdict

Verdict: CONDITIONAL_PASS

Rationale:
- Core Phase 5 controls are present: feature-gated defaults, safe baseline fallback paths, capability-gated optional privileged monitoring path, and Wasm host-call allowlist enforcement.
- Open MEDIUM/LOW items do not require emergency code changes today, but they must be tracked as hardening conditions before broader production enablement of Wasm-assisted scoring and expanded telemetry use.

## Tracked conditions (required follow-up)

1. Add Wasm module integrity verification and trusted-path enforcement.
   - Proposed task: T073
   - Severity driver: F-071-01 (MEDIUM)
   - Exit criteria: plugin load fails closed on integrity mismatch; operator doc includes key rotation/hash update flow.

2. Add telemetry minimization/redaction controls for decision/anomaly events.
   - Proposed task: T074
   - Severity driver: F-071-02 (LOW)
   - Exit criteria: configurable redaction policy with tests proving sensitive fields are transformed or dropped.

3. Convert eBPF anomaly latency from estimated to measured semantics, or explicitly mark output as advisory in downstream consumers.
   - Proposed task: T075
   - Severity driver: F-071-03 (LOW)
   - Exit criteria: either measured probe timestamps with tests, or schema-level advisory flag enforced and documented.

## Blocker status

- Active blocker: None
- Block condition check: No unresolved CRITICAL/HIGH findings, so progression is not blocked.
