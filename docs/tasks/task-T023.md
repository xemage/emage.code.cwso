# Task T023 — `gotreesitter` integration

- Phase: **2 (PoC)** · Owner: **backend-developer (Go)** · Priority: **P0**
- Depends on: T020 · Blocks: T024, T025
- Status: pending

## Objective
Embed the Tree-sitter runtime in the Go orchestrator to parse source files into ASTs across the four Phase-2 target languages: **Go, Rust, Python, TypeScript**. Provide a small, internal `ast` package that owns grammar selection, file→tree caching, and a normalized node iterator suitable for the `query_ast` tool to be built in T024.

## Inputs
- [requirements-v1.md](../artifacts/requirements-v1.md) §FR-4
- [architecture-v1.md](../artifacts/architecture-v1.md) §1, §6
- [ADR-005](../decisions/ADR-005-tree-sitter-ast-indexing.md)

## Constraints
- New package: `orchestrator/internal/ast/` (parser.go, grammars.go, iterator.go, cache.go).
- Use a pure-Go tree-sitter binding (e.g. `github.com/smacker/go-tree-sitter` or successor); avoid CGO-only deps that break Alpine builds.
- Ship grammar bindings for: Go, Rust, Python, TypeScript.
- Zero writes to disk from this package; all caches live in-process.
- Per-file parse < 5 ms p95 on a 200-LOC file (measure via micro-bench).
- All public functions documented; no panics on malformed input — return typed errors.
- POC-DEBT tags only where unavoidable; e.g. if a binding pulls CGO, document mitigation.

## Expected outputs
- `orchestrator/internal/ast/parser.go` — `Parser`, `Parse(lang, src) (*Tree, error)`
- `orchestrator/internal/ast/grammars.go` — language registry + detection by extension/path
- `orchestrator/internal/ast/iterator.go` — depth-first walker yielding normalized `Node` values (kind, name, range, parent)
- `orchestrator/internal/ast/cache.go` — file-OID-keyed LRU; respects 64 MiB ceiling
- Tests: parse fixture files for all 4 languages; verify node kinds, ranges, and cache hit/miss
- Bench: `go test -bench=BenchmarkParse -benchmem`

## Acceptance criteria
1. `Parse` returns a non-nil tree for a valid input in each of Go/Rust/Python/TypeScript.
2. Walker emits at least: `function_definition`, `type_identifier`/`identifier`, `import` (or language equivalent) for each language.
3. Parse error on malformed input returns typed error, never panics.
4. Bench: median parse < 5 ms on 200-LOC fixtures; p95 < 15 ms.
5. Cache evicts oldest entries when ceiling exceeded; verified via test.
6. `go test ./internal/ast/... -race` PASS in Docker.
7. No new direct CGO dependency in the orchestrator binary (validated by `go env CGO_ENABLED=0 && go build`).

## Blocker protocol
Same as T020.
