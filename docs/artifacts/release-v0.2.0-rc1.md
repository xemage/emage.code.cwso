# Artifact: release-v0.2.0-rc1

## Metadata
- Producer agent: release-manager
- Created: 2026-05-24
- Based on: docs/tasks/task-T076.md, docs/artifacts/release-v0.2.0-hardware-aware-v1.md, docs/artifacts/hardening-wasm-integrity-v1.md, docs/artifacts/hardening-telemetry-redaction-v1.md, docs/artifacts/hardening-ebpf-latency-semantics-v1.md, CHANGELOG.md

## Release intent
v0.2.0-rc1 is the release-candidate consolidation for the hardware-aware Phase 5 line after completion of security hardening follow-ups T073-T075.

## Scope included

### Delivered in prior Phase 5 baseline (T062-T072)
- Capability registry and dispatch telemetry fabric.
- Policy engine v2 with deterministic fallback behavior.
- Experimental sparse/quantized and SSM assist paths (default off).
- Optional Wasm scoring path (default off).
- Event-driven monitoring with eBPF-preferred and userspace fallback signal path.

### Hardening closure in this RC line (T073-T075)
- T073: enforced Wasm module integrity and trusted-path constraints.
- T074: enforced configurable telemetry minimization/redaction controls.
- T075: replaced fixed eBPF latency estimate with explicit advisory semantics.

## Security follow-up closure matrix

| Finding | Follow-up task | Status in rc1 | Evidence |
|---|---|---|---|
| F-071-01 Wasm runtime integrity | T073 | CLOSED | docs/artifacts/hardening-wasm-integrity-v1.md |
| F-071-02 Telemetry minimization | T074 | CLOSED | docs/artifacts/hardening-telemetry-redaction-v1.md |
| F-071-03 eBPF latency semantics | T075 | CLOSED | docs/artifacts/hardening-ebpf-latency-semantics-v1.md |

## Operator controls (rc1)

### Wasm integrity controls
- `CWSO_HHD_WASM_SCORING_MODULE_SHA256`
- `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`

### Telemetry minimization controls
- `CWSO_HHD_TELEMETRY_REDACTION_ENABLED`
- `CWSO_HHD_TELEMETRY_REQUEST_ID_MODE`
- `CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE`
- `CWSO_HHD_TELEMETRY_REDACTION_SALT`

### eBPF latency interpretation contract
- In `ebpf-hook` mode: `detection_latency_mode=advisory` and `detection_latency_is_advisory=true`.
- Consumers must treat `detection_latency_ms` as non-authoritative on advisory events.

## Validation and CI evidence
- Targeted hardening tests passed during T073-T075 workstream execution.
- Latest hardening-closure pipeline on `develop` reached all-green state:
  - Pipeline: `2548879153`
  - URL: https://gitlab.com/em-age/emage.code.cwso/-/pipelines/2548879153
- RC readiness commit pipeline reached all-green state:
  - Pipeline: `2548922954`
  - URL: https://gitlab.com/em-age/emage.code.cwso/-/pipelines/2548922954

## Publication evidence (2026-05-24)
- Tag published: `v0.2.0-rc1`
- GitLab release published: https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.2.0-rc1
- Uploaded assets:
  - `cwso-orchestrator-linux-amd64`
  - `cwso-git-shadow-linux-amd64`
  - `cwso-merge-engine-linux-amd64`
  - `cwso-orchestrator-image-v0.2.0-rc1.tar.gz`
  - `cwso-git-shadow-image-v0.2.0-rc1.tar.gz`
  - `cwso-merge-engine-image-v0.2.0-rc1.tar.gz`

## Smoke verification evidence (2026-05-24)
- Binary startup checks:
  - `./dist/v0.2.0-rc1/cwso-orchestrator-linux-amd64 --help` returned usage output.
  - `cwso-git-shadow` started successfully with temp overrides:
    - `CWSO_GIT_SHADOW_SOCKET=/tmp/cwso-smoke/runtime/git-shadow.sock`
    - `CWSO_GIT_SHADOW_STORAGE=/tmp/cwso-smoke/shadow`
    - startup log: `cwso-git-shadow ready`
  - `cwso-merge-engine` started successfully with temp override:
    - `CWSO_MERGE_ENGINE_SOCKET=/tmp/cwso-smoke/runtime/merge-engine.sock`
    - startup log: `cwso-merge-engine ready`
- Container/image checks:
  - Local images present for `cwso/orchestrator:v0.2.0-rc1`, `cwso/git-shadow:v0.2.0-rc1`, and `cwso/merge-engine:v0.2.0-rc1`.
  - `docker compose -f deploy/docker-compose.yml config` succeeded.

## Release candidate verdict
PASS (RC_READY)

Rationale:
- Phase 5 conditional security hardening follow-ups are closed and documented.
- Operator-facing controls and semantics are explicit for rollout.
- CI evidence shows a green end-to-end gate on the final hardening commit line.

## Next release action
- Run stakeholder RC validation against published release artifacts.
- Capture any RC feedback as follow-up tasks before GA.
- Promote to `v0.2.0` after release-manager final sign-off.
