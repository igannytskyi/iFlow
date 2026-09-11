#!/usr/bin/env python3
"""Arbiter for ground the index cannot read.

CR-016-01  a repository in a language this index cannot read is reported unread,
           not empty
CR-016-02  every answer says how much of the estate it covers
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-016-01": "direct",
    "CR-016-02": "direct",
}


def run(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def foreign(where):
    d = pathlib.Path(where)
    (d / "src").mkdir(parents=True)
    (d / "src" / "index.ts").write_text("export function alpha() { return 1 }\n")
    (d / "src" / "other.ts").write_text("export function beta() { return 2 }\n")
    return d


def cr_016_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        out = json.loads(run(d, "observability", "src"))
        if out.get("verdict") != "unknown":
            return "a region in an unreadable language was not reported unknown"
        if "unread one" not in out.get("why", ""):
            return "an unread region was not distinguished from an empty one"
    return None


def cr_016_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        for cmd in (("freshness",), ("unknown",), ("affects", "src/index.ts")):
            out = run(d, *cmd)
            if "are not absent — they are unseen" not in out:
                return f"`{cmd[0]}` reported a figure without saying what it covers"
            if ".ts" not in out:
                return f"`{cmd[0]}` did not name what it could not read"
    return None


def main():
    failures = []
    for name, fn in (("CR-016-01", cr_016_01), ("CR-016-02", cr_016_02)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
