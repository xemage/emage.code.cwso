# Task T091 — Active HAL health probing → live `health_state`/`queue_depth`

- **Status:** in_review
- **Owner:** backend-developer
- **Priority:** P1
- **Depends on:** T089 (done)
- **Phase:** 6 — Hardware Abstraction & Real Backends (Feature A) — follow-up
- **Based on:** `docs/artifacts/gate-phase6-feature-a-2026-06-02.md`, `task-T083.md`, `task-T087.md`

## Objective
Make the capability snapshot the Go control plane consumes carry **live** `health_state`
(and plumb `queue_depth`) for accelerator backends, instead of a hardcoded `healthy`/`0`,
without putting network I/O on the dispatch hot path.

## Design
The accelerator adapter keeps a lockless cached health snapshot refreshed two ways:

1. **Active probing (background):** `InferenceBackend::probe()` is a new trait method
   (default = the cheap `health()` for backends with no remote dependency, e.g. the CPU
   baseline). `OpenAiCompatibleBackend::probe()` performs a live `/models` readiness check
   and caches the resulting state. A background prober thread in `main.rs` calls
   `BackendRegistry::probe_all()` every `CWSO_HAL_HEALTH_PROBE_SECONDS` (default 10s).
   A startup probe seeds the cache at registration so the very first snapshot is accurate.
2. **Reactive (hot path, no extra I/O):** every `infer` updates the cache from its own
   outcome — a served request → `healthy`; a classified failure → mapped state
   (`timeout`/`overloaded` → `degraded`; `unavailable`/`internal` → `unavailable`).

`health()` and `capabilities()` read only the cached snapshot, so dispatch stays fast. The
Go `CapabilitySyncer` (T087 follow-up) then propagates the live `health_state` into the
policy engine, which already factors health into routing and skips `unavailable` providers.

### Health-state mapping (`FailureClass::to_health_state`)
| Failure | Health |
|---------|--------|
| `timeout`, `overloaded` | `degraded` |
| `unavailable`, `internal` | `unavailable` |
| `invalid_request` | `degraded` (says nothing bad about the backend) |

## Acceptance Criteria
- [x] Accelerator `health_state` reflects live probe/infer outcomes (not hardcoded).
- [x] Probing runs off the dispatch hot path; `health()` stays cheap (cached).
- [x] Background prober refreshes on a configurable interval; startup seeds the cache.
- [x] `cargo fmt --check` clean; `cargo test -p cwso-hal` green.

## Tests
- `openai`: `probe_success_caches_healthy…`, `probe_unreachable_caches_unavailable…`,
  `probe_overloaded_caches_degraded`, `infer_failure_reactively_marks_unavailable`,
  `infer_success_restores_healthy`.
- `registry`: `probe_all_invokes_active_probe_on_each_backend`.
- `backend`: `failure_class_maps_to_health_state`.

## Notes / Follow-ups
- `queue_depth` is plumbed through the cache + capability record but currently stays `0`:
  the OpenAI API has no standard queue-depth endpoint. A real value needs a provider-specific
  metrics scrape (e.g. vLLM `/metrics` Prometheus counters) — tracked as future work.
