# Artifact: release-v0.4.0 (GA)

## Metadata
- Producer agent: release-manager
- Created: 2026-06-09
- Based on: `release-v0.3.0.md`, `polar-gap-analysis-v1.md`, T144–T149/T153–T156, checkpoint-013
- **develop tip:** `adf34d2` (post-!66 board sync)
- **Prior GA tag:** `v0.3.0` @ `de071c0`

## Release intent

**v0.4.0** is the general-availability release for **Polar parity** and operator readiness on top of
the Next-Gen (Phases 6–9) baseline. It delivers harness-native execution, gateway staging,
evaluators, full trajectory builder parity, and a comprehensive adoption guide.

**Primary user documentation:** [`docs/user/installation-v2.md`](../user/installation-v2.md)

## Scope vs v0.3.0

| Item | Task | MR | GA status |
|------|------|-----|-----------|
| Polar harness adapters + Docker runtime | T144 | !56 (v0.3.0) | **Included** |
| Tag pipeline deploy fix (`needs:optional` e2e) | T153 | !58 | **Included** |
| Rollout `num_samples` session fan-out | T145 | !59 | **Included** |
| IDE integration guide (VS Code / Cursor) | T154 | !60 | **Included** |
| Enable-all-features script | T155 | !60 | **Included** |
| Gateway async staging + partial traces | T146 | !61 | **Included** |
| Evaluator registry + SWE-bench hook | T148 | !62 | **Included** |
| Comprehensive installation guide v2 | T156 | !64 | **Included** |
| Trajectory builder Polar parity (v2) | T149 | !65 | **Included** |
| Board sync + trajectory builder in install guide | — | !66 | **Included** |
| KV differential prompting | T150 | — | Deferred post-GA |
| Offline SFT data generation | T151 | — | Deferred post-GA |

## Polar parity summary

| Polar capability | v0.3.0 | v0.4.0 |
|------------------|--------|--------|
| Model API proxy + token capture | Done | Done |
| OpenAI Responses API route | Done (T147) | Done |
| Harness adapters + runtime launcher | Missing | **Done** (T144) |
| `num_samples` task fan-out | Missing | **Done** (T145) |
| Gateway INIT/RUNNING/POSTRUN pools | Missing | **Done** (T146) |
| Evaluator prewarm + registry | Missing | **Done** (T146/T148) |
| Partial trace recovery on timeout | Missing | **Done** (T146) |
| Per-request trajectory builder | Missing | **Done** (T149) |
| Message-level prefix merge + EOT interstitials | Partial | **Done** (T149) |
| KV differential prompting | Missing | Deferred (T150) |
| Offline SFT data generation | Missing | Deferred (T151) |
| Embedded GRPO / gRPC / ClickHouse | Out of scope | Out of scope |

Gap analysis baseline: `docs/artifacts/polar-gap-analysis-v1.md`.

## Feature flag matrix (v0.4.0 rollout surface)

All flags default **off** unless noted. See `installation-v2.md` §3 for orchestrator and sidecar flags.

| Flag | Default | Enables |
|------|---------|---------|
| `CWSO_ROLLOUT_API_ENABLED` | `false` | `/rollout/*` REST API |
| `CWSO_ROLLOUT_REWARD_ENABLED` | `false` | Merge SM programmatic rewards |
| `CWSO_ROLLOUT_KV_PREFIX_ROUTER_ENABLED` | `false` | BLAKE3 prefix router on submit |
| `CWSO_ROLLOUT_PROXY_ENABLED` | `false` | Sidecar HTTP proxy (Rust) |
| `CWSO_ROLLOUT_TRAJECTORY_BUILDER_ENABLED` | `false` | Polar trajectory builder v2 at drain |
| `CWSO_ROLLOUT_TRAJECTORY_BUILDER_STRATEGY` | `prefix_merge` | Default builder: `prefix_merge` or `per_request` |
| `CWSO_ROLLOUT_GATEWAY_STAGING_ENABLED` | `false` | INIT→READY→RUNNING→POSTRUN pools |
| `CWSO_ROLLOUT_GATEWAY_INIT_WORKERS` | `2` | INIT pool size |
| `CWSO_ROLLOUT_GATEWAY_READY_BUFFER` | `4` | READY queue depth |
| `CWSO_ROLLOUT_GATEWAY_RUNNING_WORKERS` | `4` | RUNNING pool size |
| `CWSO_ROLLOUT_GATEWAY_POSTRUN_WORKERS` | `2` | POSTRUN pool size |
| `CWSO_ROLLOUT_GATEWAY_SESSION_TIMEOUT_SECONDS` | `300` | RUNNING timeout; partial trace on expiry |
| `CWSO_ROLLOUT_EVALUATOR_PREWARM_ENABLED` | `false` | Prewarm evaluators during RUNNING |
| `CWSO_ROLLOUT_EVALUATOR_REGISTRY_ENABLED` | `false` | Post-run evaluator plugins |
| `CWSO_ROLLOUT_EVALUATOR_SESSION_REWARD_ENABLED` | `false` | Merge SM reward plugin |
| `CWSO_ROLLOUT_EVALUATOR_SWEBENCH_ENABLED` | `false` | SWE-bench stub hook |

Submit-time override: `trajectory_builder_strategy` on `/rollout/task/submit` (see T149).

## Validation and CI evidence

- Phases 6–9 gates: **PASS/PASS** (unchanged from v0.3.0)
- `develop` CI green @ `adf34d2` (MR !66 pipeline `#2588782604`)
- Primary operator doc: `docs/user/installation-v2.md`
- Tag pipeline fix validated via T153 (`deploy:registry` → `needs:optional` on `e2e:phase2`)

## GA verdict

**CONDITIONAL_PASS (GA_READY — packaging complete; tag pending approval)**

Rationale:
- All v0.4.0 scope tasks merged; dependencies T149 + T156 satisfied.
- Release artifact, CHANGELOG, and checkpoint prepared on `feature/T157-v0.4.0-release`.
- Tag `v0.4.0` and GitLab release publication require stakeholder approval post-MR merge.

### Conditions
- Merge T157 MR; verify tag pipeline green on `v0.4.0` (T153 fix in place).
- T150/T151 remain post-GA backlog items.

## Release actions

1. Merge T157 MR after CI green.
2. Tag **`v0.4.0`** on `develop` (manual / approved step).
3. Publish GitLab release with CHANGELOG excerpt; link `installation-v2.md` as primary doc.
