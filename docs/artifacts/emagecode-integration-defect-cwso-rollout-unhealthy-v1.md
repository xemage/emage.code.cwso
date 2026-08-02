# Artifact: emagecode-integration-defect-cwso-rollout-unhealthy-v1.md

- Producer agent: devops-engineer (emage.code project)
- Task: T310 (emage.code), surfaced by T304 (emage.code)
- Created: 2026-07-31
- Based on: emage.code docs/plans/plan-016-pattern-a-hardening-and-phase23-poc-closure.md,
  emage.code docs/tasks/task-T304.md, emage.code docs/tasks/task-T310.md

## Scope / Context

This artifact reports a confirmed CWSO-core defect found while independently building and
running CWSO's own documented Docker Compose profile from a consuming project (emage.code).
It is handed to CWSO's own team as a plain evidence report — it is not a GitLab issue, and no
CWSO source code has been modified or patched as part of producing this report.

The emage.code project builds and runs CWSO's 5-service stack (`git-shadow`, `merge-engine`,
`orchestrator`, `rollout`, `sia-executor`) from
`/home/emage/Code/emage/emage.code/deploy/docker-compose-t226.yml`, which builds the `rollout`
image from this repository's own `deploy/Dockerfile.rollout` (context: this repo root). All
other services started and stabilized correctly; `rollout` did not. This was independently
re-verified twice (~2 minutes apart) with unchanged results both times — this is not a
transient effect.

## Commands Run

Run from `/home/emage/Code/emage/emage.code`:

```
cd /home/emage/Code/emage/CWSO/orchestrator && /usr/local/go/bin/go build ./... ; echo "go_build_exit=$?"
# → go_build_exit=0 (host-side Go build passes cleanly)

cd /home/emage/Code/emage/emage.code
source deploy/t226-phase2.env 2>/dev/null || true
docker compose -f deploy/docker-compose-t226.yml build   # succeeded, all 4 images built clean
docker compose -f deploy/docker-compose-t226.yml up -d   # all 5 containers started
sleep 15
docker compose -f deploy/docker-compose-t226.yml ps
docker ps --filter "name=cwso-" --format "table {{.Names}}\t{{.Status}}"
```

## Evidence

### Container status (verbatim, `docker ps --filter "name=cwso-"`)

```
NAME                IMAGE                   COMMAND                  SERVICE        CREATED         STATUS                     PORTS
cwso-git-shadow     cwso/git-shadow:dev     "/usr/bin/tini -- /u…"   git-shadow     2 minutes ago   Up 2 minutes
cwso-merge-engine   cwso/merge-engine:dev   "/usr/bin/tini -- /u…"   merge-engine   2 minutes ago   Up 2 minutes
cwso-orchestrator   cwso/orchestrator:dev   "/sbin/tini -- /usr/…"   orchestrator   2 minutes ago   Up 2 minutes (healthy)     0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp
cwso-rollout        cwso/rollout:dev        "/usr/bin/tini -- /u…"   rollout        2 minutes ago   Up 2 minutes (unhealthy)   0.0.0.0:8787->8787/tcp, [::]:8787->8787/tcp
cwso-sia-executor   python:3.11-slim        "/bin/bash -c 'set -…"   sia-executor   2 minutes ago   Up 2 minutes
```

`cwso-orchestrator`, `cwso-git-shadow`, and `cwso-merge-engine` are all fine. The latter two
have no healthcheck defined in their images, so plain `Up` (no health annotation) is expected
and correct for them. `cwso-rollout` is the one service reporting `(unhealthy)`, and it stayed
unhealthy with no recovery across the full observation window (`FailingStreak: 6`).

### `docker compose -f deploy/docker-compose-t226.yml logs rollout` (full, verbatim)

```
cwso-rollout  | {"timestamp":"2026-07-31T17:42:36.557575Z","level":"INFO","fields":{"message":"trajectory Parquet store enabled","written":0},"target":"cwso_rollout"}
cwso-rollout  | {"timestamp":"2026-07-31T17:42:36.557687Z","level":"ERROR","fields":{"message":"trajectory store writer exited","error":"create rollout store \"./rollout_store\""},"target":"cwso_rollout::store"}
cwso-rollout  | {"timestamp":"2026-07-31T17:42:36.557917Z","level":"INFO","fields":{"message":"cwso-rollout IPC ready","socket_path":"\"/run/cwso/rollout.sock\""},"target":"cwso_rollout::ipc"}
cwso-rollout  | {"timestamp":"2026-07-31T17:42:36.559679Z","level":"INFO","fields":{"message":"starting rollout proxy","bind":"0.0.0.0:8787","upstream":"http://127.0.0.1:18080"},"target":"cwso_rollout"}
cwso-rollout  | {"timestamp":"2026-07-31T17:42:36.559828Z","level":"INFO","fields":{"message":"cwso-rollout proxy listening","bind":"0.0.0.0:8787"},"target":"cwso_rollout::proxy"}
```

### `docker inspect cwso-rollout --format '{{json .Config.Healthcheck}}'` (verbatim)

```json
{"Test":["CMD","curl","-f","http://127.0.0.1:8787/v1/models"],"Interval":10000000000,"Timeout":3000000000,"Retries":5}
```

### `docker inspect cwso-rollout --format '{{json .State.Health}}'` (probe history, verbatim, all 5 recorded attempts identical)

```json
{
  "Status": "unhealthy",
  "FailingStreak": 6,
  "Log": [
    {"Start":"2026-07-31T17:42:56Z","End":"2026-07-31T17:42:56Z","ExitCode":22,"Output":"curl: (22) The requested URL returned error: 405\n"},
    {"Start":"2026-07-31T17:43:06Z","End":"2026-07-31T17:43:06Z","ExitCode":22,"Output":"curl: (22) The requested URL returned error: 405\n"},
    {"Start":"2026-07-31T17:43:16Z","End":"2026-07-31T17:43:16Z","ExitCode":22,"Output":"curl: (22) The requested URL returned error: 405\n"},
    {"Start":"2026-07-31T17:43:26Z","End":"2026-07-31T17:43:26Z","ExitCode":22,"Output":"curl: (22) The requested URL returned error: 405\n"},
    {"Start":"2026-07-31T17:43:36Z","End":"2026-07-31T17:43:36Z","ExitCode":22,"Output":"curl: (22) The requested URL returned error: 405\n"}
  ]
}
```

### Relevant `rollout` service definition (from the consuming compose file)

Source: `/home/emage/Code/emage/emage.code/deploy/docker-compose-t226.yml` (healthcheck/env excerpt):

```yaml
  rollout:
    image: cwso/rollout:dev
    build:
      context: /home/emage/Code/emage/CWSO
      dockerfile: deploy/Dockerfile.rollout
    environment:
      CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED: "true"
      CWSO_ROLLOUT_TRAJECTORY_STORE_PATH: "/data/parquet-store"
      ...
    volumes:
      - "/tmp/t226-parquet-store:/data/parquet-store"
      - cwso-runtime:/run/cwso
    read_only: false
    healthcheck:
      test: ["CMD", "curl", "-f", "http://127.0.0.1:8787/v1/models"]
      interval: 10s
      timeout: 3s
      retries: 5
```

## Environment

- `docker --version` → Docker version 29.6.2, build dfc4efb
- Host OS: Linux Lap3074 6.18.33.2-microsoft-standard-WSL2 (Ubuntu 24.04.4 LTS, WSL2)
- `go version` (via `/usr/local/go/bin/go`) → go1.26.3 linux/amd64
- All 4 CWSO images built clean from this repo's own Dockerfiles (`deploy/Dockerfile.*`); the
  host-side `go build ./...` in `orchestrator/` also passes cleanly with exit code 0. The build
  toolchain and Go sources are not implicated — this is a runtime behavior defect specific to
  the `rollout` (Rust) binary and/or its documented healthcheck contract.

## Root Cause Candidates (unconfirmed)

Neither candidate below was verified by reading `cwso-rollout`'s Rust source in this session.
Both are inferred from the log and healthcheck evidence above and should be treated as leads
for investigation, not as confirmed root causes.

1. **Healthcheck method/path mismatch.** `curl -f http://127.0.0.1:8787/v1/models` issues a
   bare `GET` and receives HTTP `405 Method Not Allowed` from the rollout service's own
   `/v1/models` endpoint on every probe (`curl: (22) The requested URL returned error: 405`).
   Either the healthcheck should use a different HTTP method (e.g. `POST`), a different path, or
   the `/v1/models` endpoint itself needs to accept `GET` for a lightweight liveness/readiness
   probe.

2. **Trajectory store path mismatch.** The log line `"error":"create rollout store
   \"./rollout_store\""` shows the binary attempting to create a store at the hardcoded relative
   path `./rollout_store`, which does not match the `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH=/data/parquet-store`
   environment variable the compose file sets (and mounts as a writable volume at
   `/data/parquet-store`). This suggests the Rust binary either ignores that env var for this
   specific writer's path resolution, or there is a naming/wiring mismatch between the
   configured path and the path actually used at startup.

The `/v1/models` 405 and the trajectory-store write error are logged independently and may be
unrelated to each other — do not assume they share a single root cause without verifying against
source.

## Impact

- `cwso-rollout` never reaches a `healthy` state under Docker's healthcheck, which blocks any
  consuming project's orchestration logic that gates on container health (e.g.
  `depends_on: condition: service_healthy`) from ever proceeding past the rollout service.
- The proxy itself does appear to start and bind successfully (`cwso-rollout proxy listening`,
  `bind":"0.0.0.0:8787"`), so the service may be functionally reachable on other endpoints even
  while reporting unhealthy — this was not verified beyond the healthcheck probe itself.
- The trajectory store writer exits immediately on startup with an error, meaning trajectory
  Parquet persistence (`CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED=true`) is silently non-functional
  in this configuration, independent of the healthcheck issue.

## Source plan/task reference

- emage.code plan: `docs/plans/plan-016-pattern-a-hardening-and-phase23-poc-closure.md`
- emage.code task that found the defect: `docs/tasks/task-T304.md` (real Docker build +
  healthcheck verification of `deploy/docker-compose-t226.yml`)
- emage.code task that produced this handoff: `docs/tasks/task-T310.md`
