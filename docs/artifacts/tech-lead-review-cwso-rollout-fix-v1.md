# Artifact: tech-lead-review-cwso-rollout-fix-v1.md

- Producer agent: tech-lead (CWSO project)
- Task: pre-MR code review of uncommitted working-tree changes on
  `chore/T168-backmerge-main-to-develop`, ahead of cutting a `bugfix/*` branch for T169/T170.
- Created: 2026-08-01
- Based on:
  - `docs/artifacts/fix-verification-cwso-rollout-v1.md`
  - `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md`
  - `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md`
  - `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md`
  - `docs/tasks/task-T169.md`, `docs/tasks/task-T170.md`
  - Actual working-tree diff: `git diff services/cwso-rollout/src/proxy.rs services/cwso-rollout/src/store.rs deploy/Dockerfile.rollout`

Review mode: read-only. No source file was modified while producing this artifact.

---

## VERDICT: PASS

### Reviewed Artifacts
- Implementation: uncommitted working-tree diff (pre-branch) touching
  `services/cwso-rollout/src/proxy.rs`, `services/cwso-rollout/src/store.rs`,
  `deploy/Dockerfile.rollout`
- Against requirements: `docs/tasks/task-T170.md` (acceptance criteria 1–6)
- Against architecture/root cause: `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md`
  (T169, Issue 1 — NEEDS-REFINEMENT/confirmed no-liveness-route; Issue 2 — CONFIRMED env-var
  name drift)

### Decision IDs Referenced
- No formal ADR exists for this change (none was created, and none was required — this is a
  scoped defect fix within existing architecture, not a new architectural decision). Referenced
  instead: `root-cause-analysis-cwso-rollout-v1.md` Issue 1 and Issue 2 findings, and
  `plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md` Risks table rows 2–4.

### Merge Authorization
- Merge permitted: Yes
- Reviewed by: Tech Lead
- Review mode: read-only (no modifications made to source)

---

## Scope discipline (checked first, gates everything else)

`git diff --stat` (functional files only):
```
deploy/Dockerfile.rollout          |   5 ++
services/cwso-rollout/src/proxy.rs | 100 +++++++++++++++++++++++++++++++++++++
services/cwso-rollout/src/store.rs |  80 ++++++++++++++++++++++++++++-
3 files changed, 184 insertions(+), 1 deletion(-)
```
Confirmed no other file under `services/` or `deploy/` is touched. `docs/tasks/active-tasks.md`
and `docs/tasks/completed-tasks.md` also show as modified in `git status`, but their diffs are
pure task-ledger bookkeeping (one added disclaimer line in `active-tasks.md` unrelated to this
work, and two new append-only rows for T169/T170 in `completed-tasks.md`) — no functional scope
creep. **Confirmed: no scope creep.**

---

## 1. Correctness

### `/healthz` route (`services/cwso-rollout/src/proxy.rs:54-56`)
```rust
if req.method() == Method::GET && req.uri().path() == "/healthz" {
    return Ok(healthz_response());
}
```
- Placed before the global POST-only gate (`proxy.rs:58-63`), so it genuinely short-circuits
  before any provider dispatch, upstream call, or body read. `healthz_response()`
  (`proxy.rs:97-105`) is a pure, static `200 {"status":"ok"}` builder with no I/O, no upstream
  reference, no config/pipeline access — confirmed correct bypass of the entire
  `CapturePipeline`/upstream path.
- Exact string match on `req.uri().path()` (no normalization/trailing-slash tolerance). Any
  variant (`/Healthz`, `/healthz/`, `POST /healthz`) falls through to the existing POST-only
  gate and gets the pre-existing 405/404 behavior — fails closed, no new route surface exposed
  by accident.
- `req.uri().path()` does not include the query string in hyper, so `/healthz?x=y` still matches
  and returns 200 — harmless (no info leak, no functional impact), consistent with typical
  liveness-probe semantics.
- `/v1/models` is deliberately left untouched — confirmed by reading the rest of
  `handle_request` (`proxy.rs:58-94`): a `GET /v1/models` still falls into the POST-only gate
  (405), and a `POST /v1/models` still resolves to `Provider::Unknown` via
  `detect_provider()` → `CaptureError::UnsupportedProvider` → 404. This matches T169's explicit
  recommendation not to repurpose `/v1/models` and matches the verified pre/post-fix behavior in
  `fix-verification-cwso-rollout-v1.md` Step 7.
- No interference with other paths: the new branch only fires on the exact `(GET, "/healthz")`
  tuple; every other method/path combination reaches the same code it did before this diff,
  unchanged.

**Correctness verdict: sound.** The route genuinely bypasses provider dispatch safely and does
not alter any other path's semantics.

### `store.rs` env-var precedence (`services/cwso-rollout/src/store.rs:55-57`)
```rust
let store_path = std::env::var("CWSO_ROLLOUT_TRAJECTORY_STORE_PATH")
    .or_else(|_| std::env::var("CWSO_ROLLOUT_STORE_PATH"))
    .unwrap_or_else(|_| "./rollout_store".to_string());
```
- Precedence matches what both `fix-verification-cwso-rollout-v1.md` and the inline comment
  claim: `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` → `CWSO_ROLLOUT_STORE_PATH` → `"./rollout_store"`.
  Verified directly against the regression test (`store.rs:583-649`,
  `from_env_prefers_trajectory_alias_then_canonical_then_default`), which exercises exactly this
  chain with real `std::env::set_var`/`from_env()` calls (not a mocked/stubbed env layer) and
  restores prior env state afterward.
- **Edge case not handled: empty-string env values.** `std::env::var` returns `Ok("")` for a
  variable that is set but empty, not `Err`. Neither `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=""` nor
  `CWSO_ROLLOUT_STORE_PATH=""` would fall through to the next fallback — `store_path` would
  become `""`, `PathBuf::from("")`, and `fs::create_dir_all("")` at `store.rs:119-120` would
  fail. This is a real gap, but **it is not a regression introduced by this diff** — the
  pre-existing single-variable version (`std::env::var("CWSO_ROLLOUT_STORE_PATH").unwrap_or_else(...)`)
  had the identical gap. This diff only extends the existing (already-imperfect) pattern to a
  second variable name; it does not make the gap worse or introduce a new failure mode. Logged
  below as a non-blocking suggestion, not a condition.
- Whitespace-only values (e.g. `" "`) are similarly unvalidated and were similarly unvalidated
  before this change — same conclusion.

**Correctness verdict: matches the claimed behavior; one pre-existing (not newly introduced)
edge case noted as a suggestion.**

---

## 2. Security

### Unauthenticated `/healthz`
Checked this codebase's own precedent, as instructed: `cwso-orchestrator`'s own liveness route
(`orchestrator/internal/transport/http.go:146-149`):
```go
mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = io.WriteString(w, "ok")
})
```
This is registered directly on the mux, **outside** the `mw(...)` middleware chain
(`originMiddleware`, `securityHeadersMiddleware`, `rateLimitMiddleware`) and outside
`authMiddleware`, which are only applied to `/mcp` and the rollout-forwarding route
(`http.go:151,169`). `cwso-orchestrator`'s `/healthz` is therefore also unauthenticated,
unrated-limited, and returns a trivial static body. `cwso-rollout`'s new `/healthz` is
consistent with this established in-repo precedent.

Additionally, `cwso-rollout` has **no inbound authentication of any kind** on any of its
existing routes today — `upstream_api_key` (`capture.rs:44`) is only used for *outbound* calls
to the upstream LLM provider, never checked against inbound requests. So `/healthz` bypassing
"auth" does not actually bypass any auth mechanism that exists elsewhere in this service; there
was none to bypass. This is an appropriate and conventional choice for a container liveness
probe (Docker `HEALTHCHECK`/Kubernetes `livenessProbe` semantics assume unauthenticated,
low-cost checks), and the response body leaks no sensitive information (no version string, no
config, no upstream target — just a static `{"status":"ok"}`).

**No new security gap introduced.**

### Env-var-driven store path — traversal/unintended-write-location risk
- `store_path` was already fully attacker/operator-controlled via env var before this change
  (`CWSO_ROLLOUT_STORE_PATH`); this diff only adds a second env var name with identical trust
  level (both are operator/deployment-config-controlled, not end-user/HTTP-request-controlled —
  no request path or header ever flows into `store_path`). There is no new path-traversal vector:
  whoever can set `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` in the container environment already had
  equivalent control via `CWSO_ROLLOUT_STORE_PATH`, and both are set only via Dockerfile
  `ENV`/compose `environment:` — a trust boundary that existed unchanged before this diff.
- Confirmed via `root-cause-analysis-cwso-rollout-v1.md` that no HTTP-facing code path writes to
  or derives `store_path` — it is resolved once at `StoreConfig::from_env()` time, before any
  request is served.

**No new security risk introduced by the store-path change.**

---

## 3. Test coverage

### `proxy::tests::healthz_returns_200_and_v1_models_is_unchanged` (`proxy.rs:227-303`)
- Not tautological: spins up a real `hyper` server bound to an ephemeral port, running the
  actual `handle_request` function (not a mock), and issues three real HTTP requests through a
  real `hyper_util` client: `GET /healthz` (asserts `200` + body contains `"status":"ok"`),
  `GET /v1/models` (asserts `405`), `POST /v1/models` (asserts `404`).
- This directly covers task-T170.md acceptance criteria 2 (healthy liveness route) and 4
  (`/v1/models` contract unchanged, both GET and POST cases) at the integration level, not just
  unit level — stronger than the minimum required.

### `store::tests::from_env_prefers_trajectory_alias_then_canonical_then_default` (`store.rs:583-649`)
- Not tautological: exercises the real `StoreConfig::from_env()` against real process env vars
  across three real states (neither set → default; canonical only → canonical wins; both set →
  trajectory alias wins), with explicit save/restore of prior env state and a `Mutex` guard
  acknowledging `std::env` is process-global and could race with parallel test execution.
- Directly covers task-T170.md acceptance criterion 3 (writer starts cleanly at the configured
  path) at the config-resolution level; the actual Parquet write mechanism is separately covered
  by the pre-existing `store::tests::parquet_round_trip_preserves_records` and
  `store::tests::writer_thread_flushes_batches_without_blocking_proxy` (unmodified by this diff,
  confirmed still passing per `fix-verification-cwso-rollout-v1.md` Step 2, 35/35).
- **Suggestion (non-blocking):** the test does not include an isolated "trajectory-set,
  canonical-unset" case as a distinct assertion — it only asserts "both set → trajectory wins."
  Functionally this is equivalent given `Result::or_else`'s short-circuit-on-`Ok` behavior (if
  the first `env::var` call succeeds, the second is never evaluated regardless of whether the
  second variable is set), so there is no coverage gap in practice, but an explicit case would
  make the test's intent marginally more self-documenting.

**Test coverage verdict: meaningful, real (not mocked/tautological), and traceable to specific
task-T170.md acceptance criteria.** Combined with the independently-run `cargo test -p
cwso-rollout` result (35/35 passing, verbatim in `fix-verification-cwso-rollout-v1.md` Step 2)
and the real Docker build/run verification (5/5 sustained healthy probes, both via
`--health-cmd` override and the Dockerfile-native `HEALTHCHECK`), this is genuine, not simulated,
verification.

---

## 4. Scope discipline

Confirmed above (see "Scope discipline" section) — only the three intended files carry
functional changes, and the diff size/shape in each matches exactly what
`fix-verification-cwso-rollout-v1.md`'s own acceptance-criteria checklist (item 1) claims:
`proxy.rs` (+100/-0), `store.rs` (+80/-1), `Dockerfile.rollout` (+5/-0). No unrelated refactors,
no touched files outside `services/cwso-rollout/` and `deploy/Dockerfile.rollout`. **No scope
creep.**

---

## 5. Dockerfile `HEALTHCHECK`

`deploy/Dockerfile.rollout:29-32`:
```dockerfile
HEALTHCHECK --interval=10s --timeout=3s --retries=5 \
    CMD curl -f http://127.0.0.1:8787/healthz || exit 1
```
- Timing (`10s`/`3s`/`5` retries) exactly matches the pre-existing documented contract from the
  original `emage.code` compose healthcheck (`emagecode-integration-defect-cwso-rollout-unhealthy-v1.md`,
  compose excerpt) — no behavioral change to probe cadence, only to the target path.
- The shell form (`CMD curl ... || exit 1`, not the JSON exec-array form) causes Docker to run
  this via `CMD-SHELL` (confirmed in `fix-verification-cwso-rollout-v1.md`:
  `docker inspect ... .Config.Healthcheck` → `{"Test":["CMD-SHELL", ...]}`) — this is required
  because of the `||` shell operator and is the correct, intentional form here. **Minor
  suggestion (non-blocking):** `curl -f` already returns a non-zero exit code (`22`) on any
  HTTP-level failure, which Docker already interprets as an unhealthy probe on its own: the
  `|| exit 1` is redundant belt-and-suspenders, not incorrect. No action required, purely
  stylistic.
- Baking the `HEALTHCHECK` into the image (rather than leaving it to a consumer's compose file)
  is the right trade here: it makes the image's own liveness contract self-describing and
  correct by default for any consumer, rather than depending on every consumer independently
  discovering and hand-writing the right probe target. Docker/Compose `healthcheck:` blocks in a
  consumer's compose file still take precedence over an image's own `HEALTHCHECK` if a consumer
  explicitly defines one, so this does not remove any consumer's ability to override it.
- **Important non-blocking follow-up (out of this repo's scope, flagging for visibility):** the
  original defect was discovered via the *external* `emage.code` project's own
  `docker-compose-t226.yml`, which hardcodes its own `healthcheck: test: ["CMD", "curl", "-f",
  "http://127.0.0.1:8787/v1/models"]` (per
  `emagecode-integration-defect-cwso-rollout-unhealthy-v1.md`). A compose-level `healthcheck:`
  override takes precedence over this image's new Dockerfile `HEALTHCHECK`. That means, until
  the `emage.code` project's own compose file is updated to probe `/healthz` instead of
  `/v1/models`, its container will very likely continue to report `(unhealthy)` even after this
  fix is merged and released — not because this fix is wrong, but because the external
  consumer's own compose file still overrides it. This is explicitly out of scope for this
  repo's plan (`plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md`, Scope §Out of
  scope), so it does not block this MR, but it should be tracked as a coordination item (e.g. a
  note back to the `emage.code` devops-engineer who filed the original defect report) so the
  end-to-end fix doesn't get lost between repos.

**Dockerfile verdict: sound, matches contract, no concerns that block merge.**

---

## Positive notes
- Both fixes are precisely scoped to the two T169-confirmed root causes, with no speculative
  changes to unrelated code paths — exactly matching the plan's own risk mitigation for scope
  creep.
- Every non-trivial line of the diff carries an inline comment tracing it back to the specific
  `root-cause-analysis-cwso-rollout-v1.md` issue and the T169 recommendation it implements
  (including *why* the backward-compatible alias-first order was chosen over renaming the
  Dockerfile variable) — this is exemplary traceability for a bugfix diff.
- Verification in `fix-verification-cwso-rollout-v1.md` is real (actual `cargo build`/`cargo
  test`/`docker build`/`docker run`, verbatim output, not simulated or asserted-without-evidence),
  and explicitly reports one honest caveat (no live `.parquet` write was exercised in the smoke
  test) rather than overclaiming.
- Test additions exercise real server/client round trips rather than calling internal functions
  directly — meaningfully raises confidence beyond what the acceptance criteria strictly require.

## Suggestions (non-blocking, tracked for awareness only — not conditions on this PASS)
1. `services/cwso-rollout/src/store.rs:55-57` — empty-string/whitespace-only env var values
   silently produce an invalid store path (pre-existing gap, now shared by two variable names
   instead of one). Consider a follow-up hardening pass across all `store.rs` env parsing
   (`from_env`, `env_bool`, `env_u64`, `env_usize` already trim/validate more defensively than
   the path variable does) — not required for this merge.
2. `services/cwso-rollout/src/store.rs:583-649` — add an explicit "trajectory-set,
   canonical-unset" test case for self-documentation, even though current coverage is
   functionally equivalent due to `Result::or_else` short-circuiting.
3. `deploy/Dockerfile.rollout:31` — the `|| exit 1` in the `HEALTHCHECK CMD` is redundant given
   `curl -f`'s own non-zero exit behavior; harmless, no action required.
4. Cross-repo coordination: flag to whoever owns the `emage.code` relationship that its own
   `docker-compose-t226.yml` healthcheck must be updated to target `/healthz` (not `/v1/models`)
   for the original reported symptom to actually clear end-to-end in that consuming project —
   out of scope for this repo's MR, but should not be silently dropped.
