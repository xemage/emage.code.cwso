# Phase 6 Validation Gate — Feature A (Heterogeneous Hardware Dispatcher)

**Target:** Phase 6 Feature A — tasks T080–T088 + T087 capability live-sync follow-up
**Based on:** `docs/plans/plan-cwso-nextgen-phase6plus.md`, `docs/artifacts/cwso-nextgen-blueprint-v1.md`,
`task-T082`…`task-T088`
**Date:** 2026-06-02

Scope reviewed (merged to `develop` via MRs !13–!18):
- **Go control plane:** `dispatch/profiler.go`, `dispatch/policy_engine_v2.go`,
  `dispatch/capability_registry.go`, `dispatch/capability_syncer.go`,
  `tools/dispatch_hardware_aware_tools.go`, `hal/client.go`, `jobs/manager.go`,
  `server/server.go` wiring, `config/config.go` flags, `schemas/dispatch_hardware_aware_job.json`.
- **Rust HAL (`services/cwso-hal`):** `backend.rs`, `cpu.rs`, `registry.rs`, `openai.rs`,
  `http.rs`, `ipc.rs`, `proto.rs`, `main.rs`.

Evidence base: full `go test -race ./...` green; `gofmt -l` / `go vet` clean;
`cargo test --release -p cwso-hal` green (31 tests); CI pipelines #2571170930, #2571230438,
#2571275858 fully green (lint, 3 builds, go:test, rust:test, e2e:phase2, e2e:phase4-swarm).

---

## Gate Verdict: Implementation Review

**Gate:** implementation
**Executor:** tech-lead
**Date:** 2026-06-02
**Target:** Phase 6 Feature A

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | reliability | `hal.Client.Infer` takes no `context.Context`; a cancelled/timed-out job won't abort an in-flight HAL request (bounded only by the 60s conn deadline). | Thread the job `ctx` into the HAL call (set conn deadline from ctx). Tracked as **T090**. |
| 2 | medium | observability | Adapter `health()`/`capabilities()` are optimistic (no active probing), so live-sync reflects *registration* more than *liveness*; real liveness is enforced only at `infer` time via fallback. | Add active health probing (e.g. periodic `/models`) feeding `health_state`/`queue_depth`. Tracked as **T091**. |
| 3 | low | feature gap | The async job records only success/failure, not the `Completion` payload (fire-and-forget). Callers can't retrieve generated output yet. | Add result retrieval (poll/stream) when needed. Tracked as **T092**. |
| 4 | low | performance | `hal.Client.Call` dials a fresh UDS connection per request (no pooling). | Acceptable for current volume (UDS dial is cheap); revisit if dispatch QPS grows. |

### Summary
Implementation is correct, well-tested, and convention-compliant: deterministic routing lives
entirely in the Go control plane, every new capability is **default-off** behind `CWSO_*` flags,
and the CPU baseline remains the terminal-safe fallback at both the policy and HAL layers. The
QA gate (T088) also caught and fixed a latent `jobs.Manager` failure-classification bug. No
critical/high findings; the medium/low items are non-blocking enhancements tracked as follow-ups.

---

## Gate Verdict: Security Audit

**Gate:** security
**Executor:** security-engineer
**Date:** 2026-06-02
**Target:** Phase 6 Feature A

### Verdict: PASS

### Findings

| # | Severity | Category | Description | Recommendation |
|---|----------|----------|-------------|----------------|
| 1 | medium | transport security | The HAL → model-server HTTP client (`http.rs`) supports `http://` base URLs; with a non-loopback `http` endpoint the `Authorization: Bearer` token + prompts traverse plaintext. | Document/enforce `https` (or in-cluster network) for non-loopback `CWSO_HAL_{GPU,LPU}_BASE_URL`. Tracked as **T093**. |
| 2 | low | info exposure | Non-2xx model-server response bodies are embedded in `BackendFailure` messages that propagate to job errors. | Truncate/scrub upstream bodies before surfacing in error strings. |
| 3 | low | supply chain | No automated dependency vulnerability scan in CI for Go modules / Rust crates. | Add `govulncheck` + `cargo audit` jobs (allow_failure initially). Tracked as **T094**. |

### Security controls verified (no findings)
- **Secrets:** API keys sourced exclusively from env vars (`CWSO_HAL_*_API_KEY`); none committed.
  Sent as `Authorization: Bearer` headers (not URLs); not written to logs (tracing logs provider
  id + error only). Conforms to `SECURITY.md` "no secrets in code".
- **IPC authz:** HAL UDS guarded by `SO_PEERCRED` UID/GID allowlist (`CWSO_IPC_ALLOWED_UIDS/GIDS`,
  default = own euid/egid) **and** `0o660` socket permissions.
- **AuthZ:** `dispatch_hardware_aware_job` is orchestrator-role-only.
- **Input validation:** `task_description` sanity-checked, `context_size_estimate >= 0`,
  `target_workspace_uuid` format-validated, `quality_floor ∈ [0,1]`, latency normalized.
- **SSRF:** `base_url` is operator-configured (env), not request-controlled; prompt content goes
  in the request body, never the URL. No injection sinks (no shell/SQL) on the dispatch path.
- **Fallback safety:** non-retryable (`invalid_request`) failures stop the fallback walk rather
  than masking caller errors with the baseline.

### Summary
No critical or high security findings. Authentication material, IPC authorization, input
validation, and fail-safe fallback are sound. The medium TLS item is a deployment-configuration
concern with a clear mitigation (in-cluster/`https` endpoints) and is tracked, not blocking.

---

## Combined Gate Outcome

| Gate | Verdict |
|------|---------|
| Implementation (Tech-Lead) | **PASS** |
| Security | **PASS** |

**Phase 6 Feature A is cleared to proceed.** Five non-blocking follow-ups (T090–T094) are
tracked on the task board. The deferred capability live-sync follow-up from T087 is now complete.
