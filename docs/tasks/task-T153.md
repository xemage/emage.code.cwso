# Task T153 — Tag pipeline deploy:registry fix

- **Status:** in_review
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T152
- **Based on:** GitLab pipeline #2583110609 (`v0.3.0` tag)

## Objective

Fix invalid tag pipelines where `deploy:registry` hard-depends on `e2e:phase2`, which is
excluded by `rules` on `$CI_COMMIT_TAG` — resulting in 0 jobs / yaml invalid.

## Acceptance Criteria

- [x] `deploy:registry` uses `needs:optional` for `e2e:phase2`
- [ ] Tag pipeline validates and runs build + deploy jobs
- [ ] `main` deploy still waits for e2e when e2e job is present
- [ ] Task board updated

## Notes

E2e remains the gate on `develop` / `main` / MR pipelines. Tag pipelines assume prior
green develop CI before release tag.
