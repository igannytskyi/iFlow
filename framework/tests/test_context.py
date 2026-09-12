#!/usr/bin/env python3
"""Arbiter for the context an agent is handed.

CR-019-01  the answer carries the code, quoted verbatim with the lines it sits
           on, rather than a list of places to go and read
CR-019-02  a quoted line is the line: what is shown as a call site contains the
           call, so what is read is what is there
CR-019-03  what is firm comes first, and what is a coincidence of vocabulary is
           counted rather than quoted among it
CR-019-04  the answer is cut to a budget, and says what it left out
CR-019-05  what would judge a change is named, and where nothing would, that is
           said rather than left blank
CR-019-06  a task statement is answered with where its words land, in full, and
           how each of them landed
CR-019-07  a word that names nothing here is reported as such: the estate calls
           it something else, or that ground is unread, and both are worth
           saying
CR-019-08  a task is answered with places and never with a choice between them:
           which place a sentence means is not derivable from its words, and
           the answer says so instead of guessing
CR-019-09  several targets are expanded in one answer, because whoever chose
           them chose more than one
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-019-01": "direct",
    "CR-019-02": "direct",
    "CR-019-03": "direct",
    "CR-019-04": "direct",
    "CR-019-05": "direct",
    "CR-019-06": "direct",
    "CR-019-07": "direct",
    "CR-019-08": "direct",
    "CR-019-09": "direct",
}


def run(cwd, *args):
    return subprocess.run([sys.executable, str(ESTATE), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def estate_with(tmp):
    """A subject, a caller that imports it, a test that names it, and a stranger
    that shares a method name and imports nothing."""
    d = pathlib.Path(tmp)
    (d / "core.py").write_text(
        "class Engine:\n"
        "    def start(self):\n"
        "        return 1\n"
        "\n"
        "    def stop(self):\n"
        "        return 0\n")
    (d / "user.py").write_text(
        "from core import Engine\n"
        "\n"
        "\n"
        "def drive():\n"
        "    e = Engine()\n"
        "    return e.start()\n")
    (d / "stranger.py").write_text(
        "def start(thing):\n"
        "    return thing\n")
    (d / "tests").mkdir()
    (d / "tests" / "test_core.py").write_text(
        "from core import Engine\n"
        "\n"
        "\n"
        "def test_start():\n"
        "    assert Engine().start() == 1\n")
    (d / "tests" / "helpers.py").write_text(
        "def drive():\n"
        "    return None\n")
    return d


def cr_019_01():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "Engine")
        if "class Engine:" not in out:
            return "the definition was not quoted"
        if "core.py:1-" not in out:
            return "the quotation did not say where it came from"
        if "     1  class Engine:" not in out:
            return "the quoted code carries no line numbers"
    return None


def cr_019_02():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "Engine")
        for line in out.splitlines():
            if line.startswith("--- called from"):
                where = line.split()[3]
                rel, num = where.split(":")
                num = int(num)
                text = (d / rel).read_text().splitlines()[num - 1]
                if "Engine" not in text and "e." not in text:
                    return (f"{where} was quoted as a call site and reads {text!r}, "
                            f"which is neither")
    return None


def cr_019_03():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "Engine")
        grades = [ln.split("(")[-1].rstrip(")") for ln in out.splitlines()
                  if ln.startswith("--- called from")]
        order = {"high": 0, "medium": 1, "in the same file": 1, "low": 2}
        seen = [order.get(g, 2) for g in grades]
        if seen != sorted(seen):
            return f"the answer did not put what is firm first: {grades}"
        out = run(d, "context", "start")
        callers = [ln for ln in out.splitlines() if ln.startswith("--- called from")]
        if any("stranger.py" in ln for ln in callers):
            return "a file that shares a name and imports nothing was quoted as a caller"
        if "2 places define this name" not in out:
            return "a name defined twice was answered as though it were defined once"
    return None


def cr_019_04():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        small = run(d, "context", "Engine", "--budget=600")
        if "of a 600 budget" not in small:
            return "the answer did not say what budget it was cut to"
        big = run(d, "context", "Engine")
        if len(small) >= len(big):
            return "a smaller budget did not produce a shorter answer"
        if "not quoted" not in small:
            return "an answer that left something out did not say so"
    return None


def cr_019_05():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "Engine")
        if "tests/test_core.py" not in out.split("what would judge")[-1]:
            return "the test that names it was not reported as judging it"
        out = run(d, "context", "core.py:stop")
        if "judged by nothing" not in out:
            return "a symbol nothing names was not said to be judged by nothing"
    return None


def cr_019_06():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "--task", "the engine does not start after a restart")
        if "engine" not in out or "is the name of" not in out:
            return f"a word naming a class was not reported as naming it:\n{out}"
        if "core.py:Engine" not in out:
            return "a word that names something here did not say where it is"
        if "core.py:start" not in out or "stranger.py:start" not in out:
            return "a word that names two places was answered with fewer than two"
        both = run(d, "context", "--task", "drive is broken")
        if "user.py:drive" not in both or "tests/helpers.py:drive" not in both:
            return "a word landing in two places, one of them test ground, lost one"
        marked = [ln for ln in both.splitlines() if "tests/helpers.py:drive" in ln]
        if not marked or "(test ground)" not in marked[0]:
            return "a place that is test ground was not marked as one"
    return None


def cr_019_07():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "--task", "the carburettor floods when the engine starts")
        if "placed nothing" not in out or "carburettor" not in out:
            return "a word that names nothing here was dropped rather than reported"
        if "calls them something else" not in out:
            return "an unplaced word was not said to mean a different vocabulary or unread ground"
    return None


def cr_019_08():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "--task", "the engine does not start after a restart")
        if "--- called from" in out or "     1  class Engine:" in out:
            return ("a task was expanded into code, which means something chose which "
                    "place it meant, and nothing here can choose that")
        if "not derivable from the words" not in out:
            return "the answer did not say that choosing between the places is not its to do"
        empty = run(d, "context", "--task", "the carburettor floods at altitude")
        if "nothing in this task names anything here" not in empty:
            return "a task naming nothing here was answered as though something landed"
    return None


def cr_019_09():
    with tempfile.TemporaryDirectory() as tmp:
        d = estate_with(tmp)
        out = run(d, "context", "core.py:Engine", "user.py:drive")
        if "class Engine:" not in out or "def drive():" not in out:
            return "two targets were asked for and fewer than two were quoted"
        if "over 2 target(s)" not in out:
            return "an answer over several targets did not say how many it covered"
    return None


def main():
    failures = []
    for name, fn in (("CR-019-01", cr_019_01), ("CR-019-02", cr_019_02),
                     ("CR-019-03", cr_019_03), ("CR-019-04", cr_019_04),
                     ("CR-019-05", cr_019_05), ("CR-019-06", cr_019_06),
                     ("CR-019-07", cr_019_07), ("CR-019-08", cr_019_08),
                     ("CR-019-09", cr_019_09)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
