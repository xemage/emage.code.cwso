# ADR-005: Tree-sitter (gotreesitter) for AST queries with Merkle incremental indexing

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect

## Context
Regex-based search is fragile for multi-line signatures, decorators, and generics. We need cross-language AST queries that scale to 1000+ files with sub-50 ms p95 query latency.

## Decision
Use the **`gotreesitter`** pure-Go runtime with embedded compressed grammars (Phase 2: Go/Rust/Py/TS; Phase 4: ≥10 languages). All files are indexed via a **Merkle-hashed** file→tree map; only mutated files are re-parsed. Queries are normalized through a **Unified Symbol Protocol** abstracting language-specific node types.

## Consequences
- (+) No CGO friction; sub-millisecond per-file parses.
- (+) Cross-language queries with consistent semantics.
- (−) Grammar updates require rebuild — packaged as embedded blobs.
- (−) Unified Symbol Protocol must be maintained per language — owned by AST module with per-grammar test corpus.
