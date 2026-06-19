# Requirements Phase 5 Hardware-Aware v1 — CWSO

> Owner: product-owner · Based on: `docs/artifacts/requirements-v1.md`, `docs/artifacts/architecture-v1.md`, `docs/plans/plan-T062-phase5-hardware-aware-roadmap.md` · Status: accepted

## 1. Problem statement and user value
CWSO currently routes orchestration workloads through a single default execution path. This limits performance and cost efficiency for workloads with different hardware and runtime profiles. Phase 5 defines hardware-aware requirements so CWSO can select eligible backends (CPU baseline, GPU-capable providers, and research adapters) through policy while preserving deterministic behavior, safe fallback, and security guarantees.

User value:
- Orchestrator operators get lower latency and lower cost on eligible workloads without rewriting tool contracts.
- Platform engineers get explicit fallback and observability requirements that keep reliability and incident response stable.
- Security and QA stakeholders get measurable gates for release go/no-go decisions.

## 2. In scope / out of scope
### 2.1 In scope
- Hardware-aware dispatch requirements for policy-based backend selection.
- Capability discovery and telemetry requirements used by routing decisions.
- Wasm micro-agent integration requirements for faster policy/heuristic iteration.
- Benchmark definitions for baseline CPU and candidate hardware-aware backends.
- Hypothesis validation criteria for H1-H4 from the approved Phase 5 plan.

### 2.2 Out of scope
- Mandatory dependency on vendor-specific silicon.
- Changes to core merge semantics or role/tier authorization boundaries.
- Production commitment to sparse/quantized/SSM pathways before validation gates complete.

## 3. Functional requirements
### FR-001 Hardware-aware dispatch contract
The system SHALL expose a versioned dispatch contract that accepts a workload profile and policy context and returns one selected backend plus ranked fallback options.

### FR-002 Deterministic fallback
If the selected backend is unavailable, degraded, or times out, the system SHALL route to CPU baseline automatically and record a structured fallback reason.

### FR-003 Capability discovery pipeline
The system SHALL collect backend capabilities (latency class, cost class, queue depth, health state, supported workload tags) at a configurable interval and make the snapshot available to the dispatch policy engine.

### FR-004 Policy engine inputs
The dispatch policy SHALL evaluate at least these inputs: workload class, p95 latency budget, reliability class, cost weight, backend health, and operator allow/deny lists.

### FR-005 Feature-flagged experimental routes
Sparse/quantized and SSM assistive routes SHALL be disabled by default and enabled only via explicit feature flags.

### FR-006 Wasm micro-agent extension path
The system SHALL allow approved Wasm modules to provide dispatch scoring plugins and merge-assist heuristics through a constrained host-call interface.

### FR-007 Security invariants
Hardware-aware dispatch SHALL preserve existing security baseline controls, including server-side authorization, no secrets in source, and no untrusted execution outside mandated sandbox boundaries.

### FR-008 Traceable routing decisions
Each dispatch decision SHALL emit an auditable event including policy version, chosen backend, fallback chain, latency estimate, and confidence score.

## 4. Non-functional requirements
| ID | Target |
|----|--------|
| NFR-001 Latency gain | Eligible inference-heavy flows achieve p95 latency improvement >= 30% versus CPU baseline over identical test windows. |
| NFR-002 Fallback speed | Automatic fallback to CPU baseline completes in <= 2.0 seconds from failure detection. |
| NFR-003 Reliability parity | Error rate increase for hardware-aware path is <= 0.2 percentage points versus baseline and introduces no sev-1/sev-2 incidents in staging. |
| NFR-004 Dispatch overhead | Additional policy and capability-evaluation overhead is <= 10 ms p95 per dispatch decision. |
| NFR-005 Wasm safety envelope | Wasm plugin execution p95 <= 5 ms per invocation, memory cap <= 64 MiB/module, and host-call allowlist enforced for 100% of modules. |
| NFR-006 Observability timeliness | Event-driven anomaly signal detection median time <= 50% of current polling/log-only baseline on benchmark scenarios. |
| NFR-007 Cost efficiency | Targeted sparse/quantized or SSM-assisted workloads reduce cost-per-decision by >= 25% while meeting quality thresholds. |
| NFR-008 Quality guardrail | Merge-assist quality remains >= 98% acceptance on curated reference set, with automatic route disable on threshold breach. |

## 5. Benchmark definitions and matrix
## 5.1 Benchmark workloads (reproducible)
All benchmarks use synthetic, non-PII datasets and deterministic seeds.

| Workload ID | Purpose | Input definition | Expected output |
|-------------|---------|------------------|-----------------|
| W1 Dispatch-Latency | Measure end-to-end dispatch and execution latency under mixed traffic | 10,000 orchestration requests over 15 minutes; 60% standard tool flows, 40% inference-heavy tagged flows; fixed seed `cwso-phase5-w1` | Per-request routing record, p50/p95/p99 latency, fallback count, error rate |
| W2 Fallback-Resilience | Validate deterministic fallback during backend impairment | 2,000 inference-heavy requests with scheduled backend faults at minutes 3, 6, and 9; fixed seed `cwso-phase5-w2` | Fallback activation timeline, detection-to-fallback latency, success continuation rate |
| W3 Wasm-Iteration | Compare extension lead time and runtime overhead | Implement one scoring heuristic and one merge-assist rule via core rebuild path and Wasm plugin path using same behavior tests | Lead-time delta, runtime invocation latency, pass/fail parity on behavior tests |
| W4 Cost-Quality | Evaluate sparse/quantized and SSM assistive routes | 5,000 targeted requests with candidate routes enabled individually behind feature flags; fixed seed `cwso-phase5-w4` | Cost-per-decision comparison, quality acceptance score, auto-disable events |

## 5.2 Benchmark matrix
| Scenario | Backend class | Role in evaluation | Required workloads | Primary metrics | Pass threshold |
|----------|---------------|--------------------|--------------------|-----------------|----------------|
| B0 | CPU baseline (current path) | Control baseline for all hypotheses | W1, W2, W3, W4 | p95 latency, error rate, cost-per-decision, detection time | Baseline reference captured and reproducible |
| B1 | GPU-capable provider adapter | Candidate for latency reduction on eligible flows | W1, W2 | p95 latency delta, fallback speed, reliability delta | Meets NFR-001, NFR-002, NFR-003 |
| B2 | Quantized/sparse assist route (feature-flagged) | Candidate for cost reduction on targeted tasks | W4 | Cost-per-decision delta, quality acceptance | Meets NFR-007 and NFR-008 |
| B3 | SSM assist route (feature-flagged) | Candidate for sequence-heavy assist optimization | W4 | Cost-per-decision delta, quality acceptance, throughput | Meets NFR-007 and NFR-008 |
| B4 | Event-driven telemetry path (eBPF where allowed, fallback hooks otherwise) | Candidate for faster anomaly detection | W1, W2 | Detection time delta, telemetry overhead | Meets NFR-006 with <= 5% CPU overhead in staging |

## 6. Hypothesis success and failure criteria (H1-H4)
### H1 Policy-based heterogeneous dispatch
- Success criteria:
  - p95 latency improvement >= 30% on eligible flows vs B0.
  - Reliability delta <= 0.2 percentage points error increase.
  - Fallback to B0 within <= 2.0 seconds.
- Failure criteria:
  - Latency improvement < 20%, or
  - reliability delta > 0.2 percentage points, or
  - fallback time > 2.0 seconds in >= 5% of fault injections.

### H2 Wasm micro-agent integration speed
- Success criteria:
  - Lead time for one representative dispatch/merge heuristic change is >= 40% faster vs core binary rebuild workflow.
  - Runtime parity tests pass with no correctness regressions.
- Failure criteria:
  - Lead-time reduction < 25%, or
  - measurable correctness regressions in parity tests.

### H3 Event-driven telemetry improvement
- Success criteria:
  - Median anomaly detection time is >= 50% faster than polling/log-only baseline.
  - Additional host CPU overhead <= 5% in staging.
- Failure criteria:
  - Detection improvement < 30%, or
  - overhead > 8% sustained during W1/W2.

### H4 Sparse/quantized and SSM cost efficiency
- Success criteria:
  - Cost-per-decision reduction >= 25% on targeted workloads.
  - Quality acceptance remains >= 98% on curated reference outputs.
- Failure criteria:
  - Cost reduction < 15%, or
  - quality acceptance < 98%, or
  - repeated auto-disable due to quality guardrail breaches.

## 7. Risks, assumptions, and open questions
### 7.1 Risks
- Provider-reported capability metadata may not match real-time behavior under burst load.
- Wasm extension path can add policy drift if module versioning and signing are weak.
- eBPF access may be restricted in some environments, limiting observability gains.
- Quantized/sparse outputs may reduce merge-assist quality on complex code diffs.

### 7.2 Assumptions
- Existing Go orchestrator and Rust sidecar split remains the execution model.
- Backend adapters expose enough health/queue telemetry for policy decisions.
- Benchmark datasets remain synthetic and reproducible across staging runs.
- Security baseline controls remain mandatory for all new dispatch paths.

### 7.3 Open questions
1. What is the minimum capability schema version required for third-party provider adapters in T063?
2. Should cost-per-decision be normalized by token count, wall-clock, or both for go/no-go?
3. What operator override policy is allowed during sustained fallback events?
4. Which quality reference set owner approves threshold updates after initial Phase 5 calibration?

## 8. Traceability
- Source requirements baseline: [requirements-v1.md](requirements-v1.md)
- Source architecture baseline: [architecture-v1.md](architecture-v1.md)
- Security constraints reference: [security-baseline-v1.md](security-baseline-v1.md)
- Phase 5 roadmap and hypotheses: [plan-T062-phase5-hardware-aware-roadmap.md](../plans/plan-T062-phase5-hardware-aware-roadmap.md)