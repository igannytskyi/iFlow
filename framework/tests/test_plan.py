#!/usr/bin/env python3
"""Arbiter for area 2 — what a plan is owed before anyone writes it.

CR-020-01  what a scope reaches, what could judge a change there, who is beyond
           reach and what already holds the ground are derived from the estate
           and each says what it rests on
CR-020-02  a region nothing judges is reported as not decidable as it stands,
           and the draft carries a preparatory unit for it
CR-020-03  what is judgement is left blank and said to be judgement: the class
           of a unit, how the work divides, what each unit is called
CR-020-04  a scope naming nothing that exists derives nothing, rather than
           deriving from nothing
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

PLAN = ROOT / "framework" / "plan.py"

TESTS = {
    "CR-020-01": "direct",
    "CR-020-02": "direct",
    "CR-020-03": "direct",
    "CR-020-04": "direct",
}

SPEC = """# Specification — built

| Field | Value |
|---|---|
| Id | SPEC-900 |
| Intent | INT-900 |
| Default class | C2 |

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-900-01 | the engine starts | it is asked to | it starts | required | machine | test-run | it started |

## Scope

| Included | Excluded |
|---|---|
| {scope} | tests/ |
"""


def run(cwd, *args):
    return subprocess.run([sys.executable, str(PLAN), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def estate_with(tmp, scope="core.py"):
    d = pathlib.Path(tmp)
    (d / "core.py").write_text("class Engine:\n    def start(self):\n        return 1\n")
    (d / "unwatched.py").write_text("def nobody_tests_this():\n    return 2\n")
    (d / "user.py").write_text(
        "from core import Engine\n\n\ndef drive():\n    return Engine().start()\n")
    (d / "tests").mkdir()
    (d / "tests" / "test_core.py").write_text(
        "from core import Engine\n\n\ndef test_start():\n    assert Engine().start() == 1\n")
    (d / "changes" / "c").mkdir(parents=True)
    (d / "changes" / "c" / "01-specification.md").write_text(SPEC.format(scope=scope))
    return d


def cr_020_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "changes/c")
        if "user.py" not in out and "1 file(s)" not in out:
            return f"what the scope reaches was not derived:\n{out}"
        if "what could judge a change there" not in out:
            return "nothing was said about what could judge a change in the scope"
        if "beyond reach" not in out:
            return "nothing was said about consumers no change reaches"
        if "already holds this ground" not in out:
            return "nothing was said about unfinished work on the same ground"
        if "reached firmly" not in out:
            return "reach was reported without saying how firmly it is known"
    return None


def cr_020_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, scope="unwatched.py")
        out = run(d, "changes/c")
        if "unclaimed" not in out:
            return f"a region nothing names was not reported as unjudged:\n{out}"
        if "not decidable as it stands" not in out:
            return "an unjudged region was not said to make the criterion undecidable"
        if "preparatory unit" not in out:
            return "an unjudged region did not call for a preparatory unit"
        draft = run(d, "changes/c", "--draft")
        rows = [ln for ln in draft.splitlines() if ln.startswith("| WU-")]
        if len(rows) < 2:
            return f"the draft carried no preparatory unit for it:\n{draft}"
        if "characterization" not in rows[0]:
            return "the preparatory unit does not say what it is for"
        if "Preparatory for" not in draft or "WU-900-02" not in rows[0]:
            return "the preparatory unit does not name what depends on it"
    return None


def cr_020_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        draft = run(d, "changes/c", "--draft")
        if "<class>" not in draft:
            return "a class was filled in, and no repository says which class work is"
        if "judgement" not in draft:
            return "what was left blank was not said to be judgement"
        report = run(d, "changes/c")
        if "not derivable from a repository" not in report:
            return "the report did not say where derivation stops"
    return None


def cr_020_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp, scope="nowhere/at/all.py")
        out = run(d, "changes/c")
        if "names nothing that exists" not in out:
            return f"a scope over nothing was planned over anyway:\n{out}"
        if "reached firmly" in out:
            return "reach was derived from a scope that names nothing"
    return None


def main():
    failures = []
    for name, fn in (("CR-020-01", cr_020_01), ("CR-020-02", cr_020_02),
                     ("CR-020-03", cr_020_03), ("CR-020-04", cr_020_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
