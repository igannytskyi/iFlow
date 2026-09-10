#!/usr/bin/env python3
"""Arbiter for SPEC-009. Run: python3 framework/tests/test_human_control.py

CR-009-01  a refusal is recorded and its producer is not treated as complete
CR-009-02  correcting criteria is free until a candidate exists and impossible after
CR-009-03  whether a person read the criteria before admission is recorded either way
CR-009-04  a change built from the current shapes passes the gate
"""
import sys
import tempfile

from harness import build_change, edit, gate

TESTS = {
    "CR-009-01": "direct",
    "CR-009-02": "direct",
    "CR-009-03": "direct",
    "CR-009-04": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.


REFUSAL = """
## Refusals

| Id | Object | Boundary | Refused by | Ground | At |
|---|---|---|---|---|---|
| RF-001-01 | CA-001-01 | execution to assurance | person | the change does not serve the intent | 2026-09-10 |
"""


def cr_009_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        (d / "05-assurance.md").write_text((d / "05-assurance.md").read_text() + REFUSAL)
        out = gate(d)
        if "R17" not in out:
            return "a candidate carrying a met verdict while refused was not detected"
        if "CA-001-01" not in out:
            return "the refused object was not named"
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        (d / "05-assurance.md").write_text(
            (d / "05-assurance.md").read_text()
            + REFUSAL.replace("| the change does not serve the intent |", "|  |"))
        if "R17" not in gate(d):
            return "a refusal with no ground was not detected"
    return None


def cr_009_02():
    with tempfile.TemporaryDirectory() as tmp:            # a candidate exists: a violation
        d = build_change(tmp + "/c")
        edit(d, "01-specification.md", "| C1T |", "| C1T |\n\nan edit after a candidate exists")
        if "R1" not in gate(d):
            return "an edit after a candidate exists was not treated as a violation"
    stages = ["00-intent", "01-specification", "02-plan", "03-admission"]
    with tempfile.TemporaryDirectory() as tmp:            # nothing executed: a re-admission
        d = build_change(tmp + "/c", only=stages)
        edit(d, "01-specification.md", "| C1T |", "| C1T |\n\nan edit while nothing has executed")
        rec = d / "record.md"
        rec.write_text(rec.read_text() + "| 5 | 2026-09-10 | specification | fixture | | re-admitted |\n")
        out = gate(d)
        if "FAIL" in out:
            return f"an edit before any candidate was treated as a violation: {out.strip()}"
        if "re-admission" not in out:
            return "a re-admission was not reported as such"
    with tempfile.TemporaryDirectory() as tmp:            # nothing executed, unrecorded
        d = build_change(tmp + "/c", only=stages)
        edit(d, "01-specification.md", "| C1T |", "| C1T |\n\nan unrecorded edit")
        if "R1" not in gate(d):
            return "an unrecorded re-admission was not detected"
    return None


def cr_009_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        p = d / "03-admission.md"
        p.write_text("\n".join(l for l in p.read_text().splitlines()
                               if "Reviewed before admission" not in l
                               and not l.startswith("| yes | a person")))
        if "R18" not in gate(d):
            return "silence about whether the criteria were reviewed was not detected"
    return None


def cr_009_04():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-009-01", cr_009_01), ("CR-009-02", cr_009_02),
                     ("CR-009-03", cr_009_03), ("CR-009-04", cr_009_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
