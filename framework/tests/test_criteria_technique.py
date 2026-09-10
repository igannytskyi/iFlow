#!/usr/bin/env python3
"""Arbiter for SPEC-010. Run: python3 framework/tests/test_criteria_technique.py

CR-010-01  a criterion that can be satisfied by editing what it names is detected
CR-010-02  nothing added here claims an enforcement it lacks
CR-010-03  the gate's verdict on existing changes does not move
"""
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
RULES = ROOT / "framework" / "rules.md"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced", "arbiter-influence",
                "arbiter-acceptance", "repeatability", "escalation-ledger", "evidence-plan",
                "human-control")


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout + r.stderr


def cr_010_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp) / "c"
        shutil.copytree(ROOT / "changes" / "criteria-technique", d)
        spec = d / "01-specification.md"
        spec.write_text(spec.read_text().replace(
            "| CR-010-01 | a criterion that can be satisfied by editing what it names is detected |",
            "| CR-010-01 | framework/check.py rejects such a criterion |"))
        out = gate(d)
        if "R19" not in out:
            return "a criterion naming what the change may touch was not detected"
        if "CR-010-01" not in out:
            return "the offending criterion was not named"
    return None


def _unenforced(rules_text, gate_text):
    """Rules claiming mechanical enforcement that nothing emits, pointers followed."""
    emitted = set(re.findall(r'"(R\d+b?|B\d)[":]', gate_text))
    row = re.compile(r"^\|\s*\*\*(?P<id>[A-Z]\d+b?)\*\*\s*\|(?P<rest>.*)\|\s*$")
    missing = []
    for line in rules_text.splitlines():
        m = row.match(line.strip())
        if not m:
            continue
        cells = [c.strip() for c in m.group("rest").split("|")]
        if len(cells) < 3 or cells[1].strip("`") != "M":
            continue
        rid = m.group("id")
        if rid in emitted or any(v in emitted for v in re.findall(r"\b(R\d+b?|B\d)\b", cells[2])):
            continue
        missing.append(rid)
    return sorted(missing)


def cr_010_02():
    """Nothing claims an enforcement it lacks — and the detector can say so.

    Two assertions, because one alone would be vacuous: a rule invented for the
    purpose must be caught, and the real rules must have none.
    """
    gate_text = GATE.read_text()
    invented = "| **R99** | An invented rule | `M` | Nothing emits this |"
    if "R99" not in _unenforced(RULES.read_text() + "\n" + invented, gate_text):
        return "a rule claiming enforcement nothing provides would not be caught"
    real = _unenforced(RULES.read_text(), gate_text)
    return real and f"claimed mechanical, never emitted: {', '.join(real)}" or None


def cr_010_03():
    before = ROOT / "changes" / "criteria-technique" / "baseline" / "gate-before.txt"
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
    for name, fn in (("CR-010-01", cr_010_01), ("CR-010-02", cr_010_02),
                     ("CR-010-03", cr_010_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
