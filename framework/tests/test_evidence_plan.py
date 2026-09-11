#!/usr/bin/env python3
"""Arbiter for SPEC-008. Run: python3 framework/tests/test_evidence_plan.py

CR-008-01  a criterion needing an evidence plan has one before it is executed against
CR-008-02  a plan says what it was derived from
CR-008-03  a change built from the current shapes passes the gate
CR-008-04  a plan naming ground the change does not reach is refused: an area of
           effect is derived from the estate, and the estate can now be asked
"""
import pathlib
import sys
import tempfile

from harness import build_change, edit, gate, reseal

TESTS = {
    "CR-008-01": "direct",
    "CR-008-02": ("proxy", "the criterion is whether a plan follows from the estate; what is "
                           "checked is whether it says what it was derived from"),
    "CR-008-03": ("proxy", "the criterion is about real work; a built change is run instead"),
    "CR-008-04": "direct",
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


def estate_with_change(tmp):
    """A small estate, and a change carried inside it — because what a plan is
    derived from is the estate the change lives in, not the change folder."""
    root = pathlib.Path(tmp) / "estate"
    (root / "subject").mkdir(parents=True)
    (root / "subject" / "core.py").write_text("def alpha():\n    return 1\n")
    (root / "subject" / "user.py").write_text(
        "from core import alpha\n\n\ndef beta():\n    return alpha()\n")
    (root / "elsewhere").mkdir()
    (root / "elsewhere" / "stranger.py").write_text("def unrelated():\n    return 0\n")
    d = build_change(str(root / "changes" / "c"))
    edit(d, "01-specification.md", "| subject/ |", "| subject/core.py |")
    edit(d, "01-specification.md", "| not-required |", "| required |")
    reseal(d)
    return d


def cr_008_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with_change(tmp)
        edit(d, "03-admission.md", PLAN_HEAD,
             PLAN_HEAD + "| CR-001-01 | subject/user.py | run it | no change | "
                         "the area of effect of subject/core.py | 2026-09-11 |\n")
        out = gate(d)
        if "R16" in out:
            return f"a plan naming ground the change reaches was refused:\n{out}"
        edit(d, "03-admission.md", "| CR-001-01 | subject/user.py |",
             "| CR-001-01 | elsewhere/stranger.py |")
        out = gate(d)
        if "R16" not in out:
            return "a plan naming ground nothing in the scope reaches was accepted"
        if "elsewhere/stranger.py" not in out:
            return "the ground the plan names wrongly was not named back"
    return None


def main():
    failures = []
    for name, fn in (("CR-008-01", cr_008_01), ("CR-008-02", cr_008_02), ("CR-008-03", cr_008_03), ("CR-008-04", cr_008_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
