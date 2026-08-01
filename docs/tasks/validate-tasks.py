#!/usr/bin/env python3
from __future__ import annotations

import datetime as dt
import re
import sys
from collections import Counter
from pathlib import Path

ID_RE = re.compile(r"^T\d{3,}$")
DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
STATUS_SET = {"pending", "in_progress", "blocked", "in_review", "done", "cancelled"}
PRIORITY_SET = {"P0", "P1", "P2"}
EXACT_BRIEF_RE = re.compile(r"^task-(T\d{3,})\.md$")
ANY_BRIEF_RE = re.compile(r"^task-(T\d{3,}).*\.md$")
STATUS_LINE_RE = re.compile(r"^\*\*Status:\*\*\s*([A-Za-z_]+)\s*$", re.MULTILINE)
ALT_STATUS_LINE_RE = re.compile(r"^\*\*Status\*\*:\s*([A-Za-z_]+)\s*$", re.MULTILINE)


class Row:
    def __init__(self, line_no: int, cells: list[str]) -> None:
        self.line_no = line_no
        self.cells = cells



def pick_base_dir() -> Path:
    cwd_tasks = Path.cwd() / "docs" / "tasks"
    if cwd_tasks.is_dir():
        return cwd_tasks
    return Path(__file__).resolve().parent



def parse_table_rows(path: Path) -> list[Row]:
    if not path.exists():
        return []

    rows: list[Row] = []
    for idx, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        stripped = raw.strip()
        if not stripped.startswith("|"):
            continue
        parts = [p.strip() for p in stripped.split("|")]
        if len(parts) < 3:
            continue
        cells = parts[1:-1]
        if not cells:
            continue

        id_cell = cells[0]
        if id_cell == "ID":
            continue
        if id_cell and set(id_cell) <= {"-", ":"}:
            continue

        rows.append(Row(idx, cells))
    return rows



def parse_date(raw: str) -> dt.date | None:
    if not DATE_RE.fullmatch(raw):
        return None
    try:
        return dt.date.fromisoformat(raw)
    except ValueError:
        return None



def get_brief_status(path: Path) -> str | None:
    text = path.read_text(encoding="utf-8")
    m = STATUS_LINE_RE.search(text)
    if m:
        return m.group(1).strip().lower()
    m = ALT_STATUS_LINE_RE.search(text)
    if m:
        return m.group(1).strip().lower()
    return None



def add_fail(fails: list[tuple[str, str]], code: str, msg: str) -> None:
    fails.append((code, msg))



def main() -> int:
    base = pick_base_dir()
    active_path = base / "active-tasks.md"
    completed_path = base / "completed-tasks.md"

    fails: list[tuple[str, str]] = []

    if not active_path.exists():
        add_fail(fails, "C2", f"missing file: {active_path}")
    if not completed_path.exists():
        add_fail(fails, "C2", f"missing file: {completed_path}")

    active_rows = parse_table_rows(active_path)
    completed_rows = parse_table_rows(completed_path)

    active_ids: list[str] = []
    completed_ids: list[str] = []

    # C1, C2, C3, C4, C10 for active
    for row in active_rows:
        if len(row.cells) != 7:
            add_fail(
                fails,
                "C2",
                f"active-tasks.md:{row.line_no} has {len(row.cells)} cells (expected 7)",
            )
            continue

        task_id, _, _, status, priority, _, last_update = row.cells
        active_ids.append(task_id)

        if status in {"done", "cancelled"}:
            add_fail(
                fails,
                "C1",
                f"active-tasks.md:{row.line_no} has terminal status '{status}' for {task_id}",
            )

        if not ID_RE.fullmatch(task_id):
            add_fail(fails, "C3", f"active-tasks.md:{row.line_no} invalid ID '{task_id}'")

        if status not in STATUS_SET:
            add_fail(fails, "C4", f"active-tasks.md:{row.line_no} invalid Status '{status}'")

        if priority not in PRIORITY_SET:
            add_fail(fails, "C4", f"active-tasks.md:{row.line_no} invalid Priority '{priority}'")

        if parse_date(last_update) is None:
            add_fail(
                fails,
                "C10",
                f"active-tasks.md:{row.line_no} invalid date '{last_update}'",
            )

    # C2, C3, C10 for completed
    completed_dates: list[tuple[int, str, dt.date | None]] = []
    for row in completed_rows:
        if len(row.cells) != 5:
            add_fail(
                fails,
                "C2",
                f"completed-tasks.md:{row.line_no} has {len(row.cells)} cells (expected 5)",
            )
            continue

        task_id, _, _, done_on, _ = row.cells
        completed_ids.append(task_id)

        if not ID_RE.fullmatch(task_id):
            add_fail(fails, "C3", f"completed-tasks.md:{row.line_no} invalid ID '{task_id}'")

        parsed = parse_date(done_on)
        if parsed is None:
            add_fail(fails, "C10", f"completed-tasks.md:{row.line_no} invalid date '{done_on}'")
        completed_dates.append((row.line_no, done_on, parsed))

    # C5 duplicates and overlap
    active_counts = Counter(active_ids)
    completed_counts = Counter(completed_ids)

    for task_id, count in active_counts.items():
        if count > 1:
            add_fail(fails, "C5", f"duplicate in active-tasks.md: {task_id} appears {count} times")

    for task_id, count in completed_counts.items():
        if count > 1:
            add_fail(fails, "C5", f"duplicate in completed-tasks.md: {task_id} appears {count} times")

    overlap = sorted(set(active_ids) & set(completed_ids))
    for task_id in overlap:
        add_fail(fails, "C5", f"ID appears in both ledgers: {task_id}")

    # C6 every task-T*.md appears in exactly one ledger
    any_briefs = sorted(p for p in base.glob("task-T*.md") if p.is_file())
    for brief in any_briefs:
        m = ANY_BRIEF_RE.fullmatch(brief.name)
        if not m:
            continue
        task_id = m.group(1)
        where = int(task_id in active_counts) + int(task_id in completed_counts)
        if where != 1:
            add_fail(
                fails,
                "C6",
                f"{brief.name} has ID {task_id} present in {where} ledgers (expected 1)",
            )

    # C7 every ledger ID has matching task-<ID>.md
    for task_id in sorted(set(active_ids + completed_ids)):
        exact = base / f"task-{task_id}.md"
        if not exact.exists():
            add_fail(fails, "C7", f"missing brief file: task-{task_id}.md")

    # C8 status cross-check
    exact_briefs = sorted(p for p in base.glob("task-T*.md") if p.is_file() and EXACT_BRIEF_RE.fullmatch(p.name))
    done_in_briefs: set[str] = set()
    status_map: dict[str, str | None] = {}

    for brief in exact_briefs:
        m = EXACT_BRIEF_RE.fullmatch(brief.name)
        if not m:
            continue
        task_id = m.group(1)
        status = get_brief_status(brief)
        status_map[task_id] = status
        if status == "done":
            done_in_briefs.add(task_id)

    for task_id in sorted(done_in_briefs):
        if task_id not in completed_counts:
            add_fail(
                fails,
                "C8",
                f"task-{task_id}.md says done but {task_id} is not in completed-tasks.md",
            )

    for task_id in sorted(set(completed_ids)):
        status = status_map.get(task_id)
        if status not in {"done", "cancelled"}:
            add_fail(
                fails,
                "C8",
                f"completed ID {task_id} has brief status '{status}' (expected done or cancelled)",
            )

    # C9 completed done_on non-decreasing
    prev_date: dt.date | None = None
    prev_raw: str | None = None
    prev_line: int | None = None
    for line_no, raw, parsed in completed_dates:
        if parsed is None:
            continue
        if prev_date is not None and parsed < prev_date:
            add_fail(
                fails,
                "C9",
                f"completed-tasks.md:{line_no} date {raw} is earlier than {prev_raw} at line {prev_line}",
            )
        prev_date = parsed
        prev_raw = raw
        prev_line = line_no

    if fails:
        for code, msg in fails:
            print(f"FAIL {code}: {msg}")
        print(f"TASK LEDGER: FAIL ({len(fails)} violations)")
        return 1

    print(f"TASK LEDGER: PASS ({len(active_rows)} active, {len(completed_rows)} completed)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
