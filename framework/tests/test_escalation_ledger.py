#!/usr/bin/env python3
"""Arbiter for SPEC-007. Run: python3 framework/tests/test_escalation_ledger.py

CR-007-01  an escalation resolved by a later change stops appearing as owed
CR-007-02  settling what was never owed, or was already settled, is detected
CR-007-03  a change built from the current shapes passes the gate
"""
import subprocess
import sys
import tempfile

from harness import GATE, add_escalation, build_change, build_repo, close_escalation, gate

TESTS = {
    "CR-007-01": "direct",
    "CR-007-02": "direct",
    "CR-007-03": ("proxy", "the criterion is about real work; a built change is run instead"),
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.



def ledger(root):
    r = subprocess.run([sys.executable, str(GATE), "--escalations"],
                       capture_output=True, text=True, cwd=root)
    return r.stdout + r.stderr


def cr_007_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_repo(tmp)
        add_escalation(d / "changes" / "one", 1, "open")
        if "one#1" not in ledger(d):
            return "an open escalation was not listed as owed"
        close_escalation(d / "changes" / "two", "one#1")
        if "one#1" in ledger(d).split("still owed")[-1]:
            return "an escalation closed by another change is still reported as owed"
    return None


def cr_007_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_repo(tmp)
        close_escalation(d / "changes" / "two", "one#99")
        if "R15" not in ledger(d):
            return "settling something never owed was not detected"
    with tempfile.TemporaryDirectory() as tmp:
        d = build_repo(tmp)
        add_escalation(d / "changes" / "one", 1, "open")
        close_escalation(d / "changes" / "one", "one#1")
        close_escalation(d / "changes" / "two", "one#1")
        if "R15" not in ledger(d):
            return "settling the same debt twice was not detected"
    return None


def cr_007_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-007-01", cr_007_01), ("CR-007-02", cr_007_02), ("CR-007-03", cr_007_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
