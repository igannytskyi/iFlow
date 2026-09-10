"""Shared ground for the arbiters.

Nothing is stored. A fixture is built when it is needed, from values this
module supplies and columns the templates declare, so that a fixture cannot
drift away from the shapes it is supposed to represent — and so that a template
gaining a column is noticed here rather than the day someone fills one in.
"""
import hashlib
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent.parent
GATE = ROOT / "framework" / "check.py"
TEMPLATES = ROOT / "framework" / "templates"


def gate(folder, *args):
    """Run the gate and return everything it said."""
    r = subprocess.run([sys.executable, str(GATE), *[str(a) for a in (folder, *args)]],
                       capture_output=True, text=True, cwd=ROOT)
    return r.stdout + r.stderr


def template_columns(name):
    """Every column the named template declares, as a set of header tuples."""
    out = []
    for block in re.findall(r"(?:^\|.*\|$\n)+", (TEMPLATES / name).read_text(), re.M):
        header = block.splitlines()[0]
        out.append(tuple(c.strip() for c in header.strip("|").split("|")))
    return out


STAGES = {
    "00-intent": """# Intent — fixture

| Field | Value |
|---|---|
| Id | INT-001 |
| Stated by | a person |
| Deciding authority | a person |
| Priority | normal |
| Deadline | none |
| Statement | The stated outcome holds and nothing else moves. |
| Status | open |
""",
    "01-specification": """# Specification — fixture

| Field | Value |
|---|---|
| Id | SPEC-001 |
| Intent | INT-001 |
| Default class | C1T |

## Acceptance criteria

| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |
|---|---|---|---|---|---|---|---|
| CR-001-01 | the stated outcome holds | the declared conditions hold | the stated outcome is observed | not-required | machine | test-run | observed |
| CR-001-02 | nothing else moves | the same conditions hold | everything else behaves as before | not-required | machine | test-run | no difference |

## Termination

| Condition | Action |
|---|---|
| the outcome cannot be reached mechanically | stop |

## Scope

| Included | Excluded |
|---|---|
| subject/ | judged/ |

## Arbiter

| Path | What it arbitrates |
|---|---|
| judged/ | CR-001-01, CR-001-02 |

## Arbiter reads

| Input | In scope? |
|---|---|
| judged/ | no |
""",
    "02-plan": """# Plan — fixture

| Field | Value |
|---|---|
| Specification | SPEC-001 |
| Phases | 1 |
| Intermediate states valid | yes |
| Residue on abandonment | none |

## Work units

| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |
|---|---|---|---|---|---|---|---|
| WU-001-01 | 1 | C1T | subject/ | the subject and its dependents, derived, high | CR-001-01, CR-001-02 | | |

## Coverage

| Question | Answer |
|---|---|
| Do the units together cover the specification? | yes |
| Was the estate query that produced the scope complete? | yes |
""",
    "03-admission": """# Admission — fixture

| Field | Value |
|---|---|
| Specification digest | DIGEST |

## Criteria review

| Reviewed before admission | By | At | Note |
|---|---|---|---|
| yes | a person | 2026-09-10 | read before anything was spent |

## Decisions

| Unit | Outcome | Deciding condition | Confidence of the edges relied on | Escalates at |
|---|---|---|---|---|
| WU-001-01 | admitted | permission | high | |

## Conflicts

| Units | Intersecting region | Evidence it would invalidate | Resolution |
|---|---|---|---|

## Evidence plan

| Criterion | Observed | Method | Unchanged means | Derived from | Fixed at |
|---|---|---|---|---|---|

## Prior state captured

| Criterion | Artefact | Captured to |
|---|---|---|

## Grants

| Id | Unit | Permitted operations | Targets | Expires |
|---|---|---|---|---|
| GR-001-01 | WU-001-01 | read, write | subject/ | with the run |
""",
    "04-execution": """# Execution — fixture

| Unit | Executor | Version | Terminal state | Candidate | Trace |
|---|---|---|---|---|---|
| WU-001-01 | a deterministic transformation | 1.0 | candidate | CA-001-01 | this file |

## Candidates

| Id | Unit | Stored at | Artefacts | Produced at |
|---|---|---|---|---|
| CA-001-01 | WU-001-01 | candidates/CA-001-01/ | the proposed change | 2026-09-10 |
""",
    "05-assurance": """# Assurance — fixture

## Evidence

| Id | Subject | Kind | Producer | Independence | How established | Repeatable | Obtained at | Valid until |
|---|---|---|---|---|---|---|---|---|
| EV-001-01-01 | candidate | test-run | the arbiter under judged/ | independent-by-executor | the arbiter lies outside the scope | yes | 2026-09-10 | next change to subject/ |

## Verdicts

| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |
|---|---|---|---|---|---|---|---|
| VE-001-01 | WU-001-01 | CR-001-01 | met | EV-001-01-01 | settled | | |
| VE-001-02 | WU-001-01 | CR-001-02 | met | EV-001-01-01 | settled | | |
""",
    "06-landing": """# Landing — fixture

| Order | Unit | Verdict state | Evidence still valid | Entered at | Invalidated by this entry | Re-established before next |
|---|---|---|---|---|---|---|
| 1 | WU-001-01 | settled | yes | 2026-09-10 | none | none |

## Unplanned state

| Occurred | Area of effect halted | Resolved by |
|---|---|---|
| no | | |
""",
}

RECORD_STAGES = ["intent", "specification", "plan", "admission",
                 "execution", "assurance", "landing"]


def build_change(where, only=None):
    """Write a complete, valid change. Nothing about it is stored on disk
    between runs; it exists for as long as the check that asked for it."""
    d = pathlib.Path(where)
    d.mkdir(parents=True, exist_ok=True)
    stages = [s for s in STAGES if only is None or s in only]
    for name in stages:
        (d / f"{name}.md").write_text(STAGES[name])
    if "03-admission" in stages:
        digest = hashlib.sha256((d / "01-specification.md").read_bytes()).hexdigest()[:16]
        p = d / "03-admission.md"
        p.write_text(p.read_text().replace("DIGEST", digest))
    if "04-execution" in stages:
        (d / "candidates" / "CA-001-01").mkdir(parents=True, exist_ok=True)
        (d / "candidates" / "CA-001-01" / "README.md").write_text("the proposed change\n")
    rows = "\n".join(
        f"| {i} | 2026-09-10 | {s} | fixture | | |"
        for i, s in enumerate(RECORD_STAGES[:len(stages)], start=1))
    (d / "record.md").write_text(
        "# Record — fixture\n\n| # | At | Stage | Object | Digest | Note |\n"
        "|---|---|---|---|---|---|\n" + rows + "\n")
    return d


def edit(folder, name, before, after):
    """Change one thing in a built fixture, so a check can watch the gate react."""
    p = pathlib.Path(folder) / name
    text = p.read_text()
    assert before in text, f"{name} does not contain {before!r}"
    p.write_text(text.replace(before, after, 1))
    return folder


def add_escalation(folder, n, standing, ground="a ground"):
    """Give a built change an escalation, so the ledger has something to read."""
    p = pathlib.Path(folder) / "05-assurance.md"
    text = p.read_text()
    if "## Escalations\n" not in text:
        text += ("\n## Escalations\n\n| # | Standing | Ground | Raised against | To |\n"
                 "|---|---|---|---|---|\n")
    text += f"| {n} | {standing} | {ground} | framework | a person |\n"
    p.write_text(text)
    return folder


def close_escalation(folder, ref):
    """Record that this change settled a debt raised by another."""
    p = pathlib.Path(folder) / "05-assurance.md"
    p.write_text(p.read_text() +
                 "\n## Escalations closed\n\n| Escalation | Resolved by | Note |\n"
                 "|---|---|---|\n"
                 f"| {ref} | this change | settled |\n")
    return folder


def build_repo(where, names=("one", "two")):
    """A tree the ledger can be run against: the gate, and some changes."""
    d = pathlib.Path(where)
    (d / "framework").mkdir(parents=True, exist_ok=True)
    import shutil
    shutil.copy(GATE, d / "framework" / "check.py")
    for n in names:
        build_change(d / "changes" / n)
    return d


def reseal(folder):
    """Recompute the digest after deliberately altering the specification.

    A check that changes the specification for some reason other than testing
    immutability must reseal, or the gate reports the immutability violation
    instead of the thing being checked.
    """
    d = pathlib.Path(folder)
    p = d / "03-admission.md"
    fresh = hashlib.sha256((d / "01-specification.md").read_bytes()).hexdigest()[:16]
    p.write_text(re.sub(r"\| Specification digest \| [0-9a-f]+ \|",
                        f"| Specification digest | {fresh} |", p.read_text()))
    return d
