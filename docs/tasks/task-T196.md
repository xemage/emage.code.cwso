# Task T196 — Bump `h2` to clear RUSTSEC-2026-0258 (`rust:audit` CI drift)

**ID:** T196
**Owner:** devops-engineer
**Status:** done
**Priority:** P0
**Depends on:** —
**Created:** 2026-08-19
**Completed:** 2026-08-19
**Based on:** Discovered while landing MR !128 (T195) — `rust:audit` failed on a
pipeline whose diff (a one-line CHANGELOG text correction) cannot possibly touch
Rust dependencies. Same pattern as T190 (Go toolchain / `govulncheck` drift,
2026-08-16): a new advisory was published against an already-pinned dependency
version between an earlier successful pipeline run and this one, on the same
`develop` commit.

## Objective

`cargo audit` now fails the `rust:audit` CI job with:

```
Crate:     h2
Version:   0.4.14
Title:     <see the live job trace / rustsec.org/advisories/RUSTSEC-2026-0258 for
           the exact vulnerability description>
ID:        RUSTSEC-2026-0258
URL:       https://rustsec.org/advisories/RUSTSEC-2026-0258
Solution:  Upgrade to >=0.4.16
Dependency tree:
h2 0.4.14
└── hyper 1.10.1
    ├── hyper-util 0.1.20
    │   └── cwso-rollout 0.1.0
    └── cwso-rollout 0.1.0
error: 1 vulnerability found!
```

This blocks **every** MR pipeline that reaches the `rust:audit` stage, regardless of
diff content — confirmed independently: the failure reproduces on `develop`'s own
tip (`154cc20`) via an unrelated MR (!128, T195) whose diff is a single-file
CHANGELOG text correction with zero Rust code or dependency changes, and an earlier
pipeline run against that exact same commit (`154cc20`, pipeline `2764928021`,
2026-08-17) succeeded — the only thing that changed between the two runs is time
(a new RUSTSEC advisory was published in the interval). This is CI/dependency drift,
not a regression from any pending MR's content.

**`h2` is a transitive dependency**, not a direct one — `services/cwso-rollout/Cargo.toml`
declares `hyper = { version = "1", ... }` and `hyper-util = { version = "0.1", ... }`
(both semver-range constraints, not exact pins); `services/Cargo.lock` currently
resolves `h2` to `0.4.14`. A lockfile-only bump (`cargo update -p h2`, or a broader
`cargo update` if that alone doesn't pull a fixed version through the existing
`hyper`/`hyper-util` constraints) should clear the advisory without any `Cargo.toml`
changes — verify this is actually sufficient before considering a `Cargo.toml`
version-range change, per the "you must not" rail below.

The job output also lists 2 **allowed warnings** (`RUSTSEC-2025-0057` for `fxhash`,
unmaintained; `RUSTSEC-2024-0436` for `paste`, unmaintained) — these do not fail the
job today (the CI config already tolerates warnings, only "vulnerability found"
errors fail it) and are explicitly **out of scope** for this task; do not touch them
unless fixing `h2` incidentally resolves one, and if so, just note it, don't go
hunting for unrelated upgrades.

## Inputs

- `services/Cargo.lock` (`h2` currently resolves to `0.4.14`)
- `services/cwso-rollout/Cargo.toml` (`hyper`/`hyper-util` version constraints)
- `.gitlab-ci.yml` `rust:audit` job definition (to confirm exactly what command/flags
  run, and to confirm the 2 unmaintained-crate warnings are genuinely non-blocking
  today, not something this task needs to also address)
- Live job trace for the exact RUSTSEC-2026-0258 description (fetch it yourself —
  `rustsec.org/advisories/RUSTSEC-2026-0258` — do not guess at what the vulnerability
  actually is; cite it accurately in your commit/MR)

## Rails (read before starting)

### You MUST
- Bump `h2` to a version that clears RUSTSEC-2026-0258 (the advisory states
  `>=0.4.16`; confirm that's still accurate — a newer patched version may exist by
  the time you run this — and use the latest patch that satisfies the existing
  `hyper`/`hyper-util` semver constraints)
- Prefer a lockfile-only fix (`cargo update -p h2`, or `cargo update` scoped as
  narrowly as achieves the fix) — do not widen `Cargo.toml` version constraints
  unless a lockfile-only update genuinely cannot resolve a clean version (if so,
  explain why in the MR before doing so)
- Run `cargo audit` locally (matching the CI job's actual invocation) after the bump
  and confirm zero vulnerabilities remain (warnings for the 2 already-known
  unmaintained crates are fine to still see — those aren't in scope)
- Run the full Rust test suite (`cargo test --release` for the affected workspace
  members, matching whatever `.gitlab-ci.yml`'s `rust:test` job actually runs) and
  confirm nothing regresses from the dependency bump
- Add a CHANGELOG `## Unreleased` entry

### You MUST NOT
- Add `cargo audit --ignore RUSTSEC-2026-0258` or any other suppression flag — the
  fix is the version bump, not suppression (T171's precedent: ignore-flags are only
  acceptable when a real fix is genuinely blocked by an MSRV/toolchain constraint,
  which does not appear to be the case here — confirm this before considering it)
- Touch the 2 unrelated "allowed warning" advisories (`fxhash`, `paste`) — out of
  scope, do not go hunting for unrelated upgrades
- Change application logic in `cwso-rollout` or any other service beyond what the
  dependency bump itself requires
- Touch `orchestrator/*`, `sandbox/**`, `deploy/docker-compose.yml`, or any files
  belonging to in-flight tasks C015/T195

## File ownership

- **May create/modify:** `services/Cargo.lock`, `services/cwso-rollout/Cargo.toml`
  (only if a lockfile-only fix genuinely isn't sufficient — justify explicitly),
  `CHANGELOG.md` (Unreleased)
- **Must NOT touch:** anything else

## Acceptance criteria

1. `cargo audit` (matching CI's actual invocation) reports zero vulnerabilities
2. The 2 pre-existing "allowed warning" advisories are untouched (still present or
   absent exactly as they were before this task — not a regression either way)
3. Full Rust test suite passes with no regressions from the bump
4. `git diff --stat` touches only the files listed under "File ownership"

## Verification commands

```bash
cd services
cargo update -p h2
cargo audit
cargo test --release -p cwso-git-shadow
cargo test --release -p cwso-merge-engine
cargo test --release -p cwso-hal
cargo test --release -p cwso-sparse
cargo test --release -p cwso-rollout
```

(Adjust the exact `cargo audit` invocation to match `.gitlab-ci.yml`'s `rust:audit`
job if it uses different flags than a bare `cargo audit`.)

## Git rails

- Branch: `agent/devops-engineer/T196` from `develop`
- Commit: `chore(deps): bump h2 to clear RUSTSEC-2026-0258`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If a lockfile-only update cannot resolve a clean `h2` version under the existing
`hyper`/`hyper-util` constraints (e.g. a genuine MSRV or breaking-change conflict),
report `technical` / `major` with what you found rather than reaching for a
suppression flag.

## Execution notes

Bumped `h2` `0.4.14` → `0.4.16` via `cargo update -p h2` (lockfile-only, no
`Cargo.toml` change — `hyper = "1"` / `hyper-util = "0.1"` in
`services/cwso-rollout/Cargo.toml` were already permissive enough). Confirmed
RUSTSEC-2026-0258 cleared (`cargo audit`: 0 vulnerabilities), the 2 pre-existing
"allowed warning" advisories (`fxhash`, `paste`) left exactly as they were, and the
full Rust workspace test suite (`cwso-git-shadow`, `cwso-merge-engine`, `cwso-hal`,
`cwso-sparse`, `cwso-rollout` — 146 tests) passes with 0 regressions.

Independent Tech Lead review (MR !130) returned **PASS, no conditions**:
independently reproduced "0 vulnerabilities" locally with the exact pinned
`cargo-audit` version, confirmed lockfile-only scope and no suppression flags added,
confirmed the pre-existing warnings were unaffected, and confirmed an incidental
`windows-sys`-family lockfile reshuffle (a normal `cargo update -p` side effect) is
inert — all versions already present pre-change, Windows-only conditional
dependencies, no effect on this project's Linux-only CI/build target.

Merged to `develop` 2026-08-19 (squash), MR !130 — unblocked two independently-in-flight
MRs that had hit the same content-independent drift: T195 (MR !128) and C015 (MR
!129), both of which needed a second `develop` merge to pick this fix up before their
own pipelines could go green.
