#!/usr/bin/env python3
"""Bootstrap GitLab issues from docs/tasks/active-tasks.md.

Creates one issue per task row, applying labels (priority, phase, type, agent,
status) and assigning to the matching milestone. Idempotent — skips issues
whose title already exists.

Usage: python3 scripts/seed-gitlab-issues.py
Requires: glab authenticated to gitlab.com.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

PROJECT = "em-age%2Femage.code.cwso"
TASK_FILE = Path(__file__).resolve().parent.parent / "docs" / "tasks" / "active-tasks.md"

# T-id → milestone title prefix
TASK_PHASE = {
    range(20, 29):  ("M2 — Phase 2: Shadow Workspaces + AST (PoC)", "phase::2-shadow-ast"),
    range(29, 38):  ("M3 — Phase 3: Async + Concurrency",            "phase::3-async-concurrency"),
    range(40, 51):  ("M4 — Phase 4: Sandbox Tiers + Semantic Merge", "phase::4-sandbox-merge"),
    range(51, 54):  ("M5 — Phase 5: Release v0.1.0",                 "phase::5-release"),
}

OWNER_LABEL = {
    "backend-developer":  "agent::backend-developer",
    "devops-engineer":    "agent::devops-engineer",
    "qa-engineer":        "agent::qa-engineer",
    "security-engineer":  "agent::security-engineer",
    "tech-lead":          "agent::tech-lead",
    "release-manager":    "agent::release-manager",
    "orchestrator":       "agent::tech-lead",
}

STATUS_LABEL = {
    "pending":      "status::pending",
    "in_progress":  "status::in-progress",
    "blocked":      "status::blocked",
    "in_review":    "status::in-review",
    "deferred":     "status::deferred",
}


def glab(*args: str, check: bool = True) -> str:
    cp = subprocess.run(["glab", "api", *args], capture_output=True, text=True, check=check)
    return cp.stdout


def fetch_milestones() -> dict[str, int]:
    data = json.loads(glab(f"projects/{PROJECT}/milestones?per_page=100"))
    return {m["title"]: m["id"] for m in data}


def fetch_existing_issue_titles() -> set[str]:
    titles: set[str] = set()
    page = 1
    while True:
        data = json.loads(glab(f"projects/{PROJECT}/issues?per_page=100&page={page}"))
        if not data:
            break
        titles.update(i["title"] for i in data)
        page += 1
    return titles


def parse_tasks() -> list[dict]:
    text = TASK_FILE.read_text()
    rows = []
    for line in text.splitlines():
        m = re.match(r"\|\s*(T\d+[a-z]?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*(P\d)\s*\|\s*([^|]*?)\s*\|", line)
        if not m or m.group(1) == "ID":
            continue
        tid, title, owner, status, prio, deps = m.groups()
        rows.append({
            "id": tid, "title": title.strip(), "owner": owner.strip(),
            "status": status.strip(), "priority": prio.strip(),
            "deps": [d.strip() for d in deps.split(",") if d.strip()],
        })
    return rows


def phase_for(tid: str) -> tuple[str, str] | tuple[None, None]:
    n = int(re.match(r"T(\d+)", tid).group(1))
    for r, (ms, lbl) in TASK_PHASE.items():
        if n in r:
            return ms, lbl
    return None, None


def normalize_status(s: str) -> str:
    s = s.lower()
    if s.startswith("partial"):  return "in_progress"
    if s.startswith("deferred"): return "deferred"
    return s.replace(" ", "_")


def create_issue(task: dict, milestones: dict[str, int]) -> None:
    issue_title = f"{task['id']}: {task['title']}"
    ms_title, phase_label = phase_for(task["id"])
    labels = [
        f"priority::{task['priority']}",
        OWNER_LABEL.get(task["owner"], "agent::backend-developer"),
        STATUS_LABEL.get(normalize_status(task["status"]), "status::pending"),
        "type::feature",
    ]
    if phase_label:
        labels.append(phase_label)

    desc_lines = [
        f"_Imported from `docs/tasks/active-tasks.md` ({task['id']})._",
        "",
        f"**Owner:** {task['owner']}  ",
        f"**Priority:** {task['priority']}  ",
        f"**Status (source):** {task['status']}  ",
    ]
    if task["deps"]:
        desc_lines.append(f"**Depends on:** {', '.join(task['deps'])}  ")
    desc_lines += [
        "",
        f"See the full brief in [`docs/tasks/task-{task['id']}.md`](../blob/develop/docs/tasks/task-{task['id']}.md) when present, "
        "and the [mega-plan](../blob/develop/docs/plans/plan-cwso-mega.md) for context.",
        "",
        "## Acceptance Criteria",
        "- [ ] Implementation matches the task brief",
        "- [ ] Tests pass (`go test ./... -race`, `cargo test --release`)",
        "- [ ] Documentation updated where the change is user-facing",
        "- [ ] PoC-DEBT entries registered for any shortcuts",
    ]

    args = [
        "-X", "POST", f"projects/{PROJECT}/issues",
        "-f", f"title={issue_title}",
        "-f", f"description=" + "\n".join(desc_lines),
        "-f", f"labels=" + ",".join(labels),
    ]
    if ms_title and ms_title in milestones:
        args += ["-f", f"milestone_id={milestones[ms_title]}"]
    glab(*args)
    print(f"  + {issue_title}  [{', '.join(labels)}]" + (f"  ms={ms_title}" if ms_title else ""))


def main() -> None:
    milestones = fetch_milestones()
    existing = fetch_existing_issue_titles()
    tasks = parse_tasks()
    print(f"Parsed {len(tasks)} tasks, {len(existing)} existing issues, {len(milestones)} milestones")
    for t in tasks:
        title = f"{t['id']}: {t['title']}"
        if title in existing:
            print(f"  = {title} (skip, exists)")
            continue
        try:
            create_issue(t, milestones)
        except subprocess.CalledProcessError as e:
            print(f"  ! failed {title}: {e.stderr}", file=sys.stderr)


if __name__ == "__main__":
    main()
