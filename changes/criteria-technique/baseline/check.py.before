#!/usr/bin/env python3
"""Mechanical gate over one change folder.

Checks what artifacts can decide. What they cannot decide is listed in
framework/RULES.md under the rules marked H, and is not attempted here.

Usage: python3 framework/check.py changes/<slug>
       python3 framework/check.py --escalations
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
    "independence": {"independent-by-executor", "independent-by-precommitment",
                     "not-independent"},
    "repeatable": {"yes", "no", "unknown"},
    "plan": {"required", "not-required"},
    "refused_by": {"person", "area"},
    "reviewed": {"yes", "no"},
    "outcome": {"met", "failed", "undecided"},
    "state": {"settled", "deferred"},
    "admission": {"admitted", "held", "refused", "awaiting-authority"},
    "terminal": {"candidate", "terminated", "exhausted",
                 "failed-substrate", "failed-task", "cancelled"},
}

# Columns the checks below bind to. A check whose column is absent cannot run,
# and a check that cannot run must say so rather than pass by looking at nothing.
REQUIRED_HEADERS = {
    "01-specification": [("Criterion", "Procedure"), ("Included",), ("Path", "What it arbitrates")],
    "02-plan": [("Class",), ("Preparatory for",)],
    "03-admission": [("Outcome",), ("Field", "Value")],
    "04-execution": [("Terminal state",), ("Stored at", "Artefacts")],
    "05-assurance": [("Independence",), ("Outcome", "State")],
    "06-landing": [("Entered at",)],
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


PLACEHOLDER = re.compile(r"<[^<>]*>")


def _choice_list(cell):
    """A cell offering the template's alternatives, e.g. 'machine · human'."""
    return "·" in cell and all(part.strip() for part in cell.split("·"))


def real(rows):
    """Rows that are data rather than template placeholders.

    A row is a placeholder when a cell carries a <...> slot, or when every
    cell it has is an unchosen list of alternatives. Punctuation inside prose
    a person wrote — a two-sided bound, an arrow — is not a placeholder, and
    dropping such a row would remove a criterion from the gate's view while
    the gate went on reporting that it passed.
    """
    keep = []
    for r in rows:
        cells = [v.strip() for v in r.values() if v and v.strip()]
        if not cells:
            continue
        if any(PLACEHOLDER.search(c) for c in cells):
            continue
        if all(_choice_list(c) for c in cells):
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
    def headers_present(self):
        """B1: every column a check binds to is there, or the gate says which is not."""
        for stage, groups in REQUIRED_HEADERS.items():
            if stage not in self.text:
                continue
            seen = set()
            for headers, _ in tables(self.text[stage]):
                seen |= set(headers)
            for group in groups:
                if not set(group) <= seen:
                    missing = ", ".join(sorted(set(group) - seen))
                    self.fail("B1", f"{stage}: column {missing} is absent, so the check "
                                    f"binding to it would not run")

    def evidence_plan(self):
        """R16: a criterion that needs a plan has one, and the plan says what it
        was derived from. A plan derived from the candidate rather than from the
        estate is the failure this exists to prevent, so the derivation is the
        one field that cannot be left empty."""
        if "01-specification" not in self.text:
            return
        needs = set()
        declares = False
        for headers, rows in tables(self.text["01-specification"]):
            if "Plan" in headers and "Criterion" in headers:
                declares = True
                for r in real(rows):
                    v = r.get("Plan", "")
                    if v and v not in VOCAB["plan"]:
                        self.fail("CONVENTIONS",
                                  f"01-specification: Plan = {v!r} is not a defined value")
                    if v == "required":
                        needs.add(r.get("Id", ""))
        if not declares:
            self.notes.append("R16: this specification does not say which criteria need an "
                              "evidence plan — it predates the obligation rather than "
                              "violating it")
            return
        planned = {}
        for headers, rows in tables(self.text.get("03-admission", "")):
            if "Observed" in headers and "Derived from" in headers:
                for r in real(rows):
                    planned[r.get("Criterion", "")] = r
        for c in sorted(needs):
            row = planned.get(c)
            if row is None:
                if "04-execution" in self.text:
                    self.fail("R16", f"{c} needs an evidence plan and was executed against "
                                     f"without one")
                else:
                    self.fail("R16", f"{c} needs an evidence plan and has none")
                continue
            for field in ("Observed", "Method", "Derived from"):
                if not row.get(field):
                    self.fail("R16", f"the plan for {c} leaves {field!r} empty; a plan that "
                                     f"does not say what it was derived from was derived "
                                     f"from the change")

    def arbiter_inputs(self):
        """R7b: an arbiter input inside the scope needs its prior state captured."""
        if "01-specification" not in self.text:
            return
        reads = {}
        for headers, rows in tables(self.text["01-specification"]):
            if "Input" in headers and "In scope?" in headers:
                for r in real(rows):
                    reads[r.get("Input", "")] = r.get("In scope?", "")
        if not reads:
            self.notes.append("R7b: this specification declares no arbiter inputs — "
                              "it predates the obligation rather than violating it")
            return
        captured = ""
        for headers, rows in tables(self.text.get("03-admission", "")):
            if "Artefact" in headers and "Captured to" in headers:
                captured += " ".join(v for r in real(rows) for v in r.values() if v)
        for inp, in_scope in reads.items():
            if in_scope.startswith("yes") and inp not in captured:
                self.fail("R7b", f"{inp} is read by the arbiter and inside the scope, "
                                 f"with no prior state captured at admission")

    def every_criterion_arbitrated(self):
        """R11: a criterion nothing claims to arbitrate is not being tested."""
        if "01-specification" not in self.text:
            return
        criteria, claimed = set(), ""
        for headers, rows in tables(self.text["01-specification"]):
            if "Criterion" in headers and "Procedure" in headers:
                criteria |= {r["Id"] for r in real(rows) if r.get("Id")}
            if "Path" in headers and "What it arbitrates" in headers:
                claimed += " ".join(col(real(rows), "What it arbitrates"))
        for c in sorted(criteria):
            if c not in claimed:
                self.fail("R11", f"{c} is claimed by no arbiter, so nothing tests it")

    def arbiter_acceptance(self):
        """R12: a unit that builds an arbiter records how it failed before acceptance."""
        if "04-execution" not in self.text:
            return
        recorded, has_section = "", False
        for headers, rows in tables(self.text["04-execution"]):
            if "Before repair" in headers:
                has_section = True
                recorded += " ".join(v for r in real(rows) for v in r.values() if v)
        if not has_section:
            self.notes.append("R12: this change records no arbiter acceptance — "
                              "it predates the obligation rather than violating it")
            return
        arbiter = set()
        for headers, rows in tables(self.text.get("01-specification", "")):
            if "Path" in headers and "What it arbitrates" in headers:
                arbiter |= {v for v in col(real(rows), "Path")}
        for headers, rows in tables(self.text.get("02-plan", "")):
            if "Preparatory for" in headers and "Scope" in headers:
                for r in real(rows):
                    if not r.get("Preparatory for"):
                        continue
                    if any(a and a in r.get("Scope", "") for a in arbiter):
                        if r.get("Id", "") not in recorded:
                            self.fail("R12", f"{r.get('Id')} builds an arbiter and does not "
                                             f"record how it failed before acceptance")

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
        if recorded == actual:
            return
        # Immutability protects criteria from being shaped by what execution
        # produced. Before a candidate exists there is nothing to be shaped by,
        # so an edit then is a re-admission — free, but never silent.
        if self.candidates():
            self.fail("R1", f"specification changed after a candidate existed "
                            f"(recorded {recorded}, now {actual})")
            return
        if self.readmission_recorded():
            self.notes.append("R1: the specification differs from the digest taken at "
                              "admission and no candidate exists — a re-admission, recorded")
        else:
            self.fail("R1", f"specification changed with no candidate and no record of a "
                            f"re-admission (recorded {recorded}, now {actual})")

    def candidates(self):
        out = set()
        for headers, rows in tables(self.text.get("04-execution", "")):
            if "Stored at" in headers:
                out |= {r.get("Id", "") for r in real(rows) if r.get("Id")}
        return out

    def readmission_recorded(self):
        p = self.folder / "record.md"
        if not p.exists():
            return False
        seen_admission = False
        for headers, rows in tables(p.read_text()):
            if "Stage" in headers:
                for r in real(rows):
                    stage = r.get("Stage", "")
                    if stage == "admission":
                        seen_admission = True
                    elif stage == "specification" and seen_admission:
                        return True
        return False

    def scope_paths(self):
        out = set()
        for headers, rows in tables(self.text.get("01-specification", "")):
            if "Included" in headers:
                out |= {v for v in col(real(rows), "Included")}
        return out

    def candidates_stored(self):
        """R2: a candidate exists apart from the live tree."""
        if "04-execution" not in self.text:
            return
        for headers, rows in tables(self.text["04-execution"]):
            if "Stored at" in headers:
                for r in real(rows):
                    where = r.get("Stored at", "")
                    if not where:
                        self.fail("R2", f"{r.get('Id')} names no store")
                    elif not (self.folder / where).exists():
                        self.fail("R2", f"{r.get('Id')} is not stored at {where}")

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
        scope = self.scope_paths()
        for eid, r in ev.items():
            ind = r.get("Independence", "")
            if not ind:
                self.fail("R3", f"{eid} does not state independence")
            elif ind.startswith("independent") and not r.get("How established"):
                self.fail("R3", f"{eid} claims independence without saying how")
            producer = r.get("Producer", "")
            inside = [pth for pth in scope if pth and pth in producer]
            if inside and ind != "not-independent":
                self.fail("R3b", f"{eid} is produced from inside the scope ({inside[0]}) "
                                 f"and cannot be {ind}")
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

    def refusals(self):
        """R17: a refusal names what it returns and why, and nothing downstream
        of a refused object is accepted. A refusal is the producer's failure,
        not a request to try again."""
        refused = {}
        for stage in STAGES:
            for headers, rows in tables(self.text.get(stage, "")):
                if "Refused by" in headers and "Ground" in headers:
                    for r in real(rows):
                        obj = r.get("Object", "")
                        if not obj:
                            self.fail("R17", f"{r.get('Id')} refuses nothing in particular")
                            continue
                        if not r.get("Ground"):
                            self.fail("R17", f"{r.get('Id')} refuses {obj} without a ground")
                        by = r.get("Refused by", "")
                        if by and by not in VOCAB["refused_by"]:
                            self.fail("CONVENTIONS",
                                      f"Refused by = {by!r} is not a defined value")
                        refused[obj] = r.get("Id", "")
        if not refused:
            return
        for headers, rows in tables(self.text.get("05-assurance", "")):
            if "Outcome" in headers and "Unit" in headers:
                for r in real(rows):
                    if r.get("Outcome") != "met":
                        continue
                    for obj, rid in refused.items():
                        if obj in " ".join(v for v in r.values() if v):
                            self.fail("R17", f"{r.get('Id')} accepts {obj}, which {rid} refused")
        for headers, rows in tables(self.text.get("04-execution", "")):
            if "Stored at" in headers:
                for r in real(rows):
                    if r.get("Id") in refused:
                        unit = r.get("Unit", "")
                        for h2, rows2 in tables(self.text.get("05-assurance", "")):
                            if "Unit" in h2 and "Outcome" in h2:
                                for v in real(rows2):
                                    if v.get("Unit") == unit and v.get("Outcome") == "met":
                                        self.fail("R17", f"{v.get('Id')} accepts work whose "
                                                         f"candidate {r.get('Id')} was refused")

    def criteria_review(self):
        """R18: whether a person read the criteria before admission is recorded
        either way. Recording *no* is legitimate; saying nothing is not."""
        if "03-admission" not in self.text:
            return
        for headers, rows in tables(self.text["03-admission"]):
            if "Reviewed before admission" in headers:
                for r in real(rows):
                    v = r.get("Reviewed before admission", "")
                    if v not in VOCAB["reviewed"]:
                        self.fail("R18", f"criteria review recorded as {v!r}, which is not "
                                         f"a defined value")
                    elif v == "yes" and not r.get("By"):
                        self.fail("R18", "the criteria are recorded as reviewed and nobody "
                                         "is named as having reviewed them")
                return
        self.notes.append("R18: this change does not say whether the criteria were reviewed "
                          "before admission — it predates the obligation rather than "
                          "violating it")

    def repeatability(self):
        """R13 and R14: a record of a run says whether the run can be produced
        again, and a result does not rest on one that cannot alone."""
        if "05-assurance" not in self.text:
            return
        ev, has_column = {}, False
        for headers, rows in tables(self.text["05-assurance"]):
            if "Repeatable" in headers:
                has_column = True
                for r in real(rows):
                    ev[r.get("Id", "")] = r.get("Repeatable", "")
        if not has_column:
            self.notes.append("R13: this change's evidence does not state repeatability — "
                              "it predates the obligation rather than violating it")
            return
        for eid, value in ev.items():
            if not value:
                self.fail("R13", f"{eid} does not say whether the run behind it repeats")
            elif value not in VOCAB["repeatable"]:
                self.fail("R13", f"{eid}: Repeatable = {value!r} is not a defined value")
        for headers, rows in tables(self.text["05-assurance"]):
            if "Outcome" in headers and "Evidence" in headers:
                for r in real(rows):
                    if r.get("Outcome") != "met":
                        continue
                    cited = ID_RE.findall(r.get("Evidence", ""))
                    known = [c for c in cited if c in ev]
                    if known and all(ev[c] in ("no", "unknown") for c in known):
                        self.fail("R14", f"{r.get('Id')} is met only on runs that cannot be "
                                         f"produced again ({', '.join(known)})")

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
                    outcome = r.get("Outcome")
                    if outcome in ("failed", "undecided"):
                        states.setdefault(r["Unit"], set()).add(outcome)
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
                    elif "undecided" in states[u]:
                        self.fail("R10", f"{u} entered with an undecided criterion — "
                                         f"acceptance was not established")
                    elif r.get("Evidence still valid") == "no":
                        self.fail("A6", f"{u} entered on evidence no longer valid")

    def record_append_only(self):
        """R4: ordinals form 1..N, in order, with no gap and no repeat.

        A line removed or reordered is detected. A line removed from the tail is
        not: that needs a digest chain the record does not yet carry, and
        RULES.md says so rather than implying otherwise.
        """
        p = self.folder / "record.md"
        if not p.exists():
            return
        seen = []
        for headers, rows in tables(p.read_text()):
            if "#" in headers and "Stage" in headers:
                for r in real(rows):
                    n = r.get("#", "")
                    if n.isdigit():
                        seen.append(int(n))
        if not seen:
            return
        expected = list(range(1, len(seen) + 1))
        if seen != expected:
            self.fail("R4", f"the record is not append-only: ordinals {seen} "
                            f"are not {expected[0]}..{expected[-1]} in order")

    def arbiter_changes_separate(self):
        """R8: only a unit that is preparatory for another may touch the arbiter."""
        if "04-execution" not in self.text or "01-specification" not in self.text:
            return
        arbiter = set()
        for headers, rows in tables(self.text["01-specification"]):
            if "Path" in headers and "What it arbitrates" in headers:
                arbiter |= {v for v in col(real(rows), "Path")}
        if not arbiter:
            return
        preparatory = set()
        for headers, rows in tables(self.text.get("02-plan", "")):
            if "Preparatory for" in headers:
                for r in real(rows):
                    if r.get("Preparatory for"):
                        preparatory.add(r.get("Id", ""))
        for headers, rows in tables(self.text["04-execution"]):
            if "Artefacts" in headers and "Unit" in headers:
                for r in real(rows):
                    touched = [a for a in arbiter if a and a in r.get("Artefacts", "")]
                    if touched and r.get("Unit") not in preparatory:
                        self.fail("R8", f"{r.get('Unit')} touches the arbiter ({touched[0]}) "
                                        f"and is nobody's preparatory work")

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
        for m in (self.stage_order, self.headers_present, self.evidence_plan,
                  self.arbiter_inputs,
                  self.every_criterion_arbitrated, self.arbiter_acceptance,
                  self.vocabulary, self.spec_immutable,
                  self.candidates_stored,
                  self.scope_arbiter_disjoint, self.evidence_independence,
                  self.refusals, self.criteria_review,
                  self.repeatability, self.deferred_complete, self.verdict_coverage,
                  self.landing_backed, self.record_append_only,
                  self.arbiter_changes_separate, self.record_complete):
            m()
        return self.problems, self.notes


def ledger(root):
    """The framework's debt to itself: every escalation raised, minus those a
    later change has settled. An escalation is addressed as <slug>#<n>, because
    its number is an ordinal local to the change that raised it."""
    raised, closed, problems = {}, {}, []
    for folder in sorted((root / "changes").iterdir()):
        if not folder.is_dir():
            continue
        p = folder / "05-assurance.md"
        if not p.exists():
            continue
        text = p.read_text()
        for headers, rows in tables(text):
            if "Standing" in headers and "Ground" in headers:
                for r in real(rows):
                    n = r.get("#", "")
                    if n.isdigit():
                        raised[f"{folder.name}#{n}"] = (r.get("Standing", ""),
                                                        r.get("Ground", ""))
            if "Escalation" in headers and "Resolved by" in headers:
                for r in real(rows):
                    ref = r.get("Escalation", "")
                    if not ref:
                        continue
                    if ref in closed:
                        problems.append(f"R15: {ref} is settled twice — in "
                                        f"{closed[ref]} and in {folder.name}")
                    closed[ref] = folder.name
    for ref, where in closed.items():
        if ref not in raised:
            problems.append(f"R15: {where} settles {ref}, which was never owed")
    owed = [(k, v[1]) for k, v in sorted(raised.items())
            if v[0] == "open" and k not in closed]
    return owed, closed, problems


def main(argv):
    if len(argv) == 2 and argv[1] == "--escalations":
        root = pathlib.Path(".").resolve()
        owed, closed, problems = ledger(root)
        for p in problems:
            print(f"  FAIL    {p}")
        for ref, where in sorted(closed.items()):
            print(f"  settled {ref}  by {where}")
        print(f"  still owed: {len(owed)}")
        for ref, ground in owed:
            print(f"    {ref}  {ground[:100]}")
        return 1 if problems else 0
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
