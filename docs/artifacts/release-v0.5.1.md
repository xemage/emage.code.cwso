# Artifact: release-v0.5.1

## Metadata
- Producer agent: technical-writer / release-manager
- Created: 2026-08-01
- Based on: `docs/artifacts/release-v0.5.0.md`, v0.5.0 baseline, T169, T170, T171, T172
- **develop tip:** `493d4de` (post-T172 merge)
- **Prior GA tag:** `v0.5.0` @ 2026-07-27

## Release intent

**v0.5.1** is a patch release that delivers:
- A `cwso-rollout` liveness healthcheck route and Docker `HEALTHCHECK` instruction
- A trajectory store path env-var resolution fix (name-drift bug)
- Security dependency updates: `memmap2`, `anyhow`, `wasmtime`, and `git2`
- Rust toolchain bump to 1.87, fully clearing all outstanding `cargo audit` findings

No new features and no breaking changes are introduced.

**Primary user documentation:** [`docs/user/installation-v2.md`](../user/installation-v2.md)

## Scope vs v0.5.0

| Item | Commits | Status |
|------|---------|--------|
| T169 — root-cause investigation (rollout healthcheck 405 + trajectory store path mismatch) | n/a (investigation artifact) | **Included (investigation, folded into T170 fix)** |
| T170 — `/healthz` liveness route + trajectory store path env var fix | f7400f3 (merged via ef603ba) | **Included** |
| T171 — memmap2/anyhow/wasmtime bumps, git2 RUSTSEC scoped-ignore pending toolchain bump | 349a891 (merged via a59780e) | **Included** |
| T172 — Rust toolchain 1.87 bump, unblocks git2 0.21.0 RUSTSEC fix | 4293149 (merged via 1d29953) | **Included** |

## Changelog — v0.5.1

**Release Date:** 2026-08-01
**Previous Version:** v0.5.0

### Bug Fixes (T170)
- **`fix(rollout)`**: Added a `GET /healthz` liveness route in `cwso-rollout`
  (`services/cwso-rollout/src/proxy.rs`), placed ahead of the existing global POST-only
  gate — a pure static `200 {"status":"ok"}` response with no upstream/provider dispatch.
  `deploy/Dockerfile.rollout` now carries a `HEALTHCHECK` instruction targeting it
  (`--interval=10s --timeout=3s --retries=5`). `/v1/models` behavior is deliberately
  unchanged (still 405 on GET / 404 on POST — no route exists there).
- **`fix(rollout)`**: `StoreConfig::from_env` (`services/cwso-rollout/src/store.rs`) now
  resolves the trajectory store path via `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first,
  falling back to the canonical `CWSO_ROLLOUT_STORE_PATH`, then `./rollout_store` — fixes
  a name-drift bug where `deploy/Dockerfile.rollout`'s own env var was never read by the
  store.
- Both fixes carry new regression tests
  (`healthz_returns_200_and_v1_models_is_unchanged`;
  `from_env_prefers_trajectory_alias_then_canonical_then_default`) and were verified with
  real `cargo build`/`cargo test` (35/35 pass) and real `docker build`/`docker run`
  (sustained container `(healthy)`, 5/5 probes, `FailingStreak:0`).
- Root cause documented in T169:
  [`docs/artifacts/root-cause-analysis-cwso-rollout-v1.md`](root-cause-analysis-cwso-rollout-v1.md).

### Security (T171)
- **`memmap2`** 0.9.10 → 0.9.11 — resolves **RUSTSEC-2026-0186** (unchecked pointer
  offset), affects `cwso-sparse`.
- **`anyhow`** 1.0.102 → 1.0.104 — resolves **RUSTSEC-2026-0190**
  (`Error::downcast_mut()`), discovered live during T171, affects all Rust crates using
  `anyhow = "1"`.
- **`wasmtime`** 36.0.10 → 36.0.13 — resolves **RUSTSEC-2026-0222**, discovered live
  during T171, affects `cwso-sparse`.
- **`git2`** RUSTSEC-2026-0183/0184 (unsound `Remote::list()` / `BlameHunk` signature UB)
  were temporarily scoped-ignored in CI pending a Rust toolchain bump (blocked on Rust
  ≥1.87 MSRV for `git2 0.21.0`) — see T172 below, which resolves this fully; no ignore
  remains on `develop` after T172 landed.

### Internal (T172)
- Rust toolchain bumped **1.86 → 1.87** across all three Rust Dockerfiles
  (`git-shadow`, `merge-engine`, `rollout`) and all three Rust CI job images
  (`rust:lint`, `rust:test`, `rust:audit`).
- **`git2`** bumped 0.20.4 → 0.21.0 in `cwso-git-shadow`, resolving RUSTSEC-2026-0183/0184
  and removing the scoped `cargo audit --ignore` flags added in T171.
- `cargo audit` now exits 0 with zero ignore flags. Full workspace build+test verified
  clean across all 5 Rust crates; `cargo fmt --check` clean.

## Feature flag matrix (v0.5.1)

No new feature flags introduced. All flags unchanged from v0.5.0:

| Flag | Default | Enables |
|------|---------|---------|
| `CWSO_ROLLOUT_KV_DIFFERENTIAL_PROMPTING_ENABLED` | `false` | KV-cache differential prompting |

## Validation and CI evidence

- **T170**: `cargo build -p cwso-rollout` clean; `cargo test -p cwso-rollout` 35/35 pass;
  real `docker build`/`docker run` — 5/5 healthy probes. Tech Lead review:
  `docs/artifacts/tech-lead-review-cwso-rollout-fix-v1.md` — VERDICT: **PASS**.
- **T171**: `cargo test -p cwso-sparse` 27/27 pass; `cargo audit --ignore
  RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184` exit 0. Tech Lead review artifact:
  `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md`.
- **T172**: full workspace build+test (5 crates, 0 failures), `cargo fmt --check` clean,
  `cargo audit` (no ignores) exit 0. Evidence artifact:
  `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`.

## Conventions

> Release notes live at `docs/artifacts/release-v0.5.1.md`, following repository precedent
> (`release-v0.3.0.md`, `release-v0.4.0.md`, `release-v0.4.1.md`, `release-v0.5.0.md`). The
> orchestrator instruction referencing `docs/releases/vX.Y.Z.md` and
> `scripts/verify-release-docs.py` does not apply — neither path exists in this repository.

## Version rationale

Patch bump `v0.5.0 → v0.5.1` because scope is limited to a bug fix (T170) and
security/internal dependency and toolchain maintenance (T171, T172) — no new features, no
breaking changes, per this repo's version-decision convention (breaking → MAJOR, new
feature → MINOR, else → PATCH).

## Migration guide

No breaking changes from v0.5.0. All changes are either transparent bug fixes
(healthchecks/store-path resolution, both backward compatible —
`CWSO_ROLLOUT_STORE_PATH` still works if `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` is unset) or
internal dependency/toolchain maintenance with no operator-facing config changes. Rust
builds now require toolchain ≥1.87 (relevant only if building from source rather than the
published Docker images).

## Latest release: v0.5.1

## Install

See [`docs/user/installation-v2.md`](../user/installation-v2.md) for the full installation
guide including Docker quick start, JWT setup, MCP configuration, and rollout/gateway
workflows.

```bash
docker compose -f deploy/docker-compose.yml up
```

## Highlights

- `cwso-rollout` `/healthz` liveness route + Docker `HEALTHCHECK`
- Trajectory store path env-var fix (`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` resolution)
- Security: RUSTSEC-2026-0186/0190/0222/0183/0184 all resolved
- Rust toolchain bumped to 1.87
