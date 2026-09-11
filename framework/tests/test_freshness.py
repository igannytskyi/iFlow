#!/usr/bin/env python3
"""Arbiter for freshness per region and for reachability.

CR-015-01  freshness is answered per region, not per repository
CR-015-02  only what moved is derived again
CR-015-03  a consumer belonging to no repository is reported as beyond reach
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-015-01": "direct",
    "CR-015-02": "direct",
    "CR-015-03": "direct",
}


def run(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def git(cwd, *args):
    subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)


def build_repo(where):
    """Two files under version control, so that one can move without the other."""
    d = pathlib.Path(where)
    d.mkdir(parents=True, exist_ok=True)
    git(d, "init", "-q")
    git(d, "config", "user.email", "a@b")
    git(d, "config", "user.name", "a")
    (d / "alpha.py").write_text("def alpha():\n    return 1\n")
    (d / "beta.py").write_text("def beta():\n    return 2\n")
    git(d, "add", "-A")
    git(d, "commit", "-q", "-m", "first")
    return d


def cr_015_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_repo(tmp + "/r")
        run(d, "refresh")
        (d / "alpha.py").write_text("def alpha():\n    return 2\n")
        git(d, "add", "-A")
        git(d, "commit", "-q", "-m", "second")
        out = run(d, "freshness")
        if "stale" not in out or "alpha.py" not in out:
            return "a region that moved was not reported stale"
        if "beta.py" in out:
            return "a region that did not move was reported alongside one that did"
    return None


def cr_015_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = build_repo(tmp + "/r")
        run(d, "refresh")
        (d / "alpha.py").write_text("def alpha():\n    return 3\n")
        git(d, "add", "-A")
        git(d, "commit", "-q", "-m", "third")
        out = run(d, "refresh")
        if "re-derived  alpha.py" not in out:
            return "what moved was not derived again"
        if "beta.py" in out:
            return "what did not move was looked at again anyway"
    return None


def cr_015_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "api").mkdir()
        (d / "api" / "app.py").write_text('@app.route("/orders")\ndef create(): ...\n')
        seen = d / "seen.txt"
        seen.write_text("shipped-app http /orders\n")
        out = run(ROOT, "reachability", str(d), "api", str(seen))
        if "beyond reach" not in out or "shipped-app" not in out:
            return "a consumer belonging to no repository was not reported beyond reach"
        if "no change reaches it" not in out:
            return "it was not said why that matters"
    return None


def main():
    failures = []
    for name, fn in (("CR-015-01", cr_015_01), ("CR-015-02", cr_015_02),
                     ("CR-015-03", cr_015_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
