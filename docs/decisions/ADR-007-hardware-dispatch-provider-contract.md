# ADR-007: Hardware dispatch provider contract and deterministic fallback

- Status: accepted
- Date: 2026-05-22
- Decision-maker: solution-architect
- Supersedes: none
- Related: [docs/artifacts/requirements-phase5-hardware-v1.md](../artifacts/requirements-phase5-hardware-v1.md), [docs/artifacts/architecture-phase5-hhd-v1.md](../artifacts/architecture-phase5-hhd-v1.md), [docs/artifacts/security-baseline-v1.md](../artifacts/security-baseline-v1.md)

## Context
Phase 5 requires policy-based heterogeneous dispatch with explicit fallback and no vendor lock-in. Current architecture supports adapter boundaries but lacks a versioned provider contract and deterministic selection/fallback semantics.

Without a standard contract:
- Adapter implementations drift in capability representation.
- Fallback behavior becomes implementation-specific and non-replayable.
- Security and observability guarantees become inconsistent across providers.

## Decision
Adopt a versioned adapter-first provider contract (`dispatch.provider/v1`) and deterministic routing policy with mandatory CPU terminal fallback.

Decision elements:
1. Providers MUST expose a normalized capability schema including health, queue depth, latency/cost class, workload tags, and contract version.
2. Orchestrator MUST rank providers using a deterministic stable sort and explicit tie-breaker.
3. Fallback MUST follow ordered ranked chain and end at CPU baseline unless request is invalid or policy-denied.
4. Dispatch outcomes MUST emit auditable decision telemetry with policy version, capability epoch, selected provider, fallback chain, and reason code.
5. Experimental routes (quantized/sparse/SSM) MUST remain feature-flagged off by default and quality-guarded.
6. Contract evolution MUST be backward compatible for additive minor versions; breaking changes require major version increment.

## Alternatives considered
### A1. Vendor-native contracts per provider
- Pros: fast initial integration for first hardware provider.
- Cons: high lock-in, fragmented fallback logic, weak cross-provider replayability.
- Rejected because it violates adapter-first portability and creates long-term integration debt.

### A2. Best-effort dynamic routing without deterministic ordering
- Pros: simpler implementation, potentially adaptive under burst conditions.
- Cons: non-reproducible outcomes, difficult incident analysis, unstable behavior under ties.
- Rejected because it conflicts with deterministic fallback and traceability requirements.

### A3. CPU-only baseline plus manual operator overrides
- Pros: minimal architecture change.
- Cons: no automation for latency/cost gains, operationally heavy, weak resilience objectives.
- Rejected because it fails Phase 5 functional and non-functional goals.

## Consequences
### Positive
- Preserves portability with adapter-first model and avoids vendor lock-in.
- Establishes deterministic and auditable dispatch/fallback behavior.
- Enables phased migration: compatibility mode first, live routing later.
- Supports security and QA gateability through normalized events and failure taxonomy.

### Negative
- Requires conformance tests for all provider adapters.
- Adds policy/version management surface in orchestrator.
- Introduces schema governance responsibilities for future contract revisions.

### Neutral/operational
- Existing callers continue CPU baseline behavior unless policy context is provided.
- Additional observability volume is expected from decision events and fallback traces.

## Guardrails and implementation notes
- Authorization and permission gates remain in orchestrator router; adapters cannot elevate permissions.
- Provider capability input validation is mandatory server-side.
- Secrets are out-of-band from contract payloads and must use environment or secret mounts.
- CPU baseline remains mandatory safe fallback path.

## Validation impact
This decision directly enables T064 and T065:
- T064 consumes capability schema and event envelope shape.
- T065 consumes deterministic ranking, error taxonomy, timeout/retry/fallback semantics.

Gate checks expected:
- Determinism replay test suite for identical input snapshots.
- Fallback latency and reliability parity checks against baseline.
- Security gate confirmation against [docs/artifacts/security-baseline-v1.md](../artifacts/security-baseline-v1.md).
