# Task T087 — Wire policy_engine_v2 to live adapters (remove spike stubs)

- **Status:** in_review (live HAL execution wired; capability live-sync deferred as a Phase 6 follow-up)
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T086 (done), T082/T083/T084 (Rust `cwso-hal` adapters — done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `task-T086.md`, `task-T082.md`, `docs/artifacts/cwso-nextgen-blueprint-v1.md`

## Objective
Connect the deterministic `PolicyEngineV2` selection to executable backends so a
hardware-aware dispatch results in real inference on the selected provider.

## Implemented (live execution)
- New Go HAL client `orchestrator/internal/hal` speaks the framed-JSON UDS protocol
  (4-byte length prefix + `{id,op,params}` envelope) shared by all CWSO sidecars, with a
  typed `Infer(provider, fallbackChain, request)` call and structured `SidecarError`.
- `dispatch_hardware_aware_job` now executes the dispatched job against the live HAL when a
  socket is configured: the job body calls `Infer` on the selected provider and forwards
  `decision.RankedFallbackChain` so the HAL falls back deterministically (terminating at
  `cpu-baseline`). Without a socket it preserves the shadow-mode no-op.
- Wiring: `NewDispatchHardwareAwareJobWithHAL` constructor + `CWSO_HAL_SOCKET` config; the
  server builds a `hal.Client` and registers the live tool when the socket is set, else
  registers the shadow-mode tool. Selection, fallback, confidence, and telemetry are
  unchanged and still driven entirely by `PolicyEngineV2`.
- Gated by `CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED` (default off) + `CWSO_HAL_SOCKET`.

## Tests
- `internal/hal`: client round-trip, structured error decode, dial failure.
- `internal/tools`: live execution forwards selected provider + prompt + context + ranked
  fallback chain to the HAL; shadow-mode constructor performs no `Infer`.

## Deferred (Phase 6 follow-up)
- [ ] Replace the static shadow provider catalog with live capability heartbeats sourced
      from HAL adapter health/queue-depth/latency (capability snapshot live-sync). This is a
      separate concern from execution and does not block the Feature A critical path.

## Acceptance Criteria
- [x] Policy engine drives selection for the new tool (not hardcoded).
- [x] CPU baseline remains the terminal-safe fallback.
- [x] Selected provider executes real work via HAL adapter (over UDS).
- [ ] Capability snapshot reflects live adapter health → deferred (capability live-sync follow-up).
