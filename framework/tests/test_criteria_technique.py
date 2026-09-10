#!/usr/bin/env python3
"""Arbiter for SPEC-010. Run: python3 framework/tests/test_criteria_technique.py

CR-010-01  a criterion that can be satisfied by editing what it names is detected
CR-010-02  nothing claims an enforcement it lacks
CR-010-03  a change built from the current shapes passes the gate
"""
import sys
import tempfile

from harness import build_change, edit, gate, reseal

TESTS = {
    "CR-010-01": ("proxy", "the criterion is whether a criterion can be met by editing what "
                           "it names; what is checked is whether its text contains a path "
                           "the change may touch"),
    "CR-010-02": ("proxy", "the criterion is whether a rule is enforced; what is checked "
                           "is whether the gate can name it"),
    "CR-010-03": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.


from test_rules_enforced import rules, unenforced
from harness import GATE


def cr_010_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| the stated outcome holds |", "| subject/ is edited |")
        reseal(d)
        out = gate(d)
        if "R19" not in out:
            return "a criterion naming what the change may touch was not detected"
        if "CR-001-01" not in out:
            return "the offending criterion was not named"
    return None


def cr_010_02():
    text = GATE.read_text()
    if "R99" not in unenforced(rules() + [("R99", "M", "nothing emits this")], text):
        return "a rule claiming enforcement nothing provides would not be caught"
    real = unenforced(rules(), text)
    return real and f"claimed mechanical, never emitted: {', '.join(real)}" or None


def cr_010_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-010-01", cr_010_01), ("CR-010-02", cr_010_02), ("CR-010-03", cr_010_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
