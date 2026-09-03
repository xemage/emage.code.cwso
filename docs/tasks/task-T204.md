# Task T204 — Bump `wasmtime` in `cwso-sparse` to patch RUSTSEC-2026-0269

**ID:** T204
**Owner:** backend-developer
**Status:** in_review
**Priority:** P1
**Depends on:** —
**Created:** 2026-09-02
**Completed:** —
**Based on:** the `rust:audit` (`cargo audit`) CI job on MR !206 (task T202), which failed
on a HIGH-severity (CVSS 8.8) RustSec advisory against a dependency of `cwso-sparse`.

## Objective

`cargo audit`'s `rust:audit` CI job (`.gitlab-ci.yml`, stage `audit`, **not**
`allow_failure`) failed with:

```
Crate:     wasmtime
Version:   36.0.13
Title:     Filesystem sandbox escape when paths or symlinks contain trailing slashes
Date:      2026-08-20
ID:        RUSTSEC-2026-0269
URL:       https://rustsec.org/advisories/RUSTSEC-2026-0269
Severity:  8.8 (high)
Solution:  Upgrade to >=24.0.13, <25.0.0 OR >=36.0.14, <37.0.0 OR >=46.0.3, <47.0.0 OR >=47.0.4
Dependency tree:
wasmtime 36.0.13
└── cwso-sparse 0.1.0
```

Plus two lower-severity `unmaintained`-crate warnings (`fxhash` via
`fxprof-processed-profile` via `wasmtime`; `paste` via `parquet` via `cwso-rollout`) —
these are `warning`s, not `error`s, and `cargo audit`'s own exit code already treats them
as non-blocking (only the `wasmtime` finding produced `error: 1 vulnerability found!`).
Out of scope for this task — no action needed on them here.

**Confirmed not caused by T202**: T202's diff touches zero Rust files or dependency
manifests. `develop`'s own last pipeline (commit `2769e91`, 2026-08-29) ran clean.
`cargo audit` fetches the RustSec advisory-db fresh from GitHub on every run, so this
HIGH-severity advisory has evidently landed in the upstream database sometime in the last
few days — this is advisory-db drift, not a regression introduced by any commit in this
repo, and it would block *any* MR into `develop` right now, not just T202's.

**Confirmed not live attack surface for v1.0**: `deploy/docker-compose.yml` has no
`cwso-sparse` service block at all — not even behind an opt-in profile (contrast
`cwso-rollout`, which does have one) — and carries its own inline comment confirming this:
`# CWSO_SPARSE_SOCKET has no listening cwso-sparse service in this compose file`. This
matches `docs/plans/plan-cwso-v1.0-roadmap.md` §2.4's own architecture diagram, which
classes "sparse micro-agents" under "Deferred to v1.1+". So this is a real, HIGH-severity
finding in a sandbox-trust-boundary crate by category, but a dormant one for v1.0 — it is
purely a hard CI gate (GitLab's `ci_must_pass` branch protection) blocking merges right
now, not an active exploit path against anything v1.0 users can reach.

## Inputs

- `services/cwso-sparse/Cargo.toml` (the `wasmtime = "36"` dependency declaration — a
  caret/semver requirement, so this is fixable via `Cargo.lock` alone, no `Cargo.toml`
  edit needed)
- `services/Cargo.lock` (currently pins `wasmtime` to `36.0.13`)
- `deploy/docker-compose.yml` (confirms no `cwso-sparse` service; do not add one — that
  would be a scope violation and a separate, deliberate architectural decision this task
  does not make)
- `docs/LIMITATIONS.md` §4 "Not in v1.0 (deferred features)" (already has a row: `Sparse
  micro-agents | built (\`services/cwso-sparse\`)` — extend it, don't duplicate it)

## Rails (read before starting)

### You MUST
- Bump `wasmtime` to a version satisfying the advisory's fixed range that is also
  compatible with the existing `wasmtime = "36"` caret requirement in
  `cwso-sparse/Cargo.toml` — i.e. `>=36.0.14, <37.0.0`. Prefer the **latest available
  36.x patch release** (minimal-diff, lowest risk of API breakage within the same minor
  line) over jumping to a newer major line (`46.x`/`47.x`), unless you find `36.x` is no
  longer available/yanked, in which case report a blocker rather than guessing.
- This should be achievable with `cargo update -p wasmtime` (or `--precise <version>` if
  you want to pin exactly) inside `services/` — a `Cargo.lock`-only change. Do **not**
  edit `cwso-sparse/Cargo.toml`'s version requirement unless the `Cargo.lock`-only
  approach genuinely cannot satisfy the advisory (report a blocker if so).
- Re-run `cargo audit` yourself after the bump and confirm the `wasmtime`/RUSTSEC-2026-0269
  finding is gone (the two `unmaintained` warnings may still appear — that's fine, they're
  warnings, not errors, and out of scope here).
- Build and test `cwso-sparse` (and anything else touching the shared `Cargo.lock`) to
  confirm the bump doesn't break compilation: `cargo build -p cwso-sparse` and any
  existing `cwso-sparse` tests, at minimum. If the workspace-wide `Cargo.lock` bump
  touches other crates' resolved versions transitively, do a full `cargo build
  --workspace` to be safe and report anything that breaks.
- Add a note to `docs/LIMITATIONS.md`'s existing `Sparse micro-agents` row (or immediately
  below the table, your call on the cleanest placement) recording that `cwso-sparse` has
  zero running service and zero reachability in the shipped v1.0 `docker-compose.yml` —
  useful context for anyone reading a future `cargo-audit` alert on this crate, so they
  don't have to re-derive the reachability analysis this task already did.
- Add a `CHANGELOG.md` entry (`### Security (T204)`, `fix(security)` bullet style,
  matching T202's entry above it in `## Unreleased`).
- Fill in this brief's own "Execution notes" section with real command output (the exact
  `cargo audit` output before and after, the exact version bumped to, build/test results).

### You MUST NOT
- Add a `cwso-sparse` service block to `deploy/docker-compose.yml`, or otherwise change
  its reachability — that is out of scope; this task only patches the vulnerable
  dependency version, it does not decide whether/when sparse ships live.
- Touch `cwso-sparse/src/**` application code — this is a dependency-version-only fix.
- Touch `services/cwso-git-shadow/**` (the unrelated `rust:lint` formatting drift found in
  the same pipeline run is a separate, `allow_failure: true`, non-blocking pre-existing
  issue — not this task's concern).
- Touch anything under `orchestrator/` (T202's territory) or `docs/DEBT-REGISTER.md`.

## File ownership

- **May create/modify:** `services/Cargo.lock`, `docs/LIMITATIONS.md` (the Sparse
  micro-agents row/note only), `CHANGELOG.md`, `docs/tasks/task-T204.md` (this file's own
  execution notes)
- **Must NOT touch:** `cwso-sparse/Cargo.toml` (unless the blocker case above applies),
  `deploy/docker-compose.yml`, `services/cwso-git-shadow/**`, `orchestrator/**`,
  `docs/DEBT-REGISTER.md`

## Steps (execute in order)

1. `cd services && cargo update -p wasmtime` (or `--precise <version>` for an exact pin);
   inspect the resulting `Cargo.lock` diff to confirm the new `wasmtime` version is
   `>=36.0.14, <37.0.0`.
2. `cargo audit` — confirm the `wasmtime` finding is gone.
3. `cargo build --workspace` (or at minimum `-p cwso-sparse` plus anything whose resolved
   version changed) — confirm no breakage.
4. Run `cwso-sparse`'s existing test suite if one exists.
5. Update `docs/LIMITATIONS.md`.
6. Update `CHANGELOG.md`.
7. Fill in this brief's Execution notes.

## Expected outputs

- `services/Cargo.lock` with `wasmtime` bumped to a patched `36.x` version
- `cargo audit` clean of the `wasmtime` finding
- `docs/LIMITATIONS.md` note on `cwso-sparse`'s non-reachability in shipped v1.0
- `CHANGELOG.md` entry
- Filled-in execution notes

## Acceptance criteria

1. `cd services && cargo audit` shows zero `error:` lines for `wasmtime` (the two
   pre-existing `unmaintained` warnings are acceptable and out of scope)
2. `cargo build --workspace` succeeds
3. `cwso-sparse/Cargo.toml`'s `wasmtime = "36"` requirement is unchanged (Cargo.lock-only
   fix), unless you hit and reported the blocker case
4. `docs/LIMITATIONS.md` accurately reflects the reachability finding

## Verification commands

```bash
cd services
cargo update -p wasmtime
cargo audit
cargo build --workspace
```

## Git rails

- Branch: `agent/backend-developer/T204` from `develop`
- Commit: `fix(security): bump wasmtime to patch RUSTSEC-2026-0269 in cwso-sparse`
- MR target: `develop`, squash and merge, delete source branch
- **Do not open the MR yourself** — the orchestrator will independently re-verify the diff
  against source first, per this roadmap's standing "no rubber-stamping" discipline.

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries. If `36.x`
has no available patched release (e.g. yanked or genuinely absent from the registry) and
you must consider a `Cargo.toml` requirement bump to `46.x`/`47.x` instead, report
`technical`/`minor` with your findings and proposed range rather than making that call
silently — it's a bigger diff than intended and worth a second look before committing to.

## Execution notes

**Approach taken:** `Cargo.lock`-only fix, exactly as the brief required. `cd services &&
cargo update -p wasmtime` resolved `wasmtime` (and its 16 `wasmtime-internal-*`/
`wasmtime-environ` workspace-internal siblings, all version-locked together upstream) from
`36.0.13` to `36.0.14` — 136 lines changed (68 insertions, 68 deletions) in
`services/Cargo.lock`. The bulk of the diff (verified via `git diff services/Cargo.lock |
grep '^[+-]name'`) is `wasmtime*` package `version =` lines, no other crate's *own* version
changed. **Correction (flagged by Tech Lead review, MR !208):** that grep method missed a
smaller, benign side effect — 4 `windows-sys` dependency-edge lines inside `errno`,
`rustix`, `tempfile`, and `winapi-util`'s existing `[[package]]` blocks flip from
`windows-sys 0.61.2` to `windows-sys 0.52.0` (their own crate versions are unchanged; only
which `windows-sys` variant they resolve against as a dependency). This is a normal
`cargo update -p wasmtime`-triggered resolver side-effect (a different `wasmtime`
version pulls a different transitive `windows-sys` requirement for those shared deps), not
a manifest edit, and not a security regression — no new crate, no version regression, no
scope creep beyond what `cargo update -p wasmtime` itself does. `36.0.14` satisfies both the
advisory's fixed range (`>=36.0.14, <37.0.0`) and `cwso-sparse/Cargo.toml`'s existing
`wasmtime = "36"` caret requirement, so no `Cargo.toml` edit was needed — the blocker case
never triggered.

**`cargo audit` before (on `develop`, unmodified `Cargo.lock`):**
```
Crate:     wasmtime
Version:   36.0.13
Title:     Filesystem sandbox escape when paths or symlinks contain trailing slashes
ID:        RUSTSEC-2026-0269
Severity:  8.8 (high)
Solution:  Upgrade to >=24.0.13, <25.0.0 OR >=36.0.14, <37.0.0 OR >=46.0.3, <47.0.0 OR >=47.0.4
Dependency tree:
wasmtime 36.0.13
└── cwso-sparse 0.1.0

error: 1 vulnerability found!
```

**`cargo audit` after (this branch, bumped `Cargo.lock`):**
```
Scanning Cargo.lock for vulnerabilities (365 crate dependencies)
Crate:     fxhash        (unmaintained warning, pre-existing, out of scope)
Crate:     paste         (unmaintained warning, pre-existing, out of scope)
warning: 2 allowed warnings found
```
Exit code `0`. The `wasmtime`/RUSTSEC-2026-0269 `error:` line is gone; only the two
pre-existing `unmaintained` warnings remain, unchanged from before this task and explicitly
out of scope per the brief.

**Build verification:**
- `cargo build -p cwso-sparse` — succeeds (`Finished \`dev\` profile ... in 54.89s`), 1
  pre-existing dead-code warning (`AgentError::Disabled`/`UnknownDomain`, unrelated to this
  change).
- `cargo test -p cwso-sparse` — **27 passed, 0 failed, 0 ignored** (`gemm`, `ipc`, `agent`,
  `slice` modules all green).
- `cargo build --workspace` — fails locally, but **not because of this change**: isolated
  by stashing the `Cargo.lock` diff and re-running `cargo build -p cwso-git-shadow` alone,
  which reproduces the identical failure (`error[E0658]: use of unstable library feature
  \`inherent_str_constructors\`` in `git2-0.21.0/src/blame.rs:212`, an unrelated crate this
  task does not touch). Root cause: local dev-machine `rustc 1.86.0` vs. whatever CI's
  `rust:test`/`build:git-shadow` jobs use — those jobs pass green in CI on both `develop`
  and T202's MR !206 pipeline, so this is a pre-existing local-toolchain-only mismatch, not
  a regression from the `wasmtime` bump. `cwso-sparse` itself (this task's actual target)
  builds and tests clean in this same local environment, which is the relevant signal for
  this task's acceptance criteria.

**Acceptance criteria — verified:**
1. `cd services && cargo audit` shows zero `error:` lines for `wasmtime` — confirmed above.
2. `cargo build --workspace` — could not be verified clean locally due to the pre-existing,
   unrelated `git2`/local-toolchain issue above; `cargo build -p cwso-sparse` (this task's
   actual dependency) verified clean instead, and CI's own `rust:test`/`build:git-shadow`
   jobs (which exercise the full workspace on a different toolchain) are the authoritative
   check for the rest of the workspace once this branch's pipeline runs.
3. `cwso-sparse/Cargo.toml`'s `wasmtime = "36"` requirement unchanged — confirmed, `git
   diff` touches `services/Cargo.lock` only.
4. `docs/LIMITATIONS.md` updated with the reachability note (below the §4 table) — done.

**Docs:** `CHANGELOG.md` `### Security (T204)` entry added (`## Unreleased`, matching
T202's entry style). `docs/LIMITATIONS.md`'s "Sparse micro-agents" reachability note added.
