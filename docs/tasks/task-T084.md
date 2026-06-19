# Task T084 — LPU adapter (Groq-style deterministic low-latency)

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T082 (HAL crate — done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` (Feature A), `task-T083.md`

## Objective
Provide an ultra-low-latency LPU backend adapter (Groq-style, OpenAI-compatible) for
realtime workloads, reusing the shared OpenAI-compatible client from T083.

## Outputs
- `services/cwso-hal/src/openai.rs` — `lpu_groq_config` preset (provider `lpu-realtime`,
  latency `ultra`, cost `medium`, tag `realtime`, smaller default max_tokens).
- `services/cwso-hal/src/main.rs` — env-gated registration (`CWSO_HAL_LPU_BASE_URL`,
  `CWSO_HAL_LPU_MODEL`, `CWSO_HAL_LPU_API_KEY`).

## Acceptance Criteria
- [x] Realtime-tagged capability with `ultra` latency class, aligned with the Go shadow
      provider catalog (`lpu-realtime`).
- [x] Shares the OpenAI-compatible transport + failure mapping from T083.
- [x] Registered only when endpoint env is configured; default stays baseline-only.
- [x] Unit tests assert the realtime/ultra capability preset; fmt clean, 0 warnings.

## Follow-ups
- T087: live wiring so realtime tasks routed to `lpu-realtime` execute on this adapter.

## Blocker Protocol
Report blockers with type and severity; max 2 retries before escalation.
