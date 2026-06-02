# Artifact: release-v0.2.0-hardware-aware-v1

## Metadata
- Producer agent: technical-writer
- Created: 2026-05-23
- Based on: docs/tasks/task-T072.md, docs/artifacts/requirements-phase5-hardware-v1.md, docs/artifacts/architecture-phase5-hhd-v1.md, docs/artifacts/qa-phase5-report-v1.md, docs/artifacts/security-phase5-audit-v1.md

## Release intent
This artifact documents release readiness for the Phase 5 hardware-aware
feature set. The target posture is controlled rollout with explicit feature
flags, deterministic baseline fallback, and known hardening follow-ups tracked.

## Scope included

### Delivered capabilities
- Capability registry and dispatch telemetry fabric.
- Policy engine v2 for deterministic provider selection and fallback chain
  behavior.
- Experimental sparse/quantized assist scoring path (default off).
- Experimental SSM sequence-assist scoring path (default off).
- Optional Wasm scoring micro-agent runtime (default off).
- Event-driven monitoring with eBPF-preferred path and userspace fallback.

### Operator documentation updates
- README updates for hardware-aware feature flags.
- Compatibility notes and rollback instructions.
- Linkage to Wasm runtime operations guide.

## Configuration and install summary
No additional binary install steps beyond standard CWSO setup are required.
Phase 5 capabilities are controlled through environment flags and remain
disabled by default until explicitly enabled.

Primary enablement controls:
- `CWSO_HHD_CAPABILITY_REGISTRY_ENABLED`
- `CWSO_HHD_DECISION_TELEMETRY_ENABLED`
- `CWSO_HHD_POLICY_ENGINE_V2_ENABLED`

Experimental path controls:
- `CWSO_HHD_SPARSE_QUANTIZED_ASSIST_ENABLED`
- `CWSO_HHD_SSM_ASSIST_ENABLED`
- `CWSO_HHD_WASM_SCORING_ENABLED`
- `CWSO_HHD_EVENT_MONITOR_ENABLED`
- `CWSO_HHD_EVENT_MONITOR_EBPF_ENABLED`

## Rollback and fallback guidance

### Immediate rollback
Disable all `CWSO_HHD_*` feature flags and restart the orchestrator to return
to baseline dispatch behavior.

### Runtime fallback guarantees
- Policy v2 low-confidence decisions route to `cpu-baseline`.
- Sparse/quantized quality guardrail breaches auto-disable that assist path.
- Invalid/out-of-threshold SSM sequence signals route to baseline.
- Wasm plugin errors fall back to built-in policy scoring.
- eBPF monitor unavailability falls back to userspace anomaly monitoring.

## Known limitations and non-goals

### Limitations
- Wasm module integrity verification is not yet enforced (tracked hardening).
- Telemetry minimization/redaction policy is not yet enforced (tracked
  hardening).
- eBPF anomaly latency is currently advisory/estimated in the spike path.

### Non-goals for this release artifact
- Declaring all experimental assist paths production-default.
- Enabling eBPF monitoring unconditionally across all environments.
- Reclassifying open medium/low hardening items as closed.

## QA and security gate references
- QA verdict: PASS
  - Source: docs/artifacts/qa-phase5-report-v1.md
- Security verdict: CONDITIONAL_PASS (no CRITICAL/HIGH findings)
  - Source: docs/artifacts/security-phase5-audit-v1.md
  - Follow-up hardening tracked as T073-T075 in security audit output.

## Release readiness verdict
CONDITIONAL_PASS for Phase 5 feature publication.

Rationale:
- Functional and reliability gates are satisfied.
- Security gate allows progression with explicitly tracked medium/low hardening
  conditions.
- Operator-facing rollback and fallback guidance is now documented.