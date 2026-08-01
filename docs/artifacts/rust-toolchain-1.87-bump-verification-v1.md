# Artifact: rust-toolchain-1.87-bump-verification-v1.md

- Producer agent: devops-engineer (CWSO project)
- Task: T172
- Created: 2026-08-01
- Based on:
  - `docs/tasks/task-T172.md`
  - `docs/tasks/task-T171.md` (why this task exists — the git2 RUSTSEC blocker)
  - `docs/artifacts/tech-lead-review-t171-audit-fix-v1.md`

## Summary

Bumps the Rust toolchain from 1.86 to 1.87 across every Rust Dockerfile and every Rust CI job,
confirmed this unblocks `git2 0.21.0` (the only version fixing RUSTSEC-2026-0183/0184), bumps
`git2` to that version, and removes the scoped `cargo audit --ignore` that T171 added as a
stopgap. `cargo audit` now runs with zero ignore flags and exits clean.

## Root cause recap (from T171, not re-investigated here)

`git2 0.21.0` uses the `inherent_str_constructors` stdlib feature (`str::from_utf8` etc. as
inherent methods), targeted for stabilization in Rust 1.87 (rust-lang/rust#131114). Building it
on Rust 1.86 fails with `error[E0658]: use of unstable library feature`. This project pinned
`rust:1.86`/`rust:1.86-slim` everywhere, so the only fix path was a toolchain-wide bump.

## Changes

- `deploy/Dockerfile.git-shadow:3`, `deploy/Dockerfile.merge-engine:3`,
  `deploy/Dockerfile.rollout:3`: `FROM rust:1.86-slim` → `FROM rust:1.87-slim`.
- `.gitlab-ci.yml`: `rust:lint` image `rust:1.86` → `rust:1.87`; `rust:test` image
  `rust:1.86-slim` → `rust:1.87-slim`; `rust:audit` image `rust:1.86` → `rust:1.87`, and its
  script reverted from `cargo audit --ignore RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184` back
  to plain `cargo audit` (ignore no longer needed).
- `services/cwso-git-shadow/Cargo.toml`: `git2 = { version = "0.20", ... }` → `version = "0.21"`.
- `services/Cargo.lock`: `git2` `0.20.4` → `0.21.0` (only entry changed here — all other
  dependency versions from T171 unchanged).

## Verification (all commands run for real, verbatim results below — no simulated output)

### Step 1 — confirm git2 0.21.0 builds on rust:1.87 (the actual blocker from T171)

```
docker run --rm -v $(pwd):/repo -w /repo/services rust:1.87 bash -c '
  apt-get update -qq && apt-get install -y -qq --no-install-recommends build-essential pkg-config cmake ca-certificates
  cargo update -p git2 --precise 0.21.0
  cargo build -p cwso-git-shadow
'
```

Result: **PASS**. `Compiling git2 v0.21.0` ... `Compiling cwso-git-shadow v0.1.0` ...
`Finished \`dev\` profile [unoptimized + debuginfo] target(s) in 26.30s`. No `error[E0658]` —
directly contrasts with the identical command on `rust:1.86`, which failed with that error
during T171.

### Step 2 — full workspace build + test on rust:1.87

```
docker run --rm -v $(pwd):/repo -w /repo/services rust:1.87 bash -c '
  apt-get update -qq && apt-get install -y -qq --no-install-recommends build-essential pkg-config cmake ca-certificates
  cargo build --workspace
  cargo test --workspace
'
```

Result: **PASS**, all 5 crates:
- `cwso-git-shadow`: 12/12 passed
- `cwso-merge-engine`: 26/26 passed, 1 ignored (pre-existing, unrelated to this bump)
- `cwso-hal`: 46/46 passed
- `cwso-sparse`: 27/27 passed
- `cwso-rollout`: 35/35 passed (includes the two T170 regression tests)

### Step 3 — rust:lint parity (`cargo fmt --all -- --check`)

```
rustup component add rustfmt clippy
cargo fmt --all -- --check
```

Result: **PASS**, exit code 0. No formatting drift from the toolchain bump.

### Step 4 — `cargo audit` with zero ignore flags

```
cargo install cargo-audit --locked --version 0.22.1
cargo audit
```

Verbatim result:

```
warning: 2 allowed warnings found
Crate:     fxhash       Warning: unmaintained  ID: RUSTSEC-2025-0057
Crate:     paste        Warning: unmaintained  ID: RUSTSEC-2024-0436
```

Exit code: **0**. Both remaining items are `unmaintained` warnings (not vulnerabilities) that
don't fail the gate — same two items T171 already carried, pre-existing and out of scope for
both T171 and T172. **No `--ignore` flags needed at all** — RUSTSEC-2026-0183/0184 (the git2
findings that motivated this whole task) are gone because `git2` is now genuinely fixed, not
suppressed.

### Step 5 — real Docker image builds (mirrors CI's `build:*` jobs)

```
docker build -q -t cwso-git-shadow-t172-verify   -f deploy/Dockerfile.git-shadow .
docker build -q -t cwso-merge-engine-t172-verify -f deploy/Dockerfile.merge-engine .
docker build -q -t cwso-rollout-t172-verify       -f deploy/Dockerfile.rollout .
```

Result: **PASS**, all three images built successfully (exit 0 each), using the new
`rust:1.87-slim` builder stage. Verify images removed after the check (`docker rmi`).

## Acceptance criteria (from task-T172.md)

1. **All 5 Rust crates build and test clean on the new toolchain** — PASS (Step 2).
2. **`git2 >=0.21.0` resolves and builds cleanly; `cwso-git-shadow` tests pass** — PASS (Steps 1, 2).
3. **`cargo audit` (no ignore flags) exits 0** — PASS (Step 4).
4. **CI pipeline for the MR is green end to end (build, lint, test, audit, e2e)** — local
   equivalents of build/lint/test/audit all verified above; full CI pipeline (including the
   `e2e:*` docker-compose jobs, not reproduced locally) will be confirmed on the actual MR
   pipeline before merge.

## Blocker status

None.
