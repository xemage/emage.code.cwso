---
description: "Validate task ledger integrity. Run before every checkpoint, before every release, and after every install --update."
---

Run `python3 docs/tasks/validate-tasks.py` from the project root.

- Exit 0 → report "TASK LEDGER: PASS".
- Exit 1 → report every `FAIL C<n>` line verbatim, then propose one fix per
  violation. Do NOT auto-fix C5, C6, or C8 — ask the user which ledger is correct.

Run this command:
1. Before writing any checkpoint
2. Before `/prepare-release`
3. Immediately after `install.sh --update`
