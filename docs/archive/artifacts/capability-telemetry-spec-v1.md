# Capability + Telemetry Spec v1

Owner: backend-developer  
Based on: docs/artifacts/architecture-phase5-hhd-v1.md, docs/artifacts/requirements-phase5-hardware-v1.md  
Status: accepted (MVP)

## 1. Scope
This specification defines the MVP capability registry schema and dispatch telemetry signals required for policy-aware routing decisions in Phase 5.

MVP goals:
- Provide a deterministic capability snapshot for policy consumers.
- Emit auditable decision events with baseline envelope fields.
- Keep current dispatch behavior unchanged when feature flags are disabled.

## 2. Feature Flags
- CWSO_HHD_CAPABILITY_REGISTRY_ENABLED (default: false)
- CWSO_HHD_DECISION_TELEMETRY_ENABLED (default: false)
- CWSO_HHD_CAPABILITY_SNAPSHOT_TTL_SECONDS (default: 30, must be > 0)

Compatibility rule:
- When either flag is disabled, orchestrator behavior remains CPU-baseline compatible with existing dispatch flow.

## 3. Capability Registry Schema
Source type: orchestrator/internal/dispatch.ProviderCapability

Required fields:
- provider_id (string)
- contract_version (string)
- health_state (enum: healthy, degraded, unavailable)
- latency_class (string)
- cost_class (string)
- queue_depth (int >= 0)
- supported_workload_tags (string[])
- reliability_class (string)
- feature_flags (string[])
- last_updated_at (RFC3339 timestamp; auto-populated on upsert when omitted)

Derived registry view:
- epoch (monotonic uint64 incremented per upsert)
- captured_at (snapshot capture timestamp)
- providers (sorted by provider_id ascending for deterministic replay)

Staleness handling:
- Snapshot TTL is configurable via CWSO_HHD_CAPABILITY_SNAPSHOT_TTL_SECONDS.
- A healthy provider older than TTL is projected as degraded in policy-facing reads.

## 4. Telemetry Topics and Payloads
### 4.1 Capability snapshot topic
Topic: dispatch/capabilities

Payload fields:
- capability_epoch
- captured_at
- providers (ProviderCapability[])

Cadence:
- Emitted at startup when capability registry and decision telemetry are both enabled.
- Additional emission occurs when explicit registry snapshot publication is invoked by future refresh loops.

### 4.2 Dispatch decision topic
Topic: dispatch/decision

Payload fields:
- decision_id
- request_id (optional in MVP)
- policy_version
- capability_epoch
- selected_provider
- fallback_chain
- fallback_count
- reason_code
- confidence
- estimated_latency_ms
- actual_latency_ms
- feature_flags_applied
- quality_guardrail_state
- emitted_at

Cadence:
- One event per dispatch_concurrent_jobs invocation when decision telemetry is enabled.
- Event emission is best-effort and does not fail tool execution.

## 5. Replay and Debug Notes
Deterministic replay guidance:
- Reconstruct routing context from tuple: (policy_version, capability_epoch, selected_provider, fallback_chain, reason_code).
- Use capability snapshot records with matching capability_epoch from dispatch/capabilities.
- Sort providers by provider_id to ensure stable ordering across runs.

Debug workflow:
1. Query memory broker records for dispatch/decision by time window.
2. Extract capability_epoch from each decision.
3. Query matching dispatch/capabilities snapshot records.
4. Compare projected provider health_state and queue_depth to decision reason_code.

Known MVP limitations:
- request_id is optional and may be empty until policy-engine integration (T065).
- selected_provider defaults to cpu-baseline in current dispatch tool integration.
- periodic capability polling loop is deferred; current implementation supports registry-driven snapshot publication.

## 6. Security and Safety
- No secrets are included in capability or decision payloads.
- Payloads include only dispatch metadata and policy-facing operational counters.
- Feature flags default to disabled to preserve existing behavior.
