# Checkpoint 022 — phase5 rc1 ready

## Phase summary
Phase 5 hardware-aware work is now consolidated into a release-candidate state. Baseline Phase 5 delivery (T062-T072) and all security hardening follow-ups (T073-T075) are complete, with explicit operator documentation and green CI evidence on the final hardening line.

## Completed tasks (this phase)
| ID | Title | Owner | Outcome |
|----|-------|-------|---------|
| T073 | Wasm module integrity verification hardening | backend-developer | done |
| T074 | Telemetry minimization/redaction policy | backend-developer | done |
| T075 | eBPF latency semantics hardening | backend-developer | done |
| T076 | Release candidate v0.2.0-rc1 readiness gate | release-manager | done |

## Open / carried over
| ID | Title | Owner | Status | Notes |
|----|-------|-------|--------|-------|
| T025 | Merkle-hash incremental indexer | backend-developer | deferred | Post-v0.1.x optimization; not release-blocking for v0.2.0-rc1 |

## Key decisions
- Proceed with release-candidate packaging for v0.2.0-rc1 from current develop head.
- Keep hardware-aware paths feature-flagged by default for controlled rollout.

## Artifacts produced
- `docs/artifacts/hardening-wasm-integrity-v1.md`
- `docs/artifacts/hardening-telemetry-redaction-v1.md`
- `docs/artifacts/hardening-ebpf-latency-semantics-v1.md`
- `docs/artifacts/release-v0.2.0-rc1.md`

## Blockers (active)
| ID | Type | Severity | Owner | Reported | Status |
|----|------|----------|-------|----------|--------|
| none | none | none | none | n/a | closed |

## Token usage
| Phase | Budget | Spent | % |
|-------|--------|-------|---|
| QA / Security / Release | 60k | n/a | n/a |

## Next steps
- Phase: Release candidate packaging and promotion decision.
- Actions:
  - create tag `v0.2.0-rc1`
  - run `make release-assets TAG=v0.2.0-rc1`
  - publish GitLab release notes using `docs/artifacts/release-v0.2.0-rc1.md`
- Inputs to delegate forward: `docs/artifacts/release-v0.2.0-rc1.md`, this checkpoint.

## Compression note
This checkpoint is the canonical handoff for release-candidate packaging and promotion.
