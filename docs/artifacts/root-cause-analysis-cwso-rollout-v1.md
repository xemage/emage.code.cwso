# Artifact: root-cause-analysis-cwso-rollout-v1.md

- Producer agent: backend-developer (CWSO project)
- Task: T169
- Created: 2026-07-31
- Based on:
  - `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md`
  - `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md`
  - `deploy/Dockerfile.rollout`
  - `services/cwso-rollout/src/{main,proxy,capture,provider,store,config}.rs`

This is a read-only investigation artifact. No source code was modified while producing it.

## Issue 1 — Healthcheck `curl -f http://127.0.0.1:8787/v1/models` → HTTP 405

### What actually serves requests on `:8787`

`cwso-rollout`'s HTTP listener is a single `hyper` service function, `handle_request`, defined in
`services/cwso-rollout/src/proxy.rs:42-83`. There is no router/framework (no axum `Router`, no
per-path method table) — every inbound connection on the bind address goes through this one
function regardless of path.

The very first thing `handle_request` does is a **global, path-independent method gate**:

```rust
// services/cwso-rollout/src/proxy.rs:46-51
if req.method() != Method::POST {
    return Ok(error_response(
        StatusCode::METHOD_NOT_ALLOWED,
        "only POST is supported",
    ));
}
```

This check runs *before* `req.uri().path()` is even read (path is extracted afterward, at
`proxy.rs:53`). Any request that is not `POST` — for **any** path, including `/v1/models` — is
answered with `405 Method Not Allowed` and body `{"error":{"message":"only POST is supported"}}`.
`curl -f` treats any 4xx/5xx response as `exit 22`, which is exactly the failure mode recorded in
the evidence report (`curl: (22) The requested URL returned error: 405`).

### Does `/v1/models` exist as a route at all?

No — not under any method. After the method gate, the path is dispatched through
`CapturePipeline::handle` (`services/cwso-rollout/src/capture.rs:56`), which classifies the path
via `detect_provider()`:

```rust
// services/cwso-rollout/src/provider.rs:41-54
pub fn detect_provider(path: &str) -> Provider {
    let path = path.trim_end_matches('/');
    if path.ends_with("/v1/chat/completions") || path == "/v1/chat/completions" {
        Provider::OpenAiChat
    } else if path.ends_with("/v1/responses") || path == "/v1/responses" {
        Provider::OpenAiResponses
    } else if path.ends_with("/v1/messages") || path == "/v1/messages" {
        Provider::AnthropicMessages
    } else if path.contains(":generateContent") {
        Provider::GoogleGenerateContent
    } else {
        Provider::Unknown
    }
}
```

`/v1/models` matches none of these branches → `Provider::Unknown`. `CapturePipeline::handle`
returns `CaptureError::UnsupportedProvider` for `Provider::Unknown` (`capture.rs:56-63`), which
`proxy.rs:70-73` maps to `404 Not Found` ("unsupported provider route") — **not** `200`. This was
verified by reading `capture.rs:56-63` and `provider.rs:41-54` directly; no route in this crate
serves `/v1/models` under any method, POST included. A grep for `"models"` (case-insensitive)
across every `.rs` file in `services/cwso-rollout/src/` returns zero matches outside this
analysis, confirming there is no `/v1/models`-specific handler anywhere in the crate.

The only place `/v1/models` is mentioned as a concept in this repository is
`docs/artifacts/rollout-architecture-v1.md:58`, which lists
`/v1/models/:model:generateContent` (Google's `generateContent` route shape) as a *future*
provider-detection target — a different path shape than the bare `/v1/models` the healthcheck
probes, and one that is matched via the `:generateContent` substring branch
(`provider.rs:49`), not a literal `/v1/models` route. It is not implemented as a distinct,
independently-callable endpoint.

There is also no liveness/health endpoint of any kind in `cwso-rollout` — a grep for
`healthz|readyz|health` (case-insensitive) across `services/cwso-rollout/src/*.rs` returns zero
matches. This differs from `cwso-orchestrator`, whose own healthcheck in
`deploy/docker-compose.yml:39-43` (this repo's own compose file) targets a dedicated
`GET /healthz` route. `cwso-rollout` has no equivalent.

Neither this repository's own `deploy/docker-compose.yml` nor `deploy/docker-compose.ci.yml`
defines a `rollout` service or a healthcheck for it (`grep -n rollout` on both files returns no
matches) — the `curl -f http://127.0.0.1:8787/v1/models` healthcheck the evidence report calls
"documented" exists only in the *consuming* project's compose file
(`emage.code/deploy/docker-compose-t226.yml`), not anywhere in CWSO's own deploy configs or docs.

### Verdict on candidate 1: **NEEDS-REFINEMENT** (confirmed 405 cause, but the framing is refined)

The evidence report's diagnosis that the healthcheck fails because of a "method/path mismatch" is
directionally correct on the *method* half — confirmed at `proxy.rs:46-51`. It is refined on the
*path* half: `/v1/models` is not "the rollout service's own `/v1/models` endpoint" with a
GET/POST mismatch — it is not a route at all, under any method. Simply making the global method
gate accept GET, or adding a GET branch that then falls through to `detect_provider`, would still
produce a `404` for `/v1/models` today, not a `200`. The real fix target is "cwso-rollout has no
liveness endpoint," not "cwso-rollout's `/v1/models` endpoint has the wrong method."

### Existing callers/tests of `/v1/models`

Grep results (repo-wide, `*.rs`, `*.go`, `*.yml`, `*.yaml`, `*.md`, `*.sh`, `Dockerfile*`):

- **No Rust test, Go test, or application code in this repository calls or asserts against
  `/v1/models`.** The only in-repo hits are: this task's own inputs/outputs
  (`docs/tasks/task-T169.md`, `docs/tasks/task-T170.md`, the plan doc, and the evidence artifact
  itself, all of which are *about* the healthcheck, not callers of it), and the unrelated
  `/v1/models/:model:generateContent` mention in `docs/artifacts/rollout-architecture-v1.md:58`
  discussed above.
- `services/cwso-rollout/src/proxy.rs`'s own test module (`proxy.rs:115-204`) only exercises
  `POST /v1/chat/completions` (`proxy.rs:190`). `services/cwso-rollout/src/capture.rs`'s test
  module exercises `/v1/chat/completions` and `/v1/responses` (`capture.rs:189,218,281`).
  `services/cwso-rollout/src/provider.rs`'s tests exercise `/v1/chat/completions`,
  `/v1/responses`, `/v1/messages` (`provider.rs:553-565`). None reference `/v1/models`.

**Conclusion: no existing callers or tests depend on `/v1/models`'s current (non-existent) method
contract.** Per the plan's own risk mitigation (`plan-fix-cwso-rollout-healthcheck-and-trajectory-
store.md`, Risks table, row 2), since there are no callers to protect, T170 is free to add a
dedicated liveness endpoint (the lower-risk option) without needing to preserve any `/v1/models`
behavior — and should still prefer that option over changing `/v1/models`'s semantics, because
`/v1/models` is reserved by convention (OpenAI-compatible APIs use it for model listing) and
should not be repurposed as a health probe even though nothing currently calls it.

### Recommended fix direction (not implemented here)

Add a dedicated liveness/readiness route (e.g. `GET /healthz`, mirroring `cwso-orchestrator`'s
existing pattern at `deploy/docker-compose.yml:40`) that bypasses the global POST-only gate for
that one path only, returns `200` with a minimal body, and requires no upstream/provider
dispatch. Update the healthcheck `curl` target to that new path once implemented. Do not change
`/v1/models`'s method contract or repurpose it as a health endpoint, since it is reserved for
future OpenAI-compatible model-listing semantics and doing so would be surprising/incorrect API
design even though no current caller would break.

## Issue 2 — Trajectory store writer error `create rollout store "./rollout_store"`

### Where `./rollout_store` comes from

`services/cwso-rollout/src/store.rs:46-47`:

```rust
let store_path = std::env::var("CWSO_ROLLOUT_STORE_PATH")
    .unwrap_or_else(|_| "./rollout_store".to_string());
```

`./rollout_store` is a **fallback default**, not a hardcoded literal used unconditionally — it is
only used when the environment variable `CWSO_ROLLOUT_STORE_PATH` (note: **not**
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`) is unset or unreadable. This `store_path` becomes
`StoreConfig.store_path` (`store.rs:57`), which is later passed into
`fs::create_dir_all(&config.store_path)` in `writer_loop` (`store.rs:109-110`) — the exact call
whose failure produces the logged error string `create rollout store {:?}` (format string at
`store.rs:110`, matching the log's `"error":"create rollout store \"./rollout_store\""` verbatim,
including the `./rollout_store` value, confirming the fallback branch was taken).

### How `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` is wired in — it isn't

The consuming compose file sets `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store`
(evidence report, compose excerpt, and independently confirmed in this repo's own
`deploy/Dockerfile.rollout:27`, which sets the same env var name as an image-level default:
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store`). **`store.rs:46` never reads
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` anywhere in the crate** — a grep for
`TRAJECTORY_STORE_PATH` across `services/cwso-rollout/src/*.rs` returns zero matches; the only
trajectory-prefixed env var actually read is `CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED`
(`store.rs:42`), which correctly gates whether the store is used at all (and does work — the log
shows `"trajectory Parquet store enabled"` immediately before the failure, confirming the enable
flag *is* wired correctly; only the path variable is not).

This is a **variable-name drift between the Dockerfile/compose layer and the Rust source**, not a
"binary ignores the env var for this writer" bug and not a hardcoded-literal bug. The code's own
env var is `CWSO_ROLLOUT_STORE_PATH`. This is corroborated by two independent in-repo documentation
sources that predate the Dockerfile/compose change:
- `docs/tasks/task-T134.md:21` lists the intended config variable as `CWSO_ROLLOUT_STORE_PATH`.
- `docs/artifacts/rollout-architecture-v1.md:194` documents the env var table entry as
  `CWSO_ROLLOUT_STORE_PATH` (default `./rollout_store`), matching the source exactly.

So the source code (`store.rs:46`) and its own architecture/task documentation agree with each
other on the variable name (`CWSO_ROLLOUT_STORE_PATH`); it is `deploy/Dockerfile.rollout:27` (this
repo's own file) and the consuming compose file that both use a different, non-matching name
(`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`, presumably drifted to match the sibling
`CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED` naming convention, but never applied to the path variable
in source).

### Verdict on candidate 2: **CONFIRMED**, with refinement on the exact mechanism

The evidence report's core claim — that the configured path is not reaching the store writer, so
it falls back to `./rollout_store` — is confirmed. The refinement: the report characterized this
as possibly "the Rust binary... ignores that env var" or "a naming/wiring mismatch." Source
confirms it is specifically the second: an **env-var name mismatch between
`deploy/Dockerfile.rollout` (this repo's own file, not just the consuming project's compose file)
and `services/cwso-rollout/src/store.rs`**. The binary does not ignore the variable — it never had
a code path to read `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` at all; it consistently reads
`CWSO_ROLLOUT_STORE_PATH`, which neither the Dockerfile nor the consuming compose file sets.

Because `/data/parquet-store` (created and `chown`-ed to the `cwso` user in
`deploy/Dockerfile.rollout:20-22`) is never consulted, the writer instead attempts
`fs::create_dir_all("./rollout_store")`. The process's working directory is set by
`WORKDIR /data` in `deploy/Dockerfile.rollout:25`, so the resolved path is `/data/rollout_store` —
which does not exist and, unlike `/data/parquet-store`, was never `mkdir`'d or `chown`'d to the
`cwso` user in the Dockerfile (only `/data/parquet-store` was, at line 20-22). This is why
`create_dir_all` fails (`writer_loop`, `store.rs:109-110`) even though `/data` itself is writable
by root during image build — `/data/rollout_store` was never created, and `USER cwso`
(`Dockerfile.rollout:24`) may also lack permission to create it under `/data` depending on `/data`'s
own permissions, compounding the failure. This second-order detail was not independently verified
by inspecting the final image's `/data` permission bits (out of scope for source-only
investigation — flagged for T170 to confirm at build time if relevant).

### Other consumers of `./rollout_store` / `CWSO_ROLLOUT_STORE_PATH` / `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`

Repo-wide grep results, for T170's awareness before changing path-resolution behavior:

- `services/cwso-rollout/src/store.rs` — the only source file reading `CWSO_ROLLOUT_STORE_PATH`
  (line 46) or using the `./rollout_store` literal (line 47) or `store_path` field (lines
  32,57,109-110,175,382,385, plus test fixtures at 467,489,515,539,554 — all test fixtures use a
  `tempfile` temp dir, not the literal or the env var, so they are unaffected by any path-wiring
  fix).
- `deploy/Dockerfile.rollout:27` — sets `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` as an image-level
  `ENV` default. **This is a CWSO-owned file and is a candidate for T170 to fix or reconcile.**
- `docs/tasks/task-T134.md:21` and `docs/artifacts/rollout-architecture-v1.md:194` — both document
  `CWSO_ROLLOUT_STORE_PATH` (matching source), not `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`. These are
  documentation, not runtime consumers, but T170 should keep them consistent with whichever name
  is chosen as canonical.
- No other deploy config in this repo (`deploy/docker-compose.yml`, `deploy/docker-compose.ci.yml`,
  `deploy/cwso-all-features.env`) references either env var name or the `rollout_store` literal —
  confirmed via `grep -rln` across the repo (see Evidence commands below). The only other
  consumer of either name is the external `emage.code/deploy/docker-compose-t226.yml` file, which
  is outside this repository and out of scope to modify per the plan.

**Implication for T170:** there is no other in-repo deployment currently relying on the *working*
`CWSO_ROLLOUT_STORE_PATH` name with a non-default value, and no in-repo deployment currently
relies on the broken `./rollout_store` fallback behavior persisting. The safer fix direction is
to make `deploy/Dockerfile.rollout` set `CWSO_ROLLOUT_STORE_PATH` (the name the source already
reads) rather than adding a second env var alias into `store.rs`, since `CWSO_ROLLOUT_STORE_PATH`
is what both the source and its own architecture docs already treat as canonical. If backward
compatibility with the `_TRAJECTORY_` name is desired (e.g. because an external consumer may
already depend on it), `store.rs:46` could instead be extended to check
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first and fall back to `CWSO_ROLLOUT_STORE_PATH` — but this
repo's own source of truth (source + architecture doc) says `CWSO_ROLLOUT_STORE_PATH` is correct,
so realigning the Dockerfile is the lower-risk direction. Either way, T170 must also ensure
`fs::create_dir_all` will succeed for whatever path is resolved (i.e. that the target directory
under `/data` is created and owned by the `cwso` user in the Dockerfile, matching the existing
treatment of `/data/parquet-store` at `Dockerfile.rollout:20-22`).

### Recommended fix direction (not implemented here)

Align `deploy/Dockerfile.rollout:27`'s env var name with the one `store.rs:46` actually reads
(`CWSO_ROLLOUT_STORE_PATH`), and ensure the target directory it points at is pre-created and
`chown`-ed to the `cwso` runtime user, exactly as `/data/parquet-store` already is. Do not rename
the variable in source (`store.rs`) without also checking whether any deployment external to this
repo (e.g. emage.code's compose file) already depends on the `_TRAJECTORY_` name — that
compatibility question is explicitly out of scope for T169 to resolve and is flagged for T170 to
re-confirm against the plan's assumption ("No production dependency currently relies on the
current (broken) behavior...; this assumption should be checked during investigation") before
choosing which side (Dockerfile vs. source) to change.

## Are the two issues related?

No shared root cause. Issue 1 (405) is a proxy-layer routing/method-gate gap with no liveness
route at all (`proxy.rs:46-51`, `provider.rs:41-54`). Issue 2 (store path) is a config env-var
name mismatch between `deploy/Dockerfile.rollout` and `store.rs:46`. They fail independently and
were logged independently in the evidence report, consistent with the report's own caution not to
assume a shared cause — confirmed by source that they are, in fact, unrelated.

## Evidence commands run (for reproducibility)

```
find /home/emage/Code/emage/CWSO/services/cwso-rollout -type f -name "*.rs"
grep -n "v1/models|models" services/cwso-rollout/src/*.rs -i
grep -n "rollout_store|TRAJECTORY_STORE|store_path|trajectory_store" services/cwso-rollout/src/*.rs
grep -rn "v1/models" . --include="*.rs" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.md" --include="*.sh" --include="Dockerfile*"
grep -rln "rollout_store" .
grep -rln "CWSO_ROLLOUT_TRAJECTORY_STORE_PATH" .
grep -rln "CWSO_ROLLOUT_STORE_PATH" .
grep -n "healthz|readyz|health" services/cwso-rollout/src/*.rs -i
grep -n "rollout" deploy/docker-compose.yml deploy/docker-compose.ci.yml
```

All commands run from `/home/emage/Code/emage/CWSO`. No `docker build` / `docker compose` command
was run in this task (out of scope per constraints; runtime re-verification is T170's job).
