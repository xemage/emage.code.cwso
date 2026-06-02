# Active Tasks

Next-Gen execution (Phase 6+). Source roadmap: `docs/plans/plan-cwso-nextgen-phase6plus.md`.
Design baseline: `docs/artifacts/cwso-nextgen-blueprint-v1.md`.

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T080 | Phase 6 requirements + hardware benchmark targets | product-owner | done | P0 | — | 2026-06-02 |
| T081 | HAL design: `InferenceBackend` trait + plugin loading + `dispatch.provider/v2` | solution-architect | done | P0 | T080 | 2026-06-02 |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | in_review | P0 | T081 | 2026-06-02 |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | pending | P0 | T082 | 2026-06-02 |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | pending | P1 | T082 | 2026-06-02 |
| T085 | Profiling layer: tensor_tag derivation + workload mapping | backend-developer | in_review | P0 | T082 | 2026-06-02 |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | in_review | P0 | T083, T085 | 2026-06-02 |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | in_progress | P0 | T086 | 2026-06-02 |
| T088 | Phase 6 integration + reliability QA (fallback ≤ 2.0s, overhead ≤ 10ms) | qa-engineer | in_progress | P0 | T087 | 2026-06-02 |
| T089 | Phase 6 Tech-Lead + Security gate | tech-lead / security-engineer | pending | P0 | T088 | 2026-06-02 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

## Phase 6 execution notes (2026-06-02)

The Go control-plane half of Feature A (Heterogeneous Hardware Dispatcher) landed on
`feature/T086-hardware-aware-dispatch`:

- **T085 (done, in review):** deterministic workload profiler `dispatch.ProfileTask` →
  `WorkloadProfile` (tensor tags + recommended hardware class + request labels).
- **T086 (done, in review):** `dispatch_hardware_aware_job` MCP tool + `schemas/dispatch_hardware_aware_job.json`,
  orchestrator-only, fire-and-forget (returns `job_id` + `assigned_hardware_profile`).
- **T087 (in progress / shadow mode):** the tool routes through the existing deterministic
  `PolicyEngineV2` against a shadow provider catalog (`lpu-realtime`, `gpu-accelerated`,
  `ssm-longctx`) seeded behind `CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED`. Job bodies are
  context-respecting no-ops until live HAL adapters land. **Blocked on T082 (Rust `cwso-hal`).**
- **T088 (in progress):** Go unit tests + `go vet` + `-race` pass for all packages; reliability
  benchmarks (fallback latency, dispatch overhead) still pending live adapters.

### T082 (done, in review) — Rust HAL crate

The Hardware Abstraction Layer crate `services/cwso-hal` landed on
`feature/T082-cwso-hal`:

- `InferenceBackend` trait (`capabilities` / `health` / `infer`) + domain types
  (`ProviderCapability`, `Health`, `InferenceRequest`, `Completion`, `FailureClass`,
  `BackendFailure`) under contract `dispatch.provider/v2`.
- `CpuBaselineBackend`: always-healthy, deterministic, dependency-free terminal-safe adapter.
- `BackendRegistry::dispatch`: deterministic selected → fallback-chain → cpu-baseline walk
  (mirrors the Go `FallbackOnFailure`); non-retryable failures stop the walk instead of
  masking with the baseline.
- UDS IPC server (length-prefixed JSON frames + SO_PEERCRED authz, mirroring
  `cwso-merge-engine`) exposing `stat` / `capabilities` / `health` / `infer`.
- CI: `cargo test --release -p cwso-hal` added to the `rust:test` job (21 tests, fmt clean).

Unblocks **T083/T084** (GPU/LPU adapters register alongside the baseline) and the live half
of **T087**. Per-task briefs live alongside this file as `task-T082.md`, `task-T085.md`,
`task-T086.md`, `task-T087.md`.
