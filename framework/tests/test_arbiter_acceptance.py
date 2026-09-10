#!/usr/bin/env python3
"""Arbiter for SPEC-005. Run: python3 framework/tests/test_arbiter_acceptance.py

CR-005-01  a criterion nothing claims to arbitrate is detected
CR-005-02  a unit that builds an arbiter records its own pre-acceptance outcome
CR-005-03  a change built from the current shapes passes the gate
"""
import sys
import tempfile

from harness import build_change, edit, gate, reseal

TESTS = {
    "CR-005-01": "direct",
    "CR-005-02": "direct",
    "CR-005-03": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.



def cr_005_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| judged/ | CR-001-01, CR-001-02 |", "| judged/ | CR-001-01 |")
        reseal(d)
        out = gate(d)
        if "R11" not in out:
            return "a criterion claimed by no arbiter was not detected"
        if "CR-001-02" not in out:
            return "the unarbitrated criterion was not named"
    return None


def cr_005_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "02-plan.md", "| CR-001-01, CR-001-02 | | |", "| CR-001-01, CR-001-02 | | WU-001-02 |")
        edit(d, "02-plan.md", "| WU-001-01 | 1 | C1T | subject/ |", "| WU-001-01 | 1 | C1T | judged/ |")
        out = gate(d)
        if "R12" not in out:
            return "a unit building an arbiter with no pre-acceptance record was not detected"
    return None


def cr_005_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-005-01", cr_005_01), ("CR-005-02", cr_005_02), ("CR-005-03", cr_005_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
