# Task T172 - Bump Rust toolchain to 1.87+ to unblock git2 RUSTSEC fix

- **Status:** done
- **Owner:** devops-engineer
- **Priority:** P2
- **Depends on:** —
- **Based on:** `docs/tasks/task-T171.md` (execution notes), `.gitlab-ci.yml` `rust:audit` job

## Context

T171 found that `git2 0.20.4` (a dependency of `cwso-git-shadow`) is affected by
RUSTSEC-2026-0183 and RUSTSEC-2026-0184, both fixed only in `git2 >=0.21.0`. That version
requires Rust >=1.87 (uses the `inherent_str_constructors` stdlib feature, targeted for
stabilization in Rust 1.87 per rust-lang/rust#131114) — confirmed via a real build failure
(`error[E0658]: use of unstable library feature`) on `rust:1.86`, not assumed.

This project currently pins `rust:1.86` / `rust:1.86-slim` across:
- `deploy/Dockerfile.git-shadow`, `deploy/Dockerfile.merge-engine`, `deploy/Dockerfile.rollout`
  (builder stage `FROM rust:1.86-slim`)
- `.gitlab-ci.yml` jobs `rust:lint` (`rust:1.86`), `rust:test` (`rust:1.86-slim`), `rust:audit`
  (`rust:1.86`)

Until this is bumped, `git2` stays at `0.20.4` and RUSTSEC-2026-0183/0184 are carried as a
scoped, commented `cargo audit --ignore` in `.gitlab-ci.yml`'s `rust:audit` job (see that job's
inline comment, and `docs/tasks/task-T171.md`).

## Objective

Bump the Rust toolchain used across every Rust Dockerfile and every Rust CI job from 1.86 to
1.87 or later, verify all 5 Rust crates (`cwso-git-shadow`, `cwso-merge-engine`, `cwso-hal`,
`cwso-sparse`, `cwso-rollout`) build and test cleanly on the new toolchain, then bump `git2` to
`>=0.21.0` in `cwso-git-shadow/Cargo.toml` and remove the `--ignore RUSTSEC-2026-0183 --ignore
RUSTSEC-2026-0184` flags (and their justification comment) from `.gitlab-ci.yml`'s `rust:audit`
job.

## Scope

- **In scope:** the toolchain version bump itself (Dockerfiles + CI job images), re-verifying
  all 5 Rust crates on the new toolchain, the `git2` bump once the toolchain is confirmed
  working, and removing the now-unneeded audit ignore.
- **Out of scope:** any functional/behavioral change to any service beyond what's needed to
  compile cleanly on the new toolchain. If the bump surfaces new clippy/rustfmt drift or
  breaking API changes in other dependencies, scope those as follow-up findings rather than
  fixing them inline here unless trivial.

## Inputs

- `docs/tasks/task-T171.md` (execution notes — why this task exists)
- `deploy/Dockerfile.git-shadow`, `deploy/Dockerfile.merge-engine`, `deploy/Dockerfile.rollout`
- `.gitlab-ci.yml` (`rust:lint`, `rust:test`, `rust:audit` jobs)
- `services/cwso-git-shadow/Cargo.toml` (the `git2` dependency line)

## Expected outputs

- Updated `FROM rust:1.87-slim` (or later) in all three Rust Dockerfiles.
- Updated `image: rust:1.87` / `rust:1.87-slim` in `.gitlab-ci.yml`'s `rust:lint`, `rust:test`,
  `rust:audit` jobs.
- `git2` bumped to `>=0.21.0` in `cwso-git-shadow/Cargo.toml`, with `Cargo.lock` updated.
- The `--ignore RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184` flags and their justification
  comment removed from `.gitlab-ci.yml`'s `rust:audit` job script.
- Verbatim evidence (build + test output for all 5 Rust crates, plus a clean `cargo audit` run)
  in a new artifact, e.g. `docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`.

## Acceptance criteria

1. All 5 Rust crates build and their existing test suites pass on the new toolchain.
2. `git2 >=0.21.0` resolves and builds cleanly; `cwso-git-shadow`'s test suite passes.
3. `cargo audit` (no ignore flags) exits 0 (or only carries newly-justified, individually
   documented ignores unrelated to this task).
4. CI pipeline for the MR implementing this task is green end to end (build, lint, test, audit,
   e2e stages) on the new toolchain image.

## Blocker protocol

Report blockers as: type (`technical` | `dependency` | `unclear_requirements` | `external`)
+ severity (`critical` | `major` | `minor`) + one proposed mitigation. Max 2 retries.

## Execution notes

**Outcome: done, no blockers.** Full evidence in
`docs/artifacts/rust-toolchain-1.87-bump-verification-v1.md`.

- Confirmed empirically (not just from the tracking issue) that `rust:1.87` is sufficient:
  `git2 0.21.0` builds cleanly there (`error[E0658]` from `rust:1.86` is gone).
- Bumped `FROM rust:1.86-slim` → `FROM rust:1.87-slim` in all three Rust Dockerfiles
  (`git-shadow`, `merge-engine`, `rollout`), and the matching `image:` tags in
  `.gitlab-ci.yml`'s `rust:lint`/`rust:test`/`rust:audit` jobs.
- Bumped `git2` to `0.21.0` in `cwso-git-shadow/Cargo.toml` + `Cargo.lock`.
- Removed the `--ignore RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184` flags and their
  justification comment from `.gitlab-ci.yml`'s `rust:audit` job — no longer needed.
- Verified for real (all in a `rust:1.87` container matching CI, or real `docker build`):
  full workspace build + test across all 5 crates (0 failures), `cargo fmt --all -- --check`
  (exit 0), `cargo audit` with zero ignore flags (exit 0 — only the two pre-existing
  `unmaintained` warnings remain, same as T171 left them), and real `docker build` for all
  three affected Dockerfiles.
- Did not touch any Cargo.toml/lock entries beyond `git2` — `memmap2`/`anyhow`/`wasmtime` stay
  at the versions T171 already fixed.
