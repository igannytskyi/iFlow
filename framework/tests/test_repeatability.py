#!/usr/bin/env python3
"""Arbiter for SPEC-006. Run: python3 framework/tests/test_repeatability.py

CR-006-01  evidence says whether the run behind it can be produced again
CR-006-02  a result is not accepted on an unrepeatable run alone
CR-006-03  a change built from the current shapes passes the gate
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import GATE, ROOT, build_change, edit, gate, reseal

TESTS = {
    "CR-006-01": "direct",
    "CR-006-02": "direct",
    "CR-006-04": "direct",
    "CR-006-03": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.



def cr_006_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "05-assurance.md", "| yes | 2026-09-10 |", "|  | 2026-09-10 |")
        out = gate(d)
        if "R13" not in out:
            return "evidence that does not say whether its run repeats was not detected"
        if "EV-001-01-01" not in out:
            return "the evidence missing its repeatability was not named"
    return None


def cr_006_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "05-assurance.md", "| yes | 2026-09-10 |", "| no | 2026-09-10 |")
        if "R14" not in gate(d):
            return "a verdict met on an unrepeatable run alone was not detected"
    return None


def cr_006_04():
    """A claim to repeat must name what to re-run, and must survive re-running."""
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "05-assurance.md", "| framework/tests/test_row_filter.py |",
             "| somebody ran something |")
        if "R13" not in gate(d):
            return "a claim to repeat that names nothing to re-run was not detected"
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        wobbly = pathlib.Path(tmp) / "wobbly.py"
        wobbly.write_text("import random\nprint(random.random())\n")
        edit(d, "05-assurance.md", "| framework/tests/test_row_filter.py |", f"| {wobbly} |")
        r = subprocess.run([sys.executable, str(GATE), "--repeat", str(d)],
                           capture_output=True, text=True, cwd=ROOT)
        if "R13" not in r.stdout:
            return "a producer giving two different results was not detected"
    return None


def cr_006_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-006-01", cr_006_01), ("CR-006-02", cr_006_02), ("CR-006-04", cr_006_04), ("CR-006-03", cr_006_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
