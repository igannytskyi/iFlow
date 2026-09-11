#!/usr/bin/env python3
"""Arbiter for what the readers see.

CR-017-01  a type that names its parent reaches the file defining it, because in
           framework code inheritance is often the only dependency written down
CR-017-02  a script is part of the estate: what it runs and what it sources are
           read, whether it carries an extension or says what it is on its first
           line
CR-017-03  what nothing reaches is classified rather than counted as one thing,
           and the figure that counts a convention says which conventions it
           counted
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-017-01": "direct",
    "CR-017-02": "direct",
    "CR-017-03": "direct",
}


def run(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def reads(ext):
    """Whether a grammar for this is installed here. Without one the criterion
    is about a language this install does not read, and says so rather than
    failing."""
    return ext in run(ROOT, "readers")


def cr_017_01():
    if not reads(".rb"):
        return None
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "base.rb").write_text("class ApplicationRecord\n  def save; end\nend\n")
        (d / "post.rb").write_text("class Post < ApplicationRecord\nend\n")
        out = run(d, "affects", "base.rb")
        if "post.rb" not in out:
            return f"a class naming its parent did not reach it: {out.strip()}"
    return None


def cr_017_02():
    if not reads(".sh"):
        return None
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "lib").mkdir()
        (d / "lib" / "common.sh").write_text("build() { make all; }\n")
        (d / "deploy.sh").write_text("source ./lib/common.sh\nbuild\n")
        (d / "release").write_text("#!/usr/bin/env bash\nsource ./lib/common.sh\nbuild\n")
        out = run(d, "affects", "lib/common.sh")
        if "deploy.sh" not in out:
            return f"a script sourcing another did not reach it: {out.strip()}"
        if "release" not in out:
            return "a script that says what it is on its first line was not read"
    return None


def cr_017_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "core.py").write_text("def alpha():\n    return 1\n")
        (d / "use.py").write_text("from core import alpha\n\n\ndef beta():\n    return alpha()\n")
        (d / "tests").mkdir()
        (d / "tests" / "test_core.py").write_text("def test_alpha():\n    pass\n")
        (d / "migrations").mkdir()
        (d / "migrations" / "0001_start.py").write_text("class Start:\n    pass\n")
        (d / "settings.py").write_text('HANDLER = "lonely.Thing"\n')
        (d / "lonely.py").write_text("class Thing:\n    pass\n")
        (d / "orphan.py").write_text("def nobody_calls_this():\n    return 0\n")
        out = run(d, "coverage")
        for phrase in ("test ground a runner finds by name",
                       "where a framework loads by name",
                       "named as text and nowhere else",
                       "counting those,"):
            if phrase not in out:
                return f"the residual was not classified: {phrase!r} missing"
        if "unreached  orphan.py" not in out:
            return "a file nothing reaches at all was not reported as such"
        if "unreached  lonely.py" in out or "unreached  migrations" in out:
            return "a file something reaches by convention was counted as unreached"
    return None


def main():
    failures = []
    for name, fn in (("CR-017-01", cr_017_01), ("CR-017-02", cr_017_02),
                     ("CR-017-03", cr_017_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
