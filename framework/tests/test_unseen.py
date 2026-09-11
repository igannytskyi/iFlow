#!/usr/bin/env python3
"""Arbiter for ground the index cannot read.

CR-016-01  a repository in a language this index cannot read is reported unread,
           not empty
CR-016-02  every answer says how much of the estate it covers
CR-016-03  a verdict that nothing watches a region says how much of the estate
           it could not read before saying so
CR-016-04  an estate with nothing readable in it is reported as unreadable, not
           as an estate with no regions
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
    "CR-016-03": "direct",
    "CR-016-04": "direct",
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


def mixed(where):
    """A region this index can read, on an estate mostly written in one it
    cannot — the shape of every polyglot estate, where the tests are as likely
    to be on the unread side as anywhere else."""
    d = pathlib.Path(where)
    (d / "svc").mkdir(parents=True)
    (d / "svc" / "handler.py").write_text("def alpha():\n    return 1\n")
    (d / "other").mkdir(parents=True)
    for i in range(3):
        (d / "other" / f"svc{i}_test.go").write_text("package main\n")
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


def cr_016_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = mixed(tmp)
        out = json.loads(run(d, "observability", "svc"))
        if out.get("verdict") != "unclaimed":
            return "a region nothing readable names was not reported unclaimed"
        if "are not absent — they are unseen" not in out.get("read", ""):
            return "a verdict of unclaimed did not say what it could not read"
    return None


def cr_016_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        out = run(d, "observability")
        if "0 region(s)" in out:
            return "an estate this index cannot read was counted as having no regions"
        if "it is one this cannot see" not in out:
            return "an unreadable estate was not distinguished from an empty one"
    return None


def main():
    failures = []
    for name, fn in (("CR-016-01", cr_016_01), ("CR-016-02", cr_016_02),
                     ("CR-016-03", cr_016_03), ("CR-016-04", cr_016_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
