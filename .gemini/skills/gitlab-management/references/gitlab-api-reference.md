# GitLab API Reference

Quick reference for the GitLab REST endpoints most often used by the
`gitlab-management` skill. Prefer the `glab` CLI wrappers where possible —
they handle auth, pagination, and rate-limiting for you.

## Authentication
- Personal Access Token scopes: `api`, `read_repository`, `write_repository`
- Header: `PRIVATE-TOKEN: <token>` or `Authorization: Bearer <token>`
- CLI: `glab auth login --hostname gitlab.com`

## Common endpoints

| Resource | Method | Path |
|---|---|---|
| Project | GET | `/projects/:id` |
| Project settings | PUT | `/projects/:id` |
| Issues | GET / POST | `/projects/:id/issues` |
| Issue update | PUT | `/projects/:id/issues/:iid` |
| Merge requests | GET / POST | `/projects/:id/merge_requests` |
| MR notes | POST | `/projects/:id/merge_requests/:iid/notes` |
| Pipelines | GET | `/projects/:id/pipelines` |
| Pipeline jobs | GET | `/projects/:id/pipelines/:pipeline_id/jobs` |
| Branches | GET / POST / DELETE | `/projects/:id/repository/branches` |
| Protected branches | POST | `/projects/:id/protected_branches` |
| Labels | GET / POST | `/projects/:id/labels` |
| Milestones | GET / POST | `/projects/:id/milestones` |
| Wiki pages | GET / POST / PUT | `/projects/:id/wikis` |

## Project ID encoding
URL-encode `namespace/path` when used in place of numeric ID:
`em-age/emage.code` → `em-age%2Femage.code`

## glab equivalents

```bash
glab issue list --repo em-age/emage.code
glab issue create --title "..." --description "..."
glab mr create --source-branch feat/x --target-branch develop --title "..."
glab mr merge <iid> --squash --remove-source-branch
glab ci view
glab api projects/:id/wikis
```

## Rate limits
- 2,000 requests/min per user on gitlab.com
- 429 → back off using `Retry-After` header

## Pagination
- Query: `?per_page=100&page=N` (default 20, max 100)
- Response headers: `X-Total-Pages`, `X-Next-Page`
- `glab` auto-paginates in most subcommands

## Official docs
https://docs.gitlab.com/ee/api/
