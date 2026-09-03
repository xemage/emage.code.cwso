#!/usr/bin/env bash
# check-version-drift.sh — fail when README's stated current version lags CHANGELOG.
#
# Extraction rules (exact string comparison, no fuzzy/regex heuristics):
#   * CHANGELOG version: the first heading matching "^## vX.Y.Z " in CHANGELOG.md
#     (i.e. the newest entry; headings follow the "## vX.Y.Z - YYYY-MM-DD" format).
#   * README version: the first "vX.Y.Z" token on the README.md status-table row
#     whose first cell is "Current state" (line begins with "| Current state |").
#
# Both extracted strings are compared byte-for-byte. If they differ, the README
# status table is stale: exit 1. If they agree: exit 0.
#
# Runs fully offline (reads only CHANGELOG.md and README.md from the repo root).
set -euo pipefail

cd "$(dirname "$0")/.."

CHANGELOG="CHANGELOG.md"
README="README.md"

changelog_version=$(grep -m1 -E '^## v[0-9]+\.[0-9]+\.[0-9]+ ' "$CHANGELOG" \
  | sed -E 's/^## (v[0-9]+\.[0-9]+\.[0-9]+) .*/\1/')

readme_version=$(grep -m1 '^| Current state |' "$README" \
  | grep -o -E -m1 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true)

if [ -z "${changelog_version:-}" ]; then
  echo "ERROR: could not extract newest version from $CHANGELOG (expected a '## vX.Y.Z - ...' heading)" >&2
  exit 2
fi
if [ -z "${readme_version:-}" ]; then
  echo "ERROR: could not extract version from '$README' status-table row '| Current state |'" >&2
  exit 2
fi

if [ "$changelog_version" != "$readme_version" ]; then
  echo "VERSION DRIFT: README 'Current state' says $readme_version but newest CHANGELOG entry is $changelog_version" >&2
  echo "Update the README.md status table (and/or CHANGELOG.md) so both agree." >&2
  exit 1
fi

echo "OK: README 'Current state' and newest CHANGELOG entry both at $changelog_version"
exit 0
