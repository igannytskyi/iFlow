#!/usr/bin/env python3
"""Arbiter for SPEC-009. Run: python3 tests/test_human_control.py

CR-009-01  a refusal is recorded and its producer is not treated as complete
CR-009-02  correcting criteria is free until a candidate exists and impossible after
CR-009-03  whether a person read the criteria before admission is recorded either way
CR-009-04  the gate's verdict on existing changes does not move
"""
import pathlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
PRE_EXISTING = ("clock-instant", "placeholder-filter", "rules-enforced", "arbiter-influence",
                "arbiter-acceptance", "repeatability", "escalation-ledger", "evidence-plan")

REFUSAL = """
## Refusals

| Id | Object | Boundary | Refused by | Ground | At |
|---|---|---|---|---|---|
| RF-002-01 | CA-002-02 | execution to assurance | person | the repair does not address the reported symptom | 2026-09-10 |
"""


def gate(folder):
    r = subprocess.run([sys.executable, str(GATE), str(folder)],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout + r.stderr


def _copy(tmp, name, before_execution=False):
    """A fixture built rather than borrowed.

    Borrowing a live folder makes the fixture depend on how far that change has
    since progressed: this arbiter passed while human-control had no candidate
    and failed the moment it gained one. Where the case is "nothing has executed
    yet", the later stages are removed and the record truncated to the admission
    crossing, so the fixture says what it means regardless of the estate.
    """
    d = pathlib.Path(tmp) / "c"
    shutil.copytree(ROOT / "changes" / name, d)
    if before_execution:
        for stage in ("04-execution", "05-assurance", "06-landing"):
            f = d / f"{stage}.md"
            if f.exists():
                f.unlink()
        rec = d / "record.md"
        kept, seen_admission = [], False
        for line in rec.read_text().splitlines():
            if "| admission |" in line:
                seen_admission = True
            elif seen_admission and line.startswith("|") and "| Stage |" not in line:
                if any(f"| {s} |" in line for s in ("execution", "assurance", "landing")):
                    continue
            kept.append(line)
        rec.write_text("\n".join(kept) + "\n")
    return d


def cr_009_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = _copy(tmp, "placeholder-filter")
        a = d / "05-assurance.md"
        a.write_text(a.read_text() + REFUSAL)
        out = gate(d)
        if "R17" not in out:
            return "a candidate carrying a met verdict while refused was not detected"
        if "CA-002-02" not in out:
            return "the refused object was not named"
    with tempfile.TemporaryDirectory() as tmp:
        d = _copy(tmp, "placeholder-filter")
        a = d / "05-assurance.md"
        a.write_text(a.read_text() + REFUSAL.replace(
            "| the repair does not address the reported symptom |", "|  |"))
        if "R17" not in gate(d):
            return "a refusal with no ground was not detected"
    return None


def cr_009_02():
    with tempfile.TemporaryDirectory() as tmp:          # candidate exists — a violation
        d = _copy(tmp, "placeholder-filter")
        s = d / "01-specification.md"
        s.write_text(s.read_text() + "\nan edit after a candidate exists\n")
        if "R1" not in gate(d):
            return "an edit after a candidate exists was not treated as a violation"
    with tempfile.TemporaryDirectory() as tmp:          # no candidate — a re-admission
        d = _copy(tmp, "human-control", before_execution=True)
        s = d / "01-specification.md"
        s.write_text(s.read_text() + "\nan edit while nothing has executed\n")
        r = d / "record.md"
        n = sum(1 for l in r.read_text().splitlines() if l.startswith("| ") and l[2].isdigit())
        r.write_text(r.read_text() + f"| {n+1} | 2026-09-10 | specification | SPEC-009 | | re-admitted |\n")
        out = gate(d)
        if "FAIL" in out:
            return f"an edit before any candidate was treated as a violation: {out.strip()}"
        if "re-admission" not in out:
            return "a re-admission was not reported as such"
    with tempfile.TemporaryDirectory() as tmp:          # no candidate, unrecorded
        d = _copy(tmp, "human-control", before_execution=True)
        s = d / "01-specification.md"
        s.write_text(s.read_text() + "\nan unrecorded edit\n")
        if "R1" not in gate(d):
            return "an unrecorded re-admission was not detected"
    return None


def cr_009_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = _copy(tmp, "human-control", before_execution=True)
        a = d / "03-admission.md"
        a.write_text("\n".join(l for l in a.read_text().splitlines()
                               if "Reviewed before admission" not in l
                               and not l.startswith("| yes | Illia")))
        out = gate(d)
        if "R18" not in out:
            return "silence about whether the criteria were reviewed was not detected"
    return None


def cr_009_04():
    before = ROOT / "changes" / "human-control" / "baseline" / "gate-before.txt"
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
