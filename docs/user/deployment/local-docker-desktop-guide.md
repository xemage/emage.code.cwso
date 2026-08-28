# CWSO Local Docker Desktop Deployment Guide

**Version:** 1.2
**Last updated:** 2026-08-04
**Based on:** `docs/tasks/task-T313.md`, `docs/artifacts/t305-deployment-guide-validation-report-v1.md`,
`deploy/docker-compose-t226.yml`, `scripts/deploy/cwso-docker-desktop.sh`
**Environment:** Docker Desktop (Mac, Windows, Linux)
**Minimum requirements:** Docker 20.10+, 4GB RAM, 2GB disk space

---

## Quick Start (5 minutes)

```bash
# 1. Clone or navigate to emage.code
cd ~/Code/emage/emage.code

# 2. Run the automated setup
bash scripts/deploy/cwso-docker-desktop.sh

# 3. Verify it's running (liveness check — no auth required)
curl http://localhost:8080/healthz

# Expected response: ok
# Done! CWSO is now running locally
```

**Note:** `GET /healthz` is the public, unauthenticated liveness route. `GET /health` (no `z`) is a
*different*, authenticated endpoint — it requires a valid bearer JWT and returns
`401 Unauthorized` / `missing bearer token` without one. See Detailed Setup Step 2 for obtaining a
token and Step 4 for testing the authenticated endpoint.

---

## Detailed Setup

### Prerequisites

- **Docker Desktop installed** — [Download here](https://www.docker.com/products/docker-desktop)
- **Docker running** — Start Docker Desktop application
- **bash or zsh shell** — For running setup scripts
- **4GB minimum RAM allocated to Docker** — Check Docker preferences
- **Ports 8080, 8787 available** — Default CWSO ports

### Step 1: Prepare Environment

```bash
# Navigate to repository
cd ~/Code/emage/emage.code

# Ensure host parquet store path exists (default from deploy/t226-phase2.env)
mkdir -p /tmp/t226-parquet-store

# Load runtime environment defaults for this shell session
set -a
source deploy/t226-phase2.env
set +a
```

### Step 2: Configure JWT Secret

**Option B (recommended default — use this unless you know you need Option A):**

```bash
export JWT_SECRET=$(head -c 32 /dev/urandom | base64)
echo "JWT_SECRET is generated for ad-hoc local tests in this shell"
```

This only generates a fresh secret; it never reads an existing credential file. Use it for
client-side local test tokens only.

**Option A (human operators only — reuse the shared CWSO dev JWT):**

```bash
# Run from the emage.code repo root — the source file
# lives in a sibling repository checkout, not inside emage.code itself.
cd ~/Code/emage/emage.code
source ../CWSO/.env.jwt.dev
export JWT_SECRET
```

- This only works if you also have the `CWSO` repository checked out as a sibling directory of
  `emage.code` (i.e. both under the same parent, so `../CWSO/.env.jwt.dev` resolves from the
  `emage.code` repo root). It is a separate repository/checkout, not a file inside `emage.code`.
- **AI coding agents MUST NOT use Option A.** This repo's own
  `.claude/rules/security-guidelines.md` ("Secret and credential files") hard-blocks any agent from
  reading, `cat`-ing, or `source`-ing `.env`/`.env.*` files, including `.env.jwt.dev` — an agent
  attempting this step will be refused by its own tooling. Agents must use Option B above, which
  requires no access to any existing secret file.

**⚠️ Important:**
- The `.env.jwt.dev` JWT (Option A) is **for development only** and reachable only by a human
  operator with the `CWSO` repo checked out alongside this one
- Never use development JWTs in production
- Store JWT secrets securely (use env vars, vault, or secrets management)

### Step 3: Start CWSO Stack

```bash
# Start all containers
docker compose -f deploy/docker-compose-t226.yml up -d

# Verify all containers are running
docker compose -f deploy/docker-compose-t226.yml ps

# Expected output (actual containers are tini-wrapped compiled Go/Rust binaries,
# NOT Python — the guide previously showed "python3 ..." for all 4, which is
# inaccurate; only the non-target sia-executor helper service in this compose
# file happens to run Python):
# NAME                COMMAND                    SERVICE        STATUS
# cwso-orchestrator   "/sbin/tini -- /usr/…"    orchestrator   Up 2 seconds (healthy)
# cwso-rollout        "/usr/bin/tini -- /u…"    rollout        Up 2 seconds (healthy)
# cwso-git-shadow     "/usr/bin/tini -- /u…"    git-shadow     Up 2 seconds
# cwso-merge-engine   "/usr/bin/tini -- /u…"    merge-engine   Up 2 seconds
```

### Step 4: Verify Deployment

```bash
# Check orchestrator liveness (no auth required)
curl -i http://localhost:8080/healthz

# Expected response:
# HTTP/1.1 200 OK
# ok
#
# Note: GET /health (no "z") is a *different*, authenticated endpoint. It requires
# a valid bearer JWT in the Authorization header and returns 401 Unauthorized /
# "missing bearer token" without one — it is not a liveness check substitute.

# Check rollout proxy liveness (no auth required)
curl -i http://localhost:8787/healthz

# Expected response:
# HTTP/1.1 200 OK
# {"status":"ok"}
#
# Note: GET /health on the rollout proxy is POST-only and returns
# 405 Method Not Allowed ({"error":{"message":"only POST is supported"}}) — use
# /healthz for liveness checks instead.

# Test JWT authentication
TOKEN=$(python3 -c "
import jwt
import json
secret = '$JWT_SECRET'
payload = {'sub': 'test-user', 'role': 'admin'}
token = jwt.encode(payload, secret, algorithm='HS256')
print(token)
")

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/status
```

---

## Common Tasks

### Viewing Logs

```bash
# View all logs (follow mode)
docker compose -f deploy/docker-compose-t226.yml logs -f

# View specific service
docker compose -f deploy/docker-compose-t226.yml logs -f orchestrator

# View last 50 lines
docker compose -f deploy/docker-compose-t226.yml logs --tail=50 rollout
```

### Stopping CWSO

```bash
# Stop all containers (data preserved)
docker compose -f deploy/docker-compose-t226.yml stop

# Restart containers
docker compose -f deploy/docker-compose-t226.yml start
```

### Removing CWSO (Clean Slate)

```bash
# Stop and remove containers
docker compose -f deploy/docker-compose-t226.yml down

# Remove volumes (⚠️ deletes all data)
docker compose -f deploy/docker-compose-t226.yml down -v
```

### Updating CWSO Image

```bash
# Pull pinned images from the registry (v0.5.2 by default in compose)
docker compose -f deploy/docker-compose-t226.yml pull

# Recreate containers with the pulled images
docker compose -f deploy/docker-compose-t226.yml up -d --force-recreate

# Or via convenience script
bash scripts/deploy/cwso-docker-desktop.sh --update
```

### Testing with Sample Requests

```bash
# Generate JWT token
TOKEN=$(python3 -c "
import jwt
secret = '$JWT_SECRET'
payload = {'sub': 'test-agent', 'role': 'executor'}
print(jwt.encode(payload, secret, algorithm='HS256'))
")

# Sample dispatch request
curl -X POST http://localhost:8080/dispatch \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_role": "backend-developer",
    "task_id": "T001",
    "objective": "Test dispatch",
    "context": "Local Docker test"
  }'
```

---

## Troubleshooting

### Port Already in Use

**Problem:** `Port 8080 is already allocated`

**Solutions:**
```bash
# Option 1: Find and stop the process using the port
lsof -i :8080
kill -9 <PID>

# Option 2: Use different ports
# Edit deploy/docker-compose-t226.yml
# Change ports: "8080:8080" to "8081:8080"
# Then rebuild

# Option 3: Check if CWSO is already running
docker compose -f deploy/docker-compose-t226.yml ps
```

### Out of Memory

**Problem:** `OOMKilled` or containers crash with memory errors

**Solutions:**
1. Increase Docker memory allocation:
   - Mac/Windows: Docker Desktop → Preferences → Resources → Memory → increase to 6-8GB
  - Linux: Increase available memory or reduce container limits in deploy/docker-compose-t226.yml

2. Reduce container resource limits:
```yaml
# In deploy/docker-compose-t226.yml
services:
  orchestrator:
    mem_limit: 1024m  # Reduce from default 2G
    mem_reservation: 512m
```

### JWT Authentication Failed

**Problem:** `401 Unauthorized` or `Invalid token` errors

**Solutions:**
```bash
# 1. Verify JWT is in environment
echo "$JWT_SECRET"

# 2. Verify JWT matches in requests
TOKEN=$(python3 -c "import jwt; print(jwt.encode({'sub': 'test'}, '$(grep JWT_SECRET .env | cut -d= -f2)', algorithm='HS256'))")
echo "Token: $TOKEN"

# 3. Check logs for token validation errors
docker compose -f deploy/docker-compose-t226.yml logs orchestrator | grep -i "token\|auth"

# 4. Regenerate JWT if corrupted (recommended: Option B from "Configure JWT
#    Secret" above)
export JWT_SECRET=$(head -c 32 /dev/urandom | base64)
docker compose -f deploy/docker-compose-t226.yml restart
```

### Network Connection Issues

**Problem:** `Connection refused` or `Cannot connect to localhost:8080`

**Solutions:**
```bash
# 1. Verify containers are running
docker compose -f deploy/docker-compose-t226.yml ps

# 2. Check network configuration
docker network ls
docker network inspect <cwso-network>

# 3. Inspect container networking
docker inspect cwso-orchestrator | grep -A 10 NetworkSettings

# 4. Test from the host against the published port — do NOT exec into the
#    container. The orchestrator image is a minimal, distroless-style Go binary
#    image and ships no `curl` (or shell tooling); `docker compose exec orchestrator
#    curl ...` fails with "executable file not found in $PATH".
curl -i http://localhost:8080/healthz

# 5. Restart containers
docker compose -f deploy/docker-compose-t226.yml restart
```

### Containers Won't Start

**Problem:** `docker compose -f deploy/docker-compose-t226.yml up` fails with errors

**Solutions:**
```bash
# 1. Check logs for errors
docker compose -f deploy/docker-compose-t226.yml logs

# 2. Verify images exist
docker images | grep cwso

# 3. Pull images explicitly
docker compose -f deploy/docker-compose-t226.yml pull

# 4. Rebuild images
docker compose -f deploy/docker-compose-t226.yml -f deploy/docker-compose-t226.build.yml build --no-cache

# 5. Check configuration
docker compose -f deploy/docker-compose-t226.yml config | grep -A 5 orchestrator

# 6. Full reset
docker compose -f deploy/docker-compose-t226.yml down -v
docker system prune
docker compose -f deploy/docker-compose-t226.yml pull
docker compose -f deploy/docker-compose-t226.yml up -d
```

---

## Performance Tuning

**Note:** The `mem_limit`/`cpus` examples below are aspirational sizing guidance only — they are
**not** currently applied in `deploy/docker-compose-t226.yml` (the real compose file defines zero
`mem_limit`/`cpus:` keys on any service today). Add these keys yourself under the relevant service
block in your `deploy/docker-compose-t226.yml` copy if you want to enforce them locally.

### For Development (Default)
```yaml
# Example resource limits — not currently set in docker-compose-t226.yml
orchestrator:
  mem_limit: 2g
  cpus: 1.0
rollout:
  mem_limit: 1g
  cpus: 0.5
```

Adequate for:
- Single-developer workstations
- Testing and validation
- PoC deployments

### For Higher Load Testing
```yaml
orchestrator:
  mem_limit: 4g
  cpus: 2.0
rollout:
  mem_limit: 2g
  cpus: 1.0
```

Increase Docker memory allocation to 8GB before applying these settings.

---

## Monitoring and Healthchecks

### Built-in Health Checks

```bash
# Check orchestrator liveness (no auth required)
curl http://localhost:8080/healthz

# Check rollout proxy liveness (no auth required)
curl http://localhost:8787/healthz

# Check both published HTTP liveness routes together
for port in 8080 8787; do
  echo "Port $port:"
  curl -s http://localhost:$port/healthz
  echo ""
done
```

**Note:** `git-shadow` (previously documented as port 8788) and `merge-engine` (previously
documented as port 8789) are **not** HTTP services and publish no host port at all — they
communicate over Unix domain sockets on the shared `cwso-runtime` volume. There is nothing to
`curl` for either. See "Container Status" below for how to check their liveness instead.

### Container Status

```bash
# Continuous monitoring (orchestrator + rollout — the two services with published ports)
docker stats cwso-orchestrator cwso-rollout

# git-shadow and merge-engine have no HTTP endpoint and no defined healthcheck;
# "Up" in the STATUS column is the liveness signal for these two services
docker compose -f deploy/docker-compose-t226.yml ps git-shadow merge-engine

# Memory usage over time
docker stats cwso-orchestrator --no-stream

# Disk usage
docker system df
```

### Logs Analysis

```bash
# View logs with timestamps
docker compose -f deploy/docker-compose-t226.yml logs --timestamps

# Search for errors
docker compose -f deploy/docker-compose-t226.yml logs | grep -i error

# Stream specific service logs
docker compose -f deploy/docker-compose-t226.yml logs -f orchestrator --tail 20
```

---

## Data Persistence

### Parquet Store
The Parquet trajectory store is created at `/tmp/t226-parquet-store` by default.

```bash
# Backup trajectory data
cp -r /tmp/t226-parquet-store ~/backup/cwso-trajectories-$(date +%Y%m%d)

# Verify backup
ls -lah ~/backup/cwso-trajectories-*
```

### Volume Management

`deploy/docker-compose-t226.yml` defines exactly one named volume, `cwso-runtime` — a Unix-socket
sharing volume mounted at `/run/cwso` in `orchestrator`, `git-shadow`, `merge-engine`, and
`rollout` (not a per-service "orchestrator-data" volume). Its actual on-host name is
project-prefixed and varies by which compose project name is active
(e.g. `cwso-t226_cwso-runtime`, `cwso_cwso-runtime`, or `cwso-local-dev_cwso-runtime` if deployed
via this guide's Step 1) — find yours with the `docker volume ls` command below before inspecting
or backing it up.

```bash
# List Docker volumes
docker volume ls | grep cwso

# Inspect volume (substitute the actual project-prefixed name from the command above)
docker volume inspect <project>_cwso-runtime

# Backup volume data
docker run --rm -v <project>_cwso-runtime:/data -v $(pwd):/backup alpine tar czf /backup/volume-backup.tar.gz /data

# Restore volume from backup
docker run --rm -v <project>_cwso-runtime:/data -v $(pwd):/backup alpine tar xzf /backup/volume-backup.tar.gz -C /
```

---

## Backup and Restore

### Full Backup

```bash
# Create backup directory
mkdir -p ~/backups/cwso-local
BACKUP_DIR=~/backups/cwso-local/cwso-backup-$(date +%Y%m%d-%H%M%S)
mkdir -p $BACKUP_DIR

# Backup volumes (real volume is cwso-runtime, project-prefixed — e.g.
# cwso-local-dev_cwso-runtime if deployed via this guide's Step 1; see
# "Volume Management" above)
docker volume inspect cwso-local-dev_cwso-runtime 2>/dev/null && \
  docker run --rm -v cwso-local-dev_cwso-runtime:/data -v $BACKUP_DIR:/backup \
  alpine tar czf /backup/volumes.tar.gz /data

# Backup configuration
cp deploy/docker-compose-t226.yml $BACKUP_DIR/
cp deploy/t226-phase2.env $BACKUP_DIR/

# Backup trajectories
cp -r /tmp/t226-parquet-store $BACKUP_DIR/trajectories 2>/dev/null || true

echo "Backup complete: $BACKUP_DIR"
```

### Restore from Backup

```bash
# Identify backup to restore
ls -la ~/backups/cwso-local/

BACKUP_DIR=~/backups/cwso-local/cwso-backup-<timestamp>

# Stop CWSO
docker compose -f deploy/docker-compose-t226.yml down -v

# Restore volumes (real volume name is cwso-runtime, project-prefixed — see
# "Volume Management" above)
docker volume create cwso-local-dev_cwso-runtime
docker run --rm -v cwso-local-dev_cwso-runtime:/data -v $BACKUP_DIR:/backup \
  alpine tar xzf /backup/volumes.tar.gz -C /

# Restore configuration (if needed)
cp $BACKUP_DIR/t226-phase2.env deploy/t226-phase2.env

# Restore trajectories
mkdir -p /tmp/t226-parquet-store
cp -r $BACKUP_DIR/trajectories/* /tmp/t226-parquet-store/ 2>/dev/null || true

# Restart CWSO
docker compose -f deploy/docker-compose-t226.yml up -d

echo "Restore complete"
```

---

## Next Steps

### After Successful Deployment
1. ✅ Verify all health checks pass
2. ✅ Test sample dispatch requests
3. ✅ Review logs for any warnings
4. ✅ Set up monitoring (if needed)
5. ✅ Proceed to production deployment when ready

### For Production Deployment
- See [Proxmox LXC Deployment Guide](proxmox-lxc-guide.md) for on-premises
- See [GCP Cloud Run Deployment Guide](gcp-cloud-run-guide.md) for cloud

### For Support
- Review the troubleshooting section above
- See [Deployment Troubleshooting Guide](troubleshooting-guide.md)
- Check deployment logs
- Consult the main README

---

## Support and Questions

For issues or questions:
1. Check the troubleshooting section above
2. See [Deployment Troubleshooting Guide](troubleshooting-guide.md)
3. Review deployment logs: `docker compose -f deploy/docker-compose-t226.yml logs`
4. Consult the main README: `/home/emage/Code/emage/emage.code/README.md`
5. Check CWSO documentation: `/home/emage/Code/emage/CWSO/README.md`

---

## Appendix: Docker Compose Configuration

See `docker-compose-t226.yml` and `t226-phase2.env` in the `deploy/` directory for full
configuration details.

Key services:
- **orchestrator** (port 8080) — Main CWSO orchestration engine. Liveness: `GET /healthz`
  (no auth, plaintext `ok`). `GET /health` is a separate, authenticated endpoint.
- **rollout** (port 8787) — Model routing and request proxying (container: `cwso-rollout`).
  Liveness: `GET /healthz` (no auth, `{"status":"ok"}`). `GET /health` is POST-only and returns
  `405 Method Not Allowed` on GET.
- **git-shadow** — Git shadowing service. IPC-socket-only (shares the `cwso-runtime` volume via
  Unix socket); publishes no host port and is not curlable. Check liveness with
  `docker compose -f deploy/docker-compose-t226.yml ps git-shadow`.
- **merge-engine** — Concurrent merge engine. Same IPC-socket-only architecture as git-shadow; no
  host port, not curlable. Check liveness with
  `docker compose -f deploy/docker-compose-t226.yml ps merge-engine`.
