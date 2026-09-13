#!/usr/bin/env python3
"""Arbiter for what must have been run before anything enters.

CR-022-01  a landing whose arbiters have not been run against this framework is
           refused, and running them settles it — a note read, not a note
           written: an arbiter asking this of a real change during its own run
           is asking about a framework that includes the question
CR-022-02  a landing carrying a claim to repeat that has not been repeated
           against this command is refused, and repeating it settles it
CR-022-03  what is kept is a note against content, not a record to be believed:
           any edit to what was run voids it
CR-022-04  a change checked away from the framework that judges it is told the
           question cannot be asked, rather than failed or passed in silence
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT, build_change, gate

GATE = ROOT / "framework" / "check.py"
NOTES = ROOT / ".estate" / "verified.json"

TESTS = {
    "CR-022-01": ("proxy", "the framework's own notes are what the criterion is about, and "
                           "a test may not delete them; what is checked is that the rule "
                           "reads them — a note removed makes the gate refuse, and putting "
                           "it back settles it"),
    "CR-022-02": ("proxy", "the same as above and for the same reason: the note is "
                  "removed and put back rather than the run being un-made"),
    "CR-022-03": "direct",
    "CR-022-04": "direct",
}


def run(*args, cwd=ROOT):
    return subprocess.run([sys.executable, str(GATE), *[str(a) for a in args]],
                          capture_output=True, text=True, cwd=cwd).stdout


def held():
    import json
    return json.loads(NOTES.read_text()) if NOTES.exists() else {}


def cr_022_01():
    import json
    sys.path.insert(0, str(ROOT / "framework"))
    import check
    before = held()
    key = f"arbiters:{check._framework_state()}"
    if key not in before:
        return None                      # the arbiters have not been run here yet
    kept = dict(before)
    kept.pop(key)
    NOTES.write_text(json.dumps(kept, indent=1, sort_keys=True))
    try:
        out = run("changes/self-observation")
        if "the arbiters were run" not in out:
            return "a landing entered without the arbiters having been run against it"
        if "--arbiters" not in out:
            return "the gate refused without saying what would settle it"
    finally:
        NOTES.write_text(json.dumps(before, indent=1, sort_keys=True))
    if "the arbiters were run" in run("changes/self-observation"):
        return "putting the note back did not settle it"
    return None


def cr_022_02():
    import json
    sys.path.insert(0, str(ROOT / "framework"))
    import check
    c = check.Check("changes/debts-settled")
    rows = [e for e, v in c.repeatable().items() if v == "yes" and c.producer_command(e)]
    if not rows:
        return None
    key = f"repeat:{c.command_key(c.producer_command(rows[0]))}"
    before = held()
    if key not in before:
        return None                      # it has not been repeated here yet
    kept = dict(before)
    kept.pop(key)
    NOTES.write_text(json.dumps(kept, indent=1, sort_keys=True))
    try:
        out = run("changes/debts-settled")
        if rows[0] not in out:
            return f"a claim to repeat that was never repeated was accepted:\n{out}"
        if "--repeat" not in out:
            return "the gate refused without saying what would settle it"
    finally:
        NOTES.write_text(json.dumps(before, indent=1, sort_keys=True))
    if rows[0] in run("changes/debts-settled"):
        return "putting the note back did not settle it"
    return None


def cr_022_03():
    sys.path.insert(0, str(ROOT / "framework"))
    import check
    first = check._framework_state()
    probe = ROOT / "framework" / "_probe_state.py"
    probe.write_text("# a file inside the framework\n")
    try:
        if check._framework_state() == first:
            return "an edit inside the framework left its name unchanged"
    finally:
        probe.unlink()
    if check._framework_state() != first:
        return "removing the edit did not bring the name back"
    c = check.Check("changes/self-observation")
    cmd = [ROOT / "framework" / "check.py", "--arbiters"]
    one = c.command_key(cmd)
    two = c.command_key([ROOT / "framework" / "check.py", "--mutate"])
    if one == two:
        return "two commands over the same file were given one name"
    return None


def cr_022_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_change(tmp + "/c")
        out = gate(d)
        if "cannot be asked" not in out:
            return "the gate neither asked the question nor said it could not"
    return None


def main():
    failures = []
    for name, fn in (("CR-022-01", cr_022_01), ("CR-022-02", cr_022_02),
                     ("CR-022-03", cr_022_03), ("CR-022-04", cr_022_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
