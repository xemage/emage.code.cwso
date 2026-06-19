# GitFlow Layout

This repository follows the GitFlow branching model defined in
[.github/instructions/git-workflow.instructions.md](../.github/instructions/git-workflow.instructions.md).

## Long-lived branches

| Branch    | Purpose                                                    | Protected |
|-----------|------------------------------------------------------------|-----------|
| `main`    | Production releases only. Tagged `vX.Y.Z` on each release. | yes       |
| `develop` | Integration branch. All feature/bugfix branches merge here.| yes       |

Phase-2 closure (commit `7354206`) is the current base of both branches.
All Phase-3+ work happens off `develop`.

## Short-lived branches

| Pattern             | Created from | Merges to        | Notes                                |
|---------------------|--------------|------------------|--------------------------------------|
| `feature/<id>-<slug>` | `develop`  | `develop`        | New features, refactors              |
| `bugfix/<id>-<slug>`  | `develop`  | `develop`        | Non-critical fixes                   |
| `release/v<version>`  | `develop`  | `main` + `develop` | Release stabilization              |
| `hotfix/v<version>`   | `main`     | `main` + `develop` | Emergency production fixes         |
| `agent/<role>/<task>` | `develop`  | `develop`        | Worktree-isolated agent work         |

### Examples
```
feature/T030-streamable-http-sse
bugfix/T028a-shadow-client-unit-tests
release/v0.1.0
hotfix/v0.1.1
agent/backend-engineer/T029
```

## Merge policy
- All non-trivial work goes through a merge request.
- Squash-and-merge for `feature/*` and `bugfix/*`; preserve history for `release/*` and `hotfix/*`.
- CI must pass and at least one approval is required before merge (enforced once a hosting platform is wired up).
- Delete source branches after merge.

## Commit policy
Conventional Commits — see [.github/instructions/git-workflow.instructions.md](../.github/instructions/git-workflow.instructions.md#commit-messages-conventional-commits).

## Tag policy
Releases are tagged on `main` as `vX.Y.Z` using semantic versioning. Pre-release
suffixes (`-rc.1`, `-poc.1`) are allowed during PoC phases.
