# Task T153 — Tag pipeline deploy:registry fix

- **Status:** done
- **Owner:** devops-engineer
- **Priority:** P1
- **Depends on:** T152
- **Based on:** GitLab pipeline #2583110609 (`v0.3.0` tag)

## Objective

Fix invalid tag pipelines where `deploy:registry` hard-depends on `e2e:phase2`, which is
excluded by `rules` on `$CI_COMMIT_TAG`.

## Acceptance Criteria

- [x] `deploy:registry` uses `needs:optional` for `e2e:phase2`
- [x] MR !58 merged
- [ ] Tag pipeline re-run verified (post-merge)

Merged via MR !58.
