#!/usr/bin/env python3
"""Arbiter for the correction a landing owes the model.

CR-021-01  work that landed having touched ground nothing reaches from its
           scope is refused until that edge is recorded: the model cannot find
           it by reading again
CR-021-02  a recorded correction is answered with, as observed rather than
           derived, and outranks what reading found
CR-021-03  corrections are kept where the cache is not: rebuilding the index
           loses nothing a landing taught
"""
import json
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"
GATE = ROOT / "framework" / "check.py"

TESTS = {
    "CR-021-01": "direct",
    "CR-021-02": "direct",
    "CR-021-03": "direct",
}


def run(cwd, tool, *args):
    return subprocess.run([sys.executable, str(tool), *args],
                          capture_output=True, text=True, cwd=cwd).stdout


def estate_with_landing(tmp):
    """A change that landed having edited two files, where nothing in the
    source connects the second to the first."""
    d = pathlib.Path(tmp)
    (d / "engine.py").write_text("def start():\n    return 1\n")
    # Loaded by name at run time: no import, no call, nothing to read.
    (d / "plugin_alpha.py").write_text("def run():\n    return 2\n")
    (d / "loader.py").write_text(
        'import importlib\n\n\ndef load(which):\n'
        '    return importlib.import_module("plugin_" + which)\n')
    c = d / "changes" / "c"
    (c / "candidates" / "CA-001-01").mkdir(parents=True)
    (c / "candidates" / "CA-001-01" / "engine.py").write_text("def start():\n    return 3\n")
    (c / "candidates" / "CA-001-01" / "plugin_alpha.py").write_text("def run():\n    return 4\n")
    (c / "02-plan.md").write_text(
        "# Plan\n\n"
        "| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |\n"
        "|---|---|---|---|---|---|---|---|\n"
        "| WU-001-01 | 1 | C2 | engine.py | engine.py alone, derived, medium | CR-001-01 | | |\n")
    (c / "04-execution.md").write_text(
        "# Execution\n\n"
        "| Unit | Executor | Version | Terminal state | Candidate |\n|---|---|---|---|---|\n"
        "| WU-001-01 | agent | v1 | candidate | CA-001-01 |\n\n"
        "## Candidates\n\n"
        "| Id | Unit | Stored at | Artefacts | Produced at |\n|---|---|---|---|---|\n"
        "| CA-001-01 | WU-001-01 | candidates/CA-001-01/ | the repair | 2026-09-12 |\n")
    (c / "06-landing.md").write_text(
        "# Landing\n\n"
        "| Order | Unit | Verdict state | Evidence still valid | Entered at | "
        "Invalidated by this entry | Re-established before next |\n"
        "|---|---|---|---|---|---|---|\n"
        "| 1 | WU-001-01 | settled | yes | 2026-09-12 | none | none |\n")
    return d, c


def cr_021_01():
    with tempfile.TemporaryDirectory() as tmp:
        d, c = estate_with_landing(tmp)
        out = run(d, GATE, str(c))
        if "R21" not in out:
            return f"a landing that touched unforeseen ground was accepted:\n{out}"
        if "plugin_alpha.py" not in out:
            return "the ground nothing reaches was not named"
        if "cannot find it by reading again" not in out:
            return "the gate did not say why re-reading will not settle it"
        run(d, ESTATE, "correct", "engine.py", "plugin_alpha.py",
            "--by=c", "--why=loaded by name at run time")
        after = run(d, GATE, str(c))
        if "R21" in after:
            return f"the correction was recorded and the gate still refused:\n{after}"
    return None


def cr_021_02():
    with tempfile.TemporaryDirectory() as tmp:
        d, _c = estate_with_landing(tmp)
        before = run(d, ESTATE, "affects", "engine.py")
        if "plugin_alpha.py" in before:
            return "reading the source found an edge that is not written anywhere"
        run(d, ESTATE, "correct", "engine.py", "plugin_alpha.py",
            "--by=c", "--why=loaded by name at run time")
        after = run(d, ESTATE, "affects", "engine.py", "--all")
        row = [ln for ln in after.splitlines() if "plugin_alpha.py" in ln]
        if not row:
            return "what a landing taught was not answered with"
        if "high" not in row[0]:
            return f"an edge that was observed was not ranked above what was read: {row[0]}"
        if "landed with c" not in row[0]:
            return "the answer did not say which landing taught it"
    return None


def cr_021_03():
    with tempfile.TemporaryDirectory() as tmp:
        d, _c = estate_with_landing(tmp)
        run(d, ESTATE, "correct", "engine.py", "plugin_alpha.py",
            "--by=c", "--why=loaded by name at run time")
        cache = d / ".estate" / "index.json"
        run(d, ESTATE, "refresh")
        if cache.exists():
            cache.unlink()
        held = json.loads((d / ".estate" / "corrections.json").read_text())
        if not held:
            return "the corrections went with the cache"
        after = run(d, ESTATE, "affects", "engine.py", "--all")
        if "plugin_alpha.py" not in after:
            return "what a landing taught was lost when the index was rebuilt"
    return None


def main():
    failures = []
    for name, fn in (("CR-021-01", cr_021_01), ("CR-021-02", cr_021_02),
                     ("CR-021-03", cr_021_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
