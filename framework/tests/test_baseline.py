#!/usr/bin/env python3
"""Arbiter for the baseline — the present way of working, measured.

CR-023-01  a baseline is reconstructed from what was recorded anyway, and every
           figure says where it came from
CR-023-02  what a repository cannot answer is named rather than estimated: no
           figure is put where a measurement is claimed and none was made
CR-023-03  nothing is blended — every figure is per class, with the mix beside
           it — and the denominator is the intent, never the work unit
CR-023-04  what was accepted and taken back is reported with its lag, and said
           to be a censored estimate
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

TOOL = ROOT / "framework" / "baseline.py"

TESTS = {
    "CR-023-01": "direct",
    "CR-023-02": "direct",
    "CR-023-03": "direct",
    "CR-023-04": "direct",
}


def run(*args, cwd=ROOT):
    return subprocess.run([sys.executable, str(TOOL), *[str(a) for a in args]],
                          capture_output=True, text=True, cwd=cwd).stdout


def git(d, *args):
    subprocess.run(["git", *args], cwd=d, capture_output=True, text=True)


def history(tmp):
    """A repository with a history worth measuring: work of several kinds, one
    of it taken back."""
    d = pathlib.Path(tmp)
    git(d, "init", "-q")
    git(d, "config", "user.email", "a@b")
    git(d, "config", "user.name", "a")
    (d / "core.py").write_text("def alpha():\n    return 1\n")
    git(d, "add", "-A"); git(d, "commit", "-q", "-m", "add alpha")
    (d / "core.py").write_text("def alpha():\n    return 2\n")
    git(d, "add", "-A"); git(d, "commit", "-q", "-m", "fix alpha returning the wrong number")
    (d / "core.py").write_text("def alpha():\n    return 3\n")
    git(d, "add", "-A"); git(d, "commit", "-q", "-m", "bump the number")
    sha = subprocess.run(["git", "-C", str(d), "rev-parse", "HEAD"],
                         capture_output=True, text=True).stdout.strip()
    (d / "core.py").write_text("def alpha():\n    return 2\n")
    git(d, "add", "-A")
    git(d, "commit", "-q", "-m", f'Revert "bump the number"\n\nThis reverts commit {sha}.')
    (d / "tests").mkdir()
    (d / "tests" / "test_core.py").write_text(
        "from core import alpha\n\n\ndef test_alpha():\n    assert alpha()\n")
    git(d, "add", "-A"); git(d, "commit", "-q", "-m", "add a test")
    return d


def cr_023_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = history(tmp)
        out = run(d)
        if "commit(s) over" not in out:
            return f"nothing was reconstructed from the history:\n{out}"
        if "intents:" not in out:
            return "the figure did not say where the intents came from"
        data = json.loads(run(d, "--json"))
        if data.get("arbiter strength", {}).get("provenance") != "derived":
            return "a figure was given without saying what it rests on"
    return None


def cr_023_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = history(tmp)
        out = run(d)
        if "cannot answer" not in out:
            return "what could not be measured was not named"
        if "cost per unit" not in out or "undecided verdicts" not in out:
            return "an instrument with no baseline was passed over in silence"
        data = json.loads(run(d, "--json"))
        for key in ("cost per unit of verified change", "share of undecided verdicts"):
            if key in data:
                return f"{key} was given a figure where no measurement was made"
    return None


def cr_023_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = history(tmp)
        data = json.loads(run(d, "--json"))
        touch = data["touchpoints per intent"]
        if not isinstance(touch, dict) or len(touch) < 2:
            return f"touchpoints were blended into one number: {touch}"
        if not data.get("class mix"):
            return "figures were reported per class without the mix beside them"
        out = run(d)
        if "denominator is the intent" not in out:
            return "nothing said what the figures are per"
    return None


def cr_023_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = history(tmp)
        data = json.loads(run(d, "--json"))
        if data["reverted"] < 1:
            return "a revert in the history was not counted"
        if data["lag in days"]["median"] is None:
            return "a revert naming what it undid produced no lag"
        out = run(d)
        if "censored" not in out:
            return "the rate was reported without saying it is a censored estimate"
    return None


def main():
    failures = []
    for name, fn in (("CR-023-01", cr_023_01), ("CR-023-02", cr_023_02),
                     ("CR-023-03", cr_023_03), ("CR-023-04", cr_023_04)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
