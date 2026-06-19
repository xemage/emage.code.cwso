# Task T083 — GPU adapter (vLLM/TensorRT-LLM, OpenAI-compatible)

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P0
- **Depends on:** T082 (HAL crate — done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A)
- **Based on:** `docs/artifacts/cwso-nextgen-blueprint-v1.md` (Feature A), `task-T082.md`

## Objective
Provide a dense-model GPU backend adapter that speaks the OpenAI-compatible
`/chat/completions` API (vLLM, TensorRT-LLM), pluggable into the HAL registry.

## Outputs
- `services/cwso-hal/src/http.rs` — `HttpTransport` trait, blocking `UreqTransport`,
  offline `MockTransport` (test-only), normalized `TransportError`.
- `services/cwso-hal/src/openai.rs` — `OpenAiCompatibleBackend` + `gpu_vllm_config` preset
  (provider `gpu-accelerated`, latency `fast`, cost `high`, tags `inference-heavy` +
  `deterministic-edit`).
- `services/cwso-hal/src/main.rs` — env-gated registration (`CWSO_HAL_GPU_BASE_URL`,
  `CWSO_HAL_GPU_MODEL`, `CWSO_HAL_GPU_API_KEY`).
- `ureq` dependency added.

## Acceptance Criteria
- [x] OpenAI-compatible chat-completions request/response handling (model, messages,
      max_tokens, temperature=0); bearer auth when an API key is set.
- [x] HTTP/transport failures mapped to `FailureClass` (429→overloaded, 400/422→invalid_request,
      5xx→unavailable, timeout→timeout, malformed→internal) so registry fallback works.
- [x] Registered only when endpoint env is configured; default stays baseline-only.
- [x] Mock-transport unit tests cover parse + each failure mapping; fmt clean, 0 warnings.

## Follow-ups
- T087: Go orchestrator calls this backend (via the HAL UDS) for live execution.

## Blocker Protocol
Report blockers with type and severity; max 2 retries before escalation.
