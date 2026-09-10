#!/usr/bin/env python3
"""Arbiter for SPEC-004. Run: python3 framework/tests/test_arbiter_influence.py

CR-004-01  a change that may edit what its arbiter reads is detected
CR-004-02  a check whose column is missing says so instead of passing
CR-004-03  an obligation added later is reported as predating, not violating
CR-004-04  a change built from the current shapes passes the gate
"""
import sys
import tempfile

from harness import build_change, edit, gate, reseal

TESTS = {
    "CR-004-01": "direct",
    "CR-004-02": "direct",
    "CR-004-03": "direct",
    "CR-004-04": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.



def cr_004_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| judged/ | no |", "| subject/ | yes |")
        reseal(d)
        if "R7b" not in gate(d):
            return "an arbiter input inside the scope with nothing captured was not detected"
    return None


def cr_004_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        edit(d, "05-assurance.md", "| Independence |", "| Indep |")
        out = gate(d)
        if "B1" not in out or "Independence" not in out:
            return "a renamed column disabled a check without the gate saying so"
    return None


def cr_004_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        p = d / "01-specification.md"
        t = p.read_text()
        p.write_text(t[:t.index("## Arbiter reads")])
        reseal(d)
        out = gate(d)
        if "FAIL" in out:
            return "a change predating an obligation was failed rather than noted"
        if "predates" not in out:
            return "the absence of a later obligation was not reported as predating it"
    return None


def cr_004_04():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-004-01", cr_004_01), ("CR-004-02", cr_004_02), ("CR-004-03", cr_004_03), ("CR-004-04", cr_004_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
