#!/usr/bin/env python3
"""Walk git history from the first commit and derive x.y.z.

Rules (x is fixed at 0):
  * y, z start at 0
  * a commit whose subject starts with "feat"  -> y += 1, z = 0
  * any other commit                          -> z += 1
  * print one line every time y or z changes:
        <commit-date>  <sha8>  <subject-first-50-chars>  <x>.<y>.<z>

Run from any directory inside the target git repository:

    python scripts/version-calc.py
"""

from __future__ import annotations

import subprocess
import sys
from typing import Tuple


def run_git(*args: str) -> str:
    proc = subprocess.run(
        ["git", *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
        text=True,
    )
    return proc.stdout


def is_feature(subject: str) -> bool:
    return subject.lstrip().lower().startswith("feat")


def iter_commits() -> Tuple[str, str, str]:
    """Yield (full_hash, iso_date, subject) from oldest to newest."""
    log = run_git(
        "log",
        "--reverse",
        "--pretty=format:%H%x1f%aI%x1f%s",
    )
    for line in log.splitlines():
        parts = line.split("\x1f", 2)
        if len(parts) != 3:
            continue
        full_hash, date, subject = parts
        yield full_hash, date, subject


def main() -> int:
    x = 0
    y = 0
    z = 0

    for full_hash, date, subject in iter_commits():
        short = full_hash[:8]
        if is_feature(subject):
            y += 1
            z = 0
        else:
            z += 1
        headline = subject.split("\n", 1)[0][:50]
        print(f"{date}  {short}  {headline}  {x}.{y}.{z}")

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except subprocess.CalledProcessError as exc:
        sys.stderr.write(f"git failed: {exc.stderr.strip() or exc}\n")
        sys.exit(1)
    except KeyboardInterrupt:
        sys.exit(130)
