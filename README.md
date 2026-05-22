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
| 0 | Planning, requirements, architecture, ADRs | M0 | ✅ closed |
| 1 | MCP core (PoC) — stdio + HTTP, JWT, baseline FS tools | M1 | ✅ closed |
| 2 | Shadow Workspaces + AST (PoC) — Rust sidecar, libgit2, tree-sitter | M2 | ✅ closed (CONDITIONAL_PASS) |
| 3 | Async + concurrency — SSE, runner pool, event broker | M3 | ✅ closed |
| 4 | Sandbox tiers + semantic merge — Docker / gVisor / Firecracker | M4 | ✅ closed |
| 5 | Release v0.1.x hardening + security closure | M5 | ✅ closed |

See [docs/plans/plan-cwso-mega.md](docs/plans/plan-cwso-mega.md) for the
full task graph and [docs/tasks/active-tasks.md](docs/tasks/active-tasks.md)
for current status.

## What CWSO is

CWSO is an orchestration engine for AI coding swarms where each agent works in
its own isolated workspace and outputs are merged with deterministic policies.
At runtime, CWSO provides:

- An MCP server surface for tool calls over stdio or Streamable HTTP.
- In-memory shadow workspaces backed by libgit2 (via `cwso-git-shadow`).
- AST-powered code interrogation and merge semantics across Go, Python, Rust,
  and TypeScript.
- A merge sidecar (`cwso-merge-engine`) that enforces deterministic conflict
  classes and reason codes.
- Tiered execution routing for sandboxed workloads.

## How to use CWSO

### 1) Start the stack

```bash
make build
docker compose -f deploy/docker-compose.yml --profile phase2 --profile phase4 up -d
```

### 2) Verify service health

```bash
curl -sS http://127.0.0.1:8080/healthz
```

### 3) Acquire an auth token

CWSO enforces JWT auth for HTTP transport. Use your configured development
secret from compose (`jwt_secret`) and mint an HS256 token with:

- `iss`: `cwso`
- `aud`: `cwso-mcp`
- `role`: `orchestrator` or `worker`

### 4) Call MCP tools

Send JSON-RPC requests to `POST /mcp` with `Content-Type: application/json`
and `Authorization: Bearer <token>`.

Example (`tools/list`):

```bash
curl -sS http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Origin: http://localhost" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

### 5) Run validation suites

```bash
make test
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

## Documentation
- [Requirements](docs/artifacts/requirements-v1.md)
- [Architecture](docs/artifacts/architecture-v1.md)
- [Security Baseline](docs/artifacts/security-baseline-v1.md)
- [ADR Index](docs/decisions/)
- [Active Tasks](docs/tasks/active-tasks.md) · [Completed Tasks](docs/tasks/completed-tasks.md)
- [Phase 1 Checkpoint](docs/checkpoints/checkpoint-001-phase1.md) · [Phase 2 Checkpoint](docs/checkpoints/checkpoint-002-phase2.md) · [T027 Tech Lead Gate](docs/checkpoints/gate-T027-phase2-techlead.md)
- [PoC Debt — Phase 1](POC-DEBT-SCORECARD-phase1.md) · [PoC Debt — Phase 2](POC-DEBT-SCORECARD-phase2.md)

## Security

No secrets in repository. All untrusted code runs in Firecracker microVMs
(Phase 4). See [SECURITY.md](SECURITY.md) and
[docs/artifacts/security-baseline-v1.md](docs/artifacts/security-baseline-v1.md).

## License

MIT — see [LICENSE](LICENSE).
