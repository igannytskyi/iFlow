#!/usr/bin/env python3
"""Arbiter for SPEC-003. Run: python3 framework/tests/test_rules_enforced.py

CR-003-01  every rule the framework says it enforces mechanically is enforced
CR-003-02  a violation of a newly enforced rule is actually detected
CR-003-03  every rule not enforced mechanically says why it cannot be
CR-003-04  a change built from the current shapes passes the gate
CR-026-01  a scope naming several paths is read as all of them, however the
           list is separated
CR-026-02  a scope entry naming nothing here is said aloud, not dropped
"""
import re
import sys
import tempfile

from harness import GATE, ROOT, build_change, edit, gate, reseal

TESTS = {
    "CR-003-01": ("proxy", "the criterion is whether a rule is enforced; what is checked "
                           "is whether the gate can name it, pointers followed"),
    "CR-003-02": "direct",
    "CR-003-03": ("proxy", "the criterion is whether a reason is given; what is checked "
                           "is whether the column is non-empty"),
    "CR-003-04": ("proxy", "the criterion is about real work; a built change is run instead"),
    "CR-026-01": "direct",
    "CR-026-02": "direct",
}

# Each criterion says whether this arbiter tests it or a
# proxy for it. A proxy that is not declared has broken three times.


RULES = ROOT / "framework" / "rules.md"
ROW = re.compile(r"^\|\s*\*\*(?P<id>[A-Z]\d+b?)\*\*\s*\|(?P<rest>.*)\|\s*$")


def rules():
    out = []
    for line in RULES.read_text().splitlines():
        m = ROW.match(line.strip())
        if not m:
            continue
        cells = [c.strip() for c in m.group("rest").split("|")]
        if len(cells) >= 3:
            out.append((m.group("id"), cells[1].strip("`"), cells[2]))
    return out


def unenforced(rules_rows, gate_text):
    """A rule may be enforced under another rule's name; the pointer is followed."""
    emitted = set(re.findall(r'"(R\d+b?|B\d)[":]', gate_text))
    missing = []
    for rid, mark, how in rules_rows:
        if mark != "M" or rid in emitted:
            continue
        if any(v in emitted for v in re.findall(r"\b(R\d+b?|B\d)\b", how)):
            continue
        missing.append(rid)
    return sorted(missing)


def cr_003_01():
    text = GATE.read_text()
    invented = [("R99", "M", "nothing emits this")]
    if "R99" not in unenforced(rules() + invented, text):
        return "a rule claiming enforcement nothing provides would not be caught"
    real = unenforced(rules(), text)
    return real and f"claimed mechanical, never emitted: {', '.join(real)}" or None


def cr_003_02():
    """R4 on a record with a line removed; R8 on arbiter work that is nobody's preparation."""
    unfired = []
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/r4")
        rec = d / "record.md"
        rec.write_text("\n".join(l for l in rec.read_text().splitlines()
                                 if not l.startswith("| 3 |")) + "\n")
        if "R4" not in gate(d):
            unfired.append("R4 on a record with a line removed")
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/r8")
        edit(d, "04-execution.md", "| the proposed change |", "| judged/ altered |")
        if "R8" not in gate(d):
            unfired.append("R8 on a unit touching the arbiter that prepares nothing")
    with tempfile.TemporaryDirectory() as tmp:          # a criterion with no verdict
        d = build_change(tmp + "/a5")
        edit(d, "05-assurance.md",
             "| VE-001-02 | WU-001-01 | CR-001-02 | met | EV-001-01-01 | settled | | |", "")
        if "A5" not in gate(d):
            unfired.append("A5 on a criterion nothing rendered a verdict for")
    with tempfile.TemporaryDirectory() as tmp:          # entry on a failed verdict
        d = build_change(tmp + "/a6")
        edit(d, "05-assurance.md", "| CR-001-01 | met |", "| CR-001-01 | failed |")
        if "A6" not in gate(d):
            unfired.append("A6 on a candidate entering on a failed verdict")
    with tempfile.TemporaryDirectory() as tmp:          # a value no convention defines
        d = build_change(tmp + "/vocab")
        edit(d, "02-plan.md", "| C1T |", "| C1X |")
        if "CONVENTIONS" not in gate(d):
            unfired.append("CONVENTIONS on a code no convention defines")
    with tempfile.TemporaryDirectory() as tmp:          # scope and arbiter sharing a path
        d = build_change(tmp + "/r7")
        edit(d, "01-specification.md", "| subject/ | judged/ |", "| judged/ | subject/ |")
        reseal(d)
        if "R7" not in gate(d):
            unfired.append("R7 on a scope that intersects its own arbiter")
    return unfired and f"claimed but did not fire: {'; '.join(unfired)}" or None


def cr_003_03():
    bare = [rid for rid, mark, why in rules() if mark != "M" and not why]
    return bare and f"not mechanical and no reason given: {', '.join(bare)}" or None


def cr_003_04():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def scoped(tmp, scope, holds=("beta.py", "gamma.py")):
    """A built change whose unit is scoped to what is given, in an estate of
    three files where one names another and the third stands alone."""
    import pathlib as _p
    root = _p.Path(tmp)
    d = build_change(str(root / "changes" / "c"))
    (root / "alpha.py").write_text("def a():\n    return 1\n")
    (root / "beta.py").write_text("import alpha\n\n\ndef b():\n    return alpha.a()\n")
    (root / "gamma.py").write_text("def g():\n    return 3\n")
    for name in holds:
        (d / "candidates" / "CA-001-01" / name).write_text("def x():\n    return 4\n")
    edit(d, "02-plan.md", "| subject/ |", f"| {scope} |")
    return d


def cr_026_01():
    """Scoped to both files the candidate holds, nothing is unreached. The two
    are named the way every other list in these tables is written. Read as one
    path, the other becomes ground nothing reaches."""
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(scoped(tmp, "beta.py, gamma.py"))
        if "R21" in out:
            return f"a scope of two paths was read as one:\n{out}"
    with tempfile.TemporaryDirectory() as tmp:          # and the rule still fires
        out = gate(scoped(tmp, "beta.py", holds=("gamma.py",)))
        if "R21" not in out or "gamma.py" not in out:
            return f"reading the whole scope silenced the rule:\n{out}"
    return None


def cr_026_02():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(scoped(tmp, "beta.py, gamma.py, delta.py"))
        if "delta.py" not in out:
            return f"a scope entry naming nothing here was dropped in silence:\n{out}"
        if any("delta.py" in line and line.strip().startswith("FAIL")
               for line in out.splitlines()):
            return "naming nothing here was treated as a refusal"
    return None


def main():
    failures = []
    for name, fn in (("CR-003-01", cr_003_01), ("CR-003-02", cr_003_02),
                     ("CR-003-03", cr_003_03), ("CR-003-04", cr_003_04),
                     ("CR-026-01", cr_026_01), ("CR-026-02", cr_026_02)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
