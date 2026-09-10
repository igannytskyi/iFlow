#!/usr/bin/env python3
"""Arbiter for SPEC-008. Run: python3 tests/test_evidence_plan.py

CR-008-01  a criterion needing an evidence plan has one before it is executed against
CR-008-02  a plan says what it was derived from
CR-008-03  the gate's verdict on existing changes does not move

Each check tests exactly the criterion it names. CR-008-03 normalises away where
a folder sits and any advisory note, neither being part of a verdict.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
GATE = ROOT / "tools" / "check.py"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced", "arbiter-influence",
                "arbiter-acceptance", "repeatability", "escalation-ledger")

PLAN_HEAD = ("| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |\n"
             "|---|---|---|---|---|---|\n")


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout + r.stderr


def _needing_plan(dst, plan_row=None):
    """A change whose first criterion declares it needs a plan."""
    spec = dst / "01-specification.md"
    t = spec.read_text().replace("| not-required |", "| required |", 1)
    spec.write_text(t)
    adm = dst / "03-admission.md"
    a = adm.read_text()
    a = a.replace(PLAN_HEAD, PLAN_HEAD + (plan_row + "\n" if plan_row else ""))
    adm.write_text(a)


def cr_008_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "evidence-plan", d)
        _needing_plan(d)                       # declares required, supplies none
        (d / "04-execution.md").write_text("# Execution\n")
        out = gate(d)
        if "R16" not in out:
            return "a criterion needing a plan and having none was not detected"
        if "CR-008-01" not in out:
            return "the criterion missing its plan was not named"
    return None


def cr_008_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "evidence-plan", d)
        _needing_plan(d, "| CR-008-01 | the gate | run it | no change | | 2026-09-10 |")
        out = gate(d)
        if "R16" not in out:
            return "a plan with no area of effect behind it was not detected"
    return None


def cr_008_03():
    before = ROOT / "changes" / "evidence-plan" / "baseline" / "gate-before.txt"
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
    for name, fn in (("CR-008-01", cr_008_01), ("CR-008-02", cr_008_02),
                     ("CR-008-03", cr_008_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
