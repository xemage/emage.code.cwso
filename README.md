# CWSO — Concurrent Workspace & Swarm Orchestrator

[![pipeline status](https://gitlab.com/em-age/emage.code.cwso/badges/develop/pipeline.svg)](https://gitlab.com/em-age/emage.code.cwso/-/commits/develop)
[![coverage report](https://gitlab.com/em-age/emage.code.cwso/badges/develop/coverage.svg)](https://gitlab.com/em-age/emage.code.cwso/-/commits/develop)
[![latest release](https://gitlab.com/em-age/emage.code.cwso/-/badges/release.svg)](https://gitlab.com/em-age/emage.code.cwso/-/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A deterministic Go-kernel MCP backend that orchestrates LLM sub-agent swarms in
ephemeral, in-memory Git **Shadow Workspaces** running on tiered sandboxes,
with semantic AST-based merging.

> **Repository:** https://gitlab.com/em-age/emage.code.cwso
> **Default branch:** `develop` · **Production branch:** `main` · **Branching:** [GitFlow](docs/branching.md)

---

## Status

| Phase | Scope | Milestone | State |
|-------|-------|-----------|-------|
| Initial plan | MCP core → sandbox + merge | M0–M5 | Closed |
| Updated plan | Next-Gen HAL, sparse, rollout/Polar | M6 | RC [v0.3.0-rc1](https://gitlab.com/em-age/emage.code.cwso/-/releases/v0.3.0-rc1) |
| Current state | v0.6.1 GA plus follow-on planning | v0.6.1 | Released; Phase 6+ remains planned |

Current release artifacts and planning docs live in [docs/plans/plan-cwso-nextgen-phase6plus.md](docs/plans/plan-cwso-nextgen-phase6plus.md)
and [docs/tasks/active-tasks.md](docs/tasks/active-tasks.md).

## What CWSO is

CWSO is a deterministic MCP orchestration platform for AI coding workflows.
It runs a Go-based orchestrator, uses shadow workspaces for isolated agent
edits, and merges results with explicit conflict policies and sidecar services.
At runtime, CWSO provides:

- An MCP server surface for tool calls over stdio or Streamable HTTP.
- In-memory shadow workspaces backed by libgit2 (via `cwso-git-shadow`).
- AST-powered code interrogation and merge semantics across Go, Python, Rust,
  and TypeScript.
- A merge sidecar (`cwso-merge-engine`) that enforces deterministic conflict
  classes and reason codes.
- Tiered execution routing for sandboxed workloads and optional rollout /
  Polar capture flows.

## How to use CWSO

See **[docs/user/installation-v3.md](docs/user/installation-v3.md)** for the Linux + VS Code setup guide,
or **[docs/user/installation-v2.md](docs/user/installation-v2.md)** for the comprehensive v0.4.0 reference.
For **Cursor / VS Code** MCP wiring and troubleshooting, see
**[docs/user/ide-integration-v2.md](docs/user/ide-integration-v2.md)**.

<!-- NOTE: profiles removed in v0.8.0 (C010); this block must stay identical in README.md and installation-v3.md -->
```bash
make build
docker compose -f deploy/docker-compose.yml up -d
curl -sS http://127.0.0.1:8080/healthz
python3 scripts/phase2-integration.py
```

## Architecture in one paragraph

A small **Go kernel** speaks the [Model Context Protocol](https://modelcontextprotocol.io/)
(spec `2025-03-26`) over stdio or Streamable HTTP and exposes a permission-tiered
tool surface to LLM clients. State-changing operations are dispatched to **Rust
sidecars** over framed-JSON Unix Domain Sockets: `cwso-git-shadow` owns an
in-memory libgit2 ODB plus tree-sitter AST queries; `cwso-merge-engine` (Phase 4)
performs semantic AST merges across concurrent sub-agent results. Untrusted
sub-agent code runs in **Firecracker** microVMs with snapshot CoW.

See [docs/artifacts/architecture-v1.md](docs/artifacts/architecture-v1.md) and
the [ADR index](docs/decisions/).

## Layout
```
orchestrator/         Go MCP server kernel (Phase 1+)
  cmd/                CLI entrypoints
  internal/
    mcp/              Hand-rolled MCP subset (T029 → official go-sdk)
    transport/        stdio + Streamable HTTP
    tools/            Baseline FS tools + Phase-2 shadow tools
    shadow/           UDS client for cwso-git-shadow
    server/           Top-level dispatcher
services/
  cwso-git-shadow/    Rust libgit2 sidecar (Phase 2+)
  cwso-merge-engine/  Rust AST merge sidecar (Phase 4)
schemas/              Shared JSON schemas for MCP tools
sandbox/              Sandbox runner integrations
deploy/               Dockerfiles + compose + CI
scripts/              Integration tests, dev helpers
docs/
  plans/              Mega-plan
  artifacts/          Requirements, architecture, security baseline
  decisions/          ADRs
  checkpoints/        Phase reviews + validation gate reports
  tasks/              Per-task briefs and status
```

## Quick start

Local Go/Rust toolchains are **not required** — everything builds and runs in Docker.

```bash
# build all images (orchestrator + git-shadow sidecar)
make build

# run Go + Rust test suites in containers
make test

# bring up the Phase 2 stack (orchestrator + git-shadow over a shared UDS)
docker compose -f deploy/docker-compose.yml --profile phase2 up

# end-to-end Phase 2 integration test (orchestrator → UDS → libgit2 → AST)
python3 scripts/phase2-integration.py
```

Run `make help` (or `make`) for the full target list.

## Contributing

1. Branch off `develop` using GitFlow:
   `feature/<task-id>-<slug>` · `bugfix/<task-id>-<slug>` · `agent/<role>/<task-id>`.
2. Use [Conventional Commits](https://www.conventionalcommits.org/).
3. Open a merge request targeting `develop`. CI must pass and one review approval is required.
4. Squash-and-merge on `feature/*` and `bugfix/*`. `release/*` and `hotfix/*` preserve history.

See [docs/branching.md](docs/branching.md), [.github/instructions/git-workflow.instructions.md](.github/instructions/git-workflow.instructions.md), and [.gitlab/issue_templates](.gitlab/issue_templates/).

## Release assets

Every release should include installable assets, not just source archives:

- Linux binaries: `cwso-orchestrator`, `cwso-git-shadow`, `cwso-merge-engine`
- Container image archives: orchestrator, git-shadow, merge-engine

Common release step:

```bash
make release-assets TAG=v0.1.1
```

This command builds binaries and container archives into `dist/<tag>/` and
uploads them to the matching GitLab release entry.

## Phase 5 Hardware-Aware Features (Experimental)

Phase 5 introduces optional hardware-aware dispatch capabilities. These are
off by default and should be enabled gradually in controlled environments.

### Feature-flag configuration

Core dispatch and telemetry controls:

- `CWSO_HHD_CAPABILITY_REGISTRY_ENABLED`
- `CWSO_HHD_DECISION_TELEMETRY_ENABLED`
- `CWSO_HHD_POLICY_ENGINE_V2_ENABLED`
- `CWSO_HHD_EVENT_MONITOR_ENABLED`
- `CWSO_HHD_EVENT_MONITOR_EBPF_ENABLED`

Assist-path controls (experimental):

- `CWSO_HHD_SPARSE_QUANTIZED_ASSIST_ENABLED`
- `CWSO_HHD_SSM_ASSIST_ENABLED`

Wasm scoring controls (experimental):

- `CWSO_HHD_WASM_SCORING_ENABLED`
- `CWSO_HHD_WASM_SCORING_MODULE_PATH`
- `CWSO_HHD_WASM_SCORING_MODULE_SHA256`
- `CWSO_HHD_WASM_SCORING_TRUSTED_DIR`
- `CWSO_HHD_WASM_SCORING_TIMEOUT_MS`
- `CWSO_HHD_WASM_SCORING_MEMORY_LIMIT_PAGES`
- `CWSO_HHD_WASM_SCORING_HOST_CALL_ALLOWLIST`

Telemetry minimization controls:

- `CWSO_HHD_TELEMETRY_REDACTION_ENABLED`
- `CWSO_HHD_TELEMETRY_REQUEST_ID_MODE`
- `CWSO_HHD_TELEMETRY_ANOMALY_NOTES_MODE`
- `CWSO_HHD_TELEMETRY_REDACTION_SALT`

### Compatibility and rollout notes

- Baseline mode remains available and is the default when feature flags are
  disabled.
- Invalid or out-of-threshold sequence-assist signals fall back to
  `cpu-baseline`.
- Sparse/quantized quality guardrail failures auto-disable that assist path and
  route via baseline.
- Wasm scoring plugin initialization or execution failures fail open to built-in
  policy scoring.
- eBPF monitoring path is optional and can fall back to userspace telemetry.
- eBPF anomaly latency semantics are explicit and advisory in `ebpf-hook` mode
  (`detection_latency_mode=advisory`, `detection_latency_is_advisory=true`).

### Rollback

To quickly revert to stable behavior, disable hardware-aware features and
restart the orchestrator:

```bash
export CWSO_HHD_CAPABILITY_REGISTRY_ENABLED=false
export CWSO_HHD_DECISION_TELEMETRY_ENABLED=false
export CWSO_HHD_POLICY_ENGINE_V2_ENABLED=false
export CWSO_HHD_SPARSE_QUANTIZED_ASSIST_ENABLED=false
export CWSO_HHD_SSM_ASSIST_ENABLED=false
export CWSO_HHD_WASM_SCORING_ENABLED=false
export CWSO_HHD_EVENT_MONITOR_ENABLED=false
```

For Wasm-specific operations guidance, see
[docs/artifacts/wasm-scoring-runtime-ops-v1.md](docs/artifacts/wasm-scoring-runtime-ops-v1.md).

## Documentation
- **[Installation & usage (v3)](docs/user/installation-v3.md)** — Linux + VS Code guide with MCP auth troubleshooting
- **[Installation & usage (v2)](docs/user/installation-v2.md)** — comprehensive v0.4.0 reference guide
- **[Installation & usage (v1)](docs/user/installation-v1.md)** — v0.3.0 quick reference
- **[IDE integration (v2)](docs/user/ide-integration-v2.md)** — Cursor / VS Code + CWSO MCP troubleshooting
- **[IDE integration (v1)](docs/user/ide-integration-v1.md)** — legacy reference
- [Requirements (v2, current)](docs/artifacts/requirements-v2.md) · [Requirements v1 (archived)](docs/archive/artifacts/requirements-v1.md)
- [Next-Gen blueprint](docs/artifacts/cwso-nextgen-blueprint-v1.md)
- [Rollout / Polar architecture](docs/artifacts/rollout-architecture-v1.md)
- [Polar gap analysis](docs/artifacts/polar-gap-analysis-v1.md)
- [Release v0.3.0-rc1](docs/artifacts/release-v0.3.0-rc1.md)
- [Architecture (v1, current)](docs/artifacts/architecture-v1.md)
- [Security Baseline (v2, current)](docs/artifacts/security-baseline-v2.md) · [Security Baseline v1 (archived)](docs/archive/artifacts/security-baseline-v1.md)
- [ADR Index](docs/decisions/)
- [Active Tasks](docs/tasks/active-tasks.md) · [Completed Tasks](docs/tasks/completed-tasks.md)
- [Technical debt register](TECHNICAL-DEBT.md) · [Archived PoC scorecards](docs/archive/debt/)

## Security

No secrets in repository. All untrusted code runs in Firecracker microVMs
(Phase 4). See [SECURITY.md](SECURITY.md) and
[docs/artifacts/security-baseline-v2.md](docs/artifacts/security-baseline-v2.md).

## License

MIT — see [LICENSE](LICENSE).
