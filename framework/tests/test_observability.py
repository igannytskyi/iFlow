#!/usr/bin/env python3
"""Arbiter for observability of a region.

CR-013-01  a region nothing names is reported as needing characterising first,
           not as observed
CR-013-02  what structure claims is held apart from what execution shows, and
           only the second is trusted
CR-013-03  a region nothing executes is reported unobserved rather than unknown
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-013-01": "direct",
    "CR-013-02": "direct",
    "CR-013-03": "direct",
}


def ask(*args, cwd=None):
    r = subprocess.run([sys.executable, str(ESTATE), *args],
                       capture_output=True, text=True, cwd=cwd or ROOT)
    # What is run under tracing prints its own output first; the answer is the
    # JSON object at the end.
    out = r.stdout
    start = out.find("{")
    try:
        return json.loads(out[start:]) if start >= 0 else {"raw": out + r.stderr}
    except json.JSONDecodeError:
        return {"raw": out + r.stderr}


def build_region(where, watched):
    """A region, and a test that either names it or does not."""
    d = pathlib.Path(where)
    (d / "subject").mkdir(parents=True)
    (d / "subject" / "thing.py").write_text(
        "def alpha():\n    return 1\n\n\ndef beta():\n    return alpha() + 1\n")
    (d / "tests").mkdir(parents=True)
    (d / "tests" / "test_thing.py").write_text(
        "import sys\nsys.path.insert(0, 'subject')\nfrom thing import beta\n"
        "print(beta())\n" if watched else "print('nothing to do with the subject')\n")
    return d


def cr_013_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_region(tmp, watched=False)
        r = ask("observability", "subject", cwd=d)
        if r.get("verdict") != "unclaimed":
            return f"a region nothing names was not reported unclaimed: {r.get('verdict')}"
        if "characterising" not in r.get("means", ""):
            return "an unclaimed region was not said to need characterising first"
    return None


def cr_013_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_region(tmp, watched=True)
        structure = ask("observability", "subject", cwd=d)
        running = ask("observe", "subject/thing.py", "tests/test_thing.py", cwd=d)
        if structure.get("confidence") != "low":
            return "structure claimed more than it can know"
        if running.get("confidence") != "high" or running.get("provenance") != "derived":
            return "execution was not trusted more than structure"
        if "not a test exercising it" not in structure.get("caveat", ""):
            return "structure did not say that naming is not exercising"
    return None


def cr_013_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_region(tmp, watched=False)
        r = ask("observe", "subject/thing.py", "tests/test_thing.py", cwd=d)
        if r.get("verdict") != "unobserved":
            return f"a region nothing executed was not reported unobserved: {r.get('verdict')}"
    return None


def main():
    failures = []
    for name, fn in (("CR-013-01", cr_013_01), ("CR-013-02", cr_013_02),
                     ("CR-013-03", cr_013_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
