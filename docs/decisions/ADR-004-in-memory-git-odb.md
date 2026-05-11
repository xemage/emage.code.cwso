# ADR-004: In-memory Git ODB via libgit2 sidecar with go-git fallback

- Status: accepted
- Date: 2026-05-10
- Decision-maker: solution-architect

## Context
Shadow Workspaces require thousands of parallel Git operations against the same repo with **zero** working-tree writes and zero filesystem lock contention. `libgit2` is the de-facto low-level Git library; `go-git` is pure-Go but less optimized for very large DAGs.

## Decision
Primary path: **`libgit2`** wrapped by the Rust `git2` crate, exposed by the `cwso-git-shadow` sidecar. Fallback: **`go-git`** in-process for environments where the Rust sidecar cannot run. The Go kernel calls the sidecar over a Unix domain socket; the fallback is selected via build tag `noshadow`.

## Consequences
- (+) Mature, fast ODB ops; bare-repo + tmpfs gives in-memory semantics.
- (+) OverlayFS mount presents a virtual working tree to sandboxes without writes.
- (−) Native dep complicates cross-compilation — confined to a single sidecar image.
- (−) Two code paths to test — fallback is exercised in CI on `noshadow` matrix.
