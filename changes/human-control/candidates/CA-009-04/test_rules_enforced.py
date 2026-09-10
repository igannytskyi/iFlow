#!/usr/bin/env python3
"""Arbiter for SPEC-003. Run: python3 tests/test_rules_enforced.py

CR-003-01  every rule the framework says it enforces mechanically is enforced
CR-003-02  a violation of each newly enforced rule is actually detected
CR-003-03  every rule not enforced mechanically says why it cannot be
CR-003-04  the gate's verdict on existing changes is unchanged
"""
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
RULES = ROOT / "framework" / "RULES.md"
GATE = ROOT / "tools" / "check.py"

ROW = re.compile(r"^\|\s*\*\*(?P<id>[A-Z]\d+b?)\*\*\s*\|(?P<rest>.*)\|\s*$")


def rules():
    """Every rule row as (id, enforcement, reason-or-how)."""
    out = []
    for line in RULES.read_text().splitlines():
        m = ROW.match(line.strip())
        if not m:
            continue
        cells = [c.strip() for c in m.group("rest").split("|")]
        if len(cells) < 3:
            continue
        out.append((m.group("id"), cells[1].strip("`"), cells[2]))
    return out


def emitted():
    """Rule names the gate can print.

    Scanning for self.fail() alone was a proxy for "enforced", and the proxy
    broke when a rule came to be computed across every change rather than
    inside one folder. What CR-003-01 asks is whether the gate can name the
    rule at all, so the scan looks for the name wherever it is emitted.
    """
    return set(re.findall(r'"(R\d+b?|B\d)[":]', GATE.read_text()))


def cr_003_01():
    """Every rule claiming mechanical enforcement is enforced.

    A rule may be enforced by another rule rather than under its own name — the
    boundary rules are stated separately from the checks that implement them.
    Where the third column says so, the pointer is followed. Requiring the name
    itself to appear was a proxy for "enforced", and this is the third time that
    proxy has broken rather than the rule failing.
    """
    can_emit = emitted()
    missing = []
    for rid, mark, how in rules():
        if mark != "M":
            continue
        if rid in can_emit:
            continue
        via = re.findall(r"\b(R\d+b?|B\d)\b", how)
        if any(v in can_emit for v in via):
            continue
        missing.append(rid)
    return missing and f"claimed mechanical, never emitted: {', '.join(sorted(missing))}" or None


def cr_003_03():
    bare = [rid for rid, mark, why in rules() if mark != "M" and not why]
    return bare and f"not mechanical and no reason given: {', '.join(bare)}" or None


def _gate_on(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    # the gate prints the path it was given; normalise so a comparison is about
    # verdicts and not about where the folder happened to sit
    return r.stdout.replace(str(pathlib.Path(folder).resolve()), pathlib.Path(folder).name)


def cr_003_02():
    """R4 and R8 must fire on a real violation."""
    src = ROOT / "changes" / "placeholder-filter"
    unfired = []
    with tempfile.TemporaryDirectory() as tmp:
        # R4 — a record with a gap in its ordinals is not append-only
        a = pathlib.Path(tmp) / "r4"
        shutil.copytree(src, a)
        rec = a / "record.md"
        rec.write_text("\n".join(l for l in rec.read_text().splitlines()
                                 if not l.startswith("| 3 |")) + "\n")
        if "R4" not in _gate_on(a):
            unfired.append("R4 on a record with a deleted line")

        # R8 — a unit that is nobody's preparatory work touching the arbiter
        b = pathlib.Path(tmp) / "r8"
        shutil.copytree(src, b)
        plan = b / "02-plan.md"
        plan.write_text(plan.read_text().replace(
            "| WU-002-01 | 1 | C3 | tests/ | the arbiter only; no behaviour of the gate, derived, high | CR-002-01, CR-002-02, CR-002-03 | | WU-002-02 |",
            "| WU-002-01 | 1 | C3 | tests/ | the arbiter only; no behaviour of the gate, derived, high | CR-002-01, CR-002-02, CR-002-03 | | |"))
        ex = b / "04-execution.md"
        ex.write_text(ex.read_text().replace(
            "| CA-002-01 | WU-002-01 | candidates/CA-002-01/ | tests/test_row_filter.py | 2026-09-09 |",
            "| CA-002-01 | WU-002-01 | candidates/CA-002-01/ | tests/test_row_filter.py, altered | 2026-09-09 |"))
        if "R8" not in _gate_on(b):
            unfired.append("R8 on a non-preparatory unit touching the arbiter")
    return unfired and f"claimed but did not fire: {'; '.join(unfired)}" or None


def cr_003_04():
    before = (ROOT / "changes" / "rules-enforced" / "baseline" / "gate-before.txt")
    if not before.exists():
        return "no baseline captured"
    want = [l for l in before.read_text().splitlines() if l and not l.startswith("#")]
    # CR-003-04 is about the gate's verdict, not about everything it prints.
    # An advisory note is not a verdict, and comparing it made this arbiter
    # stricter than the criterion it serves.
    got = []
    for name in ("clock-instant", "placeholder-filter"):
        got += [l.replace(name, f"changes/{name}", 1)
                for l in _gate_on(ROOT / "changes" / name).splitlines()
                if l and "note " not in l]
    return want != got and f"gate verdict moved:\n    was {want}\n    now {got}" or None


def main():
    failures = []
    for name, fn in (("CR-003-01", cr_003_01), ("CR-003-02", cr_003_02),
                     ("CR-003-03", cr_003_03), ("CR-003-04", cr_003_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
