#!/usr/bin/env python3
"""Mechanical gate over one change folder.

Checks what artifacts can decide. What they cannot decide is listed in
framework/RULES.md under the rules marked H, and is not attempted here.

Usage: python3 tools/check.py changes/<slug>
Exit status is 0 when nothing failed.
"""
import hashlib
import pathlib
import re
import sys

STAGES = ["00-intent", "01-specification", "02-plan", "03-admission",
          "04-execution", "05-assurance", "06-landing"]

VOCAB = {
    "class": {"C1P", "C1T", "C2", "C3", "C4", "C5"},
    "provenance": {"derived", "attested", "produced"},
    "confidence": {"high", "medium", "low"},
    "procedure": {"machine", "human"},
    "subject": {"candidate", "transformation"},
    "kind": {"test-run", "static-analysis", "runtime-observation",
             "human-affirmation", "transformation-proof"},
    "independence": {"independent", "not-independent"},
    "outcome": {"met", "failed", "undecided"},
    "state": {"settled", "deferred"},
    "admission": {"admitted", "held", "refused", "awaiting-authority"},
    "terminal": {"candidate", "terminated", "exhausted",
                 "failed-substrate", "failed-task", "cancelled"},
}

ID_RE = re.compile(r"\b(?:INT|SPEC|CR|WU|GR|CA|EV|VE)-\d{3}(?:-\d{2}){0,2}\b")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def tables(text):
    """Every markdown table as (headers, [row dicts]). Placeholder rows dropped."""
    out, rows, headers = [], None, None
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("|") and s.endswith("|"):
            cells = [c.strip() for c in s[1:-1].split("|")]
            if headers is None:
                headers, rows = cells, []
            elif set("".join(cells)) <= set("-: "):
                continue
            else:
                rows.append(dict(zip(headers, cells)))
        else:
            if headers is not None:
                out.append((headers, rows))
            headers, rows = None, None
    if headers is not None:
        out.append((headers, rows))
    return out


def real(rows):
    """Rows that are data rather than template placeholders."""
    keep = []
    for r in rows:
        joined = " ".join(v for v in r.values() if v)
        if not joined.strip():
            continue
        if "<" in joined and ">" in joined:
            continue
        if "·" in joined:            # an unfilled choice list
            continue
        keep.append(r)
    return keep


def col(rows, *names):
    for r in rows:
        for n in names:
            if n in r and r[n]:
                yield r[n]


class Check:
    def __init__(self, folder):
        self.folder = pathlib.Path(folder)
        self.problems = []
        self.notes = []
        self.text = {}
        for st in STAGES:
            p = self.folder / f"{st}.md"
            if p.exists():
                self.text[st] = p.read_text()

    def fail(self, rule, msg):
        self.problems.append(f"{rule}: {msg}")

    # ---- checks -----------------------------------------------------
    def stage_order(self):
        present = [s for s in STAGES if s in self.text]
        if not present:
            self.fail("B1", "no stage files found")
            return
        first = STAGES.index(present[0])
        if first != 0:
            self.fail("B1", f"{present[0]} present without 00-intent")
        for a, b in zip(present, present[1:]):
            if STAGES.index(b) != STAGES.index(a) + 1:
                self.fail("B1", f"{b} present but {STAGES[STAGES.index(b)-1]} is missing")

    def vocabulary(self):
        checks = [
            ("01-specification", "Procedure", "procedure"),
            ("02-plan", "Class", "class"),
            ("03-admission", "Outcome", "admission"),
            ("03-admission", "Confidence of the edges relied on", "confidence"),
            ("04-execution", "Terminal state", "terminal"),
            ("05-assurance", "Subject", "subject"),
            ("05-assurance", "Kind", "kind"),
            ("05-assurance", "Independence", "independence"),
            ("05-assurance", "Outcome", "outcome"),
            ("05-assurance", "State", "state"),
        ]
        for stage, header, vocab in checks:
            if stage not in self.text:
                continue
            for _, rows in tables(self.text[stage]):
                for v in col(real(rows), header):
                    if v not in VOCAB[vocab]:
                        self.fail("CONVENTIONS",
                                  f"{stage}: {header} = {v!r} is not a defined value")

    def spec_immutable(self):
        if "03-admission" not in self.text or "01-specification" not in self.text:
            return
        recorded = None
        for _, rows in tables(self.text["03-admission"]):
            for r in real(rows):
                if r.get("Field", "").startswith("Specification digest"):
                    recorded = r.get("Value")
        if not recorded:
            self.fail("R1", "no specification digest recorded at admission")
            return
        actual = digest(self.folder / "01-specification.md")
        if recorded != actual:
            self.fail("R1", f"specification changed after admission "
                            f"(recorded {recorded}, now {actual})")

    def scope_arbiter_disjoint(self):
        if "01-specification" not in self.text:
            return
        included, arbiter = set(), set()
        for headers, rows in tables(self.text["01-specification"]):
            if "Included" in headers:
                included |= {v for v in col(real(rows), "Included")}
            if "Path" in headers and "What it arbitrates" in headers:
                arbiter |= {v for v in col(real(rows), "Path")}
        overlap = included & arbiter
        if overlap:
            self.fail("R7", f"scope intersects the arbiter: {sorted(overlap)}")
        if included and not arbiter:
            self.notes.append("R7: no arbiter paths declared — nothing to be disjoint from")

    def evidence_independence(self):
        if "05-assurance" not in self.text:
            return
        ev = {}
        for headers, rows in tables(self.text["05-assurance"]):
            if "Independence" in headers:
                for r in real(rows):
                    ev[r.get("Id", "")] = r
        for eid, r in ev.items():
            if not r.get("Independence"):
                self.fail("R3", f"{eid} does not state independence")
            elif r["Independence"] == "independent" and not r.get("How established"):
                self.fail("R3", f"{eid} claims independence without saying how")
        for headers, rows in tables(self.text["05-assurance"]):
            if "Outcome" in headers and "Evidence" in headers:
                for r in real(rows):
                    if r.get("Outcome") != "met":
                        continue
                    for cited in ID_RE.findall(r.get("Evidence", "")):
                        src = ev.get(cited)
                        if src is None:
                            self.fail("B2", f"{r.get('Id')} cites undefined evidence {cited}")
                        elif src.get("Independence") == "not-independent":
                            self.fail("R3", f"{r.get('Id')} is met on non-independent {cited}")

    def deferred_complete(self):
        if "05-assurance" not in self.text:
            return
        for headers, rows in tables(self.text["05-assurance"]):
            if "State" in headers:
                for r in real(rows):
                    if r.get("State") == "deferred" and not (r.get("Window") and r.get("Baseline")):
                        self.fail("R6", f"{r.get('Id')} is deferred without a window and a baseline")

    def verdict_coverage(self):
        if "05-assurance" not in self.text or "01-specification" not in self.text:
            return
        criteria = set()
        for headers, rows in tables(self.text["01-specification"]):
            if "Criterion" in headers and "Procedure" in headers:
                criteria |= {r["Id"] for r in real(rows) if r.get("Id")}
        judged = set()
        for headers, rows in tables(self.text["05-assurance"]):
            if "Criterion" in headers and "Outcome" in headers:
                judged |= {r["Criterion"] for r in real(rows) if r.get("Criterion")}
        missing = criteria - judged
        if missing:
            self.fail("A5", f"no verdict for {sorted(missing)}")

    def landing_backed(self):
        if "06-landing" not in self.text:
            return
        states = {}
        for headers, rows in tables(self.text.get("05-assurance", "")):
            if "Unit" in headers and "State" in headers:
                for r in real(rows):
                    if r.get("Outcome") == "failed":
                        states.setdefault(r["Unit"], set()).add("failed")
                    else:
                        states.setdefault(r["Unit"], set()).add(r.get("State", ""))
        for headers, rows in tables(self.text["06-landing"]):
            if "Unit" in headers and "Entered at" in headers:
                for r in real(rows):
                    u = r.get("Unit")
                    if not r.get("Entered at"):
                        continue
                    if u not in states:
                        self.fail("A6", f"{u} entered with no verdict")
                    elif "failed" in states[u]:
                        self.fail("A6", f"{u} entered on a failed verdict")
                    elif r.get("Evidence still valid") == "no":
                        self.fail("A6", f"{u} entered on evidence no longer valid")

    def record_complete(self):
        p = self.folder / "record.md"
        if not p.exists():
            self.fail("B4", "no record.md — crossings are not recorded")
            return
        text = p.read_text()
        logged = set()
        for headers, rows in tables(text):
            if "Stage" in headers:
                logged |= {r["Stage"] for r in real(rows) if r.get("Stage")}
        for st in self.text:
            name = st.split("-", 1)[1]
            if name not in logged:
                self.fail("B4", f"stage {name} present but not recorded")

    def run(self):
        for m in (self.stage_order, self.vocabulary, self.spec_immutable,
                  self.scope_arbiter_disjoint, self.evidence_independence,
                  self.deferred_complete, self.verdict_coverage,
                  self.landing_backed, self.record_complete):
            m()
        return self.problems, self.notes


def main(argv):
    if len(argv) != 2:
        print(__doc__.strip())
        return 2
    c = Check(argv[1])
    if not c.folder.is_dir():
        print(f"not a directory: {argv[1]}")
        return 2
    problems, notes = c.run()
    stages = ", ".join(s for s in STAGES if s in c.text) or "none"
    print(f"{c.folder}  stages: {stages}")
    for n in notes:
        print(f"  note    {n}")
    for p in problems:
        print(f"  FAIL    {p}")
    print(f"  {'passed' if not problems else str(len(problems)) + ' problem(s)'}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
