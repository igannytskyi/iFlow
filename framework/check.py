#!/usr/bin/env python3
"""Mechanical gate over one change folder.

Checks what artifacts can decide. What they cannot decide is listed in
framework/RULES.md under the rules marked H, and is not attempted here.

Usage: python3 framework/check.py changes/<slug>     one change
       python3 framework/check.py --status [dir]      every change, at a glance
       python3 framework/check.py --arbiters          run every arbiter
       python3 framework/check.py --mutate            break each rule, see who notices
       python3 framework/check.py --repeat <slug>     run what claims to repeat, twice
       python3 framework/check.py --unused            fields nobody reads, codes nobody checks
       python3 framework/check.py --escalations       what the method owes itself
Exit status is 0 when nothing failed.
"""
import hashlib
import pathlib
import re
import subprocess
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


PATHISH = re.compile(r"[A-Za-z0-9_.\-/]+/[A-Za-z0-9_.\-/]*")


def paths_in(text):
    """Every path-shaped word in a cell. Prose around it is prose; what can be
    checked is what names ground."""
    return {m.rstrip("/.,;:") for m in PATHISH.findall(text or "")}


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
    def __init__(self, folder, root=None):
        self.folder = pathlib.Path(folder)
        # The estate a change is carried against is the repository it lives in,
        # which is not the folder the change lives in.
        self.root = pathlib.Path(root) if root else self.folder.resolve().parent.parent
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

    def criteria_vocabulary(self):
        """R19: a criterion naming something the change may touch can be met by
        editing that thing. The probe is crude — a path appearing in the text —
        and crude is enough, because the mistake it catches is not subtle."""
        if "01-specification" not in self.text:
            return
        scope = {p for p in self.scope_paths() if p and "/" in p or (p and "." in p)}
        if not scope:
            return
        for headers, rows in tables(self.text["01-specification"]):
            if "Criterion" in headers and "Procedure" in headers:
                for r in real(rows):
                    said = " ".join(r.get(k, "") for k in ("Criterion", "When", "Then"))
                    named = [p for p in scope if p in said]
                    if named:
                        self.fail("R19", f"{r.get('Id')} names {named[0]}, which the change "
                                         f"may touch: it can be met by editing what it names")

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
        self.plan_against_estate(needs, planned)
        self.plan_covers_reach(needs, planned)

    def plan_against_estate(self, needs, planned):
        """R16, the half that could not be checked until there was a model.

        The gate could see that a plan named an area of effect and never that
        the area was real: a plan derived from the candidate names whatever the
        author had in mind, and reads exactly like one derived from the estate.
        Now the estate can be asked. Ground the change provably does not reach
        is not an area of effect, and a plan that names it was derived from
        something else.

        Where the estate cannot answer — it is not this repository, or nothing
        here reads that language — the check is not silently skipped: it says
        so, because an unasked question and an answered one look the same in a
        log otherwise.
        """
        scope = sorted(p for p in self.scope_paths() if p)
        if not needs or not scope:
            return
        try:
            sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
            import estate
            reach = {r["file"] for r in estate.affects(scope, self.root)}
            named_as_text = {r["file"] for r in estate.named_by(scope, self.root)}
        except Exception as e:                       # noqa: BLE001 — reported, not raised
            self.notes.append(f"R16: the estate could not be asked about this scope "
                              f"({type(e).__name__}), so what the plan names is recorded "
                              f"and not verified")
            return
        known = reach | named_as_text | set(scope)
        for c in sorted(needs):
            row = planned.get(c)
            if row is None:
                continue
            for field in ("Observed", "Derived from"):
                for path in paths_in(row.get(field, "")):
                    if path in known or any(k.startswith(path) for k in known):
                        continue
                    if not (self.root / path).exists():
                        continue                     # not a path, or not here to judge
                    self.fail("R16", f"the plan for {c} names {path} under {field!r}, and "
                                     f"nothing in the scope reaches it — an area of effect "
                                     f"names what the change touches, so this was derived "
                                     f"from something other than the estate")

    def plan_covers_reach(self, needs, planned):
        """R16, the other half: a plan can name nothing wrong and still miss
        everything that matters.

        The demand is not that a plan observe everywhere a change reaches —
        a change to a central module reaches hundreds of regions, and a rule
        producing a list that long is a rule nobody reads. It is narrower and
        it is the part nobody else covers: ground the change reaches firmly,
        where nothing already names a symbol in a test, and which no plan
        observes. Anywhere else, something is watching; there, nothing is.

        Measured across five estates, that residual is nought for most changes
        and never more than three regions — which is what makes it a failure
        rather than a report.
        """
        scope = sorted(p for p in self.scope_paths() if p)
        if not needs or not scope:
            return
        try:
            sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
            import estate
            rows = estate.affects(scope, self.root)
            regions = estate.every_region(self.root)
        except Exception:                        # already reported by the half above
            return
        firm = {self.region_of(r["file"]) for r in rows
                if r["confidence"] in ("high", "medium")}
        watched = {r["region"] for r in regions
                   if r["verdict"] != "unclaimed" or r["is test ground"]}
        observed = {self.region_of(p) for row in planned.values()
                    for field in ("Observed", "Derived from")
                    for p in paths_in(row.get(field, ""))}
        mine = {self.region_of(p) for p in scope}
        for region in sorted(firm - watched - observed - mine):
            self.fail("R16", f"the change reaches {region} firmly, nothing there is named "
                             f"by a test, and no plan observes it — a plan that covers "
                             f"none of an unwatched area leaves it judged by nothing")

    def region_of(self, path):
        """The region a path names: itself when it is a directory, its parent
        when it is a file."""
        p = pathlib.PurePosixPath(str(path).rstrip("/"))
        return str(p) if (self.root / p).is_dir() else str(p.parent)

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
            elif value == "yes":
                # A claim to repeat is worth nothing unless it says what to re-run.
                # Naming it is what makes the claim falsifiable at all.
                if not self.producer_command(eid):
                    self.fail("R13", f"{eid} claims to repeat and names nothing that can be "
                                     f"re-run, so the claim cannot be tested")
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

    def producers(self):
        """Evidence marked repeatable, and the command each names."""
        out = {}
        for headers, rows in tables(self.text.get("05-assurance", "")):
            if "Repeatable" in headers and "Producer" in headers:
                for r in real(rows):
                    if r.get("Repeatable") == "yes":
                        out[r.get("Id", "")] = r.get("Producer", "")
        return out

    def producer_command(self, eid):
        """A path inside the repository that could be run again, or nothing."""
        said = self.producers().get(eid, "")
        for token in re.findall(r"[\w./-]+\.py", said):
            for base in (pathlib.Path("."), self.folder):
                if (base / token).is_file():
                    return base / token
        return None

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
        for m in (self.stage_order, self.headers_present,
                  self.criteria_vocabulary, self.evidence_plan,
                  self.arbiter_inputs,
                  self.every_criterion_arbitrated,
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


def arbiters(root):
    """Run every arbiter, and check each says whether it tests its criterion or
    a proxy for it. Running them is one action so that it can be done where a
    change lands and not only in an isolated tree."""
    import importlib.util
    tests = sorted((root / "framework" / "tests").glob("test_*.py"))
    problems, results = [], []
    for t in tests:
        r = subprocess.run([sys.executable, str(t)], capture_output=True, text=True, cwd=root)
        results.append((t.name, r.stdout.strip().splitlines()[-1].strip() if r.stdout.strip()
                        else "no output", r.returncode))
        spec = importlib.util.spec_from_file_location(t.stem, t)
        m = importlib.util.module_from_spec(spec)
        sys.path.insert(0, str(t.parent))
        try:
            spec.loader.exec_module(m)
        finally:
            sys.path.pop(0)
        declared = getattr(m, "TESTS", None)
        if declared is None:
            problems.append(f"R20: {t.name} does not say which of its criteria it tests "
                            f"directly and which through a proxy")
            continue
        checks = {n.upper().replace("_", "-") for n in dir(m)
                  if n.startswith("cr_") and callable(getattr(m, n))}
        for c in sorted(checks - set(declared)):
            problems.append(f"R20: {t.name} checks {c} without saying whether that is the "
                            f"criterion or a proxy for it")
        for c, how in declared.items():
            if isinstance(how, tuple) and not how[1]:
                problems.append(f"R20: {t.name} calls {c} a proxy without saying for what")
    return results, problems


def unused(root):
    """Ceremony: a column no check reads, a code no check validates.

    A field people are asked to fill and nothing consults is a record kept for
    its own sake, which is the thing this method exists to remove rather than
    to accumulate.
    """
    source = (root / "framework" / "check.py").read_text()
    read = set(re.findall(r'["\'](.*?)["\'] in headers', source))
    read |= set(re.findall(r'\.get\(["\'](.*?)["\']', source))
    read |= set(re.findall(r'col\(real\(rows\), ["\'](.*?)["\']', source))
    # the vocabulary check names its columns in a list of tuples, not in a test
    read |= set(re.findall(r'\("[\w-]+", "([^"]+)", "[\w-]+"\)', source))
    read |= set(re.findall(r'r\.get\("([^"]+)", ""\)', source))
    columns = {}
    for t in sorted((root / "framework" / "templates").glob("*.md")):
        for block in re.findall(r"(?:^\|.*\|$\n)+", t.read_text(), re.M):
            for c in (x.strip() for x in block.splitlines()[0].strip("|").split("|")):
                if c and c not in ("#",):
                    columns.setdefault(c, set()).add(t.name)
    dead_columns = {c: sorted(v) for c, v in columns.items() if c not in read}
    checked = set()
    for group in re.findall(r"VOCAB = \{(.*?)\n\}", source, re.S):
        checked |= set(re.findall(r'"([a-z0-9-]+)"', group))
    declared = set()
    for line in (root / "framework" / "conventions.md").read_text().splitlines():
        if line.strip().startswith("|"):
            declared |= set(re.findall(r"`([A-Za-z0-9-]+)`", line))
    dead_codes = sorted(c for c in declared - checked
                        if c.islower() and "-" in c or c in ("yes", "no", "unknown"))
    return dead_columns, dead_codes


def mutate(root):
    """Break each rule in turn and see whether any arbiter notices.

    A rule nothing notices the loss of is not being tested, whatever the
    arbiters claim. This replaces asking an arbiter to record that it once
    failed: the demonstration is repeated now rather than attested about a
    run that has ended.
    """
    import shutil
    import tempfile
    source = (root / "framework" / "check.py").read_text()
    labels = sorted(set(re.findall(r'self\.fail\("([^"]+)"', source))
                    | set(re.findall(r'problems\.append\(f"([A-Z]\d+)', source)))
    unnoticed, tests = [], sorted((root / "framework" / "tests").glob("test_*.py"))
    for label in labels:
        broken = re.sub(r'self\.fail\("' + re.escape(label) + r'"[^\n]*\n(\s+f"[^\n]*\n)*',
                        "pass\n", source)
        broken = re.sub(r'problems\.append\(f"' + re.escape(label) + r'[^\n]*\n(\s+f"[^\n]*\n)*',
                        "pass\n", broken)
        if broken == source:
            continue
        with tempfile.TemporaryDirectory() as tmp:
            d = pathlib.Path(tmp) / "iflow"
            # What is copied is the method, not the work carried through it.
            # Changes are local material — on this repository they came to hold
            # the estates the model was tried against — and copying them once
            # per rule turned a two minute demonstration into an hour of it.
            shutil.copytree(root, d, ignore=shutil.ignore_patterns(
                ".git", "__pycache__", "_bmad*", "node_modules", "changes", ".estate",
                "_lib"))
            (d / "framework" / "check.py").write_text(broken)
            noticed = False
            for t in tests:
                r = subprocess.run([sys.executable, str(d / "framework" / "tests" / t.name)],
                                   capture_output=True, text=True, cwd=d)
                if r.returncode:
                    noticed = True
                    break
            if not noticed:
                unnoticed.append(label)
    return labels, unnoticed


def status(root, where):
    """Every change at a glance: how far it got, and what holds it."""
    rows = []
    base = pathlib.Path(where)
    for folder in sorted(base.iterdir()) if base.is_dir() else []:
        if not folder.is_dir() or not (folder / "00-intent.md").exists():
            continue
        c = Check(folder)
        reached = [s for s in STAGES if s in c.text]
        problems, _ = c.run()
        held, waiting, undecided, open_esc = 0, 0, 0, 0
        for headers, rws in tables(c.text.get("03-admission", "")):
            if "Outcome" in headers:
                for r in real(rws):
                    held += r.get("Outcome") == "held"
                    waiting += r.get("Outcome") == "awaiting-authority"
        for headers, rws in tables(c.text.get("05-assurance", "")):
            if "Outcome" in headers and "Criterion" in headers:
                undecided += sum(1 for r in real(rws) if r.get("Outcome") == "undecided")
            if "Standing" in headers:
                open_esc += sum(1 for r in real(rws) if r.get("Standing") == "open")
        rows.append((folder.name, reached[-1] if reached else "-", len(problems),
                     held, waiting, undecided, open_esc))
    return rows


def main(argv):
    root = pathlib.Path(".").resolve()
    if len(argv) == 2 and argv[1] == "--arbiters":
        results, problems = arbiters(root)
        for name, last, code in results:
            print(f"  {'FAIL' if code else 'ok  '}  {name}  {last}")
        for p in problems:
            print(f"  FAIL    {p}")
        bad = sum(1 for _, _, c in results if c) + len(problems)
        print(f"  {'passed' if not bad else str(bad) + ' problem(s)'}")
        return 1 if bad else 0
    if len(argv) == 3 and argv[1] == "--repeat":
        c = Check(argv[2])
        claims = c.producers()
        if not claims:
            print("  nothing here claims to repeat")
            return 0
        bad = 0
        for eid in sorted(claims):
            cmd = c.producer_command(eid)
            if cmd is None:
                print(f"  FAIL    {'R13'}: {eid} claims to repeat and names nothing to re-run")
                bad += 1
                continue
            runs = [subprocess.run([sys.executable, str(cmd)], capture_output=True,
                                   text=True).stdout for _ in range(2)]
            if runs[0] == runs[1]:
                print(f"  ok      {eid}  {cmd} gave the same result twice")
            else:
                print(f"  FAIL    {'R13'}: {eid} claims to repeat and {cmd} gave two "
                      f"different results")
                bad += 1
        print(f"  {'passed' if not bad else str(bad) + ' problem(s)'}")
        return 1 if bad else 0
    if len(argv) == 2 and argv[1] == "--unused":
        dead_columns, dead_codes = unused(root)
        for c, where in sorted(dead_columns.items()):
            print(f"  column   {c!r} — filled in {', '.join(where)}, read by nothing")
        for c in dead_codes:
            print(f"  code     {c!r} — defined in conventions, validated by nothing")
        total = len(dead_columns) + len(dead_codes)
        print(f"  {total} field(s) nobody reads" if total else "  nothing is filled in for its own sake")
        return 0
    if len(argv) == 2 and argv[1] == "--mutate":
        labels, unnoticed = mutate(root)
        for label in labels:
            if label in unnoticed:
                print(f'  FAIL    {"R12"}: nothing notices the loss of {label}, '
                      f'so nothing is testing it')
            else:
                print(f"  ok      {label}")
        print(f"  {len(labels) - len(unnoticed)} of {len(labels)} rules are actually tested")
        return 1 if unnoticed else 0
    if len(argv) in (2, 3) and argv[1] == "--status":
        where = argv[2] if len(argv) == 3 else "changes"
        rows = status(root, where)
        if not rows:
            print(f"  no changes under {where}")
            return 0
        print(f"  {'change':28} {'reached':16} {'gate':>5} {'held':>5} {'wait':>5} "
              f"{'undec':>6} {'owed':>5}")
        for name, reached, probs, held, waiting, undec, esc in rows:
            print(f"  {name:28} {reached:16} {('ok' if not probs else str(probs)):>5} "
                  f"{held:>5} {waiting:>5} {undec:>6} {esc:>5}")
        print(f"  {len(rows)} change(s)")
        return 0
    if len(argv) == 2 and argv[1] == "--escalations":
        owed, closed, problems = ledger(root)
        for p in problems:
            print(f"  FAIL    {p}")
        for ref, where in sorted(closed.items()):
            print(f"  settled {ref}  by {where}")
        print(f"  still owed: {len(owed)}")
        by_rule = {}
        for ref, ground in owed:
            for rule in sorted(set(re.findall(r"\b(R\d+b?|B\d)\b", ground))):
                by_rule.setdefault(rule, []).append(ref)
        for ref, ground in owed:
            print(f"    {ref}  {ground[:100]}")
        shared = {r: refs for r, refs in by_rule.items() if len(refs) > 1}
        if shared:
            print("  named by more than one, which is how one defect looks like several:")
            for rule, refs in sorted(shared.items()):
                print(f"    {rule}  {', '.join(refs)}")
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
