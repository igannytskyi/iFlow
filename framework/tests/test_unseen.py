#!/usr/bin/env python3
"""Arbiter for ground the index cannot read.

CR-016-01  a repository in a language this index cannot read is reported unread,
           not empty
CR-016-02  every answer says how much of the estate it covers
CR-016-03  a verdict that nothing watches a region says how much of the estate
           it could not read before saying so
CR-016-04  an estate with nothing readable in it is reported as unreadable, not
           as an estate with no regions
CR-016-05  what is unread is derived from the readers installed, not declared, so
           a language a reader covers is read and counted as read
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-016-01": "direct",
    "CR-016-02": "direct",
    "CR-016-03": "direct",
    "CR-016-04": "direct",
    "CR-016-05": "direct",
}


def run(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def unread_extension():
    """An extension nothing installed here reads — asked of the tool rather than
    assumed, because what is unread changes the moment a grammar is installed
    and a fixture that assumes otherwise tests the install, not the criterion."""
    out = run(ROOT, "readers")
    inside = out[out.index("known and unread ("):]
    return inside[inside.index("(") + 1:inside.index(")")].split()[0]


def foreign(where):
    d = pathlib.Path(where)
    ext = unread_extension()
    (d / "src").mkdir(parents=True)
    (d / "src" / f"index{ext}").write_text("defmodule Alpha do\nend\n")
    (d / "src" / f"other{ext}").write_text("defmodule Beta do\nend\n")
    return d


def mixed(where):
    """A region this index can read, on an estate mostly written in one it
    cannot — the shape of every polyglot estate, where the tests are as likely
    to be on the unread side as anywhere else."""
    d = pathlib.Path(where)
    (d / "svc").mkdir(parents=True)
    (d / "svc" / "handler.py").write_text("def alpha():\n    return 1\n")
    (d / "other").mkdir(parents=True)
    ext = unread_extension()
    for i in range(3):
        (d / "other" / f"svc{i}_test{ext}").write_text("defmodule T do\nend\n")
    return d


def cr_016_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        out = json.loads(run(d, "observability", "src"))
        if out.get("verdict") != "unknown":
            return "a region in an unreadable language was not reported unknown"
        if "unread one" not in out.get("why", ""):
            return "an unread region was not distinguished from an empty one"
    return None


def cr_016_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        ext = unread_extension()
        for cmd in (("freshness",), ("unknown",), ("affects", f"src/index{ext}")):
            out = run(d, *cmd)
            if "are not absent — they are unseen" not in out:
                return f"`{cmd[0]}` reported a figure without saying what it covers"
            if ext not in out:
                return f"`{cmd[0]}` did not name what it could not read"
    return None


def cr_016_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = mixed(tmp)
        out = json.loads(run(d, "observability", "svc"))
        if out.get("verdict") != "unclaimed":
            return "a region nothing readable names was not reported unclaimed"
        if "are not absent — they are unseen" not in out.get("read", ""):
            return "a verdict of unclaimed did not say what it could not read"
    return None


def cr_016_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = foreign(tmp)
        out = run(d, "observability")
        if "0 region(s)" in out:
            return "an estate this index cannot read was counted as having no regions"
        if "it is one this cannot see" not in out:
            return "an unreadable estate was not distinguished from an empty one"
    return None


def cr_016_05():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        read_here = [ln.split()[1:] for ln in run(ROOT, "readers").splitlines()
                     if ln.strip().startswith("tree-sitter")]
        if not read_here or not read_here[0]:
            return None            # no grammar installed here; nothing to prove
        ext = read_here[0][0]
        (d / "src").mkdir()
        (d / "src" / f"thing{ext}").write_text(
            "func alpha() {}\nfunc beta() { alpha() }\n")
        (d / "src" / f"other{unread_extension()}").write_text("defmodule A do\nend\n")
        out = run(d, "freshness")
        if "this covers 1 file(s)" not in out:
            return f"a language a reader covers was not counted as read: {out.strip()}"
        if unread_extension() not in out:
            return "a language no reader covers was not named as unseen"
    return None


def main():
    failures = []
    for name, fn in (("CR-016-01", cr_016_01), ("CR-016-02", cr_016_02),
                     ("CR-016-03", cr_016_03), ("CR-016-04", cr_016_04),
                     ("CR-016-05", cr_016_05)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
