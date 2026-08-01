# Wiki Sources

These are the **canonical sources** for your project wiki.

Latest release: vX.Y.Z

## Sync to live Wiki

The wiki pages in this folder can be synced to your Git provider's wiki if needed.
Refer to your provider's documentation for wiki configuration.

## Release documentation contract

- Update release marker: `Latest release: vX.Y.Z` in this file and other key docs
- Before tagging, run: `python3 scripts/verify-release-docs.py --tag vX.Y.Z`
- This gate ensures documentation is current before release publication

## Implementation guide

For complex documentation about your implementation, create an `implementation-guide.md`
file that covers architecture, design patterns, and how to contribute.
