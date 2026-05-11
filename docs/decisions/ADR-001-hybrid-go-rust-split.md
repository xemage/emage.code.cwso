# ADR-001: Hybrid Go + Rust language split

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect
- References: requirements-v1.md §3, architecture-v1.md §1-2

## Context
The CWSO must serve thousands of concurrent JSON-RPC streams (I/O-bound) and execute CPU-intensive AST diffing and `libgit2` ODB manipulation (CPU/memory-bound). A pure-Go stack pays CGO overhead at AST/Git boundaries; a pure-Rust stack adds friction to the highly concurrent network gateway and lacks an equally mature MCP SDK ecosystem.

## Decision
Use **Go** for the orchestration kernel (transport, router, job manager, event bus) leveraging `modelcontextprotocol/go-sdk`. Use **Rust** for two CPU-intensive sidecars: `cwso-git-shadow` (`libgit2`) and `cwso-merge-engine` (`tree-sitter` + custom diff). Sidecars run as separate processes communicating with the Go kernel over Unix domain sockets with framed JSON.

## Consequences
- (+) Best-of-both: Go's M:N goroutine scheduler for I/O; Rust's zero-cost abstractions for CPU.
- (+) Independent versioning and security boundary between kernel and CPU services.
- (−) Two toolchains in the build (mitigated by Docker dev env and Makefile).
- (−) IPC overhead — measured target < 1 ms per call; if violated, evaluate WASM modules in-process.

## Alternatives considered
- Pure Go (`go-git` + `gotreesitter`): viable but `libgit2` is more battle-tested for low-level ODB ops.
- Pure Rust (`rmcp`): MCP SDK less mature; concurrency model heavier for our workload.
