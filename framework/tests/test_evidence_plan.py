#!/usr/bin/env python3
"""Arbiter for SPEC-008. Run: python3 framework/tests/test_evidence_plan.py

CR-008-01  a criterion needing an evidence plan has one before it is executed against
CR-008-02  a plan says what it was derived from
CR-008-03  a change built from the current shapes passes the gate
"""
import sys
import tempfile

from harness import build_change, edit, gate, reseal

TESTS = {
    "CR-008-01": "direct",
    "CR-008-02": ("proxy", "the criterion is whether a plan follows from the estate; what is "
                           "checked is whether it says what it was derived from"),
    "CR-008-03": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.



PLAN_HEAD = ("| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |\n"
             "|---|---|---|---|---|---|\n")


def cr_008_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| not-required |", "| required |")
        reseal(d)
        out = gate(d)
        if "R16" not in out:
            return "a criterion needing a plan and having none was not detected"
        if "CR-001-01" not in out:
            return "the criterion missing its plan was not named"
    return None


def cr_008_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| not-required |", "| required |")
        reseal(d)
        edit(d, "03-admission.md", PLAN_HEAD,
             PLAN_HEAD + "| CR-001-01 | the subject | run it | no change | | 2026-09-10 |\n")
        if "R16" not in gate(d):
            return "a plan with no area of effect behind it was not detected"
    return None


def cr_008_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-008-01", cr_008_01), ("CR-008-02", cr_008_02), ("CR-008-03", cr_008_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
