# CWSO Deployment Troubleshooting Guide

Central troubleshooting reference for CWSO deployments across Docker Desktop, Proxmox LXC, and Google Cloud Run.

---

## Quick Triage Workflow

1. Confirm service health endpoint responds.
2. Check logs for the failing component.
3. Validate environment variables and secrets.
4. Verify storage and disk space.
5. Verify network and firewall reachability.

---

## Environment-Specific Fast Checks

### Docker Desktop

```bash
cd deploy/local-dev

docker-compose ps
docker-compose logs --tail=200 orchestrator rollout-proxy
curl -s http://localhost:8080/health
```

### Proxmox LXC

```bash
pct status 201
pct exec 201 -- docker-compose -f /opt/cwso/repo/deploy/docker-compose-t226.yml ps
pct exec 201 -- docker-compose -f /opt/cwso/repo/deploy/docker-compose-t226.yml logs --tail=200
pct exec 201 -- curl -s http://localhost:8080/health
```

### Google Cloud Run

```bash
gcloud run services describe cwso-orchestrator --region us-central1

gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=cwso-orchestrator" \
  --limit 100 --format json
```

---

## Common Issues and Fixes

### 1) Service Fails to Start

Symptoms:
- container keeps restarting
- service is unhealthy

Checks:
- verify image pull succeeded
- inspect startup logs for missing env vars
- validate referenced files/paths exist

Fixes:
- re-pull images and restart
- restore `.env` values from known-good config
- correct path mappings for data volumes

### 2) Port Conflict

Symptoms:
- bind errors for 8080, 8787, 8788, 8789

Checks:

```bash
lsof -i :8080 -i :8787 -i :8788 -i :8789
```

Fixes:
- stop conflicting services
- change local port mappings in compose config

### 3) JWT / Authentication Errors

Symptoms:
- 401/403 responses
- token validation failures in logs

Checks:
- compare configured JWT secret across services
- verify token issuer and expiration

Fixes:
- set a single consistent `JWT_SECRET`
- rotate and redeploy if secret drift is detected

### 4) Storage / Parquet Issues

Symptoms:
- missing trajectories
- write failures

Checks:

```bash
df -h
ls -la /tmp/t226-parquet-store
```

Fixes:
- free disk space
- correct parquet path env var and permissions
- restore data from backup snapshot

### 5) Network / Reachability Failures

Symptoms:
- health checks timeout
- endpoints inaccessible from client

Checks:
- verify container/service IP and routes
- verify firewall rules and allowed ingress

Fixes:
- open required ports
- correct bridge/subnet configuration
- confirm cloud service ingress settings

---

## Recovery Runbook (Minimal)

### Restart Services

```bash
# Docker Desktop
cd deploy/local-dev && docker-compose restart

# Proxmox LXC
pct exec 201 -- bash -lc 'cd /opt/cwso/repo && docker-compose -f deploy/docker-compose-t226.yml restart'

# GCP Cloud Run (new revision)
gcloud run deploy cwso-orchestrator --region us-central1 --image gcr.io/$PROJECT_ID/cwso:latest
```

### Roll Back to Last Good State

1. Identify the last known-good image/tag or compose state.
2. Re-deploy using known-good version.
3. Re-run health checks and smoke tests.
4. Restore data backup if data corruption is suspected.

---

## Escalation Checklist

Collect before escalation:
- environment (`docker`, `proxmox`, or `gcp`)
- exact failing command
- recent logs (last 200 lines)
- current config hash/version
- timestamp and timezone of failure

Then open an issue in the project tracker with those artifacts.

---

## Related Guides

- [Deployment Index](README.md)
- [Docker Desktop Deployment](local-docker-desktop-guide.md)
- [Proxmox LXC Deployment](proxmox-lxc-guide.md)
- [GCP Cloud Run Deployment](gcp-cloud-run-guide.md)
