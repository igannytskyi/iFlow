#!/usr/bin/env python3
"""Arbiter for SPEC-005. Run: python3 tests/test_arbiter_acceptance.py

CR-005-01  a criterion nothing claims to arbitrate is detected
CR-005-02  a unit that builds an arbiter records its own pre-acceptance outcome
CR-005-03  the gate's verdict on existing changes is unchanged

Each check below tests exactly the criterion it names and nothing more. Where a
comparison could be perturbed by something the criterion does not mention —
where a folder sits, what advisory notes the gate prints — that is normalised
away and said so here, because an arbiter stricter than its criterion fails for
reasons that have nothing to do with its subject.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced", "arbiter-influence")


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout


def cr_005_01():
    """An unarbitrated criterion is detected."""
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "arbiter-acceptance", d)
        spec = d / "01-specification.md"
        spec.write_text(spec.read_text().replace(
            "| tests/ | CR-005-01, CR-005-02, CR-005-03 |", "| tests/ | CR-005-01 |"))
        out = gate(d)
        if "R11" not in out:
            return "a criterion claimed by no arbiter was not detected"
        if "CR-005-02" not in out:
            return "the unarbitrated criterion was not named"
    return None


def cr_005_02():
    """A unit building an arbiter must record how it failed before acceptance.

    The fixture is an existing change with the section added, so that it is a
    post-obligation folder rather than one that predates the rule: without the
    section at all a change predates and is noted, which is Y7 and not this.
    """
    src = ROOT / "changes" / "placeholder-filter"
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(src, d)
        ex = d / "04-execution.md"
        ex.write_text(ex.read_text() + """
## Arbiter acceptance

| Unit | Criterion | Before repair |
|---|---|---|
| WU-002-99 | CR-002-01 | failed |
""")
        out = gate(d)
        if "R12" not in out:
            return "an arbiter unit with no pre-acceptance record was not detected"
        if "WU-002-01" not in out:
            return "the unit missing its record was not named"
    return None


def cr_005_03():
    """Only the verdict is compared: not the path printed, not advisory notes."""
    before = ROOT / "changes" / "arbiter-acceptance" / "baseline" / "gate-before.txt"
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
    for name, fn in (("CR-005-01", cr_005_01), ("CR-005-02", cr_005_02),
                     ("CR-005-03", cr_005_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
