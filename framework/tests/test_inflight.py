#!/usr/bin/env python3
"""Arbiter for the overlay of work that has not landed.

CR-014-01  a unit planned and not entered is held to be in flight; one that
           entered is not
CR-014-02  ground already held by unfinished work is reported, and says whether
           the two merely declare the same region or one reaches into the other
CR-014-03  a declaration is trusted less than a reach, because a declaration is
           what someone wrote down
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT, build_change, edit

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-014-01": "direct",
    "CR-014-02": "direct",
    "CR-014-03": "direct",
}


def ask(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def estate_with(where, entered):
    """One change holding a region, either entered or still in flight."""
    d = pathlib.Path(where)
    (d / "changes").mkdir(parents=True)
    c = build_change(d / "changes" / "held")
    if not entered:
        edit(c, "06-landing.md", "| 1 | WU-001-01 | settled | yes | 2026-09-10 |",
             "| 1 | WU-001-01 | settled | yes |  |")
    (d / "subject").mkdir(exist_ok=True)
    (d / "subject" / "thing.py").write_text("def alpha():\n    return 1\n")
    return d


def cr_014_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, entered=False)
        if "held/WU-001-01" not in ask(d, "inflight", "changes"):
            return "a unit planned and not entered was not held to be in flight"
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, entered=True)
        if "held/WU-001-01" in ask(d, "inflight", "changes"):
            return "a unit that entered was still reported in flight"
    return None


def cr_014_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, entered=False)
        out = ask(d, "conflicts", "changes", "subject/")
        if "held/WU-001-01" not in out:
            return "ground already held by unfinished work was not reported"
        if "declare the same region" not in out:
            return "the report did not say on what footing the two collide"
    return None


def cr_014_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, entered=False)
        out = ask(d, "conflicts", "changes", "subject/")
        line = [l for l in out.splitlines() if "held/WU-001-01" in l][0]
        if "low" not in line:
            return f"a collision of two declarations was trusted too far: {line.strip()}"
    return None


def main():
    failures = []
    for name, fn in (("CR-014-01", cr_014_01), ("CR-014-02", cr_014_02),
                     ("CR-014-03", cr_014_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
