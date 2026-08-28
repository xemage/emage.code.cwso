# Task C043 — Connection pooling in the shadow client

**ID:** C043
**Owner:** backend-developer
**Status:** pending
**Priority:** P1
**Depends on:** C020–C025 (gate CG2), C030–C034 (gate CG3)
**Created:** 2026-08-12
**Completed:** —
**Based on:** docs/plans/plan-cwso-v1.0-roadmap.md (B13, P2-6); docs/plans/plan-cwso-v1.0-phase4-correctness-v1.md

## Objective

`orchestrator/internal/shadow/client.go` opens one connection per request and
serializes every RPC through `Client.mu` — it "will throttle under Phase-3 concurrent
dispatch", i.e. under the concurrency the product is named for. Add connection pooling
so concurrent dispatch is honest.

## Inputs

- `orchestrator/internal/shadow/client.go` (the client; P2-6 noted at line 5)
- `orchestrator/internal/tools/dispatch_tools.go` (the concurrent-dispatch caller)

## Rails (read before starting)

### You MUST
- Implement a bounded connection pool (configurable size, sensible default) over the UDS socket
- Remove the global serialization of all RPCs (per-connection synchronization is fine; one global mutex for everything is not)
- Add a soak test: N concurrent dispatches (N ≥ 16) complete without connection exhaustion, deadlock, or cross-talk between responses
- Remove the P2-6 marker and update `docs/DEBT-REGISTER.md` (B13 → `fixed`, closing task C043)
- Keep the client's external API unchanged for callers

### You MUST NOT
- Change the wire protocol or the tool surface
- Introduce unbounded connection growth (a leak under load is worse than a throttle)
- Touch the Rust git-shadow service (the fix is client-side)
- Add retry/circuit-breaker logic beyond pooling (v1.1)

## File ownership

- **May create/modify:** `orchestrator/internal/shadow/**`, `docs/DEBT-REGISTER.md` (B13 row)
- **Must NOT touch:** `services/*`, other `orchestrator/internal/*` packages, `schemas/*`

## Steps (execute in order)

1. Read the client and its callers.
2. Implement the bounded pool.
3. Soak test: N≥16 concurrent dispatches.
4. Remove marker; update DEBT-REGISTER.

## Expected outputs

- Pooled shadow client
- Concurrency soak test
- P2-6 marker removed; DEBT-REGISTER updated

## Acceptance criteria

1. N≥16 concurrent dispatches complete without exhaustion/deadlock/cross-talk
2. `go test ./internal/shadow/...` passes (including the soak test)
3. Client API unchanged for callers
4. DEBT-REGISTER B13 = `fixed` / C043

## Verification commands

```bash
cd orchestrator && go test ./internal/shadow/... -count=1
go vet ./internal/shadow/...
grep -n "P2-6" internal/shadow/client.go   # = no hits
```

## Git rails

- Branch: `agent/backend-developer/C043` from `develop`
- Commit: `perf(shadow): add connection pooling to sidecar client`
- MR target: `develop`, squash and merge, delete source branch

## Blocker protocol

Report blockers as: type + severity + one proposed mitigation. Max 2 retries.
If pooling exposes a pre-existing race, do not paper over it with the old mutex —
report `technical` / `major` with a race-detector log (`go test -race`).

## Execution notes

**Executed by:** backend-developer, in worktree `agent-backend-developer-C043`
(branch `agent/backend-developer/C043`, from `origin/develop` @ `0bec0f7`).

### What changed

- `orchestrator/internal/shadow/client.go` — replaced the single global
  `Client.mu`-serialized, one-connection-per-`Call` model with a bounded
  connection pool:
  - `Client` now holds `sem chan struct{}` (bounds live connections at
    `poolSize`) and `idle chan net.Conn` (ready-to-reuse connections), plus a
    `closed` channel + `sync.Once` for graceful shutdown.
  - `acquire()`/`release()` check a connection out exclusively for the
    duration of one `Call` round trip and return it to `idle` afterward (or
    discard it + free its slot if the transport round trip failed). This
    means synchronization is per-connection, not global — up to `poolSize`
    RPCs are genuinely concurrent.
  - `NewClient(socket)` keeps its exact original signature/behavior for
    existing callers, now sized via `CWSO_SHADOW_POOL_SIZE` env var (falls
    back to `defaultPoolSize = 8` if unset/invalid/non-positive).
  - Added `NewClientWithPoolSize(socket, size)` as an additive, explicit
    constructor (used by the pool/soak tests; available for future callers
    that want direct control) and an additive `Close()` for graceful
    connection-pool drain (not required by any existing caller, but avoids
    leaking idle sockets if a caller wants to shut down cleanly).
  - Added a defensive `resp.ID != env.ID` cross-talk check in `Call()` — the
    Rust sidecar already echoes the request id, so this actively guards the
    "no cross-talk" acceptance criterion at runtime, not just structurally.
  - Removed the `POC-DEBT`/P2-6 marker comment (lines 5-6 of the old file);
    replaced with a package doc describing the pool and referencing the
    sidecar's actual connection-handling behavior (see "Why persistent
    connections" below).
- `orchestrator/internal/shadow/client_test.go`:
  - Upgraded the fake sidecar test double (`startTestSidecar` /
    `startCountingSidecar`) from single-shot (close after one request) to a
    per-connection read loop, matching the real `cwso-git-shadow`
    `handle_client` (verified read-only against
    `services/cwso-git-shadow/src/main.rs:137-168`, which loops
    `read_frame`/`write_frame` on one accepted `UnixStream` until EOF). This
    is required to exercise genuine connection reuse.
  - Added `client.Close()` cleanup (`t.Cleanup`) to existing tests that open
    a real socket, so per-connection server goroutines terminate cleanly.
  - Added `TestPoolSizeConfigurable`, `TestCallReusesPooledConnections`,
    `TestSoakConcurrentDispatch` (the acceptance-criteria soak test: 32
    concurrent `Call`s over a pool of 4, verifies zero errors, zero
    cross-talk via a per-job echo check, sidecar served all 32 requests, and
    the sidecar never saw more than `poolSize` distinct connections),
    and `TestClosePreventsNewCheckouts`.
- `docs/DEBT-REGISTER.md` — B13 row (live register + marker-location table +
  historical Phase-2 scorecard row) updated from `open`/`v1.0-blocker` to
  `closed`/`fixed`, with evidence pointing at the pool implementation and
  `TestSoakConcurrentDispatch`.

### Why persistent (reusable) connections, not just "dial fresh but allow N in flight"

Read-only inspection of `services/cwso-git-shadow/src/main.rs` (out of scope
to modify, in scope to read) confirms `handle_client` loops
`read_frame`/`write_frame` on a single accepted `UnixStream` until the peer
closes it — i.e. the real sidecar already supports multiple sequential
requests per connection. A pool that reuses persistent connections (rather
than one that just bounds concurrent fresh dials) is therefore both correct
against production and the design the brief's "connection pool" language
implies. Per-connection request/response ordering stays strictly sequential
(one round trip owns a connection exclusively at a time) — this is pooling,
not request pipelining/multiplexing, matching the "per-connection
synchronization is fine" rail and staying out of v1.1 retry/multiplex scope.

### Verification (real output)

```
$ go vet ./internal/shadow/...
(no output — clean)

$ go test ./internal/shadow/... -count=1 -v
=== RUN   TestCallSuccess
--- PASS: TestCallSuccess (0.00s)
=== RUN   TestCallSidecarError
--- PASS: TestCallSidecarError (0.00s)
=== RUN   TestCallResultDecodeError
--- PASS: TestCallResultDecodeError (0.00s)
=== RUN   TestCallMarshalParamsError
--- PASS: TestCallMarshalParamsError (0.00s)
=== RUN   TestWriteFrameTooLarge
--- PASS: TestWriteFrameTooLarge (0.00s)
=== RUN   TestReadFrameOutOfRange
=== RUN   TestReadFrameOutOfRange/zero
=== RUN   TestReadFrameOutOfRange/too_large
--- PASS: TestReadFrameOutOfRange (0.00s)
=== RUN   TestCallDialError
--- PASS: TestCallDialError (0.00s)
=== RUN   TestCallSidecarFailureWithoutBody
--- PASS: TestCallSidecarFailureWithoutBody (0.00s)
=== RUN   TestCallWithNilOutIgnoresResultDecode
--- PASS: TestCallWithNilOutIgnoresResultDecode (0.00s)
=== RUN   TestCallEnvelopeMarshalError
--- PASS: TestCallEnvelopeMarshalError (0.00s)
=== RUN   TestPoolSizeConfigurable
--- PASS: TestPoolSizeConfigurable (0.00s)
=== RUN   TestCallReusesPooledConnections
--- PASS: TestCallReusesPooledConnections (0.00s)
=== RUN   TestSoakConcurrentDispatch
--- PASS: TestSoakConcurrentDispatch (0.02s)
=== RUN   TestClosePreventsNewCheckouts
--- PASS: TestClosePreventsNewCheckouts (0.00s)
PASS
ok  	github.com/emage/cwso/orchestrator/internal/shadow	0.037s

$ go test ./internal/shadow/... -race -count=1 -v
(identical PASS list)
ok  	github.com/emage/cwso/orchestrator/internal/shadow	1.046s

$ go test ./internal/shadow/... -race -run 'TestSoakConcurrentDispatch|TestCallReusesPooledConnections|TestClosePreventsNewCheckouts' -count=25 -v
(all 75 sub-runs PASS, no flakes)
ok  	github.com/emage/cwso/orchestrator/internal/shadow	1.773s

$ grep -n "P2-6" internal/shadow/client.go
(no output — 0 hits, exit 1)

$ go build ./...      # whole orchestrator module
(clean)

$ go test ./internal/tools/... ./internal/rollout/... -count=1
ok  	github.com/emage/cwso/orchestrator/internal/tools	0.169s
ok  	github.com/emage/cwso/orchestrator/internal/rollout	0.176s
```

### Acceptance criteria

1. **N≥16 concurrent dispatches, no exhaustion/deadlock/cross-talk** — MET.
   `TestSoakConcurrentDispatch` runs 32 concurrent `Call`s against a pool of
   4; asserts zero errors, all 32 served, per-job echo matches (no
   cross-talk), and the sidecar accepted `>1` and `<= poolSize` distinct
   connections (bounded, not exhausted). Passes clean under `-race`, 25x
   repeated with no flakes.
2. **`go test ./internal/shadow/... -count=1` passes, including the soak
   test** — MET (output above).
3. **Client API unchanged for callers** — MET. `NewClient(socket) *Client`
   and `(*Client).Call(op, params, out) error` are byte-for-byte unchanged
   signatures. `git diff` on `internal/tools/dispatch_tools.go` and
   `internal/tools/shadow_tools.go` is empty; both packages build and their
   existing tests pass with zero modification.
4. **DEBT-REGISTER B13 = `fixed` / C043** — MET (three locations updated:
   live register row, marker-location table, historical Phase-2 scorecard
   row).

### Known regression outside this task's ownership (reported as a blocker below)

`go test ./...` across the whole `orchestrator` module has exactly one
failure caused by this change:
`internal/server` › `TestWriteShadowFileFeedsSpikeMonitorEndToEnd` (in
`internal/server/ast_feeder_test.go`, a file outside this task's file
ownership — may not modify `services/*` or other `orchestrator/internal/*`
packages).

Root cause (confirmed via an isolated repro, since deleted from the
worktree): that test's local `startFakeShadow` fixture closes the accepted
connection after serving exactly one request (single-shot), unlike the real
`cwso-git-shadow` sidecar. The test makes 3 sequential `write_shadow_file`
calls through one shared `*shadow.Client`. With genuine connection reuse,
call 2 reuses call 1's now-peer-closed connection and fails
(`write: broken pipe`); that failure surfaces as an MCP tool-level error
(`isError` in the JSON-RPC *result*, not a top-level RPC error), which this
particular test's `env["error"] != nil` check doesn't catch, so the write is
silently dropped and the AST spike (which needs 3 observed writes) never
fires, and the test times out. Confirmed as a genuine regression (not
pre-existing flake) by running the same test against the unmodified
`origin/develop` baseline (`git stash`), where it passes 100%.

This is *expected* fallout of a correct fix, not a bug in the pool: the real
sidecar (read-only-verified in
`services/cwso-git-shadow/src/main.rs:137-168`, `handle_client`) already
loops reads on one connection until EOF, so production is unaffected: this
failure is specific to a stale test double.

**Blocker:** `dependency` / `minor`. Proposed mitigation: update
`internal/server/ast_feeder_test.go`'s `startFakeShadow` to loop
`readFrame`/`writeFrame` per accepted connection until error/EOF instead of
closing after one request — the exact same fix already applied to
`internal/shadow/client_test.go`'s `startCountingSidecar` in this change,
which is a ~5-line, low-risk diff. Left unmodified here because
`internal/server/**` is outside this task's file-ownership boundary; needs
routing to whoever owns that package (or an explicit scope exception).

### Addendum — blocker resolved via scope exception

The orchestrator granted a narrow, one-off scope exception authorizing exactly
`orchestrator/internal/server/ast_feeder_test.go` — specifically only the
`startFakeShadow` fixture — as an addition to this task's file ownership,
folded into this same branch/MR (not routed to a separate owner/task), to
close the blocker above.

**What changed:** `startFakeShadow`'s per-connection goroutine previously read
exactly one frame, wrote exactly one response, and returned (closing the
connection via `defer`). Wrapped the existing read-header/read-body/write-
response logic in a `for {}` loop that keeps servicing frames on the same
connection until a read or write error (EOF on close) — mirroring the exact
read-loop shape already used in this same commit's
`internal/shadow/client_test.go`'s `startCountingSidecar`, per the proposed
mitigation above. Also switched the two response writes from ignored-error
(`_, _ = c.Write(...)`) to error-checked (`if _, err := c.Write(...); err !=
nil { return }`) so a write failure ends the per-connection goroutine cleanly
instead of looping on a broken connection, consistent with
`startCountingSidecar`'s pattern. No change to the test's
assertions/logic, and no other file touched.

**File touched:** `orchestrator/internal/server/ast_feeder_test.go` (only
`startFakeShadow`).

**Verification (real output, run from `orchestrator/` in this worktree):**

```
$ go build ./...
(clean, no output)

$ go test ./internal/server/... -run TestWriteShadowFileFeedsSpikeMonitorEndToEnd -count=1 -v
=== RUN   TestWriteShadowFileFeedsSpikeMonitorEndToEnd
--- PASS: TestWriteShadowFileFeedsSpikeMonitorEndToEnd (0.01s)
PASS
ok  	github.com/emage/cwso/orchestrator/internal/server	0.013s

$ go test ./internal/server/... -count=1
ok  	github.com/emage/cwso/orchestrator/internal/server	0.540s

$ go test ./... -count=1
?   	github.com/emage/cwso/orchestrator/cmd/cwso-orchestrator	[no test files]
ok  	github.com/emage/cwso/orchestrator/internal/config	0.013s
ok  	github.com/emage/cwso/orchestrator/internal/dashboard	0.014s
ok  	github.com/emage/cwso/orchestrator/internal/dispatch	2.674s
ok  	github.com/emage/cwso/orchestrator/internal/eventbus	0.087s
ok  	github.com/emage/cwso/orchestrator/internal/hal	0.061s
ok  	github.com/emage/cwso/orchestrator/internal/harness	0.161s
ok  	github.com/emage/cwso/orchestrator/internal/integration	0.297s
ok  	github.com/emage/cwso/orchestrator/internal/jobs	0.061s
ok  	github.com/emage/cwso/orchestrator/internal/logging	0.005s
ok  	github.com/emage/cwso/orchestrator/internal/mcp	0.003s
ok  	github.com/emage/cwso/orchestrator/internal/memorybroker	0.195s
ok  	github.com/emage/cwso/orchestrator/internal/mergeengine	0.020s
ok  	github.com/emage/cwso/orchestrator/internal/rollout	0.187s
ok  	github.com/emage/cwso/orchestrator/internal/sandbox	0.208s
ok  	github.com/emage/cwso/orchestrator/internal/server	0.544s
ok  	github.com/emage/cwso/orchestrator/internal/shadow	0.042s
?   	github.com/emage/cwso/orchestrator/internal/sparse	[no test files]
ok  	github.com/emage/cwso/orchestrator/internal/tools	0.194s
ok  	github.com/emage/cwso/orchestrator/internal/transport	0.448s

$ go vet ./...
(clean, no output)
```

`go test ./...` now passes cleanly across the whole orchestrator module (all
5 verification steps green), which C043's original acceptance criteria
implicitly required but could not claim before this fix. Blocker above is
now resolved — not deferred to a follow-up task.

Committed as a separate commit on `agent/backend-developer/C043` (not
amended into the pooling commit): `test(server): fix startFakeShadow fixture
to loop per-connection reads (C043 fallout)`. Not pushed; awaiting
review/merge per git rails.
