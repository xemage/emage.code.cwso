# CWSO Google Cloud Run Deployment Guide

**Version:** 1.0
**Last updated:** 2026-06-28
**Platform:** Google Cloud Run
**Region:** us-central1 (configurable)

> **⚠️ Not yet validated end-to-end.** This guide was written from the original deployment plan
> (`docs/plans/plan-013-cwso-deployment-guides.md`) but has not been run against a real
> GCP project since. Unlike `local-docker-desktop-guide.md` (validated and
> corrected in `docs/tasks/task-T313.md`), no task in this repo’s history confirms this guide
> works as written. Treat it as a starting point, not a proven procedure, until a future task
> validates it for real and removes this notice.

---

## Quick Start (20 minutes)

Prerequisites: Google Cloud account with billing enabled, `gcloud` CLI installed

```bash
# 1. Set up GCP project
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"

gcloud config set project $GCP_PROJECT_ID
gcloud config set compute/region $GCP_REGION

# 2. Build and push Docker image
docker build -t gcr.io/$GCP_PROJECT_ID/cwso:latest .
docker push gcr.io/$GCP_PROJECT_ID/cwso:latest

# 3. Deploy to Cloud Run
gcloud run deploy cwso-orchestrator \
  --image gcr.io/$GCP_PROJECT_ID/cwso:latest \
  --memory 2Gi \
  --cpu 2 \
  --port 8080 \
  --region $GCP_REGION \
  --allow-unauthenticated

# 4. Get service URL and test
SERVICE_URL=$(gcloud run services describe cwso-orchestrator \
  --platform managed \
  --region $GCP_REGION \
  --format='value(status.url)')

curl $SERVICE_URL/health

# Done! CWSO is running on Cloud Run
```

---

## Prerequisites

### Google Cloud Setup
- Google Cloud account (create at https://cloud.google.com)
- Billing enabled
- Project created and selected
- Cloud Run API enabled
- Container Registry or Artifact Registry enabled

### Local Tools
- `gcloud` CLI installed and authenticated
- `docker` installed and running
- Repository cloned locally

### Initial Setup

```bash
# Install gcloud CLI (if needed)
# https://cloud.google.com/sdk/docs/install

# Authenticate with Google Cloud
gcloud auth login

# Set default project
gcloud config set project your-project-id

# Enable required APIs
gcloud services enable run.googleapis.com
gcloud services enable containerregistry.googleapis.com
gcloud services enable cloudtrace.googleapis.com
gcloud services enable logging.googleapis.com
```

---

## Architecture

```
Google Cloud
├─ Cloud Run Services
│  ├─ cwso-orchestrator (8080)
│  ├─ cwso-rollout-proxy (8787)
│  ├─ cwso-git-shadow (8788)
│  └─ cwso-merge-engine (8789)
├─ Artifact Registry (Docker images)
├─ Cloud Storage (Parquet store)
├─ Cloud Logging
└─ Cloud Trace
```

---

## Step 1: Build Docker Image

### Option A: Build Locally and Push

```bash
# Set environment variables
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"
export IMAGE_NAME="cwso"
export IMAGE_TAG="latest"

# Build image (from repository root)
docker build \
  -f CWSO/Dockerfile \
  -t gcr.io/$GCP_PROJECT_ID/$IMAGE_NAME:$IMAGE_TAG \
  .

# Tag as 'latest' if needed
docker tag gcr.io/$GCP_PROJECT_ID/$IMAGE_NAME:$IMAGE_TAG \
  gcr.io/$GCP_PROJECT_ID/$IMAGE_NAME:latest

# Configure Docker auth for GCP
gcloud auth configure-docker gcr.io

# Push to Container Registry
docker push gcr.io/$GCP_PROJECT_ID/$IMAGE_NAME:$IMAGE_TAG
```

### Option B: Build in Cloud Build

```bash
# No local Docker build needed - GCP builds it
gcloud builds submit \
  --tag gcr.io/$GCP_PROJECT_ID/$IMAGE_NAME:$IMAGE_TAG \
  --source . \
  --config CWSO/cloudbuild.yaml
```

---

## Step 2: Configure Environment

### Create Secret for JWT

```bash
# Create secret in Secret Manager
echo -n "your-jwt-secret-here" | \
  gcloud secrets create cwso-jwt-secret \
    --data-file=-

# Or use existing JWT
export JWT_SECRET=$(cat deploy/.env.jwt.dev)
gcloud secrets create cwso-jwt-secret --data-file=-

# Grant Cloud Run service access to secret
gcloud secrets add-iam-policy-binding cwso-jwt-secret \
  --member serviceAccount:$(gcloud config get-value project)@appspot.gserviceaccount.com \
  --role roles/secretmanager.secretAccessor
```

### Create Cloud Storage Bucket for Parquet

```bash
# Create bucket
gsutil mb -l $GCP_REGION gs://$GCP_PROJECT_ID-cwso-parquet

# Set lifecycle policy (optional - auto-delete old data)
gsutil lifecycle set - gs://$GCP_PROJECT_ID-cwso-parquet << 'EOF'
{
  "lifecycle": {
    "rule": [
      {
        "action": {"type": "Delete"},
        "condition": {"age": 90}
      }
    ]
  }
}
EOF

# Grant Cloud Run service access
gsutil iam ch \
  serviceAccount:$(gcloud config get-value project)@appspot.gserviceaccount.com:roles/storage.objectAdmin \
  gs://$GCP_PROJECT_ID-cwso-parquet
```

---

## Step 3: Deploy to Cloud Run

### Deploy Orchestrator Service

```bash
# Deploy main orchestrator
gcloud run deploy cwso-orchestrator \
  --image gcr.io/$GCP_PROJECT_ID/cwso:latest \
  --platform managed \
  --region $GCP_REGION \
  --memory 2Gi \
  --cpu 2 \
  --port 8080 \
  --timeout 3600 \
  --max-instances 10 \
  --min-instances 1 \
  --allow-unauthenticated \
  --update-env-vars \
    JWT_SECRET_NAME=cwso-jwt-secret,\
    PARQUET_BUCKET=gs://$GCP_PROJECT_ID-cwso-parquet,\
    ENVIRONMENT=production

# Get service URL
gcloud run services describe cwso-orchestrator \
  --platform managed \
  --region $GCP_REGION \
  --format='value(status.url)'
```

### Deploy Additional Services (Optional)

```bash
# Deploy rollout proxy
gcloud run deploy cwso-rollout-proxy \
  --image gcr.io/$GCP_PROJECT_ID/cwso:latest \
  --platform managed \
  --region $GCP_REGION \
  --memory 1Gi \
  --cpu 1 \
  --port 8787 \
  --timeout 600 \
  --max-instances 20 \
  --allow-unauthenticated

# Deploy git shadow (if needed)
gcloud run deploy cwso-git-shadow \
  --image gcr.io/$GCP_PROJECT_ID/cwso:latest \
  --platform managed \
  --region $GCP_REGION \
  --memory 1Gi \
  --cpu 1 \
  --port 8788 \
  --timeout 1800
```

---

## Step 4: Configure Load Balancer (Optional)

### Set Up Global Load Balancer

```bash
# Create backend service pointing to Cloud Run
gcloud compute backend-services create cwso-backend \
  --protocol HTTP2 \
  --global \
  --enable-cdn \
  --cache-mode CACHE_ALL_STATIC \
  --default-ttl 3600

# Create URL map
gcloud compute url-maps create cwso-lb \
  --default-service cwso-backend

# Create HTTP proxy
gcloud compute target-http-proxies create cwso-http-proxy \
  --url-map cwso-lb

# Create firewall rule
gcloud compute firewall-rules create allow-cwso-lb \
  --allow tcp:80,tcp:443 \
  --source-ranges 0.0.0.0/0

# Create IP address
gcloud compute addresses create cwso-ip \
  --global

# Create forwarding rule
gcloud compute forwarding-rules create cwso-fwd \
  --global \
  --target-http-proxy cwso-http-proxy \
  --address cwso-ip \
  --ports 80,443

# Get load balancer IP
gcloud compute addresses describe cwso-ip --global --format='value(address)'
```

---

## Step 5: Configure Custom Domain (Optional)

### With Cloud Run

```bash
# Add custom domain to Cloud Run service
gcloud beta run domain-mappings create \
  --service cwso-orchestrator \
  --domain api.cwso.example.com

# Verify domain ownership and update DNS
# Follow gcloud's instructions for DNS records

# Verify setup
gcloud beta run domain-mappings describe \
  --domain api.cwso.example.com
```

---

## Monitoring and Logging

### Cloud Logging

```bash
# View recent logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=cwso-orchestrator" \
  --limit 50 \
  --format json

# Follow logs (streaming)
gcloud alpha logging read "resource.type=cloud_run_revision" \
  --limit 50 \
  --follow

# Set up log sink for archival
gcloud logging sinks create cwso-archive \
  gs://$GCP_PROJECT_ID-cwso-logs \
  --log-filter='resource.type=cloud_run_revision'
```

### Cloud Monitoring

```bash
# Create alert policy for high error rate
gcloud monitoring policies create \
  --notification-channels=$CHANNEL_ID \
  --display-name "CWSO High Error Rate" \
  --condition-display-name "Error rate > 5%" \
  --condition-threshold-value 5 \
  --condition-threshold-comparison COMPARISON_GT
```

### Cloud Trace

```bash
# View traces
gcloud trace list --limit 20

# Describe specific trace
gcloud trace describe $TRACE_ID
```

---

## Testing Deployment

### Health Check

```bash
# Get service URL
SERVICE_URL=$(gcloud run services describe cwso-orchestrator \
  --platform managed \
  --region $GCP_REGION \
  --format='value(status.url)')

# Test health endpoint
curl $SERVICE_URL/health

# Expected response:
# {"status":"healthy","timestamp":"..."}
```

### Performance Testing

```bash
# Load test with Apache Bench (ab)
ab -n 1000 -c 10 $SERVICE_URL/health

# Or use Google Cloud Load Testing
gcloud compute http-health-checks create cwso-health-check

# Test with curl (verbose)
curl -v -H "Authorization: Bearer $JWT_TOKEN" \
  $SERVICE_URL/api/status
```

---

## Scaling Configuration

### Auto-scaling

```bash
# Cloud Run auto-scales automatically
# Minimum instances (warm start, no cold starts)
gcloud run services update cwso-orchestrator \
  --min-instances 2 \
  --region $GCP_REGION

# Maximum instances (cost control)
gcloud run services update cwso-orchestrator \
  --max-instances 100 \
  --region $GCP_REGION

# Request concurrency per instance
gcloud run services update cwso-orchestrator \
  --concurrency 10 \
  --region $GCP_REGION
```

### Resource Allocation

```bash
# Increase memory
gcloud run services update cwso-orchestrator \
  --memory 4Gi \
  --region $GCP_REGION

# Increase CPU
gcloud run services update cwso-orchestrator \
  --cpu 4 \
  --region $GCP_REGION

# Adjust timeout
gcloud run services update cwso-orchestrator \
  --timeout 1800 \
  --region $GCP_REGION
```

---

## Troubleshooting

### Service Won't Deploy

```bash
# Check deployment status
gcloud run operations list

# Get detailed error
gcloud run services describe cwso-orchestrator \
  --platform managed \
  --region $GCP_REGION

# Check build logs (if using Cloud Build)
gcloud builds log $BUILD_ID

# Common issues:
# - Image not found: Check image was pushed to correct registry
# - Permission denied: Check IAM roles
# - Port mismatch: Verify port 8080 in Dockerfile and deployment
```

### High Latency

```bash
# Check cold start issues
gcloud logging read "resource.type=cloud_run_revision AND textPayload=~\"cold_start\"" \
  --limit 10

# Increase minimum instances
gcloud run services update cwso-orchestrator \
  --min-instances 5

# Check for external dependencies
# - Database latency
# - Storage latency (Cloud Storage)
# - Network bandwidth
```

### Authentication Issues

```bash
# Verify JWT secret in Secret Manager
gcloud secrets versions list cwso-jwt-secret

# Check service account permissions
gcloud projects get-iam-policy $GCP_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:cloudrun"

# Grant missing permissions
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member serviceAccount:$SERVICE_ACCOUNT \
  --role roles/secretmanager.secretAccessor
```

### Storage Access Issues

```bash
# Verify bucket exists and permissions
gsutil ls gs://$GCP_PROJECT_ID-cwso-parquet

# Grant Cloud Run service access
gsutil iam ch \
  serviceAccount:$PROJECT_NUMBER@cloudrun.gserviceaccount.com:roles/storage.objectAdmin \
  gs://$GCP_PROJECT_ID-cwso-parquet

# Test write access from Cloud Run
# Add test code to check bucket connectivity
```

---

## Monitoring Costs

### Estimate Monthly Cost

- Cloud Run invocations: $0.40 per million
- vCPU seconds: $0.00002400 per second
- Memory seconds: $0.00000250 per GB-second
- Data transfer: $0.12 per GB (egress)

```bash
# Check billing
gcloud billing accounts list
gcloud billing budgets list --billing-account=$BILLING_ACCOUNT_ID

# Set up budget alert
gcloud billing budgets create \
  --billing-account=$BILLING_ACCOUNT_ID \
  --display-name="CWSO Budget" \
  --budget-amount=100 \
  --threshold-rule=percent=50 \
  --threshold-rule=percent=100
```

---

## Backup and Disaster Recovery

### Backup Configuration

```bash
# Export service configuration
gcloud run services describe cwso-orchestrator \
  --platform managed \
  --region $GCP_REGION \
  > cwso-service-config.yaml

# Export Cloud Storage bucket policy
gsutil acl get gs://$GCP_PROJECT_ID-cwso-parquet \
  > cwso-bucket-acl.yaml

# Export secrets (encrypted)
gcloud secrets versions access latest cwso-jwt-secret \
  > cwso-jwt-secret-backup.txt.enc  # Store securely!
```

### Disaster Recovery Procedure

```bash
# Redeploy service (if needed)
gcloud run deploy cwso-orchestrator \
  --image gcr.io/$GCP_PROJECT_ID/cwso:latest \
  --platform managed \
  --region $GCP_REGION

# Restore Parquet data from backup (if available)
gsutil -m cp -r gs://backup-bucket/cwso-parquet/* \
  gs://$GCP_PROJECT_ID-cwso-parquet/

# Restore secrets
gcloud secrets create cwso-jwt-secret \
  --data-file=cwso-jwt-secret-backup.txt
```

---

## Production Checklist

- [ ] Cloud Run service deployed and tested
- [ ] Custom domain configured (if needed)
- [ ] Load balancer configured (if needed)
- [ ] Monitoring and logging enabled
- [ ] Backup strategy documented
- [ ] Scaling parameters tuned
- [ ] Cost budget set
- [ ] Incident response plan prepared
- [ ] Documentation updated
- [ ] Team trained on operational procedures

---

## Next Steps

### After Deployment
1. ✅ Verify service health: `curl $SERVICE_URL/health`
2. ✅ Monitor logs: `gcloud logging read ...`
3. ✅ Set up monitoring alerts
4. ✅ Test disaster recovery

### For Other Environments
- See [Docker Desktop Guide](local-docker-desktop-guide.md) for development
- See [Proxmox LXC Guide](proxmox-lxc-guide.md) for on-premises

---

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Run Quotas and Limits](https://cloud.google.com/run/quotas)
- [Cloud Run Pricing](https://cloud.google.com/run/pricing)
- [Cloud Run Best Practices](https://cloud.google.com/run/docs/quickstarts/build-and-deploy)

---

## Support

For GCP-specific issues:
1. Check [Cloud Run troubleshooting](https://cloud.google.com/run/docs/troubleshooting)
2. See [Deployment Troubleshooting Guide](troubleshooting-guide.md)
3. Review [Cloud Run logs](https://cloud.google.com/run/docs/logging)
4. Consult [GCP documentation](https://cloud.google.com/docs)
5. Create support ticket via Google Cloud Console
