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
| T088 | Phase 6 integration + reliability QA (fallback ≤ 2.0s, overhead ≤ 10ms) | qa-engineer | in_review | P0 | T087 | 2026-06-02 |
| T089 | Phase 6 Tech-Lead + Security gate | tech-lead / security-engineer | in_review | P0 | T088 | 2026-06-02 |
| T090 | Thread job context into `hal.Client.Infer` (cancellation propagation) | backend-developer | in_review | P1 | T089 | 2026-06-02 |
| T091 | Active HAL health probing → live `health_state`/`queue_depth` | backend-developer | in_review | P1 | T089 | 2026-06-02 |
| T092 | Hardware-aware job result retrieval (poll/stream completion) | backend-developer | in_review | P2 | T089 | 2026-06-02 |
| T093 | Enforce/document TLS for non-loopback HAL accelerator endpoints | devops-engineer | in_review | P1 | T089 | 2026-06-02 |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | in_review | P2 | T089 | 2026-06-02 |
| T095 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | in_review | P2 | T094 | 2026-06-02 |

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
- **T087 capability live-sync (follow-up, done):** `hal.Client.Capabilities()` +
  `dispatch.CapabilitySyncer` refresh the capability registry from the live HAL (immediate
  sync at boot + background refresh on `CWSO_HAL_CAPABILITY_SYNC_SECONDS`, default 15s).
  With a HAL socket the catalog is loaded from the live HAL instead of the static seed; if
  the HAL is unreachable at boot it falls back to the static catalog, and stale providers
  age out via the registry's TTL rule. The CPU baseline stays fresh/terminal-safe.
- **T089 (done, in review):** Phase 6 validation gate — Tech-Lead (implementation) **PASS**
  and Security **PASS**, recorded in `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`.
  No critical/high findings. Five non-blocking follow-ups tracked as **T090–T094** (ctx
  propagation, active health probing, result retrieval, TLS guidance, CI dependency audit).
  **Phase 6 Feature A is cleared to proceed.**
- **T088 (done, in review):** Phase 6 integration + reliability QA. Reliability budgets
  verified: dispatch overhead median ≈ 4µs (budget ≤ 10ms), fallback end-to-end ≈ 51ms
  (budget ≤ 2.0s), failure propagation sub-ms with preserved error. Server-level
  integration test exercises the full `config → server → hal.Client → UDS` path against a
  fake `cwso-hal`. Full suite passes under `go test -race ./...` + gofmt + vet.
  **QA found & fixed a latent reliability bug:** `jobs.Manager.runRecord` cancelled the job
  context *before* reading `ctx.Err()`, misclassifying genuine failures as `cancelled` and
  dropping the error reason — fixed (capture ctxErr pre-cancel) with regression guard
  `jobs.TestLifecycleFailedPreservesError`.

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

### T090 / T092 / T094 (done, in review) — Phase 6 Go follow-ups

Landed on `feature/T090-T092-T094-go-followups` (Rust-side follow-ups T091/T093 deferred to a
separate MR):

- **T090 — context propagation (P1):** `hal.Client.Call` now takes a `context.Context` and
  `hal.Client.Infer(ctx, …)` threads the job context through. Cancelling the job (or hitting
  its deadline) closes the UDS connection to unblock in-flight I/O and returns the context
  error, so an aborted hardware-aware job stops waiting on the HAL. Guards:
  `TestClientInferContextCancelled`, `TestClientInferContextDeadline`.
- **T092 — job result retrieval (P2):** `jobs.Manager` gained a `RunResult func(ctx) (string,
  error)` body variant; the returned payload is captured into `Job.Result` and published on
  the job-state SSE notification. `dispatch_hardware_aware_job` now executes via `RunResult`
  and stores a compact completion summary (`served_by`, `fallback_count`, `tokens_out`,
  `deterministic`, `output`) retrievable via `Manager.Get` / job-state stream. Guards:
  `jobs.TestLifecycleRunResultCaptured`, `jobs.TestEnqueueRejectsBothRunAndRunResult`,
  `tools.TestHardwareAwareDispatchCapturesJobResult`.
- **T094 — CI dependency audit (P2):** new `audit` stage runs `govulncheck ./...` (Go) and
  `cargo audit` (Rust). Non-blocking (`allow_failure: true`) during the PoC phase so a fresh
  advisory surfaces in the pipeline without wedging delivery.

Briefs: `task-T090.md`, `task-T092.md`, `task-T094.md`.

### T091 / T093 (done, in review) — Phase 6 Rust HAL follow-ups

Landed on `feature/T091-T093-rust-hal-followups`:

- **T091 — active health probing (P1):** the `InferenceBackend` trait gained a `probe()`
  method (default = cheap `health()` for dependency-free backends). The OpenAI adapter now
  keeps a lockless cached health snapshot refreshed (a) actively by a background prober
  thread calling `BackendRegistry::probe_all()` every `CWSO_HAL_HEALTH_PROBE_SECONDS`
  (default 10s) via a `/models` readiness check, seeded by a startup probe at registration,
  and (b) reactively from every `infer` outcome (served → healthy; failure → mapped state).
  `health()`/`capabilities()` read only the cache, so dispatch stays fast and the capability
  snapshot the Go `CapabilitySyncer` consumes now carries live `health_state`. `queue_depth`
  is plumbed but stays 0 (no standard OpenAI endpoint; provider-metrics scrape is future work).
- **T093 — endpoint TLS enforcement (P1):** new `security::validate_endpoint` refuses
  plaintext `http://` to non-loopback hosts (bearer key would be sent in cleartext); `https`
  and loopback `http` are allowed, with a `CWSO_HAL_ALLOW_INSECURE_ENDPOINTS=true` override
  (warned). A rejected endpoint is not registered (falls back to CPU baseline). Documented in
  `SECURITY.md`.
- Validation: `cargo fmt --check` clean, `cargo test -p cwso-hal` green (46 tests).

Briefs: `task-T091.md`, `task-T093.md`. This completes the Phase 6 gate follow-ups
**T090–T094**.

### T095 (done, in review) — bump Go toolchain to 1.25

Landed on `feature/T095-bump-go-toolchain`. The first `go:audit` run (T094) flagged 18 Go
**standard-library** advisories ("Fixed in go1.24.x"); these are toolchain-version artifacts,
not dependency bugs. Verified in-container: go1.23.12 → 18 advisories, go1.24.13 → 7 (fixed
only in 1.25.8), **go1.25.10 → none**. Bumped `orchestrator/go.mod` to `go 1.25.0` (dropped
the explicit `toolchain` line), the CI `go:*` images and orchestrator Dockerfile builder to
`golang:1.25`, and `go:audit` to `govulncheck@latest`. Build/vet/test green; `govulncheck`
clean under 1.25. Audits stay `allow_failure: true` for the PoC phase.
