#!/usr/bin/env python3
"""Arbiter for SPEC-007. Run: python3 tests/test_escalation_ledger.py

CR-007-01  an escalation resolved by a later change stops appearing as owed
CR-007-02  settling what was never owed, or was already settled, is detected
CR-007-03  the gate's verdict on existing changes is unchanged

Each check tests exactly the criterion it names. CR-007-03 normalises away where
a folder sits and any advisory note, neither being part of a verdict.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced", "arbiter-influence",
                "arbiter-acceptance", "repeatability")

CLOSED = """
## Escalations closed

| Escalation | Resolved by | Note |
|---|---|---|
| {ref} | this change | settled |
"""


def gate(*args):
    r = subprocess.run([sys.executable, str(GATE), *[str(a) for a in args]],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout + r.stderr


def ledger(root=None):
    r = subprocess.run([sys.executable, str(GATE), "--escalations"],
                       capture_output=True, text=True, cwd=root or ROOT)
    return r.stdout + r.stderr


def _tree(tmp):
    """A copy of changes/ that can be edited without touching the repository."""
    d = pathlib.Path(tmp) / "repo"
    d.mkdir()
    (d / "framework").mkdir()
    shutil.copy(GATE, d / "framework" / "check.py")
    shutil.copytree(ROOT / "changes", d / "changes")
    return d


def cr_007_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = _tree(tmp)
        before = ledger(d)
        if "repeatability#2" not in before:
            return "an open escalation was not listed as owed"
        a = d / "changes" / "clock-instant" / "05-assurance.md"
        a.write_text(a.read_text() + CLOSED.format(ref="repeatability#2"))
        after = ledger(d)
        if "repeatability#2" in after.split("still owed")[-1]:
            return "an escalation closed by another change is still reported as owed"
    return None


def cr_007_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = _tree(tmp)
        a = d / "changes" / "clock-instant" / "05-assurance.md"
        a.write_text(a.read_text() + CLOSED.format(ref="repeatability#99"))
        if "R15" not in ledger(d):
            return "settling something never owed was not detected"
    with tempfile.TemporaryDirectory() as tmp:
        d = _tree(tmp)
        a = d / "changes" / "clock-instant" / "05-assurance.md"
        b = d / "changes" / "placeholder-filter" / "05-assurance.md"
        a.write_text(a.read_text() + CLOSED.format(ref="repeatability#2"))
        b.write_text(b.read_text() + CLOSED.format(ref="repeatability#2"))
        if "R15" not in ledger(d):
            return "settling the same debt twice was not detected"
    return None


def cr_007_03():
    before = ROOT / "changes" / "escalation-ledger" / "baseline" / "gate-before.txt"
    lines = [l for l in before.read_text().splitlines() if l and not l.startswith("#")]
    want = [l for l in lines if "note " not in l]
    got = []
    for name in PRE_EXISTING:
        got += [l.replace(str(ROOT / "changes" / name), f"changes/{name}")
                for l in gate(ROOT / "changes" / name).splitlines()
                if l and "note " not in l]
    return want != got and f"gate verdict moved:\n    was {want}\n    now {got}" or None


def main():
    failures = []
    for name, fn in (("CR-007-01", cr_007_01), ("CR-007-02", cr_007_02),
                     ("CR-007-03", cr_007_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
