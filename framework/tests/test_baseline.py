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
CR-026-03  every class this instrument prints is one the vocabulary defines,
           a family is printed as a family, and the classes no title can name
           are not guessed
CR-026-04  an estate that is not there is refused rather than measured as an
           estate with no history
CR-026-05  every figure says which state of the estate it was read from
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
    "CR-026-03": "direct",
    "CR-026-04": "direct",
    "CR-026-05": "direct",
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


def defined_classes():
    """The codes the conventions define, read from the table that defines them."""
    text = (ROOT / "framework" / "conventions.md").read_text()
    out = set()
    for line in text.splitlines():
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) >= 2 and cells[0].startswith("`C") and cells[0].endswith("`"):
            out.add(cells[0].strip("`"))
    return out


def cr_026_03():
    sys.path.insert(0, str(ROOT / "framework"))
    import baseline                                       # noqa: PLC0415
    defined = defined_classes()
    if not defined:
        return "the conventions define no class, so nothing here can be judged"
    for label, _pattern in baseline.SAYS:
        if label.endswith("*"):
            family = [c for c in defined if c.startswith(label[:-1])]
            if len(family) < 2:
                return f"{label} is printed as a family and names {len(family)} class(es)"
            continue
        if label not in defined:
            return f"{label} is printed and the vocabulary does not define it"
    decided_elsewhere = {"C4", "C5"}
    guessed = {label.rstrip("*") for label, _ in baseline.SAYS}
    if guessed & decided_elsewhere:
        return (f"{', '.join(sorted(guessed & decided_elsewhere))} is guessed from a "
                f"title, and it is defined by how acceptance is decided")
    with tempfile.TemporaryDirectory() as tmp:
        out = run(history(tmp))
        printed = set()
        for line in out.splitlines():
            head = line.strip().split()
            if head and head[0].startswith("C"):
                printed.add(head[0])
        for code in printed:
            if code.rstrip("*") not in {c.rstrip("*") for c in guessed}:
                return f"the instrument printed {code}, which it does not claim"
        if not printed:
            return "no class was printed at all, so nothing was judged"
    return None


def cr_026_04():
    with tempfile.TemporaryDirectory() as tmp:
        missing = pathlib.Path(tmp) / "not-an-estate"
        out = subprocess.run([sys.executable, str(TOOL), str(missing)],
                             capture_output=True, text=True, cwd=ROOT)
        said = out.stdout + out.stderr
        if "0 commit(s)" in said:
            return "an estate that is not there was measured as one with no history"
        if str(missing) not in said:
            return f"nothing said which estate could not be read:\n{said}"
        if out.returncode == 0:
            return "reading nothing was reported as a reading"
    return None


def cr_026_05():
    with tempfile.TemporaryDirectory() as tmp:
        d = history(tmp)
        sha = subprocess.run(["git", "-C", str(d), "rev-parse", "HEAD"],
                             capture_output=True, text=True).stdout.strip()
        if sha not in run(d):
            return "the figure does not say which state of the estate it was read from"
        if sha not in run(d, "--json"):
            return "what is kept does not say which state it was read from"
    return None


def main():
    failures = []
    for name, fn in (("CR-023-01", cr_023_01), ("CR-023-02", cr_023_02),
                     ("CR-023-03", cr_023_03), ("CR-023-04", cr_023_04),
                     ("CR-026-03", cr_026_03), ("CR-026-04", cr_026_04),
                     ("CR-026-05", cr_026_05)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
