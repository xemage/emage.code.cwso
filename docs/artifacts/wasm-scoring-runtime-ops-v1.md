# Wasm Scoring Runtime Ops v1

> Owner: backend-developer · Based on: [docs/tasks/task-T066.md](../tasks/task-T066.md), [docs/artifacts/architecture-phase5-hhd-v1.md](architecture-phase5-hhd-v1.md), [docs/artifacts/security-baseline-v1.md](security-baseline-v1.md)

## Scope
This note defines operator controls for the optional Wasm dispatch scoring plugin integrated into policy engine v2.

## Module loading controls
- `CWSO_HHD_WASM_SCORING_ENABLED`: default `false`; when `false`, baseline policy scoring is used.
- `CWSO_HHD_WASM_SCORING_MODULE_PATH`: required only when enabled; absolute or container-visible path to the Wasm module.
- `CWSO_HHD_WASM_SCORING_TIMEOUT_MS`: per-call timeout budget for `adjust_score` execution.
- `CWSO_HHD_WASM_SCORING_MEMORY_LIMIT_PAGES`: hard runtime memory ceiling in WebAssembly pages.
- `CWSO_HHD_WASM_SCORING_HOST_CALL_ALLOWLIST`: CSV allowlist for host calls; empty means deny all host calls.

## Safety and rollback constraints
- Deny-by-default host capabilities: only allowlisted host calls are exposed; unknown entries fail plugin initialization.
- Resource guardrails: runtime uses explicit memory page limits and per-call timeout deadlines.
- Fail-open behavior: if module load or execution fails, dispatch selection falls back to built-in policy scoring and continues safely.
- Rollback path: set `CWSO_HHD_WASM_SCORING_ENABLED=false` and restart orchestrator.

## Operational notes
- The expected plugin export is `adjust_score(provider_hash i64, current_score_milli i64) -> i64`.
- Returned score is clamped by orchestrator to `[0, 1]` before ranking.
- Plugin usage is intentionally narrow to scoring adjustment only; no merge/state mutation hooks are exposed in this integration.
