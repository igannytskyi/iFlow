#!/usr/bin/env python3
"""Arbiter for observability of a region.

CR-013-01  a region nothing names is reported as needing characterising first,
           not as observed
CR-013-02  what structure claims is held apart from what execution shows, and
           only the second is trusted
CR-013-03  a region nothing executes is reported unobserved rather than unknown
CR-013-05  asked of the whole estate, regions that are themselves test ground are
           held apart from the code under test, and what is worst comes first
CR-013-04  a test is recognised by where tests are kept and how they are named,
           not by one project's layout, and a directory that ships as an
           importable package is not counted as one
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
    "CR-013-04": "direct",
    "CR-013-05": "direct",
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


def cr_013_05():
    """Every region at once, which is a different question from one region.

    A fixture directory nothing names is true and useless, and on a real estate
    there are hundreds of them: if they are counted with the code under test,
    the answer that matters is no longer visible in the answer.
    """
    with tempfile.TemporaryDirectory() as tmp:
        d = build_region(tmp, watched=True)
        (d / "tests" / "fixtures").mkdir()
        (d / "tests" / "fixtures" / "sample.py").write_text("def unused_fixture():\n    pass\n")
        (d / "unwatched").mkdir()
        (d / "unwatched" / "big.py").write_text(
            "".join(f"def gamma{i}():\n    return {i}\n\n\n" for i in range(9)))
        r = subprocess.run([sys.executable, str(ESTATE), "observability"],
                           capture_output=True, text=True, cwd=d).stdout
        if "tests/fixtures" in r.split("region(s)")[0]:
            return "a fixture directory was listed among the code under test"
        if "themselves where tests are kept" not in r:
            return "test ground was not said to be held apart"
        lines = [ln for ln in r.splitlines() if "symbol(s) named by a test" in ln]
        if not lines or "unwatched" not in lines[0]:
            return f"the region nothing names was not first: {lines[:1]}"
    return None


def cr_013_04():
    """Two ways to be wrong about what a test is, in one region.

    Tests at the root of a repository are the common layout, and a project's
    own testing library is the common thing mistaken for one. Getting either
    wrong turns "I looked in the wrong place" into "nothing watches this".
    """
    with tempfile.TemporaryDirectory() as tmp:
        d = build_region(tmp, watched=True)          # tests/ lies at the root
        r = ask("observability", "subject", cwd=d)
        if not r.get("named by a test"):
            return "a test at the root of the repository was not found"
        (d / "test").mkdir()
        (d / "test" / "__init__.py").write_text("")
        (d / "test" / "client.py").write_text(
            "import sys\nsys.path.insert(0, 'subject')\nfrom thing import alpha\n")
        r = ask("observability", "subject", cwd=d)
        if any("test/client.py" in w for w in r.get("watched by", [])):
            return "a directory that ships as an importable package was counted as tests"
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
                     ("CR-013-03", cr_013_03), ("CR-013-04", cr_013_04),
                     ("CR-013-05", cr_013_05)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
