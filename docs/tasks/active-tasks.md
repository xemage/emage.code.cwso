# Active Tasks

Next-Gen execution (Phase 6+). Source roadmap: `docs/plans/plan-cwso-nextgen-phase6plus.md`.
Design baseline: `docs/artifacts/cwso-nextgen-blueprint-v1.md`.

| ID | Title | Owner | Status | Priority | Depends on | Last update |
|----|-------|-------|--------|----------|-----------|-------------|
| T080 | Phase 6 requirements + hardware benchmark targets | product-owner | done | P0 | — | 2026-06-02 |
| T081 | HAL design: `InferenceBackend` trait + plugin loading + `dispatch.provider/v2` | solution-architect | done | P0 | T080 | 2026-06-02 |
| T082 | Rust `cwso-hal` crate + CPU-baseline adapter | backend-developer | done | P0 | T081 | 2026-06-02 |
| T083 | GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible) | backend-developer | done | P0 | T082 | 2026-06-02 |
| T084 | LPU adapter (Groq-style deterministic low-latency) | backend-developer | done | P1 | T082 | 2026-06-02 |
| T085 | Profiling layer: tensor_tag derivation + workload mapping | backend-developer | done | P0 | T082 | 2026-06-02 |
| T086 | `dispatch_hardware_aware_job` MCP tool + schema | backend-developer | done | P0 | T083, T085 | 2026-06-02 |
| T087 | Wire policy_engine_v2 to live adapters (remove spike stubs) | backend-developer | done | P0 | T086 | 2026-06-02 |
| T088 | Phase 6 integration + reliability QA (fallback ≤ 2.0s, overhead ≤ 10ms) | qa-engineer | done | P0 | T087 | 2026-06-02 |
| T089 | Phase 6 Tech-Lead + Security gate | tech-lead / security-engineer | done | P0 | T088 | 2026-06-02 |
| T090 | Thread job context into `hal.Client.Infer` (cancellation propagation) | backend-developer | done | P1 | T089 | 2026-06-02 |
| T091 | Active HAL health probing → live `health_state`/`queue_depth` | backend-developer | done | P1 | T089 | 2026-06-02 |
| T092 | Hardware-aware job result retrieval (poll/stream completion) | backend-developer | done | P2 | T089 | 2026-06-02 |
| T093 | Enforce/document TLS for non-loopback HAL accelerator endpoints | devops-engineer | done | P1 | T089 | 2026-06-02 |
| T094 | CI dependency audit (`govulncheck` + `cargo audit`) | devops-engineer | done | P2 | T089 | 2026-06-02 |
| T114 | Bump Go toolchain to 1.25 (clear `go:audit` stdlib advisories) | devops-engineer | done | P2 | T094 | 2026-06-02 |
| T115 | AST write-spike monitor (generalize `anomaly_monitor`) + userspace fallback | backend-developer | done | P0 | T089 | 2026-06-03 |
| T116 | Spike filter (semantic classifier) + semantic-conflict pre-warning | backend-developer | done | P1 | T115 | 2026-06-03 |
| T117 | `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated) | backend-developer | done | P1 | T116 | 2026-06-03 |
| T118 | AST write-event feeder wiring (`write_shadow_file` → monitor/filter) | backend-developer | done | P1 | T117 | 2026-06-03 |
| T119 | Sparse Wasm micro-agent sandbox tier design + security envelope review | solution-architect | done | P0 | T089 | 2026-06-03 |
| T120 | Rust `cwso-sparse` sidecar: deterministic ternary GEMM kernel + UDS protocol | backend-developer | done | P0 | T119 | 2026-06-03 |
| T121 | `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning | backend-developer | done | P1 | T120 | 2026-06-03 |
| T122 | `create_ephemeral_sparse_agent` MCP tool + wasmtime lifecycle + agent telemetry resource | backend-developer | done | P0 | T120, T121 | 2026-06-04 |
| T123 | Quality-floor guardrail → dense GPU escalation (reuse `quality_guardrail_autodisable`) | backend-developer | done | P0 | T122 | 2026-06-04 |
| T124 | Phase 7 integration QA (cold start < 10 ms, 0% idle CPU, escalation) | qa-engineer | done | P0 | T118, T123 | 2026-06-04 |
| T125 | Phase 7 Tech-Lead + Security gate (Feature B + C) | tech-lead / security-engineer | done | P0 | T124 | 2026-06-04 |
| T126 | Sparse AST tensor encoding spec (photonic-ready kernel contract) | solution-architect | done | P0 | T125 | 2026-06-04 |
| T127 | AVX-512 / `std::simd` sparse diff kernel in cwso-merge-engine | backend-developer | done | P0 | T126 | 2026-06-04 |
| T128 | Sparse pre-filter integration (skip shared base subtrees) | backend-developer | done | P0 | T127 | 2026-06-04 |
| T129 | Sparse↔dense conflict-matrix conformance suite | qa-engineer | done | P0 | T128 | 2026-06-04 |
| T130 | Phase 8 Tech-Lead gate + large-repo merge benchmark | tech-lead | done | P0 | T129 | 2026-06-04 |
| T131 | Rollout architecture: proxy boundary + Polar REST API | solution-architect | done | P0 | T130 | 2026-06-04 |
| T132 | Rust `hyper` reverse proxy + zero-copy capture | backend-developer | done | P0 | T131 | 2026-06-04 |
| T133 | Trajectory builder + prefix merging | backend-developer | done | P0 | T132 | 2026-06-05 |
| T134 | Trajectory store (Arrow + LZ4 + Parquet) | backend-developer | done | P1 | T133 | 2026-06-05 |
| T136 | Programmatic reward emission (merge SM hook) | backend-developer | done | P0 | T133 | 2026-06-05 |
| T137 | Polar REST API + trainer e2e | backend-developer | done | P0 | T134, T136 | 2026-06-05 |
| T138 | Phase 9 integration QA + security gate | qa / security | done | P0 | T137 | 2026-06-06 |
| T139 | v0.3.0 release readiness (Phases 6–9) | release-manager | done | P0 | T138 | 2026-06-06 |
| T135 | KV-cache prefix router | backend-developer | done | P1 | T132 | 2026-06-06 |
| T140 | CI audit hardening (promote go:audit/rust:audit to blocking) | devops-engineer | done | P1 | T094, T139 | 2026-06-06 |
| T141 | Publish GitLab release v0.3.0-rc1 + GA prep checkpoint | release-manager | done | P0 | T139, T140 | 2026-06-07 |
| T143 | Root hygiene & PoC debt archive | tech-lead | done | P2 | T141 | 2026-06-07 |
| T142 | Installation & usage documentation | technical-writer | done | P0 | T141 | 2026-06-07 |
| T147 | OpenAI Responses API + proxy hardening | backend-developer | done | P1 | T132 | 2026-06-07 |
| T152 | v0.3.0 GA release readiness | release-manager | done | P0 | T142, T147 | 2026-06-07 |
| T153 | Tag pipeline deploy fix (`needs:optional` e2e) | devops-engineer | done | P1 | T152 | 2026-06-07 |
| T144 | Polar harness adapters + runtime launcher | backend-developer | done | P1 | T137, T142 | 2026-06-07 |
| T145 | Rollout `num_samples` session fan-out | backend-developer | in_review | P1 | T137 | 2026-06-07 |
| T146 | Gateway async staging + partial traces | backend-developer | pending | P1 | T132 | 2026-06-07 |
| T149 | Trajectory builder Polar parity | backend-developer | pending | P2 | T133 | 2026-06-07 |
| T148 | Evaluator registry + SWE-bench hook | backend-developer | pending | P2 | T146 | 2026-06-07 |
| T150 | KV differential prompting | backend-developer | pending | P2 | T135 | 2026-06-07 |
| T151 | Offline SFT data generation mode | backend-developer | pending | P2 | T134 | 2026-06-07 |

> Status values: `pending` · `in_progress` · `blocked` · `in_review` · `done` · `cancelled`
> Priority values: `P0` (critical path) · `P1` (important) · `P2` (nice-to-have)
> Owners are agent names from `knowledge/agents/`.

## Task-numbering reconciliation (authoritative)

The roadmap `plan-cwso-nextgen-phase6plus.md` pre-allocated **T090–T113** to Phases 7–9 when
it was authored. During execution the Phase 6 gate (T089) follow-ups consumed **T090–T094**
and the toolchain chore took **T114**, so the roadmap's Phase 7–9 IDs now collide with
already-completed control-plane work. Rule going forward:

- **Active IDs are assigned sequentially from the board, continuing after T114** — they are
  the single source of truth for what is actually being executed.
- The roadmap's `T090`–`T113` are **feature placeholders**, not active IDs. Each is mapped to
  a fresh active ID when the work is picked up (recorded here + in the task brief).
- Mapping so far:
  - roadmap **Feature C / placeholder T095 (eBPF AST write-spike monitor)** → **active T115**.
  - roadmap **Feature C / placeholder T096 (spike filter + semantic-conflict pre-warning)** →
    **active T116**.
  - roadmap **Feature C / placeholder T097 (`subscribe_ast_spikes` MCP resource)** →
    **active T117** (the MCP Resources protocol layer + tool + threshold-gated SSE). The
    runtime write-event feeder portion of roadmap T097 is split into **active T118**.
  - roadmap **Feature B / placeholder T090 (T0 Wasm sandbox tier design + security envelope
    review)** → **active T119**. The Feature B implementation tasks are pre-mapped in the
    design artifact: roadmap **T091 → active T120**, **T092 → T121**, **T093 → T122**,
    **T094 → T123**, and the Phase 7 QA/gate **T098 → T124**, **T099 → T125**.
  - roadmap **Feature B / placeholder T093 (`create_ephemeral_sparse_agent`)** → **active T122**.
  - roadmap **Feature D / placeholder T100 (sparse AST tensor encoding spec)** → **active T126**
    (Phase 8 kickoff; implementation T127–T130 mapped in `sparse-ast-tensor-encoding-v1.md`).
  - roadmap **Feature E+F+G / placeholder T105 (rollout architecture)** → **active T131**
    (Phase 9 kickoff; implementation T132–T138 mapped in `rollout-architecture-v1.md`;
    release T113 → **active T139**).

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

### T114 (done, in review) — bump Go toolchain to 1.25

> Numbering note: merged as `feature/T095-bump-go-toolchain` (MR !22) before the collision with
> the roadmap's reserved T095 (eBPF AST monitor) was spotted; renumbered to **T114** here.

Landed on `feature/T095-bump-go-toolchain`. The first `go:audit` run (T094) flagged 18 Go
**standard-library** advisories ("Fixed in go1.24.x"); these are toolchain-version artifacts,
not dependency bugs. Verified in-container: go1.23.12 → 18 advisories, go1.24.13 → 7 (fixed
only in 1.25.8), **go1.25.10 → none**. Bumped `orchestrator/go.mod` to `go 1.25.0` (dropped
the explicit `toolchain` line), the CI `go:*` images and orchestrator Dockerfile builder to
`golang:1.25`, and `go:audit` to `govulncheck@latest`. Build/vet/test green; `govulncheck`
clean under 1.25. Audits stay `allow_failure: true` for the PoC phase.

### T115 (done, in review) — AST write-spike monitor + userspace fallback

> **Phase 7 kickoff (Feature C).** Roadmap placeholder T095 → active T115 (see numbering
> reconciliation above). Depends only on T089.

Landed on `feature/T115-ast-write-spike-monitor`. Generalizes the event-driven core of the
dispatch `anomaly_monitor` into a filesystem AST write-spike detector:

- **Shared signal-path machinery (`signal_path.go`):** extracted the eBPF-vs-userspace
  resolver, detection-latency semantics (`advisory`/`measured`/`estimated`), and the
  conservative `defaultEBPFChecker` out of `anomaly_monitor.go` into a reusable
  `signalPathResolver`. The dispatch anomaly monitor now consumes it — identical behaviour,
  guarded by the existing anomaly tests.
- **`ASTWriteSpikeMonitor` (`ast_spike_monitor.go`):** per-workspace sliding-window spike
  detector. `ObserveWrite(WriteEvent)` is source-agnostic (an eBPF write probe **or** a
  userspace filesystem watcher can feed it). On threshold crossing it publishes an
  `ASTSpikeEvent` to topic `ast/spike` with hot paths, distinct-path count, languages,
  severity (`warning`, escalating to `critical` at 2× threshold), and the resolved signal
  path / privilege / detection-latency characterization. Debounce (default = window)
  collapses a sustained burst into a single event.
- **Userspace fallback:** when eBPF is preferred but unavailable (non-Linux, missing
  `CAP_BPF`, no bpffs) detection degrades to the unprivileged userspace path with measured
  latency and the reason recorded in notes — detection never depends on privilege.
- **Privacy:** hot paths are dropped alongside notes under the `anomaly_notes_mode=drop`
  redaction policy (filesystem structure is treated as sensitive).
- 9 new unit tests (threshold, silence-below-threshold, window pruning, debounce, severity
  escalation, eBPF-hook + fallback semantics, redaction, workspace isolation). Full
  orchestrator suite + gofmt + vet green.

Follow-ups (next Phase 7 tasks): spike-filter sparse mini-model + semantic-conflict
pre-warning (roadmap T096), and the `subscribe_ast_spikes` MCP SSE resource + write-event
feeder wiring (roadmap T097). Brief: `task-T115.md`.

### T116 (done, in review) — semantic spike filter + conflict pre-warning

> **Phase 7 (Feature C, step 2 + 5).** Roadmap placeholder T096 → active T116. Depends on T115.

Landed on `feature/T116-ast-spike-filter-semantic-prewarning`. Sits downstream of the T115
volume monitor: the monitor detects write *volume*, the filter decides whether an edit
*matters* and whether it overlaps a sibling agent's in-flight change.

- **`ASTSpikeFilter` (`ast_spike_filter.go`):** classifies each `WriteEvent` into a
  `SpikeKind` (`none` < `cosmetic` < `symbol_added`/`symbol_removed` < `signature_change`)
  and a confidence, then gates on a configurable `SemanticThreshold` (`signature_change`
  default, or `symbol_added`/`symbol_removed`/`any`). Emits `SemanticSpikeEvent` on topic
  `ast/semantic-spike` only when the threshold is crossed (zero-noise / neuromorphic
  event-driven principle).
- **Pluggable `SemanticScorer` seam:** the default `HeuristicSemanticScorer` is deterministic
  and dependency-free (trusts a feeder-supplied `ChangeKind`, else diffs the symbol's
  `SignatureHash` against the last seen one). The blueprint's *sparse Wasm mini-model* (from
  Feature B, not yet built) can drop in later via the `Scorer` config field without touching
  correlation logic.
- **Semantic-conflict pre-warning (step 5):** a per-symbol recent-writers index (bounded by a
  correlation window) detects when ≥2 distinct workspaces produce semantic spikes on the same
  symbol, and publishes `SemanticConflictWarning` on `ast/conflict-warning` with
  `potential_conflict_with: [workspace…]` — letting the orchestrator pre-warn agents *before*
  `merge_concurrent_results` runs. Severity escalates to `critical` for `signature_change`.
- **`WriteEvent`** gained optional semantic hints (`Symbol`, `NodePath`, `SignatureHash`,
  `ChangeKind`); the T115 volume monitor ignores them, so the two stages share one ingestion
  shape.
- **Reuse + privacy:** shares the `signalPathResolver` (eBPF-hook vs userspace fallback) and
  detection-latency semantics; under `anomaly_notes_mode=drop` it blanks symbol/path/node-path
  (source structure) alongside notes, after correlation has already happened.
- 9 new unit tests (classification, threshold gating incl. `any`, conflict detection, single-
  workspace no-op, window pruning, eBPF fallback, redaction, custom-scorer seam). Full suite +
  `go test -race` + gofmt + vet green.

Next: `subscribe_ast_spikes` MCP SSE resource + concrete write-event feeders (eBPF probe /
userspace fs watcher) — roadmap T097. Brief: `task-T116.md`.

### T117 (done) — `subscribe_ast_spikes` MCP Resources layer (SSE, threshold-gated)

> **Phase 7 (Feature C, step 3).** Roadmap placeholder T097 → active T117. Depends on T116.
> The runtime write-event **feeder** half of roadmap T097 is split into **T118** (the resource
> machinery filters broker records regardless of producer, so it ships and is testable first).

Landed on `feature/T117-subscribe-ast-spikes-resource`. Exposes the T115/T116 spike topics as
subscribable MCP resources under the `cwso://` scheme — the first MCP **Resources** surface in
the server (previously tools-only):

- **`subscribe_ast_spikes` tool** (`tools/ast_spike_tools.go`, orchestrator + worker): validates
  `path` (glob), `semantic_threshold` (`signature_change` default · `symbol_added` · `symbol_removed`
  · `any`), and `workspace_scope`, registers a subscription, and returns
  `{subscription_id, stream_resource: "cwso://spikes/<id>", topics, transport_hint}`.
- **Subscription registry + filter** (`dispatch/spike_subscriptions.go`): concurrency-safe store;
  `SpikeSubscription.Allow(topic, payload)` is the shared predicate (threshold rank gating, path
  glob via `path.Match`, workspace scope; volume `ast/spike` events only pass the `any` threshold,
  matched against hot paths). Implements the transport's `RecordFilter` contract.
- **MCP Resources handlers** (`server.go`): `resources/list`, `resources/templates/list`
  (`cwso://spikes/{subscription_id}`), `resources/read` (a threshold/path/workspace-filtered
  **snapshot** replayed from the broker log), `resources/subscribe`, `resources/unsubscribe`.
  The `resources` capability (`subscribe:true`) is advertised in `initialize` only when enabled;
  when disabled the methods return method-not-found.
- **Threshold-gated scoped SSE** (`transport/http.go`): `GET /mcp?subscription=<id>` resolves the
  subscription via a new `WithSubscriptionResolver` option and streams **only** matching spike
  events (unknown id → 404). Added as a variadic option rather than a 8th positional param
  (respects TD-02).
- **Config:** `CWSO_AST_SPIKE_RESOURCES_ENABLED` (default false) gates registry construction +
  tool registration + capability + SSE resolver.
- 20 new unit tests across dispatch (filter matrix), tools (tool contract), server (capability,
  routing, list/read/subscribe lifecycle, threshold-gated snapshot), transport (scoped SSE filter,
  404 paths), and config. Full suite + `go test -race` + gofmt + vet green.

Next (**T118**): wire a concrete write-event feeder (`write_shadow_file` → `ASTWriteSpikeMonitor` +
`ASTSpikeFilter`, config-gated) so live edits drive the stream end-to-end; eBPF/fs-watch sources
remain a later option. Brief: `task-T117.md`.

### T118 (done) — AST write-event feeder (`write_shadow_file` → monitor/filter)

> **Phase 7 (Feature C, runtime feeder half of roadmap T097).** Active T118. Depends on T117.

Landed on `feature/T118-ast-write-event-feeder`. Lights up the T115/T116 monitors with a real
in-process write source so the T117 `cwso://spikes` resources stream live edits end-to-end:

- **`dispatch.WriteEventSink` + `NewWriteEventFanout`** (`write_event_sink.go`): one write feeds
  both the volume monitor (`ASTWriteSpikeMonitor`) and the semantic filter (`ASTSpikeFilter`);
  nil sinks drop out, a failing sink doesn't starve the others.
- **`write_shadow_file` feeder:** `NewWriteShadowFileWithObserver` emits a `dispatch.WriteEvent`
  after each *successful* write (failed writes never feed). Language is derived from the file
  extension (Go/Python/Rust/TS/JS, matching `query_ast`). **Symbol surface is approximated by
  the file path and the signature by a content SHA-256** — so the volume monitor sees real
  write rates, and the semantic filter detects content changes (`signature_change`) and
  cross-workspace edits to the same file (conflict pre-warning). AST-symbol-level extraction
  (via `query_ast`) is the documented next refinement.
- **Server wiring + config:** `buildASTWriteSink` constructs monitor+filter (sharing the HHD
  telemetry redaction policy) when `CWSO_AST_SPIKE_MONITOR_ENABLED=true`, and
  `registerShadowTools` injects the sink into `write_shadow_file`. New
  `CWSO_AST_SPIKE_{WINDOW_MS,THRESHOLD,DEBOUNCE_MS,MAX_HOT_PATHS,SEMANTIC_THRESHOLD,
  CONFLICT_WINDOW_MS,SIGNATURE_TTL_MS,MAX_CONFLICT_PEERS,EBPF_ENABLED}` knobs with validation.
- 11 new unit tests (observer fires/doesn't-fire on success/failure, language detection,
  fanout delivery/nil-drop/error-continue, fanout-through-real-stages, config defaults +
  validation, and a server end-to-end test: 3 worker `write_shadow_file` calls → `ast/spike`
  on the broker). Full suite + `go test -race` + gofmt + vet green.

Feature C is now wired end-to-end (write → spike topics → `cwso://spikes` resources). Remaining
Phase 7 work: real eBPF/fs-watch write sources and the sparse-model scorer (scorer seam already
in place). Brief: `task-T118.md`.

### T119 (done) — Sparse Wasm micro-agent tier design (Feature B kickoff)

> **Phase 7 (Feature B — Ephemeral Wasm Micro-Agents). Active T119 = roadmap placeholder T090.**
> Docs/architecture only; gates the Feature B implementation tasks T120–T125.

Landed on `feature/T119-wasm-sparse-agent-design`. Opens the second Phase 7 track (the sparse
1.58-bit Wasm micro-agent tier) with its gating design + decision record:

- **`ADR-008-wasm-sparse-agent-tier.md`** (accepted): two-runtime split — control-side scorers reuse
  the existing wazero host (`wasm_scoring_plugin.go`) verbatim; data-side inference runs in a new
  Rust `cwso-sparse` **wasmtime** sidecar over UDS (cwso-hal pattern). The deterministic **ternary
  GEMM is a native Rust kernel behind a tight `ternary_gemm` host-call allowlist** (full-Wasm kernel
  deferred as the promotion path). Weights are SHA-256-pinned, pruned `{-1,0,+1}` skill-slices,
  mmap'd copy-on-write so N agents share one resident copy. Quality-floor breaches reuse the existing
  `quality_guardrail_autodisable` path to escalate to a dense GPU backend. Alternatives A–D tabled.
- **`wasm-sparse-agent-design-v1.md`**: positions the new **T0** tier beneath the ADR-003
  gVisor/Firecracker tiers (< 10 ms cold start), specifies the `create_ephemeral_sparse_agent` schema
  + lifecycle, the `cwso://agents/{id}/telemetry` resource (reusing the T117 SSE layer), the security
  envelope mapping (no new capabilities beyond `ternary_gemm`), and the T120–T125 implementation
  breakdown.

Next (**T120**): build the `cwso-sparse` wasmtime sidecar + native ternary GEMM host-call + UDS
protocol (deterministic kernel, feature-flagged). Brief: `task-T119.md`.

### T120 (done) — `cwso-sparse` sidecar: deterministic ternary GEMM + UDS protocol

> **Phase 7 (Feature B). Active T120 = roadmap placeholder T091.** Depends on T119.

Landed on `feature/T120-cwso-sparse-sidecar`. Establishes the data-side sidecar for the sparse
micro-agent tier with its deterministic compute core and wire protocol:

- **New Rust crate `services/cwso-sparse`** (added to the workspace + `rust:test` CI job).
- **`gemm.rs` — deterministic 1.58-bit ternary GEMM kernel** (BitNet b1.58, weights ∈ {-1,0,+1}
  packed 2-bit/4-per-byte + per-row `f32` scale). Pure add/subtract/skip inner product in fixed
  k-ascending order → byte-identical output across runs. Validated against an independent dense
  reference, with pack/unpack, multi-row, determinism (1000×), shape-mismatch and invalid-encoding
  tests.
- **`ipc.rs` — UDS framed-JSON protocol** (4-byte length prefix + JSON body) with `SO_PEERCRED`
  peer-auth, identical envelope to `cwso-hal`. Ops: `stat` and the single bounds-checked
  `ternary_gemm` host-call (the only compute capability — no FS/network/process surface).
- 15 unit tests; `cargo test --release -p cwso-sparse`, `cargo fmt --check`, and workspace build
  all green locally.

**Delivery note:** per ADR-008 the data-side runtime is wasmtime, but the heavy
module-instantiation envelope is deferred to the agent-lifecycle slice (**T122**) to keep this MR a
focused, dependency-light, deterministic core; T120 ships the sidecar + protocol + kernel +
host-call contract. Next (**T121**): the `.cwsl` pruned-slice container + COW mmap loader feeding
this kernel. Brief: `task-T120.md`.

### T121 (done) — `.cwsl` pruned-slice container + COW mmap loader + SHA-256 pinning

> **Phase 7 (Feature B). Active T121 = roadmap placeholder T092.** Depends on T120.

Landed on `feature/T121-cwsl-slice-loader`. Gives the kernel its weight-supply path so N agents can
share one resident copy of a pruned skill slice:

- **Kernel refactor (`gemm.rs`):** extracted a borrowed `TernaryView<'a>` (the GEMM now runs over
  borrowed `scales`/`packed` slices); owning `TernaryWeights` delegates via `as_view()`. This lets
  the loader run inference **directly over the mmap** without copying weights per agent.
- **`.cwsl` container (`slice.rs`):** little-endian header (magic `CWSL`, version, quantization,
  `n`/`k`/`scale_count`/`packed_len`) + `f32` scales + 2-bit-packed ternary weights. `serialize` +
  `SliceHeader::parse` + `content_hash` (SHA-256 = content address).
- **`MappedSlice::open` (memmap2):** maps the file **read-only** (OS shares resident weight pages
  across agents — the COW story), verifies declared length, **verifies SHA-256 against the pinned
  hash** (hard error on mismatch), validates dims against the kernel contract, materialises only the
  small scale vector, and exposes a zero-copy `view()` borrowing `packed` from the mmap.
- **`SliceManifest`:** JSON `skill_domain → {path, sha256}`, resolves relative paths and
  `load_slice(domain)` → integrity-verified `MappedSlice`.
- 24 unit tests (9 new): serialize/parse round-trip, hash stability, gemm-from-mmap, integrity
  mismatch, non-hex pin, truncation, bad magic/version, length mismatch, manifest resolve. New deps:
  `memmap2`, `hex` (+ existing `sha2`). `cargo test --release`, `fmt --check`, workspace build green.

Next (**T122**): `create_ephemeral_sparse_agent` MCP tool + wasmtime instantiation +
`cwso://agents/{id}/telemetry`, consuming this loader. Brief: `task-T121.md`.

### T122 (done) — `create_ephemeral_sparse_agent` + wasmtime lifecycle + agent telemetry

> **Phase 7 (Feature B). Active T122 = roadmap placeholder T093.** Depends on T120, T121.

Landed on `feature/T122-sparse-agent-lifecycle`. Wires the sparse micro-agent tier end-to-end from
the orchestrator MCP surface through the `cwso-sparse` sidecar:

- **Rust (`agent.rs` + IPC v2):** `AgentRegistry` resolves a skill domain via the T121 manifest,
  mmap-pins the slice, instantiates a **wasmtime** sandbox module with a per-agent memory cap
  (`StoreLimits`), measures `cold_start_ms`, and tracks agents until `drop_agent`. New IPC ops:
  `create_agent`, `drop_agent`, `agent_stat` (contract version bumped to 2). Feature-flagged via
  `CWSO_SPARSE_SLICE_MANIFEST` on the sidecar.
- **Go sparse client (`internal/sparse`):** framed-JSON UDS client mirroring `hal.Client`.
- **`create_ephemeral_sparse_agent` tool:** orchestrator-only; validates `skill_domain`,
  `quantization` (1.58-bit only for PoC), and `max_ram_mb` (host cap from
  `CWSO_SPARSE_HOST_RAM_CAP_MB`); calls sidecar; registers agent; publishes initial telemetry to
  `agents/telemetry`; returns `{wasm_agent_id, cold_start_ms, resident_ram_mb, stream_resource}`.
- **Telemetry resource:** reuses the T117 broker → SSE layer —
  `cwso://agents/{wasm_agent_id}/telemetry` via `resources/list|read|subscribe` and scoped SSE
  (`GET /mcp?subscription=<wasm_agent_id>`). Composite subscription resolver covers spike + agent
  streams.
- **Schema:** `schemas/create_ephemeral_sparse_agent.json`. Config:
  `CWSO_SPARSE_AGENTS_ENABLED`, `CWSO_SPARSE_SOCKET`, `CWSO_SPARSE_HOST_RAM_CAP_MB`.
- 26 Rust + new Go unit tests; full orchestrator suite green.

Brief: `task-T122.md`.

### T123 (done) — Quality-floor guardrail → dense GPU escalation

> **Phase 7 (Feature B). Active T123 = roadmap placeholder T094.** Depends on T122.

Landed on `feature/T123-quality-floor-escalation`. When `CWSO_SPARSE_QUALITY_GUARDRAIL_ENABLED=true`
and policy/capability registry are configured, `create_ephemeral_sparse_agent` accepts optional
`quality_floor` (0..1). A breach below `CWSO_HHD_SPARSE_QUANTIZED_QUALITY_GUARDRAIL_MIN_SCORE`
skips sparse instantiation and returns `escalated: true` with
`reason_code: quality_guardrail_autodisable`, selecting a dense GPU provider via
`SelectDenseGPUEscalation` (sparse/quantized assist off). When HAL + jobs are wired, a hardware-aware
inference job is enqueued on the selected provider. Shared helper `QualityGuardrailBreached` is
also used by `policy_engine_v2` sparse-quantized autodisable.

Brief: `task-T123.md`.

### T124 (done) — Phase 7 integration QA (Feature B + C)

> **Phase 7 QA gate input.** Active T124 = roadmap placeholder T098. Depends on T118, T123.

Landed on `feature/T124-phase7-integration-qa` (MR !33, merge `eb4aa45`). Guards Phase 7 budgets:

- **Cold start `< 10 ms` (p95, warm slice):** `cwso-sparse` `cold_start_warm_p95_under_budget` +
  Go control-plane overhead budget tests.
- **0% idle CPU (event-driven spike pipeline):** `TestASTSpikePipelineZeroIdleEmissions` — idle
  monitor+filter emits zero broker records (no polling timers).
- **Quality-floor escalation:** server integration test exercises guardrail → dense HAL `infer`
  without sparse agent creation.

QA report: `docs/artifacts/qa-phase7-report-v1.md`. CI pipeline #2575895437 green at `e964e1b`.

### T125 (done) — Phase 7 Tech-Lead + Security gate (Feature B + C)

> **Phase 7 validation gate.** Active T125 = roadmap placeholder T099. Depends on T124.

Landed on `feature/T125-phase7-gate` (MR !34, merge `146f208` on `develop`). Gate artifacts:

- **Tech-Lead + Security:** `docs/artifacts/gate-phase7-feature-bc-2026-06-04.md` — both **PASS**
- **OWASP checklist:** `docs/artifacts/security-phase7-checklist-v1.md`
- CI pipeline #2575994520 green at `70019c3`

**Phase 7 Features B + C are complete.** Brief: `task-T125.md`. Checkpoint:
`docs/checkpoints/checkpoint-009-phase7-complete.md`.

### T126 (done) — Sparse AST tensor encoding spec (Feature D kickoff)

> **Phase 8 (Feature D — Semantic Sparse-Merging).** Active T126 = roadmap placeholder T100.
> Depends on T125. Docs/architecture only; gates T127–T130 implementation.

Landed via MR !35 (squash merge to `develop`, source `57aa2f4`). Artifacts: ADR-009,
`sparse-ast-tensor-encoding-v1.md`. Brief: `task-T126.md`. Next: **T127** (SIMD kernel).

### T127 (done) — AVX2 sparse diff kernel in cwso-merge-engine

> **Phase 8 (Feature D).** Active T127 = roadmap placeholder T101. Depends on T126.

Landed via MR !36 → `develop` (`3a45f8a`, source tip `f4f8392` / conflict merge `5c76783`).
Adds `sparse_tensor`, `sparse_diff`, and AVX2 digest comparison in `services/cwso-merge-engine`.
Brief: `task-T127.md`. Next: **T128** (pre-filter hook).

### T128 (done) — Sparse pre-filter in `merge_three_way`

> **Phase 8 (Feature D).** Active T128 = roadmap placeholder T102. Depends on T127.

Landed via MR !37 → `develop` (`7f489b4`, source `aa509fb`). Wires `sparse_diff` before
`resolve_base_decisions`; `BothUnchanged` rows skip per-side byte compare. Brief: `task-T128.md`.
Next: **T129** (sparse↔dense conformance).

### T129 (done) — Sparse↔dense conflict-matrix conformance

> **Phase 8 (Feature D).** Active T129 = roadmap placeholder T103. Depends on T128.

Landed via MR !38 → `develop` (`0977483`, squash `787c244`, source `5a94f67`, pipeline #2577297839).
Brief: `task-T129.md`. Next: **T130** (Phase 8 Tech-Lead gate + benchmark).

### T130 (done) — Phase 8 Tech-Lead gate + large-repo merge benchmark

> **Phase 8 validation gate.** Active T130 = roadmap placeholder T104. Depends on T129.

Landed via MR !39 → `develop` (merge `7dc4e7a`, source `d77d08c`, pipeline #2577485639). Gate artifacts:

- **Tech-Lead + Security:** `docs/artifacts/gate-phase8-feature-d-2026-06-04.md` — both **PASS**
- **OWASP checklist:** `docs/artifacts/security-phase8-checklist-v1.md`
- **Benchmark:** `docs/benchmarks/phase8-large-repo-merge-benchmark-v1.md`

**Phase 8 Feature D is complete.** Brief: `task-T130.md`. Checkpoint:
`docs/checkpoints/checkpoint-010-phase8-complete.md`. Next: **T131** (Phase 9 rollout architecture).

### T131 (done) — Rollout architecture (Feature E kickoff)

> **Phase 9 (Rollout-as-a-Service).** Active T131 = roadmap placeholder T105. Depends on T130.

Merged via MR !40 → `2d40413` on `develop`. Docs/architecture only; gates T132–T138:

- **ADR-010** + `rollout-architecture-v1.md` (proxy sidecar, trajectory builder, Polar REST API)
- Schemas: `rollout_task_submit.json`, `rollout_task_status.json`

Brief: `task-T131.md`. Next (**T132**): Rust `hyper` reverse proxy + zero-copy capture in
`cwso-rollout`. Brief: `task-T132.md`.

### T132 (done) — Rust hyper reverse proxy + capture

Merged via MR !41 → `267922c` on `develop` (squash `0896f98`; feature tip `6f04fcd`).
Implements `services/cwso-rollout`:

- `hyper` reverse proxy for OpenAI/Anthropic/Google routes
- Four-step capture pipeline (detect → normalize → forward+store → denormalize/synthetic SSE)
- Framed-JSON UDS control plane + non-blocking `crossbeam-channel` capture queue
- Unit/integration tests; wired into workspace `Cargo.toml` + CI `rust:test`
- CI: branch pipeline https://gitlab.com/em-age/emage.code.cwso/-/pipelines/2577824713 green; MR
  pipelines intermittently failed on Docker Hub 429 / DinD (no code defect).

Brief: `task-T132.md`. Next (**T133**): trajectory builder. Brief: `task-T133.md`.

### T133 (done) — Trajectory builder + prefix merging

Merged via MR !42 → `develop` (`18b5a40`, squash `5bd981b`; source `59026df`, pipeline #2578342413).
Go `orchestrator/internal/rollout`: prefix merge, loss masks, UDS `drain_capture` client.

Brief: `task-T133.md`. Next (**T134**): Parquet trajectory store in `cwso-rollout`. Brief: `task-T134.md`.

### T134 (done) — Trajectory store (Arrow + LZ4 + Parquet)

> **Phase 9 (Feature E).** Active T134 = roadmap placeholder T108. Depends on T133.

Merged via MR !43 → `develop` (`26761ab`; source `374d672`, pipeline #2579771042).
Rust `cwso-rollout/src/store.rs`: Parquet/LZ4 writer thread, fan-out enqueue, retention sweep.
CI socket-runner layout landed in same merge (`.gitlab-ci.yml`, `docker-compose.ci.yml`).

Brief: `task-T134.md`. Next (**T136**, P0): programmatic merge rewards. Brief: `task-T136.md`.

### T136 (done) — Programmatic reward emission

> **Phase 9 (Feature G).** Active T136 = roadmap placeholder T110. Depends on T133.

Merged via MR !45 → `develop` (`892142f`; source `faf40c7`, pipeline #2579820940).
Go merge SM hook publishes `rollout/reward` when `CWSO_ROLLOUT_REWARD_ENABLED=true`.

Brief: `task-T136.md`. Next (**T137**, P0): Polar REST API. Brief: `task-T137.md`.

### T137 (done) — Polar REST API + trainer e2e

> **Phase 9.** Active T137 = roadmap placeholder T111. Depends on T134 + T136.

Merged via MR !46 → `develop` (`c1c56d6`; source `3a72ad7`, pipeline #2579885204).
`/rollout/*`, `/callbacks/*`, `/nodes/*` when `CWSO_ROLLOUT_API_ENABLED=true`.

Brief: `task-T137.md`. Next (**T138**, P0): Phase 9 QA + security gate. Brief: `task-T138.md`.

### T138 (done) — Phase 9 integration QA + security gate

> **Phase 9.** Active T138 = roadmap placeholder T112. Depends on T137.

Merged via MR !47 → `develop` (`5d2cfca`, squash `011d8c8`; pipeline green on feature branch
pre-merge). Gate PASS/PASS; trainer e2e integration tests in `orchestrator/internal/rollout/integration_test.go`.

Artifacts: `qa-phase9-report-v1.md`, `gate-phase9-feature-efg-2026-06-05.md`,
`security-phase9-checklist-v1.md`, `checkpoint-011-phase9-complete.md`.

Brief: `task-T138.md`. **Phase 9 Features E+F+G are complete.**

### T139 (done) — v0.3.0 release readiness (Phases 6–9)

> **Release packaging.** Active T139 = roadmap placeholder T113. Depends on T138.

Merged via MR !48 → `develop` (`d693c3f`; squash `804d5df`, pipeline #2581160040 all 11 jobs green).
Tagged **`v0.3.0-rc1`** on `develop`. Delivers `release-v0.3.0-rc1.md`, CHANGELOG
`v0.3.0-rc1`, plan status (Phases 6–9 complete).

Brief: `task-T139.md`. **Next-Gen Phases 6–9 RC is published.** GA (`v0.3.0`) deferred on
stakeholder validation + audit hardening (T094).

### T135 (done) — KV-cache prefix router

> **Phase 9 (Feature F).** Active T135 = roadmap placeholder T109. Depends on T132.

Merged via MR !49 → `develop` (`0685893`; squash `f9f0199`, pipeline #2581257390 all 11 jobs green).
BLAKE3 prefix keying, git-shadow `get_workspace`, cwso-rollout LRU prewarm IPC. Flag:
`CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` (default false).

Brief: `task-T135.md`.

### T140 (done) — CI audit hardening

> **GA hardening.** T094 follow-up. Depends on T094, T139.

Merged via MR !50 → `develop` (`130a254`; squash `1f4364e`, pipeline #2581717294 all 11 jobs green).
Removed `allow_failure: true` from `go:audit` and `rust:audit`; `rust:lint` remains advisory.

Brief: `task-T140.md`.

### T141 (done) — Publish GitLab release v0.3.0-rc1 + GA prep checkpoint

> **RC publication.** Depends on T139 (tag), T140 (post-RC hardening).

Published GitLab release for existing tag `v0.3.0-rc1` @ `2032b33` via `glab release create`.
Delivered `checkpoint-012-nextgen-ga-prep.md`; reconciled `release-v0.3.0-rc1.md`, plan, and task board.
Post-RC `develop` tip: `f5db055`. GA path blocked on stakeholder RC validation.

Brief: `task-T141.md`. Release: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.3.0-rc1

### T143 (done) — Root hygiene & PoC debt archive

Archived `POC-DEBT-SCORECARD-phase{1,2}.md` → `docs/archive/debt/`; moved original
`CWSO_ Agentic AI Orchestration Blueprint.md` → `input/`. Kept `TECHNICAL-DEBT.md` at root.

Brief: `task-T143.md`.

### T142 (done) — Installation & usage documentation

Delivers `docs/user/installation-v1.md` — Docker quick start, JWT, MCP, Next-Gen flags,
troubleshooting. Critical for GA adopters.

Brief: `task-T142.md`.

### T147 (done) — OpenAI Responses API + proxy hardening

Merged @ `2c2b873` (MR !53). `/v1/responses` route, provider-specific SSE, rollout architecture §3.3.

### T144 (done) — Polar harness adapters + runtime launcher

Merged @ `50f3406` (MR !56). Registry, Docker runtime, shell-command reference harness + e2e.

### T152 (done) — v0.3.0 GA release

Tagged **`v0.3.0`** @ `de071c0`. Release: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.3.0

Brief: `task-T152.md`. **Next:** T145 (Polar session fan-out).

### T153 (done) — Tag pipeline deploy fix

Merged MR !58 — `needs:optional` on `e2e:phase2` for tag pipelines.

Brief: `task-T153.md`. **Next:** T145.

### Polar parity backlog (T145–T151)

Gap analysis: `docs/artifacts/polar-gap-analysis-v1.md`. T144 done; post-GA: session fan-out,
gateway staging, evaluators, trajectory parity, differential prompting, offline SFT.
