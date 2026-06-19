# Artifact: release-v0.1.1

## Metadata
- Producer agent: release-manager
- Created: 2026-05-22
- Based on: active-tasks.md, completed-tasks.md, checkpoint-020-phase4-t051-pass.md, CHANGELOG.md

## Release Intent
v0.1.1 is a release-readiness and documentation consolidation release after the
v0.1.0 security closure line. Its focus is to ensure no tracked release blockers
remain open and to improve operator onboarding with clearer CWSO usage guidance.

## Scope Included

### Blocker closure
- T054: CI gate for merge-engine unit tests required
- T055: `merge_inputs` schema/runtime contract alignment
- T056: ADR-006 conflict-detail reconciliation
- T057: e2e policy-path validation for sidecar reason mapping

### Documentation refresh
- Updated README with:
  - concise definition of CWSO platform capabilities
  - practical step-by-step usage flow (boot, auth, MCP call, validation)
- Changelog updated with v0.1.1 release notes

## Validation Summary
- Develop branch pipeline reached all-green state before release cut:
  - lint: go/rust success
  - build: orchestrator/git-shadow/merge-engine success
  - test: go/rust success
  - e2e: phase2 and phase4-swarm success

## Attached Install Assets
- Linux binaries:
  - `cwso-orchestrator-linux-amd64`
  - `cwso-git-shadow-linux-amd64`
  - `cwso-merge-engine-linux-amd64`
- Container archives:
  - `cwso-orchestrator-image-v0.1.1.tar.gz`
  - `cwso-git-shadow-image-v0.1.1.tar.gz`
  - `cwso-merge-engine-image-v0.1.1.tar.gz`

Consumers can install without source builds by downloading binaries directly or
loading archives with `docker load`.

## Security and Gate Posture
- T051 security gate remains PASS after T058-T061 remediations.
- T050 conditional-pass follow-up conditions are closed via T054-T057.
- Remaining deferred items are non-blocking optimization/debt items and are not
  release gates for v0.1.1.

## Known Non-Blocking Follow-Ups
- T025 (Merkle incremental indexer): performance optimization deferred.
- OverlayFS bind-mount approach superseded by current sidecar/sandbox architecture.

## Release Readiness Verdict
PASS: No tracked blockers remain for v0.1.1 release packaging.