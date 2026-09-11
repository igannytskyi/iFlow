#!/usr/bin/env python3
"""Arbiter for freshness per region and for reachability.

CR-015-01  freshness is answered per region, not per repository
CR-015-02  only what moved is derived again
CR-015-03  a consumer belonging to no repository is reported as beyond reach
CR-015-04  an estate of several repositories takes each region's history from the
           repository it belongs to, and a region no history covers is never
           reported current
CR-015-05  what is looked at is what it is pointed at: a repository kept under a
           directory whose name is excluded is still read, and derived material
           can be kept in one place away from the repositories
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
    "CR-015-04": "direct",
    "CR-015-05": "direct",
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


def cr_015_04():
    """The estate is a directory of repositories, and is not one itself."""
    with tempfile.TemporaryDirectory() as tmp:
        estate = pathlib.Path(tmp) / "estate"
        build_repo(str(estate / "one"))
        build_repo(str(estate / "two"))
        (estate / "loose.py").write_text("def loose():\n    return 0\n")
        run(estate, "refresh")
        out = run(estate, "freshness")
        if "one/alpha.py" in out or "two/alpha.py" in out:
            return "a region covered by a repository's history was not taken from it"
        if "no history" not in out or "loose.py" not in out:
            return "a region no history covers was not reported as such"
        if "derived again every time rather than trusted" not in out:
            return "a region with no history was left to be trusted as current"
        (estate / "one" / "alpha.py").write_text("def alpha():\n    return 3\n")
        git(estate / "one", "add", "-A")
        git(estate / "one", "commit", "-q", "-m", "second")
        after = run(estate, "freshness")
        if "stale" not in after or "one/alpha.py" not in after:
            return "a region that moved inside one repository of the estate was not stale"
    return None


def cr_015_05():
    import os
    with tempfile.TemporaryDirectory() as tmp:
        # `changes` is the one directory this method excludes by name. Kept
        # under it, an estate reported nought of nought regions.
        d = build_repo(tmp + "/changes/experiments/repos/one")
        keep = pathlib.Path(tmp) / "derived"
        env = dict(os.environ, IFLOW_ESTATE=str(keep),
                   IFLOW_ESTATE_ROOT=str(pathlib.Path(tmp) / "changes/experiments/repos"))
        out = subprocess.run([sys.executable, str(ESTATE), "refresh"], cwd=d,
                             capture_output=True, text=True, env=env).stdout
        if "2 of 2 region(s)" not in out:
            return f"a repository under an excluded directory was not read: {out.strip()}"
        if (d / ".estate").exists():
            return "derived material was written beside the repository after being sent elsewhere"
        if not (keep / "cache" / "one" / "index.json").exists():
            return "derived material was not kept where it was asked to be kept"
    return None


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
                     ("CR-015-03", cr_015_03), ("CR-015-04", cr_015_04), ("CR-015-05", cr_015_05)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
