#!/usr/bin/env python3
"""Arbiter for SPEC-006. Run: python3 tests/test_repeatability.py

CR-006-01  evidence says whether the run behind it can be produced again
CR-006-02  a result is not accepted on an unrepeatable run alone
CR-006-03  the gate's verdict on existing changes is unchanged

Each check tests exactly the criterion it names. The comparison in CR-006-03
normalises away where a folder sits and any advisory note, because neither is
part of a verdict.
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced",
                "arbiter-influence", "arbiter-acceptance")

EV_HEAD = ("| Id | Subject | Kind | Producer | Independence | How established "
           "| Repeatable | Obtained at | Valid until |")


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout


def _with_evidence(dst, repeatable, outcome="met"):
    """An assurance stage carrying the new column, so the folder is subject to
    the obligation rather than predating it."""
    a = dst / "05-assurance.md"
    a.write_text(f"""# Assurance — fixture

## Evidence

{EV_HEAD}
|---|---|---|---|---|---|---|---|---|
| EV-002-01-01 | candidate | test-run | an agent's account of a past run | independent-by-precommitment | written before the candidate | {repeatable} | 2026-09-09 | — |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-002-01 | WU-002-01 | CR-002-01 | {outcome} | EV-002-01-01 | settled | | |
| VE-002-02 | WU-002-01 | CR-002-02 | met | EV-002-01-01 | settled | | |
| VE-002-03 | WU-002-02 | CR-002-03 | met | EV-002-01-01 | settled | | |
""")


def cr_006_01():
    """Evidence with the column present but empty is detected."""
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "placeholder-filter", d)
        _with_evidence(d, "")
        out = gate(d)
        if "R13" not in out:
            return "evidence that does not say whether its run repeats was not detected"
        if "EV-002-01-01" not in out:
            return "the evidence missing its repeatability was not named"
    return None


def cr_006_02():
    """A met verdict resting only on an unrepeatable run is detected."""
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "placeholder-filter", d)
        _with_evidence(d, "no")
        out = gate(d)
        if "R14" not in out:
            return "a verdict met on an unrepeatable run alone was not detected"
    return None


def cr_006_03():
    before = ROOT / "changes" / "repeatability" / "baseline" / "gate-before.txt"
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
    for name, fn in (("CR-006-01", cr_006_01), ("CR-006-02", cr_006_02),
                     ("CR-006-03", cr_006_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
