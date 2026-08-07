# Artifact: release-v0.6.0

## Metadata
- Producer: release-manager
- Created: 2026-08-06
- Based on: docs/artifacts/release-v0.5.2.md, docs/plans/feature-operator-dashboard.md, T001–T010
- develop tip: 8875eca
- Prior GA tag: v0.5.2

## Latest release: v0.6.0

## Release intent

v0.6.0 is a minor release introducing the **Operator Dashboard** — a read-only
observability surface for platform engineers. It adds two new HTTP endpoints
(`/dashboard/status` and `/dashboard`) protected by a separate operator bearer
token. All existing API contracts, MCP tools, and auth flows are unchanged.

## Install

```bash
# Docker Compose (recommended)
export CWSO_DASHBOARD_TOKEN=<your-operator-token>
docker compose -f deploy/docker-compose.yml up --pull always

# Container registry (individual images)
docker pull registry.gitlab.com/emage/cwso/orchestrator:v0.6.0
docker pull registry.gitlab.com/emage/cwso/git-shadow:v0.6.0
docker pull registry.gitlab.com/emage/cwso/merge-engine:v0.6.0
docker pull registry.gitlab.com/emage/cwso/rollout:v0.6.0
```

## Highlights

### Operator Dashboard (new feature)

CWSO previously had only `/healthz` (returns `ok` unconditionally). v0.6.0 adds:

**`GET /dashboard/status`** — machine-readable JSON snapshot covering:
- Sidecar connectivity (`git_shadow`, `merge_engine`, `hal`, `rollout`, `sparse`) via
  timeout-guarded Unix socket dial with 5 s cache
- Config snapshot: transport mode, sandbox runner, feature flags, config warnings
- Job queue: workers, queue depth/capacity, active jobs, completed/failed totals
- Client activity: total requests, auth failures, rate-limit hits, per-tool call counts
- Rollout pipeline: enabled state, active task count
- Top-level `overall` field: `healthy | degraded | unhealthy`

**`GET /dashboard`** — embedded HTML page that polls `/dashboard/status` every 10 s and
renders all panels without requiring a separate Grafana stack.

**Security:** Protected by `CWSO_DASHBOARD_TOKEN` (SHA-256 hashed at startup, constant-time
compare). Routes return `501 Not Implemented` when the token is unset. The dashboard token
is completely separate from the MCP JWT secret and grants read-only operator access only.

### Configuration

Set `CWSO_DASHBOARD_TOKEN` in the environment (or `docker-compose.yml`) before starting:

```bash
export CWSO_DASHBOARD_TOKEN=my-operator-token
curl -H "Authorization: Bearer my-operator-token" http://localhost:8080/dashboard/status
```

### JSON Schema

`schemas/dashboard_status.json` — full JSON Schema with `additionalProperties: false`
at all nesting levels, suitable for CI smoke-test validation.

## Changelog — v0.6.0

**Release Date:** 2026-08-06
**Previous Version:** v0.5.2

### New Features
- Add operator dashboard with `/dashboard/status` (JSON) and `/dashboard` (HTML) endpoints (T001–T010)
- Add `Stats()` method to `jobs.Manager` exposing live queue/worker/counter snapshot
- Add `ClientMetrics` in-memory counters (requests, auth failures, rate-limit hits, tool calls)
- Add `SidecarChecker` with UDS connectivity probing and 5 s cache
- Add `schemas/dashboard_status.json` JSON Schema with `additionalProperties: false`
- Add `CWSO_DASHBOARD_TOKEN` env var support in orchestrator and `docker-compose.yml`
- Add `ADR-011-operator-dashboard.md` documenting all architectural decisions

### Bug Fixes / CI
- Complete registry publishing for all four CWSO service images (`fix(ci)` T178/T179 — was in v0.5.x develop but now GA)

### No Breaking Changes
- All existing `/healthz`, `/mcp`, and `/rollout/*` routes are unchanged
- No changes to MCP tool contracts or JWT auth flows
- Dashboard routes are additive and disabled (501) when `CWSO_DASHBOARD_TOKEN` is unset

## Version rationale

New backward-compatible feature → MINOR increment: v0.5.2 → v0.6.0.
