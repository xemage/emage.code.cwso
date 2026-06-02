# Task T087 — Wire policy_engine_v2 to live adapters (remove spike stubs)

- **Status:** in_progress (shadow mode; blocked on T082 for live adapters)
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T086 (done), T082 (Rust `cwso-hal` — pending)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `task-T086.md`, `docs/artifacts/cwso-nextgen-blueprint-v1.md`

## Objective
Connect the deterministic `PolicyEngineV2` selection to executable backends so a
hardware-aware dispatch results in real inference on the selected provider.

## Current State (shadow mode)
- `dispatch_hardware_aware_job` selects among a seeded shadow provider catalog
  (`lpu-realtime`, `gpu-accelerated`, `ssm-longctx`, plus `cpu-baseline`) via the live
  `PolicyEngineV2`. Selection, fallback chain, confidence, and telemetry are exercised
  end-to-end. Job bodies are context-respecting no-ops (`hardwareAwareRunFunc`).
- Gated by `CWSO_HHD_HARDWARE_AWARE_DISPATCH_ENABLED` (default off).

## Remaining Work (blocked on T082-T084)
- [ ] Replace `hardwareAwareRunFunc` no-op with a call into the Rust `cwso-hal`
      `InferenceBackend` adapter for the selected provider (over UDS).
- [ ] Replace static shadow provider catalog with live capability heartbeats sourced
      from registered adapters (health, queue depth, latency class).
- [ ] Remove experimental spike stubs once real adapters supersede them.

## Acceptance Criteria
- [x] Policy engine drives selection for the new tool (not hardcoded).
- [x] CPU baseline remains the terminal-safe fallback.
- [ ] Selected provider executes real work via HAL adapter.
- [ ] Capability snapshot reflects live adapter health.

## Blocker
- **Type:** dependency · **Severity:** major
- **Detail:** Live execution requires the Rust `cwso-hal` crate + adapters (T082-T084),
  not yet implemented. Shadow mode unblocks Go-side integration and tests in the meantime.
