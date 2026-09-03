# ADR-011: Operator Dashboard

> Based on: `requirements-v2.md`, `docs/plans/feature-operator-dashboard.md`  
> Date: 2026-08-06  
> Author: solution-architect

## Status

Accepted

---

## Context

CWSO (v0.4.1 GA) is a Go/Rust orchestration platform exposing the Model Context Protocol (MCP) over
HTTP at `:8080`. Platform engineers currently have no runtime visibility into the system state:
`/healthz` returns a single `ok` string regardless of sidecar health; job queue depth, auth failure
counts, and capability registry state are all internal-only. Operators must tail structured JSON logs
to detect degraded conditions.

Requirements v2 (NFR-5) mandates JSON logs and OTEL spans, but stops short of a human-readable
operational view. The operator persona (`requirements-v2.md §2`) needs to know:

- Whether all sidecars (git-shadow, merge-engine, HAL, rollout, sparse) are reachable.
- Current job queue depth, worker utilisation, completed/failed totals.
- LLM client auth failure counts and rate-limit hits.
- Active feature flags and configuration completeness warnings.
- Rollout trajectory state (when enabled).
- HAL capability registry snapshot.

The solution must be **read-only and additive** — no changes to the MCP protocol, existing routes,
sandbox runners, or JWT auth on `/mcp`.

### Relevant existing patterns

| Pattern | Detail |
|---------|--------|
| Auth | JWT HS256 Bearer token enforced by `authMiddleware` on `/mcp` and rollout routes |
| Origin enforcement | `originMiddleware` applied to all routes via `chain(…)` |
| Rate limiting | `rateLimitMiddleware` via per-IP token bucket store |
| Logging | Structured JSON via zerolog; correlation request IDs on every request |
| No metrics library | No Prometheus client or OTEL metric SDK present today |
| Sidecars | Connected via Unix Domain Sockets (UDS); no TCP health endpoints exposed |

---

## Decision

### 1. Auth mechanism — Separate dashboard bearer token

**Decision:** The dashboard routes use a dedicated `CWSO_DASHBOARD_TOKEN` environment variable,
**not** the MCP JWT secret.

At startup, `config.go` reads `CWSO_DASHBOARD_TOKEN`, computes its SHA-256 hash, and stores only
`DashboardTokenHash []byte` in `Config`. The raw token is never retained in memory beyond startup.
Incoming `Authorization: Bearer <token>` values on `/dashboard*` routes are hashed and
compared in constant time (`subtle.ConstantTimeCompare`).

**Rationale:**

- The MCP JWT (`JWT_SECRET`) is a full-privilege credential that can invoke all MCP tools. Issuing
  it to operators for a read-only dashboard violates the principle of least privilege.
- A separate token allows the dashboard to be deployed to CI smoke-test pipelines, monitoring
  scripts, and shared operator runbooks without exposing the ability to dispatch jobs or write files
  to shadow workspaces.
- The JWT `authMiddleware` validates issuer/subject claims from a signed token; this machinery is
  unnecessary overhead for a static pre-shared secret intended for operator tooling.
- SHA-256 hashing at startup means the plaintext token is never present in a heap dump, pprof
  output, or core file taken after startup.

If `CWSO_DASHBOARD_TOKEN` is unset or empty, both `/dashboard` and `/dashboard/status` respond with
`501 Not Implemented`, preventing accidental open exposure in environments that have not opted in.

### 2. Stats exposure pattern — `Stats()` snapshot method on `jobs.Manager`

**Decision:** Add a single exported `Stats() JobsSnapshot` method to `jobs.Manager` that returns an
immutable value-type snapshot of queue state. The dashboard reads this snapshot; it does not poll
the job store directly.

`JobsSnapshot` is a plain struct with exported fields:

```
Workers        int
QueueCapacity  int
QueueDepth     int
Active         int
TotalCompleted uint64
TotalFailed    uint64
```

**Rationale:**

- Direct access to the job store from the dashboard package would couple two packages across an
  internal boundary and force the dashboard to understand queue internals.
- A snapshot method is idiomatic Go for safe concurrent reads; the manager holds the lock for the
  duration of the copy, keeping the critical section small.
- The snapshot is a value type (no pointers), so the caller cannot observe partial updates after
  `Stats()` returns.
- `TotalCompleted` and `TotalFailed` are stored as `sync/atomic` `uint64` values inside the
  manager and read without acquiring the queue lock, keeping contention minimal on the hot path.

### 3. Client activity counters — In-memory atomic counters on the HTTP middleware layer

**Decision:** Client activity metrics (total requests, auth failures, rate-limit hits, per-tool call
counts) are tracked as `sync/atomic` `uint64` counters on a `ClientMetrics` struct owned by the
HTTP transport layer. The dashboard reads them via a snapshot method analogous to `Stats()`.

**Restart-reset behaviour:** These counters are process-scoped and reset to zero on every restart.
The dashboard JSON response includes `system.uptime_seconds` so consumers can interpret counts
relative to uptime. This limitation is documented in the `/dashboard/status` response schema
(`schemas/dashboard_status.json`) under a `"counters_reset_on_restart": true` field.

**Rationale:**

- An external store (Redis, SQLite) would introduce a new infrastructure dependency for metrics that
  are currently absent from the system. The feature plan explicitly defers Prometheus/persistent
  metrics to a later phase.
- `sync/atomic` operations are safe under arbitrarily high concurrency with no contention on the
  HTTP hot path (no mutex required).
- The operator's primary use case is *trend awareness during a live session* (is the auth failure
  count climbing?), not long-term time-series analysis. Restart-reset is acceptable for this use
  case.
- Persisted counters are listed as deferred scope (see §7 below).

### 4. Sidecar health checks — Ping via existing UDS client

**Decision:** Sidecar reachability is determined by attempting a lightweight no-op exchange on the
existing Unix Domain Socket connection for each sidecar (git-shadow, merge-engine, HAL, rollout,
sparse). Results are cached for 5 seconds at the dashboard aggregator layer to avoid adding latency
to burst reads.

TCP health probes are **not** introduced.

**Rationale:**

- All five sidecars communicate exclusively over UDS today. Adding TCP listeners solely for health
  checks would expand the attack surface, require port assignments, and add network configuration
  to `docker-compose.yml`.
- A UDS ping exercises the actual communication channel (same socket, same protocol framing), so a
  successful ping is evidence that the sidecar is both running *and* reachable via the path the
  orchestrator uses in production.
- The 5-second cache prevents the dashboard's health aggregate from becoming a denial-of-service
  vector against sidecars when the endpoint is polled rapidly (e.g. by a CI smoke-test loop).
- If a sidecar is not configured (e.g. `rollout` is disabled), the aggregator marks it
  `connected: false` with a `note` field rather than attempting a connection, matching the
  API contract draft in the feature plan.

### 5. HTML delivery — `embed.FS` embedded in the binary

**Decision:** The operator HTML page (`index.html`, inline CSS, inline JS) is embedded into the Go
binary via `//go:embed` and served by `net/http` from an `embed.FS`. No separate static file server
or CDN is used.

**Rationale:**

- A single self-contained binary is the existing deployment contract for CWSO's Go orchestrator.
  Introducing a separate static-file server would add an operational dependency (volume mount,
  separate container, path configuration) for a sub-10 KB asset.
- `embed.FS` is standard library (Go 1.16+); it adds no dependencies, no build toolchain changes,
  and no attack surface beyond what is already present.
- The HTML page makes a single fetch to `/dashboard/status` (same origin) and renders the response.
  All logic is inline; there are no external CDN requests, no NPM build step, and no JavaScript
  framework dependency — keeping the binary size impact negligible (<10 KB).
- Embedded assets cannot be accidentally deleted from the host filesystem or become stale relative
  to the binary version.

### 6. Route placement — `/dashboard` and `/dashboard/status` in `newHTTPHandler`

**Decision:** Both routes are registered in `newHTTPHandler` in `internal/transport/http.go`:

```
mux.Handle("/dashboard/status", mw(dashboardTokenMiddleware(cfg, log)(statusHandler)))
mux.Handle("/dashboard",        mw(dashboardTokenMiddleware(cfg, log)(htmlHandler)))
```

Where `mw` is the existing middleware chain (recover → request-ID → origin → security-headers →
rate-limit). `authMiddleware` (JWT) is **not** applied; instead `dashboardTokenMiddleware` performs
the SHA-256 token comparison described in §1.

**Rationale:**

- Placing routes inside `newHTTPHandler` keeps all HTTP surface area in one place, consistent with
  the existing pattern for `/mcp`, `/healthz`, and rollout routes.
- Applying the full `mw` chain (including origin enforcement and rate limiting) ensures the dashboard
  inherits the same DNS-rebinding protection and per-IP rate limiting that `/mcp` already has.
- A dedicated `dashboardTokenMiddleware` rather than reusing `authMiddleware` avoids coupling the
  dashboard auth path to JWT parsing logic that would be dead code for a static bearer secret.
- The `WithDashboardHandler` HTTP option (following the established `HTTPOption` / functional-option
  pattern in the transport package) is used to inject the dashboard handler from `server.go`,
  keeping the transport package free of a direct import dependency on the new `dashboard` package.

### 7. What is NOT included in this feature

The following are **explicitly out of scope** and deferred to a later phase:

| Item | Reason for deferral |
|------|-------------------|
| Prometheus `/metrics` endpoint | No Prometheus client in the dependency graph today; adding it doubles the binary's dependency footprint. Deferred to an observability phase. |
| Persistent counter storage | Adds infrastructure dependency (Redis/SQLite). Restart-reset counters are sufficient for the operator's live-session use case. |
| Grafana dashboard | External tooling; requires Prometheus as prerequisite. |
| Alert rules / PagerDuty integration | Out of scope for a read-only dashboard; requires an alerting backend. |
| OTEL metric SDK | OTEL *traces* are planned (NFR-5); metrics SDK is a separate concern deferred to the same observability phase. |
| `/dashboard` write operations | The dashboard is strictly read-only. No operator actions (pause queue, drain workers) are included. |

---

## Consequences

### Positive

- **Zero breaking changes.** All existing routes, auth flows, and MCP behaviour are unchanged.
- **Single binary deployment unchanged.** No new infrastructure dependencies; `embed.FS` keeps the
  operator HTML co-deployed with the binary automatically.
- **Least-privilege access.** Operators get a read-only token scoped only to dashboard routes;
  full MCP credentials are never shared with monitoring pipelines.
- **Low operational overhead.** In-memory counters and UDS pings require no external state store,
  no additional containers, and no persistent storage.
- **Incremental observability path.** The `Stats()` method and `ClientMetrics` struct establish
  clean interfaces that a future Prometheus exporter or OTEL metric SDK can consume without
  rearchitecting.

### Negative / Trade-offs

- **Counter loss on restart.** Cumulative totals (completed jobs, auth failures) reset to zero on
  every process restart. Operators who care about long-term trends must correlate with log
  aggregation.
- **No alerting.** The dashboard is pull-only; it does not push alerts. A degraded sidecar will not
  notify an on-call engineer — the operator must observe the dashboard or configure an external
  poller against `/dashboard/status`.
- **5-second sidecar health cache staleness.** A sidecar that disconnects and reconnects within the
  5-second window may show a stale `connected: true`. Acceptable for operator awareness; not
  suitable as a hard availability SLO signal.

### Security trade-offs

- **Pre-shared secret vs JWT.** The `CWSO_DASHBOARD_TOKEN` is a static bearer secret, not a
  short-lived signed JWT. It does not expire automatically. Operators must rotate it manually via
  environment variable update and process restart. This is a weaker credential lifecycle than the
  MCP JWT, which is acceptable given the dashboard is read-only — exposure of the token enables
  information disclosure but not system mutation.
- **Information disclosure risk.** The `/dashboard/status` JSON response exposes internal
  configuration (feature flags, socket paths, Go version, build commit). All fields in the response
  must be explicitly allowlisted in the dashboard aggregator; no reflection-based serialisation of
  `Config` is permitted. The security gate (T010) must verify no sensitive values (JWT secret,
  token hashes, sandbox credentials) appear in the response.
- **Origin enforcement.** Both dashboard routes inherit `originMiddleware`, which enforces the
  `CWSO_ALLOWED_ORIGINS` allowlist. This prevents DNS-rebinding attacks that could allow a
  malicious webpage to read the status JSON via a browser with a saved `Authorization` header or
  cookie. The dashboard does **not** set a session cookie; it requires the `Authorization` header
  on every request, which browsers do not send automatically — this further limits CSRF exposure.
- **Rate limiting.** Dashboard routes inherit `rateLimitMiddleware`, and — as of T202
  (F-C061-01) — are genuinely subject to its per-IP token bucket (`burst=10`, then `429`),
  the same limiter that governs `POST /mcp`. This was not always true: prior to T202, the
  limiter's exemption was scoped to "skip every non-`POST` request", a blanket rule intended
  only to keep `GET /mcp`'s long-lived SSE stream from being throttled (see D6). Because both
  dashboard routes are `GET`-only, they silently inherited that exemption too, leaving
  unauthenticated `GET /dashboard/status` brute-force attempts completely unthrottled — the
  audit (`security-v1.0-audit-v1.md`, finding F-C061-01, SECURITY:MEDIUM) live-verified 150
  wrong-token requests with zero `429`s. T202 replaced the blanket method-based exemption with
  a narrow, path-specific one (only `GET /mcp` bypasses the limiter); dashboard `GET` traffic
  now shares the same throttling behavior as any other route, closing the reconnaissance/
  brute-force vector this bullet originally (and, until T202, inaccurately) claimed to be
  mitigated. T202 also added structured logging of dashboard auth failures (failing IP + a
  generic message, never the attempted token), giving operators a log-based signal of an
  in-progress brute-force attempt rather than only an atomic counter.
- **Token entropy.** Because `CWSO_DASHBOARD_TOKEN` is a long-lived static secret (see the
  pre-shared-secret trade-off above) that is now the sole remaining barrier once rate limiting
  is in place, operators should generate it with a cryptographically random, high-entropy value
  — e.g. `openssl rand -hex 32` — rather than a short or predictable literal. Low-entropy tokens
  remain guessable well within the rate limiter's throttled request budget even after T202.

### Risks

| Risk | Mitigation |
|------|-----------|
| Dashboard JSON leaks JWT secret or token hash | Allowlist-only serialisation; security gate T010 blocks merge on violation |
| Auth bypass via header manipulation | `subtle.ConstantTimeCompare` prevents timing oracle; missing token → 501; wrong token → 401 |
| Sidecar ping storm from rapid polling | 5-second result cache in dashboard aggregator |
| Counter integer overflow | `uint64` max ≈ 1.8 × 10¹⁹; effectively unbounded for this use case |

---

## References

- `requirements-v2.md` — NFR-5 (observability), FR-7 (security), operator persona
- `docs/plans/feature-operator-dashboard.md` — feature plan, API contract draft, risk assessment
- `orchestrator/internal/transport/http.go` — `newHTTPHandler`, middleware chain, JWT auth pattern
- ADR-010: Rollout proxy boundary — prior art for `HTTPOption` functional-option pattern
