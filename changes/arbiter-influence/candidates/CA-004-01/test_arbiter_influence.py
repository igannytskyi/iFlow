#!/usr/bin/env python3
"""Arbiter for SPEC-004. Run: python3 tests/test_arbiter_influence.py

CR-004-01  a change that may edit what its arbiter reads is detected
CR-004-02  a check whose column is missing says so instead of passing
CR-004-03  an obligation added later is reported as predating, not violating
CR-004-04  the gate's verdict on existing changes is unchanged
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
GATE = ROOT / "tools" / "check.py"


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout


def cr_004_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "arbiter-influence", d)
        adm = d / "03-admission.md"
        adm.write_text(adm.read_text().replace(
            "| CR-004-01 | framework/RULES.md before this change | baseline/RULES.md.before |", ""))
        if "R7b" not in gate(d):
            return "an in-scope arbiter input with no prior state captured was not detected"
    return None


def cr_004_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "placeholder-filter", d)
        a = d / "05-assurance.md"
        a.write_text(a.read_text().replace("| Independence |", "| Indep |"))
        out = gate(d)
        if "B1" not in out or "Independence" not in out:
            return "a renamed column disabled a check without the gate saying so"
    return None


def cr_004_03():
    out = gate(ROOT / "changes" / "clock-instant")
    if "FAIL" in out:
        return "a specification admitted before the obligation was failed retroactively"
    if "predates" not in out:
        return "the absence of a later obligation was not reported as predating it"
    return None


# The changes that already existed when this one was intended. Named explicitly:
# the baseline was captured while this change's own folder was half-created, so
# "everything under changes/" is not a stable definition of what to compare.
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced")


def _verdicts(text, name):
    return [l.replace(str(ROOT / "changes" / name), f"changes/{name}")
            for l in text.splitlines() if l and "predates" not in l]


def cr_004_04():
    before = ROOT / "changes" / "arbiter-influence" / "baseline" / "gate-before.txt"
    lines = [l for l in before.read_text().splitlines() if l and not l.startswith("#")]
    want, keep = [], False
    for l in lines:
        if l.startswith("changes/"):
            keep = any(n in l for n in PRE_EXISTING)
        if keep:
            want.append(l)
    got = []
    for name in PRE_EXISTING:
        got += _verdicts(gate(ROOT / "changes" / name), name)
    return want != got and f"gate verdict moved:\n    was {want}\n    now {got}" or None


def main():
    failures = []
    for name, fn in (("CR-004-01", cr_004_01), ("CR-004-02", cr_004_02),
                     ("CR-004-03", cr_004_03), ("CR-004-04", cr_004_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
