# Artifact: feature-operator-dashboard-requirements-v1.md

> Producer: product-owner · Based on: `docs/plans/feature-operator-dashboard.md`, `docs/artifacts/requirements-v2.md` · Date: 2026-08-06
> Task: T001 · Status: **COMPLETE** · Immutable — revisions create `feature-operator-dashboard-requirements-v2.md`

---

## 1. Feature Overview

CWSO currently exposes a `/healthz` endpoint that returns `"ok"` unconditionally, regardless of whether any sidecar (git-shadow, merge-engine, HAL, rollout, sparse) is reachable. Platform engineers operating CWSO in production have no runtime visibility into system health, active job load, LLM client behaviour, feature-flag state, or the rollout/trajectory pipeline — all of which are required to diagnose incidents, respond to degradation, and validate deployments. The **Operator Dashboard** adds two read-only endpoints — `GET /dashboard/status` (machine-readable JSON) and `GET /dashboard` (human-readable embedded HTML) — protected by a separate bearer token, that surface an authoritative operational snapshot of the CWSO orchestrator and all of its sidecars without modifying any existing MCP API, auth flow, or tool contract.

---

## 2. Primary Persona

**Platform Engineer / Operator** — responsible for deploying, operating, and monitoring CWSO in a Docker Compose or KVM environment. This persona needs to verify system health at deploy-time (CI smoke tests), triage incidents (which sidecar is down, which job is stuck), and confirm that feature flags and config are consistent with the intended rollout state. They do not have or need full MCP JWT tokens; they require a narrower, lower-privilege credential for read-only visibility.

---

## 3. User Story

> As a **platform engineer operating CWSO**,
> I want a **read-only operator dashboard** that shows me the health of all sidecars, current job activity, client authentication stats, active feature flags, and rollout pipeline status in a single request,
> so that I can **detect degraded states, diagnose incidents, and verify deployments without tailing logs or attaching a debugger**.

---

## 4. Acceptance Criteria

### AC-01 — Dashboard routes exist and respond correctly

1. `GET /dashboard/status` with a valid `Authorization: Bearer <CWSO_DASHBOARD_TOKEN>` header returns HTTP 200 with `Content-Type: application/json` and a JSON body that validates against `schemas/dashboard_status.json`.
2. `GET /dashboard` with a valid `Authorization: Bearer <CWSO_DASHBOARD_TOKEN>` header returns HTTP 200 with `Content-Type: text/html` and an HTML page that references `/dashboard/status` to render operator data.
3. Both endpoints return HTTP 401 when the `Authorization` header is absent or carries an incorrect token.
4. Both endpoints return HTTP 501 when `CWSO_DASHBOARD_TOKEN` is not set in the environment at startup, preventing accidental open exposure.

### AC-02 — System health aggregation (sidecar reachability)

5. The JSON response includes a `sidecars` object with an entry for each of the five named sidecars: `git_shadow`, `merge_engine`, `hal`, `rollout`, and `sparse`.
6. Each sidecar entry carries a boolean `connected` field that reflects whether the orchestrator can reach the sidecar's Unix socket; it does NOT unconditionally return `true`.
7. When at least one sidecar has `connected: false`, the top-level `overall` field is `"degraded"` or `"unhealthy"` (not `"healthy"`).
8. When all sidecars have `connected: true` and no config warnings are present, `overall` is `"healthy"`.
9. Sidecar connectivity is evaluated lazily and cached for no more than 5 seconds, so repeated rapid requests do not generate sidecar load.

### AC-03 — Config and feature-flag visibility

10. The JSON response includes a `config` object with: the active `transport` mode (`stdio` or `http`), the active `sandbox_runner` tier, and a `feature_flags` object listing each named flag with its current boolean value.
11. If any required environment variable is unset at startup, the `config.warnings` array contains at least one human-readable entry naming the missing variable; the array is empty when all required vars are present.
12. No secret value (JWT secret, dashboard token, TLS private key, database password) appears anywhere in the `config` subtree; only the presence or absence of optional vars is signalled.

### AC-04 — Job activity metrics

13. The JSON response includes a `jobs` object containing: `workers` (configured pool size), `queue_capacity`, `queue_depth` (current depth), `active` (jobs currently executing), `total_completed`, and `total_failed` — all as non-negative integers.
14. `active` reflects the count of jobs with status `in_progress` at the moment the snapshot is taken; it is sourced from `jobs.Manager.Stats()` and is not a cached constant.
15. `total_completed` and `total_failed` increment correctly under concurrent load; all counter mutations use atomic operations to prevent data races (verified by running `go test -race ./internal/jobs/...`).

### AC-05 — Client health (auth failures, tool call counts)

16. The JSON response includes a `clients` object with: `total_requests`, `auth_failures`, `rate_limit_hits`, and a `tool_calls` map keyed by MCP tool name with per-tool invocation counts.
17. `auth_failures` increments by 1 for each inbound request that fails JWT or Bearer validation on any endpoint; it does not increment for requests that never reached auth middleware.
18. `tool_calls` is populated only for tools that have been invoked at least once since the last restart; tools with zero calls are omitted from the map.
19. All client counters reset to zero on orchestrator restart (in-memory only; no persistence).

### AC-06 — Rollout / learning pipeline status

20. The JSON response includes a `rollout` object. When rollout is disabled (`CWSO_ROLLOUT_ENABLED=false` or env var absent), the object contains `{ "enabled": false }` only; no further fields are present.
21. When rollout is enabled, the `rollout` object additionally contains: `active_tasks` (count), `trajectory_capture_rate` (captures per minute, float), `capture_drops` (cumulative count of drops when queue was saturated), and `last_reward_signal` (RFC 3339 timestamp or `null`).
22. `capture_drops` is non-zero only when the trajectory queue has been saturated since the last restart; it never decrements.

### AC-07 — Dashboard authentication (separate from MCP JWT)

23. `CWSO_DASHBOARD_TOKEN` is a distinct configuration field from `CWSO_JWT_SECRET`; the two are never aliases of each other, and setting one to the value of the other is not required.
24. A valid MCP JWT token does NOT grant access to `/dashboard` or `/dashboard/status`; only the dashboard bearer token is accepted on these routes.
25. The dashboard bearer token is never embedded in the HTML page returned by `GET /dashboard` (not in source, comments, or JavaScript variables).
26. The `CWSO_DASHBOARD_TOKEN` value is stored server-side as a SHA-256 hash for constant-time comparison; it is not logged, not included in JSON responses, and not stored in plaintext in any config dump.

### AC-08 — No secret or sensitive data leakage in JSON response

27. A security review (T010) confirms that none of the following appear in the `GET /dashboard/status` JSON body at any verbosity level: JWT secrets, dashboard tokens, TLS private key material, shadow workspace internal UUIDs beyond counts, individual user identifiers, or filesystem paths outside the declared socket paths.
28. Socket path values in `sidecars[*].socket` contain only the basename of the socket file if the full path contains a system-specific prefix (e.g. `/run/cwso/`); they MUST NOT expose home directory paths, container-internal overlays, or secrets-mount paths.
29. The JSON schema at `schemas/dashboard_status.json` enumerates all permitted top-level keys; any key not in the schema is rejected by the schema validator in tests (`additionalProperties: false` at all levels).

---

## 5. Out of Scope

The following are explicitly **not** included in this feature:

- **Write operations** — the dashboard does not expose any endpoint that modifies system state, toggles feature flags, cancels jobs, or resets counters.
- **Persistent metrics / time-series storage** — all counters are in-memory and reset on restart; integration with Prometheus, Grafana, or any TSDB is a separate feature.
- **Per-workspace or per-agent drill-down** — individual shadow workspace details, agent session histories, and sandbox execution logs are not surfaced.
- **MCP JWT token management** — the dashboard does not issue, revoke, or inspect MCP JWT tokens.
- **Multi-node / distributed view** — the dashboard reflects the state of the single orchestrator process it is served by; aggregation across multiple CWSO nodes is out of scope.
- **Alert routing or notification** — the dashboard is a passive read endpoint; no alerting, webhooks, or PagerDuty integration.
- **Real-time streaming / websocket updates** — the dashboard is polled; live push of status changes is not included.
- **Rollout trajectory payload inspection** — the dashboard shows rollout pipeline counters only; individual trajectory records and model outputs are not exposed.

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-D1 | Response latency for `/dashboard/status` | p95 < 100 ms under normal load (sidecar checks use 5 s cache) |
| NFR-D2 | Sidecar check cache TTL | ≤ 5 seconds; never serves stale data older than 5 s |
| NFR-D3 | Binary size impact from embedded HTML | `GET /dashboard` HTML template ≤ 10 KB uncompressed |
| NFR-D4 | Race safety | All counter mutations use `sync/atomic`; `go test -race` passes on dashboard package |
| NFR-D5 | No breaking changes | All existing endpoints (`/healthz`, `/mcp`, `/rollout/*`) are unchanged and unaffected |
| NFR-D6 | Dashboard disabled by default | Routes return 501 when `CWSO_DASHBOARD_TOKEN` is unset; no opt-out required |
