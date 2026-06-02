# Active Tasks

Next-Gen execution (Phase 6+). Source roadmap: `docs/plans/plan-cwso-nextgen-phase6plus.md`.
Design baseline: `docs/artifacts/cwso-nextgen-blueprint-v1.md`.

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T080 | Phase 6 requirements + hardware benchmark targets | product-owner | done | P0 | — | 2026-06-02 |
| T081 | HAL design: `InferenceBackend` trait + plugin loading + `dispatch.provider/v2` | solution-architect | done | P0 | T080 | 2026-06-02 |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | in_review | P0 | T081 | 2026-06-02 |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | in_review | P0 | T082 | 2026-06-02 |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | in_review | P1 | T082 | 2026-06-02 |
| T085 | Profiling layer: tensor_tag derivation + workload mapping | backend-developer | in_review | P0 | T082 | 2026-06-02 |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | in_review | P0 | T083, T085 | 2026-06-02 |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | in_review | P0 | T086 | 2026-06-02 |
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
- **T087 (done, in review):** live HAL execution wired. New Go HAL client
  `orchestrator/internal/hal` (framed-JSON UDS, typed `Infer`). `dispatch_hardware_aware_job`
  now executes the dispatched job against the live HAL when `CWSO_HAL_SOCKET` is set —
  calling `Infer` on the selected provider and forwarding `RankedFallbackChain` so the HAL
  falls back deterministically to `cpu-baseline`; without a socket it preserves the
  shadow-mode no-op. Constructor `NewDispatchHardwareAwareJobWithHAL` + server wiring select
  live vs. shadow. Selection/fallback/telemetry still driven entirely by `PolicyEngineV2`.
  Live capability heartbeat sync is deferred to T089.
- **T088 (in progress):** Go unit tests (incl. `internal/hal` round-trip + tool live-exec)
  + `go vet` + gofmt pass for all packages; reliability benchmarks (fallback latency,
  dispatch overhead) still pending.

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
of **T087**.

### T083 / T084 (done, in review) — GPU + LPU adapters

Landed on `feature/T083-T084-gpu-lpu-adapters`:

- `http.rs`: `HttpTransport` trait + blocking `UreqTransport` (production) + a mock for
  offline unit tests; transport errors normalized to `Timeout` / `Unreachable` / `Other`.
- `openai.rs`: `OpenAiCompatibleBackend` over `/chat/completions`, with `gpu_vllm_config`
  (**T083**, provider `gpu-accelerated`, latency `fast`, tags `inference-heavy` +
  `deterministic-edit`) and `lpu_groq_config` (**T084**, provider `lpu-realtime`, latency
  `ultra`, tag `realtime`) presets. HTTP/transport failures map to `FailureClass`
  (429→overloaded, 400/422→invalid_request, 5xx→unavailable, timeout→timeout) so the
  registry fallback behaves correctly. `health()` is cheap/optimistic to protect the
  dispatch hot path; `probe_models()` does an explicit live readiness check.
- `main.rs`: adapters register only when `CWSO_HAL_{GPU,LPU}_BASE_URL` + `_MODEL` are set,
  so the default deployment (and CI/e2e) runs baseline-only and safe.
- 31 unit tests (mock-transport), fmt clean, 0 warnings.

Per-task briefs live alongside this file as `task-T082.md`, `task-T083.md`, `task-T084.md`,
`task-T085.md`, `task-T086.md`, `task-T087.md`.
