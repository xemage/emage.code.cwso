---
name: "api-design"
description: "Design REST and GraphQL APIs with OpenAPI specifications, endpoint contracts, request/response schemas, error handling patterns, pagination, and versioning. Use when designing new APIs, creating API contracts, writing OpenAPI specs, or planning API architecture."
---

# API Design

Skill for designing well-structured, consistent APIs following industry best practices.

## When to Use
- Designing a new REST or GraphQL API
- Creating OpenAPI/Swagger specifications
- Defining API contracts between services
- Standardizing error handling and pagination
- Planning API versioning strategy

## Procedure

### 1. API Style Decision
Choose based on use case:
| Style | Best For |
|-------|---------|
| REST | CRUD operations, resource-oriented, broad compatibility |
| GraphQL | Complex queries, mobile clients, avoiding over-fetching |
| gRPC | Service-to-service, high performance, streaming |

### 2. URL Design (REST)

```
# Resource conventions
GET    /api/v1/resources          # List (with pagination)
POST   /api/v1/resources          # Create
GET    /api/v1/resources/:id      # Read
PUT    /api/v1/resources/:id      # Full update
PATCH  /api/v1/resources/:id      # Partial update
DELETE /api/v1/resources/:id      # Delete

# Sub-resources
GET    /api/v1/users/:id/orders   # User's orders

# Actions (non-CRUD)
POST   /api/v1/orders/:id/cancel  # Action on resource

# Filtering, sorting, pagination
GET    /api/v1/resources?status=active&sort=-created_at&page=2&limit=20
```

### 3. Standard Response Envelope

```json
{
  "data": { ... },
  "meta": {
    "total": 150,
    "page": 2,
    "limit": 20,
    "totalPages": 8
  },
  "links": {
    "self": "/api/v1/resources?page=2",
    "first": "/api/v1/resources?page=1",
    "prev": "/api/v1/resources?page=1",
    "next": "/api/v1/resources?page=3",
    "last": "/api/v1/resources?page=8"
  }
}
```

### 4. Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": [
      {
        "field": "email",
        "message": "Must be a valid email address",
        "code": "INVALID_FORMAT"
      }
    ],
    "requestId": "req_abc123"
  }
}
```

### 5. Standard HTTP Status Codes

| Code | Meaning | When to Use |
|------|---------|-------------|
| 200 | OK | Successful GET, PUT, PATCH |
| 201 | Created | Successful POST (resource created) |
| 204 | No Content | Successful DELETE |
| 400 | Bad Request | Validation failed |
| 401 | Unauthorized | Missing or invalid auth |
| 403 | Forbidden | Authenticated but not authorized |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate resource |
| 422 | Unprocessable | Semantically invalid |
| 429 | Too Many Requests | Rate limited |
| 500 | Internal Error | Server error |

### 6. OpenAPI Specification Template

```yaml
openapi: 3.1.0
info:
  title: [API Name]
  version: 1.0.0
  description: [API description]

servers:
  - url: https://api.example.com/v1
    description: Production
  - url: https://staging-api.example.com/v1
    description: Staging

paths:
  /resources:
    get:
      summary: List resources
      parameters:
        - $ref: '#/components/parameters/PageParam'
        - $ref: '#/components/parameters/LimitParam'
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ResourceList'

components:
  schemas:
    Resource:
      type: object
      required: [id, name]
      properties:
        id:
          type: string
          format: uuid
        name:
          type: string
        createdAt:
          type: string
          format: date-time

  parameters:
    PageParam:
      name: page
      in: query
      schema:
        type: integer
        default: 1
        minimum: 1
    LimitParam:
      name: limit
      in: query
      schema:
        type: integer
        default: 20
        minimum: 1
        maximum: 100

  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
```

### 7. Versioning Strategy
- URL path versioning: `/api/v1/`, `/api/v2/`
- Support N-1 versions minimum
- Deprecation notice in response headers
- Migration guides between versions

## Output
- API endpoint list with methods and descriptions
- OpenAPI 3.1 specification file
- Request/response examples for each endpoint
- Error code reference
- Authentication documentation

---

## Protocol-Aware Enhancements

### API Contract Artifact Versioning

All API contract documents follow the immutable artifact versioning convention. When an API contract is created or modified, it MUST be stored as a versioned artifact:

```
docs/artifacts/api-contract-v1.md
docs/artifacts/api-contract-v2.md
```

**Rules:**
- Each version is immutable once published — never overwrite, always create a new version.
- The latest version is the authoritative contract; prior versions are kept for audit and rollback.
- Reference contracts by version in checkpoint summaries: `artifact_refs=[api-contract-v3]`.
- When a breaking change is introduced, increment the major version and document the migration path inside the new artifact.

**Contract Artifact Structure:**
```markdown
# API Contract v{N}

## Version: {N}
## Date: {YYYY-MM-DD}
## Status: draft | approved | deprecated

## Changes from v{N-1}
- [List of changes]

## Endpoints
[Full endpoint specifications]

## Schemas
[Request/response schemas]
```

### Blocker Protocol for API Design Issues

When an API design decision cannot be resolved (e.g., conflicting requirements between consumers, unresolvable versioning constraint, dependency on an unavailable upstream service), raise a blocker using the standard blocker protocol:

```
[BLOCKER] id=api-{issue-id} | type=design | severity=high|medium | description="..." | owner={role} | escalation={next-role}
```

**When to raise an API design blocker:**
- Two or more consumers require incompatible contract shapes.
- A required upstream API is not yet available or is unstable.
- Security review flags an endpoint pattern as non-compliant.
- Performance requirements conflict with the proposed API structure.

Blockers must be included in the next checkpoint summary and tracked in `docs/tasks/active-tasks.md` until resolved.
