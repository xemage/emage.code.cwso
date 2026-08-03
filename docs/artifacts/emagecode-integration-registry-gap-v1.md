# CWSO Container Registry: Missing Images and Incomplete Publishing (emage.code Integration Gap)

**Source:** emage.code repository, `docs/plans/plan-017-deployment-docs-and-registry-hardening.md`
**Filed by:** orchestrator (emage.code), 2026-08-03
**Severity:** P1 — blocks `deploy/docker-compose-t226.yml` portability

---

## Summary

`deploy/docker-compose-t226.yml` in the emage.code repository needs to pull CWSO images from
`registry.gitlab.com/em-age/emage.code.cwso/...` instead of building from a hardcoded absolute
path (`context: /home/emage/Code/emage/CWSO`). This requires all 4 CWSO service images to be
published to the registry. Currently, the CI pipeline is incomplete: two images are not published,
and no semver tags are pushed.

---

## Evidence

### Finding 1: `build:rollout` job is entirely absent from `.gitlab-ci.yml`

`.gitlab-ci.yml` has these build jobs:
- `build:orchestrator` (line 165)
- `build:git-shadow` (line 175)
- `build:merge-engine` (line 185)

There is **no `build:rollout` job**, despite `deploy/Dockerfile.rollout` existing in the repo.
The `deploy/docker-compose-t226.yml` in emage.code uses a `rollout` service (container name
`cwso-rollout`) as an integral part of the working stack (confirmed by plan-016 T304/T305 runs).

### Finding 2: `deploy:registry` only pushes `orchestrator` and `git-shadow`

`deploy:registry` (line 238) `needs:` only:
```yaml
needs:
  - job: build:orchestrator
  - job: build:git-shadow
  - job: e2e:phase2
    optional: true
```

Its `script:` only ensures and pushes `orchestrator` and `git-shadow`. `merge-engine` is built
(`build:merge-engine` runs) but **never pushed** to the registry. `rollout` is neither built nor
pushed.

### Finding 3: Only `:latest` tag — no semver tags

`deploy:registry` pushes only `:latest` even when `$CI_COMMIT_TAG` is set. Semver tags (e.g.
`:v0.5.1`) are never pushed, making it impossible for downstream consumers to pin to a specific
release.

### Finding 4: Registry access confirmed non-blocking

- CWSO project `visibility: public` ✓
- `container_registry_access_level: enabled` ✓

No new credentials or access changes are needed. This is purely a CI completeness gap.

---

## Impact on emage.code

Until all 4 images are published:
- `deploy/docker-compose-t226.yml` cannot reference registry images (it uses hardcoded
  `context: /home/emage/Code/emage/CWSO` — an absolute, machine-specific path)
- Users installing emage.code on any machine other than the one where CWSO was developed cannot
  run `docker compose up -d` without also cloning CWSO separately and placing it at that exact path
- The guide's "Updating CWSO Image" instructions (`docker compose pull`) are broken because
  `docker compose pull` has no `image:` field to pull from

---

## Requested actions (see companion plan: `docs/plans/plan-registry-publishing-completion.md`)

1. Add a `build:rollout` CI job (mirrors `build:merge-engine` structure)
2. Expand `deploy:registry` `needs:` and push script to include `merge-engine` and `rollout`
3. Add semver-on-tag push (push both `:latest` and `:$CI_COMMIT_TAG` when `$CI_COMMIT_TAG` is set)

These are suggestions for CWSO's own team to adapt — not mandates. See the companion plan for a
concrete suggested YAML snippet.
