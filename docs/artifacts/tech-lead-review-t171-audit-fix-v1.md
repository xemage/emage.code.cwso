# Tech Lead Review — T171 rust:audit Dependency Bump (MR !82)

**Based on:** `docs/tasks/task-T171.md`, `docs/tasks/task-T172.md`
**Reviewed:** branch `chore/T171-rust-audit-dependency-bump`, commit `349a891` (on top of `ef68050`),
target `develop` (merge-base `a25cb01`) — GitLab MR !82
**Reviewer role:** Tech Lead (read-only review — approve/reject/annotate only, no code edits made)
**Review mode:** read-only, per `AGENTS.md` Validation Gates and `.claude/rules/security-guidelines.md`

## VERDICT: PASS

The fix is scoped, honest, and independently verifiable against the real diff. It resolves the
hard-blocking `rust:audit` gate correctly and does not touch `cwso-rollout` or
`deploy/Dockerfile.rollout`, keeping it cleanly separated from MR !81. The `cargo audit --ignore
RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184` command was independently reproduced end-to-end in
a `rust:1.86` container matching CI exactly, from a read-only mount of the actual branch
checkout, and exited `0` with output matching `task-T171.md`'s claimed evidence verbatim (see
§4). Two cosmetic nits and one non-blocking recommendation are noted below (§6, §8) — none
require a fix-up before merge, but the recommendation should be tracked (ideally folded into
T172).

---

## 1. Scope discipline — CONFIRMED

`git diff develop...chore/T171-rust-audit-dependency-bump --stat`:

```
 .gitlab-ci.yml                |   7 +-
 docs/tasks/active-tasks.md    |   1 +
 docs/tasks/completed-tasks.md |   1 +
 docs/tasks/task-T171.md       | 144 ++++++++++++++++++++++++++++++++++++++++++
 docs/tasks/task-T172.md       |  80 +++++++++++++++++++++++
 services/Cargo.lock           | 144 +++++++++++++++++++++---------------------
 6 files changed, 304 insertions(+), 73 deletions(-)
```

No file under `services/cwso-rollout/` or `deploy/Dockerfile.rollout` appears in the diff
(verified via `git diff ... --name-only | grep -Ei 'rollout|dockerfile'` → no matches). No
`Cargo.toml` file changed anywhere in the diff — consistent with the claim that existing version
constraints (`memmap2 = "0.9"`, `wasmtime = "36"`, `anyhow = "1"` in the respective crate
manifests) already permitted the bumps without a manifest edit. Scope matches
`docs/tasks/task-T171.md`'s stated boundaries (lines 63–74).

## 2. git2 revert — CONFIRMED clean

`git diff develop -- services/cwso-git-shadow/Cargo.toml` returns empty output. The file still
reads (line 10):

```
git2 = { version = "0.20", default-features = false, features = ["vendored-libgit2"] }
```

byte-identical to `develop`. `services/Cargo.lock` confirms `git2` is still resolved at `0.20.4`
(unchanged). The attempted 0.21.0 bump was fully reverted — no partial/dangling artifacts from
the abandoned attempt.

## 3. `.gitlab-ci.yml` ignore — justification is specific and accurately scoped

Diff (`.gitlab-ci.yml:145-153`):

```diff
   script:
     - cargo install cargo-audit --locked --version 0.22.1
     - cd services
-    - cargo audit
+    # RUSTSEC-2026-0183/0184 (git2 <0.21.0, cwso-git-shadow): the only fix is git2 0.21.0,
+    # which requires Rust >=1.87 (uses the `inherent_str_constructors` feature, stabilized in
+    # 1.87) and this project pins rust:1.86 across every Rust Dockerfile/CI job. Ignored here
+    # pending a dedicated Rust-toolchain-bump task (T172); do not widen this ignore list without
+    # the same toolchain check. See docs/tasks/task-T171.md.
+    - cargo audit --ignore RUSTSEC-2026-0183 --ignore RUSTSEC-2026-0184
```

- Cites two, and only two, specific RUSTSEC IDs — not a blanket `--ignore` and not a `--deny`
  removal. `rust:audit` still has **no `allow_failure`** (confirmed: the only `allow_failure:
  true` in the file is on an unrelated `clippy`/`fmt` job at line 68) — the gate remains hard,
  the ignore is the only carve-out.
- Reason given (Rust 1.87 MSRV requirement for `inherent_str_constructors`, project pinned to
  1.86 across three Dockerfiles and three CI jobs) is concrete and checkable, not vague. It
  matches the same claim independently documented in `docs/tasks/task-T171.md:114-121` and
  `docs/tasks/task-T172.md:11-21`.
- Names a real, existing follow-up (`T172`, confirmed present and correctly scoped — see §6) and
  warns future editors not to widen the ignore casually ("do not widen this ignore list without
  the same toolchain check").
- Nothing broader is ignored: `RUSTSEC-2026-0186` (memmap2) is **not** in the ignore list because
  it was actually fixed by the version bump, not suppressed — correct.

## 4. Dependency bump verification (independently reproduced from `services/Cargo.lock`)

Read directly from the lockfile on the branch (not from the task doc's prose):

| Crate | develop | branch | Claimed fix | Verified |
|---|---|---|---|---|
| memmap2 | 0.9.10 | **0.9.11** | RUSTSEC-2026-0186 | Matches |
| anyhow | 1.0.102 | **1.0.104** | RUSTSEC-2026-0190 | Matches |
| wasmtime | 36.0.10 | **36.0.13** | RUSTSEC-2026-0222 | Matches |
| git2 | 0.20.4 | **0.20.4** (unchanged) | N/A — reverted, ignored instead | Matches |

The wasmtime bump's lockfile fan-out was inspected line-by-line: every other version change in
the ~144-line `Cargo.lock` diff resolves to wasmtime's own internal workspace crates
(`wasmtime-environ`, `wasmtime-internal-*`, `cranelift-*`, `pulley-interpreter`,
`pulley-macros`, `winch-codegen`, all bumped in lockstep to `0.123.13`/`36.0.13`) — the expected,
mechanical fan-out of a single `wasmtime` patch bump, not an unrelated or unexplained dependency
drift.

**Live reproduction — succeeded.** Ran `cargo audit --ignore RUSTSEC-2026-0183 --ignore
RUSTSEC-2026-0184` from `services/` inside a `rust:1.86` Docker container (matching CI's `image:
rust:1.86` exactly), mounting the actual branch checkout read-only (no repo mutation). Verbatim
result:

```
Fetching advisory database from `https://github.com/RustSec/advisory-db.git`
  Loaded 1177 security advisories (from /usr/local/cargo/advisory-db)
Scanning Cargo.lock for vulnerabilities (363 crate dependencies)
warning: 2 allowed warnings found
Crate:     fxhash    Version: 0.2.1   Warning: unmaintained  ID: RUSTSEC-2025-0057
  fxhash 0.2.1 └── fxprof-processed-profile 0.6.0 └── wasmtime 36.0.13 └── cwso-sparse 0.1.0
Crate:     paste     Version: 1.0.15  Warning: unmaintained  ID: RUSTSEC-2024-0436
  paste 1.0.15 └── parquet 53.4.1 └── cwso-rollout 0.1.0
EXIT_CODE:0
```

This matches `docs/tasks/task-T171.md:136-139`'s claimed evidence exactly: same two
`unmaintained`-only warnings (`fxhash`, `paste`), same non-fatal outcome (no `--deny` on this
invocation, so warnings don't fail the job), same exit code `0`. The task doc's central claim —
that the scoped ignore fully clears the gate — is independently confirmed, not just trusted.
Note `paste`'s dependency tree runs through `cwso-rollout 0.1.0` (parquet); this is pre-existing,
unrelated to this MR's changes, and does not fail the build either way, but is worth noting for
whoever eventually addresses `unmaintained` warnings project-wide.

## 5. Security judgment

- Scoping an ignore to two individually-named RUSTSEC IDs, with an inline comment stating the
  concrete blocking reason (verified MSRV mismatch) and a tracked follow-up task, is acceptable
  practice and is exactly the fallback T171's own brief pre-authorized (`docs/tasks/task-T171.md`
  lines 69–71, 90–91: "propose a `cargo-audit` ignore entry scoped to that specific advisory ID
  with a written justification... for Tech Lead sign-off"). It is materially different from (and
  much safer than) removing `cargo audit` from the gate, using `--ignore` with no ID, or adding a
  global `[advisories] ignore = [...]` in an `audit.toml` that would silently apply to all future
  runs without the same per-line visibility in CI output.
- Both RUSTSEC-2026-0183 and RUSTSEC-2026-0184 are `unsound`/UB-class advisories in `git2`
  (`BlameHunk` signature construction, `Remote::list()`), not memory-safety issues with a known
  public exploit path in this project's usage (`cwso-git-shadow` uses `git2` for local
  shadow-repo operations, not for parsing untrusted remote blame/signature data from an
  attacker-controlled source, based on the crate's role described in `task-T171.md`). Deferring
  is a defensible risk/effort tradeoff, not a shortcut on a live exploit path — but this review
  did not independently audit `cwso-git-shadow`'s call sites against the two advisories'
  preconditions (see condition 2, §8).
- Deferring the git2 fix to T172 rather than force-bumping the toolchain in this MR is the right
  call: a toolchain bump is a cross-cutting change (3 Dockerfiles, 3 CI job images, 5 crates to
  re-verify) with real blast radius, and forcing it into a narrowly-scoped dependency-bump MR
  would itself be a scope violation of the kind this same review is checking for elsewhere.
  `docs/tasks/task-T172.md` is concrete, correctly scoped, and has real acceptance criteria (all
  5 crates build/test on the new toolchain, git2 bumped to >=0.21.0, ignore flags removed,
  full CI green) — it is not a vague IOU.

## 6. Task hygiene — internally consistent, two minor nits

- `docs/tasks/completed-tasks.md:64` — `T171` row present, with owner, done-on date
  (`2026-08-01`), and a substantive outcome summary matching the execution notes. Correct.
- `docs/tasks/active-tasks.md` — `T171` row removed (no longer present); `T172` row added
  (`pending`, `P2`, owner `devops-engineer`, no dependency, dated `2026-08-01`). No duplicate or
  orphaned `T171`/`T172` rows found in either file.
- **Minor nit 1**: `docs/tasks/task-T171.md:3` still reads `**Status:** pending` in the brief's
  own header metadata, even though the same file's "Execution notes" section documents the task
  as finished and it has been moved to `completed-tasks.md`. Cosmetic inconsistency within the
  artifact itself — the task board is the source of truth and is correct, but the brief file's
  header should be updated to `done` for anyone reading `task-T171.md` in isolation.
- **Minor nit 2 (pre-existing, not introduced by this MR)**: `docs/tasks/active-tasks.md` on
  `develop` already carries nine rows with `Status: done` (T150, T151, T158–T164), which is a
  standing violation of `AGENTS.md`'s invariant that `active-tasks.md` must never hold a `done`
  or `cancelled` row (terminal rows belong in `completed-tasks.md`). This predates
  `chore/T171-rust-audit-dependency-bump` (confirmed via `git show develop:docs/tasks/active-tasks.md`)
  and this MR does not add to it — T171 itself was correctly archived — but it is unrelated
  technical debt worth a follow-up task since it's now visibly adjacent to correctly-handled
  bookkeeping in the same file.

## 7. Process note (carried from task-T171.md, endorsed)

`task-T171.md:140-144` flags that `rust:audit`'s advisory database is fetched fresh, unpinned, on
every CI run — meaning the gate can red at any time on advisories unrelated to a given MR's diff,
as happened twice in this task alone (anyhow, wasmtime). This review agrees this is worth a
separate conversation (pin an advisory-DB snapshot per release vs. accept periodic drift) but
correctly treats it as out of scope for T171 itself.

## 8. Recommendations (non-blocking — PASS carries no required conditions)

1. **(Resolved during this review)** The `cargo audit --ignore ...` claim was independently
   reproduced end-to-end (§4) and matches `task-T171.md` exactly. No outstanding verification gap
   remains on this point; the live MR !82 pipeline should still show `rust:audit` green as a
   matter of course, but this review found no reason to expect otherwise.
2. **(Recommended, optional, non-blocking)** No independent audit was performed of whether
   `cwso-git-shadow`'s actual call sites into `git2` trigger the preconditions of
   RUSTSEC-2026-0183 (`Remote::list()`) or RUSTSEC-2026-0184 (`BlameHunk`-derived `Signature`).
   Suggest folding a short confirmation of this into T172's execution (since that task will touch
   this exact code path when bumping `git2` to `>=0.21.0` anyway) to fully close the
   risk-acceptance rationale in §5. Not required for this MR to merge.
3. **(Recommended, optional, non-blocking)** Fix the cosmetic header-status nit in
   `docs/tasks/task-T171.md:3` (§6) and consider filing a lightweight cleanup task for the
   pre-existing `active-tasks.md` `done`-row hygiene debt (§6, nit 2) — neither is introduced by
   or blocks this MR.

---

## Summary

The MR does what it says: two clean patch bumps (memmap2, and two originally-out-of-scope but
correctly-verified bonus bumps: anyhow, wasmtime), one honest revert (git2, with a real,
reproducible MSRV blocker), and one narrowly-scoped, well-commented, individually-ID'd
`cargo-audit` ignore with a tracked follow-up task. Scope boundaries versus MR !81 are respected.
Task bookkeeping is consistent for T171/T172 specifically. The central claim — that the scoped
ignore clears the gate — was independently reproduced in a matching `rust:1.86` container, not
just trusted from the task doc. **PASS** — clear to merge. Two optional, non-blocking
recommendations are noted above for T172 / follow-up cleanup.
