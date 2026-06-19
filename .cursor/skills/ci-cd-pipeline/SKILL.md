---
name: "ci-cd-pipeline"
description: "Create and configure CI/CD pipelines for GitLab CI, Docker builds, deployment automation, and infrastructure as code. Use when setting up continuous integration, continuous deployment, Docker containerization, or automated testing pipelines."
---

# CI/CD Pipeline Configuration

Skill for creating production-ready CI/CD pipelines, Docker configurations, and deployment automation.

## When to Use
- Setting up a new project's CI/CD pipeline
- Adding stages to an existing pipeline
- Configuring Docker builds
- Setting up deployment automation
- Creating infrastructure as code

## Procedure

### 1. Assess Requirements
Determine pipeline needs based on:
- Language/framework (Node.js, Python, .NET, Go, etc.)
- Testing requirements (unit, integration, e2e)
- Deployment targets (Docker, K8s, VMs, serverless)
- Security scanning requirements
- Environment strategy (dev, staging, prod)

### 2. Select Pipeline Template

#### Node.js / TypeScript
See [./references/templates/nodejs-pipeline.yml](./references/templates/nodejs-pipeline.yml)

#### Python
See [./references/templates/python-pipeline.yml](./references/templates/python-pipeline.yml)

#### .NET
See [./references/templates/dotnet-pipeline.yml](./references/templates/dotnet-pipeline.yml)

### 3. Pipeline Stages

```
validate → build → test → security → deploy:staging → approve → deploy:production
```

Each stage should:
- Have clear success/failure criteria
- Cache dependencies between runs
- Produce artifacts where needed
- Have appropriate timeout limits

### 4. Docker Configuration

Create optimized multi-stage Dockerfiles:
- Use specific base image versions (not `latest`)
- Run as non-root user
- Include health checks
- Minimize layer count and image size
- Use `.dockerignore` to exclude unnecessary files

### 5. Environment Configuration

Define environment-specific settings:
- Use GitLab CI/CD variables for secrets
- Use environment-specific config files for non-secrets
- Document all required environment variables

## Templates

### GitLab CI Base Template
```yaml
default:
  retry:
    max: 2
    when:
      - runner_system_failure
      - stuck_or_timeout_failure

stages:
  - validate
  - build
  - test
  - security
  - deploy

variables:
  DOCKER_TLS_CERTDIR: "/certs"

.deploy_template: &deploy_template
  image: alpine:3.19
  before_script:
    - apk add --no-cache curl
  script:
    - echo "Deploying to $CI_ENVIRONMENT_NAME"
```

### Docker Compose Base
```yaml
version: '3.8'
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
      target: development
    volumes:
      - .:/app
      - /app/node_modules
    ports:
      - "${APP_PORT:-3000}:3000"
    env_file:
      - .env
    depends_on:
      db:
        condition: service_healthy
```

## Output
- `.gitlab-ci.yml` pipeline configuration
- `Dockerfile` (multi-stage, optimized)
- `docker-compose.yml` (development)
- `.dockerignore`
- Environment variable documentation

---

## Protocol-Aware Enhancements

### Pipeline Configuration Artifact Versioning

Pipeline configuration documents follow the immutable artifact versioning convention. When a pipeline configuration is created or substantially modified, store it as a versioned artifact:

```
docs/artifacts/pipeline-config-v1.md
docs/artifacts/pipeline-config-v2.md
```

**Rules:**
- Each version is immutable once published — never overwrite, always create a new version.
- The artifact captures the intent and rationale behind pipeline changes, not a copy of `.gitlab-ci.yml` itself.
- Reference pipeline config by version in checkpoint summaries: `artifact_refs=[pipeline-config-v2]`.
- When pipeline stages are added, removed, or reordered, create a new version documenting the change and its justification.

**Pipeline Config Artifact Structure:**
```markdown
# Pipeline Configuration v{N}

## Version: {N}
## Date: {YYYY-MM-DD}
## Status: draft | approved | deprecated

## Changes from v{N-1}
- [List of changes and rationale]

## Stage Definitions
[Stage names, ordering, and purpose]

## Environment Matrix
[Which environments are targeted and how]

## Secret/Variable Requirements
[Required CI/CD variables for this config version]
```

### Validation Gates Enforced by CI/CD

The CI/CD pipeline is responsible for enforcing the following validation gates. Each gate must produce a clear PASS/FAIL signal before the pipeline proceeds:

| Gate | Stage | Pass Criteria |
|------|-------|---------------|
| **Lint & Format** | `validate` | Zero lint errors, formatting conformant |
| **Unit Tests** | `test` | All pass, coverage ≥ threshold |
| **Integration Tests** | `test` | All pass, API contract conformant |
| **Security Scan** | `security` | No critical/high vulnerabilities |
| **Code Review Verdict** | `approve` | PASS or CONDITIONAL_PASS from code-review skill |
| **E2E Critical Paths** | `test` (or `deploy:staging`) | All critical-path scenarios pass |

**Gate failure handling:**
- If any gate fails, the pipeline MUST halt and report the failure in the next checkpoint summary.
- Gate failures that cannot be resolved by the current agent should be raised as blockers.
- `CONDITIONAL_PASS` verdicts from code review allow progression but require tracked follow-up items in `docs/tasks/active-tasks.md`.
