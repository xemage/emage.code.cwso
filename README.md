# CWSO — Concurrent Workspace & Swarm Orchestrator

Deterministic Go-kernel MCP backend that orchestrates LLM sub-agent swarms in ephemeral, in-memory Git "Shadow Workspaces" running on tiered microVM sandboxes, with semantic AST-based merging.

## Status
- **Phase 1 (PoC)**: ✅ MVP MCP server (stdio + Streamable HTTP) with baseline tools.
- **Phase 2 (PoC)**: 🚧 Shadow Workspaces + AST queries — scaffold in place, implementation pending.
- **Phase 3 (Production)**: 🚧 Async dispatch + SSE telemetry — scaffold in place.
- **Phase 4 (Production)**: 🚧 Sandbox swarm + semantic merge — scaffold in place.

See [`docs/plans/plan-cwso-mega.md`](docs/plans/plan-cwso-mega.md) for the full roadmap.

## Layout
```
orchestrator/         Go MCP server kernel (Phase 1+)
services/
  cwso-git-shadow/    Rust libgit2 sidecar (Phase 2+)
  cwso-merge-engine/  Rust AST merge sidecar (Phase 4)
schemas/              Shared JSON schemas for MCP tools
sandbox/              Sandbox runner integrations (Docker / gVisor / Firecracker)
deploy/               Docker, compose, CI
docs/                 Plans, ADRs, requirements, checkpoints, task briefs
```

## Quick start (Docker)

```bash
make build      # build all images
make test       # run Go + Rust test suites in containers
make run        # docker compose up
make inspector  # launch mcp-inspector against the running server
make demo       # end-to-end Phase 1 demo
```

Local Go/Rust toolchains are **not required** — everything runs in Docker.

## Documentation
- [Requirements](docs/artifacts/requirements-v1.md)
- [Architecture](docs/artifacts/architecture-v1.md)
- [Security Baseline](docs/artifacts/security-baseline-v1.md)
- [ADR Index](docs/decisions/)
- [Active Tasks](docs/tasks/active-tasks.md)

## Security
No secrets in repo. See [`SECURITY.md`](SECURITY.md). All untrusted code runs in Firecracker microVMs (Phase 4).
