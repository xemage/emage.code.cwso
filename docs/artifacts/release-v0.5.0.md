# Artifact: release-v0.5.0

## Metadata
- Producer agent: technical-writer / release-manager
- Created: 2026-07-27
- Based on: `docs/artifacts/release-v0.4.1.md`, v0.4.1 baseline, T164, T235.1
- **develop tip:** `f7d8841` (post-T164 merge)
- **Prior GA tag:** `v0.4.1` @ 2026-06-19

## Release intent

**v0.5.0** is a Phase 3.1 and transport-hardening release that delivers:
- Executor node registry with deterministic round-robin task assignment (Phase 3.1)
- SSE and MCP transport reliability fixes
- Go toolchain and Rust dependency security updates
- Jobs manager close-path reliability fix
- Production / integration branch sync complete

**Primary user documentation:** [`docs/user/installation-v2.md`](../user/installation-v2.md)

## Scope vs v0.4.1

| Item | Commits | Status |
|------|---------|--------|
| Phase 3.1 task assignment (executor node registry) | ad97aed, f002ee2 | **Included** |
| Transport SSE: disable WriteTimeout on long-lived streams | 0bf71da | **Included** |
| MCP rate limiting refinement + HTTP 429 docs | 4561c27 | **Included** |
| Jobs manager close-path fix (queue drain before cancel) | c62ca24 (via MR !74) | **Included** |
| Go 1.25.12 toolchain (GO-2026-5856) | c62ca24 (via MR !74) | **Included** |
| crossbeam-epoch 0.9.20 (RUSTSEC-2026-0204) | c62ca24 (via MR !74) | **Included** |
| Deterministic round-robin node ordering | f002ee2 | **Included** |
| main branch sync into develop | c62ca24 | **Included** |

## Changelog — v0.5.0

**Release Date:** 2026-07-27
**Previous Version:** v0.4.1

### Features
- **Phase 3.1 task assignment** (T235.1): Executor node registry (`NodeRegistry`) with
  round-robin task distribution. Nodes register via `RegisterNode`, tasks are assigned via
  `AssignTask`, and load is spread deterministically across available executors.

### Fixes
- **`fix(transport)`**: Disabled `WriteTimeout` on SSE connections so long-lived event
  streams are not severed mid-session by the Go HTTP server timeout.
- **`fix(mcp)`**: Rate-limiting burst raised to 10 with localhost exemption; HTTP 429
  behaviour documented in the MCP handler.
- **`fix(jobs)`**: `Manager.Close()` now drains the queued-job channel before cancelling
  the root context, ensuring queued jobs reach `StateCancelled` instead of racing to
  `StateCompleted` or `StateFailed`.
- **`fix(rollout)`**: `getActiveNodesLocked()` sorts active nodes by `NodeID` before
  round-robin indexing, eliminating non-deterministic map iteration that caused all tasks
  to land on a single node.
- **`fix(rollout)`**: Restored CI tests for typed-nil rollout client and proxy config.

### Security

- **Go toolchain raised to 1.25.12** (`orchestrator/go.mod`, all three CI job images in
  `.gitlab-ci.yml`). Remediates **GO-2026-5856**: Encrypted Client Hello (ECH) privacy
  leak in `crypto/tls` — server could expose the SNI in plaintext despite ECH negotiation.
- **`crossbeam-epoch` pinned to 0.9.20** in `services/cwso-sparse/Cargo.toml`. Remediates
  **RUSTSEC-2026-0204**: invalid pointer dereference in `fmt::Pointer` for `Atomic`/`Shared`
  types. Transitive path: `wasmtime → rayon-core → crossbeam-deque → crossbeam-epoch`.

### Operations
- `main` branch integrated into `develop` (MR !74). Production and integration lines are
  back in sync; the v0.4.1 hardening commits are now present in both branches.
- CI `go:audit` and `rust:audit` remain blocking gates (T140 precedent).

## Feature flag matrix (v0.5.0)

No new feature flags introduced. All flags unchanged from v0.4.1:

| Flag | Default | Enables |
|------|---------|---------|
| `CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED` | `false` | KV-cache differential prompting |

## Validation and CI evidence

- **`go test -race -count=20 ./internal/rollout/`**: 20/20 PASS (T164 verification)
- **`go test -race ./...`**: 17/17 packages PASS (full orchestrator suite)
- **MR !75 pipeline** (#2708554626): 11/11 jobs success (lint, build, test, audit, e2e)
- **develop pipeline** (#2708603492): success — `go:test` green post-T164 merge
- **MR !74 pipeline** (#2708218027): 11/11 jobs success (main integration)
- **Security audits**: `go:audit` and `rust:audit` both pass on develop

## Conventions

> Release notes live at `docs/artifacts/release-v0.5.0.md`, following repository precedent
> (`release-v0.3.0.md`, `release-v0.4.0.md`, `release-v0.4.1.md`). The orchestrator
> instruction referencing `docs/releases/vX.Y.Z.md` and `scripts/verify-release-docs.py`
> does not apply — neither path exists in this repository.

## Version rationale

Minor bump `v0.4.1 → v0.5.0` because Phase 3.1 task assignment is a new feature (`feat`
scope). No code-level version constant requires updating: `services/Cargo.toml` is
unmanaged at `0.1.0` and the Go module declares no version constant. The version change is
documentation-only.

## Migration guide

No breaking changes from v0.4.1. All new Phase 3.1 infrastructure is additive.

The `NodeRegistry` is an internal orchestrator component — no operator configuration
changes are required. All existing environment flags, API endpoints, and Docker Compose
profiles remain unchanged.

## Latest release: v0.5.0

## Install

See [`docs/user/installation-v2.md`](../user/installation-v2.md) for the full installation
guide including Docker quick start, JWT setup, MCP configuration, and rollout/gateway
workflows.

```bash
docker compose -f deploy/docker-compose.yml up
```

## Highlights

- Phase 3.1 executor node registry with round-robin task assignment
- SSE WriteTimeout disabled — long-lived streams no longer severed
- Security: Go 1.25.12 (GO-2026-5856) + crossbeam-epoch 0.9.20 (RUSTSEC-2026-0204)
- Jobs manager close-path fix: queued jobs now reliably cancelled on shutdown
- Deterministic round-robin: consistent load distribution across executor nodes
