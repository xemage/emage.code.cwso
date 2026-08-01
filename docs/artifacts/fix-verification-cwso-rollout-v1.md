# Artifact: fix-verification-cwso-rollout-v1.md

- Producer agent: backend-developer (CWSO project)
- Task: T170
- Created: 2026-08-01
- Based on:
  - `docs/tasks/task-T170.md`
  - `docs/artifacts/root-cause-analysis-cwso-rollout-v1.md` (T169 — confirmed root causes)
  - `docs/artifacts/emagecode-integration-defect-cwso-rollout-unhealthy-v1.md` (original evidence
    and exact failing scenario re-verified below)
  - `docs/plans/plan-fix-cwso-rollout-healthcheck-and-trajectory-store.md`
  - `deploy/Dockerfile.rollout`

This artifact verifies the fix already implemented in
`services/cwso-rollout/src/proxy.rs` and `services/cwso-rollout/src/store.rs` (working-tree
changes present before this verification pass began), plus one Dockerfile addition made during
this verification (see "Dockerfile change" section below). No fix logic in `proxy.rs` or
`store.rs` was altered during this task — only build/test/Docker verification was performed.

## Fix summary (for traceability — already implemented, not redone here)

- **Issue 1** (`root-cause-analysis-cwso-rollout-v1.md` Issue 1, CONFIRMED with refinement):
  `services/cwso-rollout/src/proxy.rs` now serves a dedicated `GET /healthz` liveness route ahead
  of the global POST-only gate, returning `200 {"status":"ok"}` without touching the upstream
  provider pipeline. `/v1/models`'s existing behavior (405 for GET, 404 for POST) is deliberately
  left unchanged, matching T169's finding that no in-repo caller/test depends on it.
- **Issue 2** (`root-cause-analysis-cwso-rollout-v1.md` Issue 2, CONFIRMED): `StoreConfig::from_env`
  in `services/cwso-rollout/src/store.rs` now reads `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH` first,
  falling back to the canonical `CWSO_ROLLOUT_STORE_PATH`, then the historical `./rollout_store`
  default — implementing T169's recommended backward-compatible option (b).

## Dockerfile change made during this verification task

`deploy/Dockerfile.rollout` had **no `HEALTHCHECK` instruction at all** prior to this task
(confirmed: `grep -n HEALTHCHECK deploy/Dockerfile.rollout` returned no matches before this
change). Per T170's task brief guidance, a `HEALTHCHECK` instruction was added targeting the new
`/healthz` route, so the image owns its own healthcheck contract instead of relying on a
consuming project's compose file to probe the non-existent `/v1/models` endpoint:

```dockerfile
EXPOSE 8787
# T170 fix for root-cause-analysis-cwso-rollout-v1.md Issue 1: own the healthcheck contract at
# the image level via the new GET /healthz liveness route (proxy.rs) instead of relying on a
# consuming project's compose file to probe the non-existent /v1/models endpoint.
HEALTHCHECK --interval=10s --timeout=3s --retries=5 \
    CMD curl -f http://127.0.0.1:8787/healthz || exit 1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/cwso-rollout"]
```

This is a minimal, in-scope extension of the T169 Issue 1 fix (the `curl` binary was already
installed in the runtime image at `deploy/Dockerfile.rollout:18`, so no new dependency was added).
It was verified to work standalone (i.e. without any `docker run --health-cmd` override) — see
"Dockerfile-native healthcheck" verification below.

## Environment

- `rustc --version` → `rustc 1.86.0 (05f9846f8 2025-03-31)`
- `cargo --version` → `cargo 1.86.0 (adf9b6ad1 2025-02-28)`
- `docker --version` → `Docker version 29.6.2, build dfc4efb`
- Host OS: Linux 6.18.33.2-microsoft-standard-WSL2 (WSL2)
- All commands run from `/home/emage/Code/emage/CWSO` unless noted otherwise.

## Note on port 8787 and the pre-existing `cwso-rollout` container

A container named `cwso-rollout` (the emage.code stack's own long-running instance, the one
originally reporting `(unhealthy)` in the evidence report) was already running on the host and
bound to host port `8787` for the entire duration of this verification. That container belongs to
a different project (`emage.code`) and is out of scope for this task to stop, restart, or modify.
All verification containers below were therefore published on alternate host ports (`8788`,
`8789`) while the container's own internal bind (`0.0.0.0:8787`, matching the Dockerfile/image
contract) was left unchanged — this does not affect the validity of the healthcheck evidence,
since `docker inspect .State.Health` reads the *in-container* probe (`curl` runs inside the
container's network namespace, always against `127.0.0.1:8787`), independent of the host port
mapping.

## Step 1 — `cargo build -p cwso-rollout`

Command run from `/home/emage/Code/emage/CWSO/services` (the workspace root containing
`Cargo.toml`):

```
cd /home/emage/Code/emage/CWSO/services && cargo build -p cwso-rollout 2>&1
```

Verbatim output (tail — full output showed only pre-existing dead-code warnings, no errors):

```
warning: method `store` is never used
  --> cwso-rollout/src/capture.rs:89:12
...
warning: `cwso-rollout` (bin "cwso-rollout") generated 8 warnings
    Finished `dev` profile [unoptimized + debuginfo] target(s) in 0.65s
```

**Result: PASS.** Build succeeds with exit code 0 (implied by `Finished` line and no `error[...]`
entries). All 8 warnings are pre-existing dead-code/unused-field warnings unrelated to the T170
changes (confirmed by file/line: `capture.rs:89`, `config.rs:22`, `config.rs:88`, `record.rs:87`,
`security.rs:42`, `store.rs:416`, `upstream.rs:24`, `upstream.rs:83` — none in the modified
`healthz_response`/`from_env` code paths).

## Step 2 — `cargo test -p cwso-rollout`

Command run from the same directory:

```
cd /home/emage/Code/emage/CWSO/services && cargo test -p cwso-rollout 2>&1
```

Verbatim test result summary (full output includes the same 6 dead-code warnings during test
compilation, omitted here for brevity — no errors):

```
running 35 tests
test config::tests::upstream_path_for_responses_uses_chat_after_normalize ... ok
test config::tests::upstream_chat_path_joins_cleanly ... ok
test ipc::tests::authorize_stream_rejects_unauthorized_peer ... ok
test ipc::tests::prefix_prewarm_reports_cache_hit ... ok
test ipc::tests::stat_reports_service ... ok
test prefix_cache::tests::evicts_oldest_entry ... ok
test prefix_cache::tests::prewarm_miss_then_hit ... ok
test provider::tests::detect_anthropic_path ... ok
test provider::tests::anthropic_normalize_maps_messages ... ok
test capture::tests::pipeline_forwards_and_captures ... ok
test capture::tests::differential_prompting_strips_prefix_tokens_on_cache_hit ... ok
test provider::tests::detect_openai_path ... ok
test provider::tests::detect_openai_responses_path ... ok
test provider::tests::extract_capture_reads_logprobs ... ok
test provider::tests::extract_capture_for_responses_provider ... ok
test provider::tests::openai_responses_normalize_maps_input_and_instructions ... ok
test provider::tests::synthetic_sse_anthropic_format ... ok
test provider::tests::synthetic_sse_google_format ... ok
test provider::tests::synthetic_sse_openai_chat_format ... ok
test record::tests::enqueue_and_drain_round_trip ... ok
test provider::tests::openai_normalize_forces_logprobs_and_disables_upstream_stream ... ok
test provider::tests::openai_responses_denormalize_maps_output_array ... ok
test provider::tests::synthetic_sse_openai_responses_format ... ok
test security::tests::rejects_plaintext_remote_upstream ... ok
test store::tests::from_env_prefers_trajectory_alias_then_canonical_then_default ... ok
test record::tests::saturated_queue_increments_drop_counter ... ok
test security::tests::redacts_bearer_tokens ... ok
test proxy::tests::healthz_returns_200_and_v1_models_is_unchanged ... ok
test capture::tests::pipeline_routes_responses_path ... ok
test proxy::tests::proxy_endpoint_returns_openai_shape ... ok
test store::tests::retention_purges_stale_partitions ... ok
test store::tests::saturated_store_queue_increments_drop_counter ... ok
test store::tests::parquet_round_trip_preserves_records ... ok
test store::tests::writer_thread_flushes_batches_without_blocking_proxy ... ok
test store::tests::fanout_enqueue_is_fast ... ok

test result: ok. 35 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.02s
```

**Result: PASS.** 35/35 tests pass, including both new T170 regression tests:
`proxy::tests::healthz_returns_200_and_v1_models_is_unchanged` and
`store::tests::from_env_prefers_trajectory_alias_then_canonical_then_default`.

## Step 3 — `docker build`

Command run from `/home/emage/Code/emage/CWSO` (repo root as build context, matching the
Dockerfile's `COPY services/...` paths):

```
docker build -f deploy/Dockerfile.rollout -t cwso-rollout-t170-verify .
```

Verbatim output (elided to key milestones — full multi-stage Rust dependency compilation log
omitted for length; no `ERROR` lines occurred):

```
#18 [builder 10/10] RUN cargo build --release -p cwso-rollout
...
#18 88.42 warning: method `store` is never used
   ...(same 8 pre-existing warnings as Step 1)...
#18 95.30 warning: `cwso-rollout` (bin "cwso-rollout") generated 8 warnings
#18 95.30     Finished `release` profile [optimized] target(s) in 1m 34s
#18 DONE 96.1s

#19 [stage-1 2/4] RUN apt-get update && ... mkdir -p /run/cwso /data/parquet-store && ...
#19 CACHED
#20 [stage-1 3/4] COPY --from=builder /src/target/release/cwso-rollout /usr/local/bin/
#20 DONE 0.2s
#21 [stage-1 4/4] WORKDIR /data
#21 DONE 0.2s
#22 exporting to image
#22 naming to docker.io/library/cwso-rollout-t170-verify:latest
#22 DONE 1.9s
```

**Result: PASS.** Image `cwso-rollout-t170-verify:latest` built successfully with no errors.

## Step 4/5 — Container run and healthcheck sustain (>= 5 consecutive probes)

### Run command (test container A — explicit `--health-cmd` override, matching task brief)

```
mkdir -p /tmp/.../scratchpad/t170-parquet-store && chmod 777 /tmp/.../scratchpad/t170-parquet-store

docker run -d --name cwso-rollout-t170-verify-run \
  -e CWSO_ROLLOUT_PROXY_ENABLED=true \
  -e CWSO_ROLLOUT_UPSTREAM_URL=http://127.0.0.1:18080 \
  -e CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED=true \
  -e CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store \
  -v /tmp/.../scratchpad/t170-parquet-store:/data/parquet-store \
  -p 8788:8787 \
  --health-cmd 'curl -f http://127.0.0.1:8787/healthz' \
  --health-interval=10s --health-timeout=3s --health-retries=5 \
  cwso-rollout-t170-verify
```

Note: `CWSO_ROLLOUT_PROXY_ENABLED=true` and `CWSO_ROLLOUT_UPSTREAM_URL=http://127.0.0.1:18080`
were added beyond the task brief's literal env-var list because the original evidence report's
own log (`emagecode-integration-defect-cwso-rollout-unhealthy-v1.md`, "Commands Run" section)
shows the real failing scenario had the HTTP proxy enabled (log line `"starting rollout proxy",
"upstream":"http://127.0.0.1:18080"`), and the compose excerpt in that same artifact elided
additional env vars with `...`. Without `CWSO_ROLLOUT_PROXY_ENABLED=true` the binary starts in
"IPC-only mode" and never binds port 8787 at all (confirmed: first attempt without this var logged
`"CWSO_ROLLOUT_PROXY_ENABLED=false; IPC-only mode"` and the healthcheck failed with `curl: (7)
Failed to connect` — a different, unrelated failure mode from the one under test). Adding these two
vars, with the same loopback upstream value `http://127.0.0.1:18080` from the original evidence
log, faithfully reproduces the original failing scenario's startup path.

### `docker inspect cwso-rollout-t170-verify-run --format '{{json .State.Health}}'` (verbatim, after 5 probes)

```json
{"Status":"healthy","FailingStreak":0,"Log":[
  {"Start":"2026-08-01T05:18:46.001162329Z","End":"2026-08-01T05:18:46.090372467Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}..."},
  {"Start":"2026-08-01T05:18:56.089416057Z","End":"2026-08-01T05:18:56.178601866Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}..."},
  {"Start":"2026-08-01T05:19:06.180383589Z","End":"2026-08-01T05:19:06.268384232Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}..."},
  {"Start":"2026-08-01T05:19:16.269978664Z","End":"2026-08-01T05:19:16.348894731Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}..."},
  {"Start":"2026-08-01T05:19:26.347879739Z","End":"2026-08-01T05:19:26.432207328Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}..."}
]}
```

(Output bodies truncated of curl progress-meter noise for readability; each entry's `Output`
field contained the literal `{"status":"ok"}` response body and each `ExitCode` was `0`.)

**Result: PASS.** `"Status":"healthy"`, `"FailingStreak":0`, 5/5 consecutive probes with
`ExitCode: 0`, 10s apart as configured — matching acceptance criterion 2 exactly. This directly
contrasts with the original evidence report's `{"Status":"unhealthy","FailingStreak":6, ...
"ExitCode":22 ... "curl: (22) The requested URL returned error: 405"}`.

### Dockerfile-native healthcheck (no `--health-cmd` override) — validates the Dockerfile edit

A second container was run with identical env vars but **no** `--health-cmd`/`--health-interval`
flags, relying solely on the `HEALTHCHECK` instruction baked into the image by this task's
Dockerfile edit:

```
docker run -d --name cwso-rollout-t170-verify-dockerfile-hc \
  -e CWSO_ROLLOUT_PROXY_ENABLED=true \
  -e CWSO_ROLLOUT_UPSTREAM_URL=http://127.0.0.1:18080 \
  -e CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED=true \
  -e CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store \
  -p 8789:8787 \
  cwso-rollout-t170-verify
```

`docker inspect cwso-rollout-t170-verify --format '{{json .Config.Healthcheck}}'` (the built
image's own config, confirming the Dockerfile `HEALTHCHECK` instruction was baked in):

```json
{"Test":["CMD-SHELL","curl -f http://127.0.0.1:8787/healthz || exit 1"],"Interval":10000000000,"Timeout":3000000000,"Retries":5}
```

`docker inspect cwso-rollout-t170-verify-dockerfile-hc --format '{{json .State.Health}}'`
(verbatim, after 5 probes):

```json
{"Status":"healthy","FailingStreak":0,"Log":[
  {"Start":"2026-08-01T05:20:25.521906344Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}"},
  {"Start":"2026-08-01T05:20:35.612475066Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}"},
  {"Start":"2026-08-01T05:20:45.690232489Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}"},
  {"Start":"2026-08-01T05:20:55.775257799Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}"},
  {"Start":"2026-08-01T05:21:05.835667612Z","ExitCode":0,"Output":"...{\"status\":\"ok\"}"}
]}
```

**Result: PASS.** The Dockerfile's own `HEALTHCHECK` instruction (no external override) also
sustains `healthy`/`FailingStreak:0` across 5 consecutive probes, confirming the image now owns
a working healthcheck contract end-to-end.

## Step 6 — Trajectory store writer startup and path creation

### `docker logs cwso-rollout-t170-verify-run` (verbatim, full)

```
{"timestamp":"2026-08-01T05:18:35.993354Z","level":"INFO","fields":{"message":"trajectory Parquet store enabled","written":0},"target":"cwso_rollout"}
{"timestamp":"2026-08-01T05:18:35.994045Z","level":"INFO","fields":{"message":"cwso-rollout IPC ready","socket_path":"\"/run/cwso/rollout.sock\""},"target":"cwso_rollout::ipc"}
{"timestamp":"2026-08-01T05:18:35.995122Z","level":"INFO","fields":{"message":"starting rollout proxy","bind":"0.0.0.0:8787","upstream":"http://127.0.0.1:18080"},"target":"cwso_rollout"}
{"timestamp":"2026-08-01T05:18:35.995206Z","level":"INFO","fields":{"message":"cwso-rollout proxy listening","bind":"0.0.0.0:8787"},"target":"cwso_rollout::proxy"}
```

**No `"error":"create rollout store ..."` line is present** (compare to the original evidence
report's `docker compose ... logs rollout`, which showed exactly this error immediately after the
`"trajectory Parquet store enabled"` line). This confirms Issue 2 is fixed: with
`CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store` set (the same variable name and value
used by the emage.code compose file and this repo's own Dockerfile default), the writer thread no
longer falls back to `./rollout_store` and no longer fails to create its target directory.

### `docker exec cwso-rollout-t170-verify-run ls -la /data/parquet-store` (verbatim)

```
total 8
drwxrwxrwx 2 1000 1000 4096 Aug  1 05:18 .
drwxr-xr-x 3 root root 4096 Jul 31 15:55 ..
```

### `docker exec cwso-rollout-t170-verify-run ls -la /data` (verbatim, parent dir)

```
total 12
drwxr-xr-x 3 root root 4096 Jul 31 15:55 .
drwxr-xr-x 1 root root 4096 Aug  1 05:18 ..
drwxrwxrwx 2 1000 1000 4096 Aug  1 05:18 parquet-store
```

### Host bind-mounted scratch dir, `ls -la` (verbatim)

```
total 8
drwxrwxrwx 2 emage emage 4096 Aug  1 07:18 .
drwx------ 3 emage emage 4096 Aug  1 07:18 .
```

**Result: PASS (directory creation), with one honest caveat.** `fs::create_dir_all` for
`/data/parquet-store` succeeded cleanly (visible from both inside the container and on the host
bind mount, with mtime matching container start time `05:18`) — this is the direct fix for the
`create rollout store "./rollout_store"` failure documented in the original evidence report. No
`.parquet` data files were produced in this smoke test because this verification did not send any
`/v1/chat/completions`/`/v1/responses`/`/v1/messages` traffic through the proxy (the configured
upstream `http://127.0.0.1:18080` is a loopback placeholder matching the original evidence log,
not a live LLM backend, and stand up a live upstream reachable from inside the container's network
namespace was judged out of scope for this healthcheck-focused verification). The actual
Parquet-file write path (`flush_batch` → `ArrowWriter`) is independently exercised and passing in
`cargo test`'s `store::tests::parquet_round_trip_preserves_records` and
`store::tests::writer_thread_flushes_batches_without_blocking_proxy` (Step 2 above), which confirm
records placed on the channel are correctly flushed to `.parquet` files at the configured
`store_path`. Combined, this confirms: (a) the path-wiring bug (the actual T169 root cause) is
fixed — the writer now resolves and creates the *correct* configured directory instead of failing
on the wrong fallback path — and (b) the write mechanism itself is correct and already covered by
existing unit tests, which were not modified by this fix.

## Step 7 — `/v1/models` behavior unchanged

```
curl -i -X GET http://127.0.0.1:8788/v1/models
```

Verbatim response:

```
HTTP/1.1 405 Method Not Allowed
content-type: application/json
content-length: 46
date: Sat, 01 Aug 2026 05:19:58 GMT

{"error":{"message":"only POST is supported"}}
```

**Result: PASS.** Identical to pre-fix behavior (405, same error body) — confirmed unchanged, as
required by acceptance criterion 4. (For contrast, `GET /healthz` on the same running container
returns `200 {"status":"ok"}`, confirmed separately during Step 4/5 manual spot-check.)

## Step 8 — Cleanup

```
docker rm -f cwso-rollout-t170-verify-run cwso-rollout-t170-verify-dockerfile-hc
rm -rf /tmp/.../scratchpad/t170-parquet-store
```

Both verification containers were stopped and removed; the temporary host bind-mount scratch
directory was deleted. The built image `cwso-rollout-t170-verify:latest` was **left in place**
(not removed) in case a reviewer wants to re-inspect it without rebuilding; it can be removed with
`docker rmi cwso-rollout-t170-verify` at the reviewer's discretion. The pre-existing, unrelated
`cwso-rollout` container (from the separate `emage.code` project's own compose stack, already
running before this task started and still reporting its original `(unhealthy)` status from the
unfixed image `cwso/rollout:dev`) was left completely untouched, as it is out of scope for this
task and belongs to a different project's deployment.

## Acceptance criteria checklist (task-T170.md)

1. **Only T169-confirmed root causes fixed** — PASS. `git diff --stat` shows only
   `services/cwso-rollout/src/proxy.rs` (+100/-0) and `services/cwso-rollout/src/store.rs`
   (+80/-1) as functional changes, both reviewed above and matching exactly the two T169-confirmed
   issues; `deploy/Dockerfile.rollout` (+5/-0, this task) adds only a `HEALTHCHECK` instruction
   tied to Issue 1, no other Dockerfile changes. No other file under `services/cwso-rollout/`
   was modified.
2. **Rebuilt image reaches and sustains `(healthy)` across >= 5 consecutive probes, no
   `FailingStreak` growth** — PASS. See Step 4/5: both the explicit `--health-cmd` invocation and
   the Dockerfile-native `HEALTHCHECK` alone independently reached `"Status":"healthy"`,
   `"FailingStreak":0` across 5/5 probes.
3. **Trajectory store writer starts cleanly, writes land at the configured path** — PASS (with
   caveat documented in Step 6): no `create rollout store` error; `/data/parquet-store` directory
   confirmed created at the exact path from `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`, both inside the
   container and on the host bind mount; actual `.parquet` file writes are covered by existing,
   passing unit tests (no live traffic was sent in this smoke test — see Step 6 for the honest
   scope statement).
4. **`/v1/models` behavior confirmed unchanged** — PASS. See Step 7: identical `405` response,
   byte-for-byte matching error body, to pre-fix behavior.
5. **`fix-verification-cwso-rollout-v1.md` produced with verbatim evidence** — PASS. This file.
6. **`active-tasks.md` untouched by this task** — PASS. This task did not edit
   `docs/tasks/active-tasks.md` (the file shows as modified in `git status` only because of a
   prior, unrelated change already present in the working tree before this task began; verify via
   `git diff docs/tasks/active-tasks.md` — no edits were made to it during T170 execution).

## Blocker status

None. Both build/test verification and Docker runtime verification completed successfully on the
first attempt after correcting the container invocation to include
`CWSO_ROLLOUT_PROXY_ENABLED=true` and a loopback `CWSO_ROLLOUT_UPSTREAM_URL` (required for the
HTTP listener to bind at all — not itself a defect, just a startup precondition inferred from the
original evidence report's own log output, since the task brief's literal env-var list omitted it
and the original evidence artifact's compose excerpt elided it with `...`).
