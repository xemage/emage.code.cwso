# Task T082 — Rust `cwso-hal` crate + CPU-baseline adapter

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T081 (HAL design — done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` (Feature A), `task-T085.md`, `task-T086.md`

## Objective
Stand up the Rust Hardware Abstraction Layer crate that defines the `InferenceBackend`
contract and ships the terminal-safe CPU baseline adapter, so real accelerator adapters
(T083/T084) and live policy routing (T087) have a stable foundation.

## Inputs
- HAL trait design from the Next-Gen blueprint (`InferenceBackend { capabilities / infer / health }`).
- Existing `cwso-merge-engine` IPC conventions (length-prefixed JSON frames, SO_PEERCRED authz).
- Go capability-registry field shape (`dispatch.ProviderCapability`) for wire alignment.

## Outputs
- `services/cwso-hal/` crate (new workspace member):
  - `backend.rs` — `InferenceBackend` trait + types (`ProviderCapability`, `Health`,
    `InferenceRequest`, `Completion`, `FailureClass`, `BackendFailure`), contract `dispatch.provider/v2`.
  - `cpu.rs` — `CpuBaselineBackend` (always healthy, deterministic, dependency-free).
  - `registry.rs` — `BackendRegistry::dispatch` (selected → fallback chain → cpu-baseline).
  - `proto.rs` / `ipc.rs` / `main.rs` — UDS server exposing `stat` / `capabilities` / `health` / `infer`.
- `services/Cargo.toml` workspace member added.
- `.gitlab-ci.yml` `rust:test` extended with `cargo test --release -p cwso-hal`.

## Acceptance Criteria
- [x] `InferenceBackend` trait with `capabilities` / `health` / `infer`.
- [x] CPU baseline adapter is always healthy and produces deterministic output.
- [x] Deterministic fallback walk ending at cpu-baseline; non-retryable failures stop early.
- [x] UDS server reuses the merge-engine frame protocol + SO_PEERCRED authz.
- [x] `cargo fmt --all -- --check` clean; `cargo test --release -p cwso-hal` green (21 tests, 0 warnings).

## Follow-ups (not in this task)
- T083 GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible).
- T084 LPU adapter (Groq-style deterministic low-latency).
- T087 live wiring: Go orchestrator calls the HAL over UDS; replace shadow no-op execution.

## Blocker Protocol
Report blockers with type and severity; max 2 retries before escalation.
