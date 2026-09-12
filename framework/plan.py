#!/usr/bin/env python3
"""Area 2 — turning a specification into work, from what the estate says.

Usage: python3 framework/plan.py changes/<slug>            what the estate says about this specification
       python3 framework/plan.py changes/<slug> --draft    the same, as the derivable half of a plan

What this does and what it refuses to do is the whole point of it.

A plan says which units of work an intent becomes, what each may touch, what
judges each, and in what order they may run. Some of that is judgement — how
to divide the work, what to call each unit, whether a boundary is the right
one — and judgement is not derivable from a repository. Whoever plans has a
model of the intent; this has none.

The rest is derivable, and it is the part a planner most often gets wrong by
asserting it: what a scope reaches, whether anything can judge a change there,
which consumers no change can reach, and what unfinished work already holds
the same ground. Those four are answered here, from the estate, each carrying
how far it is to be trusted.

So this is not a planner. It is what a planner is owed before it starts, and
what the gate will later hold its plan to.
"""
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import estate                                        # noqa: E402
from check import Check, real, tables                # noqa: E402


def criteria_of(text):
    """Every criterion the specification states, and whether it says a plan for
    evidencing it is needed."""
    out = []
    for headers, rows in tables(text):
        if "Criterion" in headers and "Id" in headers:
            for r in real(rows):
                out.append({"id": r.get("Id", ""), "text": r.get("Criterion", ""),
                            "plan": r.get("Plan", ""), "class": r.get("Class", "")})
    return out


def ground(check):
    """The scope the specification declares, as paths that exist."""
    return sorted(p for p in check.scope_paths()
                  if p and (check.root / p).exists())


def judged(paths, root):
    """What could judge a change in each of these regions, and what could not."""
    out = []
    for path in paths:
        # The path as given: asking about a file's parent answers about every
        # file beside it, which on one run said the same thing about a rule
        # book and the gate that reads it.
        answer = estate.observability(path, root)
        out.append({"path": path, "region": path,
                    "verdict": answer.get("verdict", "unknown"),
                    "named": answer.get("named by a test", 0),
                    "symbols": answer.get("symbols", 0),
                    "means": answer.get("means") or answer.get("why", "")})
    return out


def report(folder):
    check = Check(folder)
    root = check.root
    estate.ROOT = root
    spec = check.text.get("01-specification", "")
    if not spec:
        print(f"  {folder} has no specification: there is nothing to plan from")
        return 2
    paths = ground(check)
    crits = criteria_of(spec)
    print(f"=== the specification states {len(crits)} criterion(s) over {len(paths)} path(s)")
    for c in crits:
        need = f", evidence plan {c['plan']}" if c["plan"] else ""
        print(f"    {c['id']}  {c['text'][:90]}{need}")
    if not paths:
        print("    the scope names nothing that exists here, so nothing below can be "
              "derived: name the ground before planning over it")
        return 1

    print("=== what a change to this scope reaches")
    rows = estate.affects(paths, root)
    firm = [r for r in rows if r["confidence"] != "low"]
    where = {}
    for r in firm:
        key = str(pathlib.PurePosixPath(r["file"]).parent)
        where.setdefault(key, []).append(r["file"])
    for region, files in sorted(where.items(), key=lambda kv: -len(kv[1]))[:12]:
        print(f"    {region}  {len(files)} file(s)")
    if len(where) > 12:
        print(f"    … and {len(where) - 12} more region(s)")
    print(f"    {len(firm)} file(s) reached firmly, {len(rows) - len(firm)} more on a "
          f"name that matched and nothing else")

    print("=== what could judge a change there")
    for row in judged(paths, root):
        print(f"    {row['path']}  {row['verdict']}  "
              f"({row['named']} of {row['symbols']} symbol(s) named by a test)")
        if row["verdict"] == "unclaimed":
            print(f"        nothing names it, so a criterion about behaviour here is not "
                  f"decidable as it stands: this needs a preparatory unit — a "
                  f"characterization of what it does now — before the work itself")

    print("=== who is beyond reach")
    try:
        offers, unmatched = estate.reachability(str(root), paths[0], None)
    except Exception:
        offers, unmatched = [], []
    beyond = [o for o in offers if o.get("beyond reach")]
    if beyond:
        for o in beyond:
            print(f"    {o['repo']} offers {o['kind']} {o['key']} — "
                  f"{', '.join(o['beyond reach'])} consume it and no change here reaches them")
        print("    a contract change with a consumer nothing here reaches cannot complete "
              "without a person, and that is known now rather than at the end")
    else:
        print("    nothing this scope offers is consumed from outside what can be changed "
              "here — as far as the contracts this index can read go")

    print("=== what already holds this ground")
    held = estate.conflicts(str(check.folder.parent), paths)
    for h in held:
        print(f"    {h['change']}/{h['unit']}  holds {', '.join(h['on'])}  ({h['how']})")
    if not held:
        print("    nothing unfinished holds it")
    print(estate.coverage_note(root))
    print("  — everything above is derived and carries what it rests on. How the work "
          "divides, what each unit is called, and which class each falls in are not "
          "derivable from a repository and are not attempted here.")
    return 0


def draft(folder):
    """The derivable half of a plan, in the shape the gate will read."""
    check = Check(folder)
    root = check.root
    estate.ROOT = root
    paths = ground(check)
    crits = criteria_of(check.text.get("01-specification", ""))
    rows = estate.affects(paths, root)
    firm = [r for r in rows if r["confidence"] != "low"]
    regions = sorted({str(pathlib.PurePosixPath(r["file"]).parent) for r in firm})
    unjudged = [j for j in judged(paths, root) if j["verdict"] == "unclaimed"]
    ids = [c["id"] for c in crits]
    n = (ids[0].split("-")[1] if ids and "-" in ids[0] else "nnn")
    print("## Work units")
    print()
    print("| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | "
          "Preparatory for |")
    print("|---|---|---|---|---|---|---|---|")
    unit = 1
    prep = {}
    for j in unjudged:
        wid = f"WU-{n}-{unit:02d}"
        prep[j["path"]] = wid
        print(f"| {wid} | 1 | C2 | {j['region']} | a characterization of what this region "
              f"does now; nothing else, derived, high | <the criterion this unit settles> "
              f"| | WU-{n}-{unit + 1:02d} |")
        unit += 1
    wid = f"WU-{n}-{unit:02d}"
    reach = ", ".join(regions[:4]) + (f" and {len(regions) - 4} more" if len(regions) > 4 else "")
    print(f"| {wid} | 1 | <class> | {' '.join(paths)} | {reach or 'nothing else here'}; "
          f"{len(firm)} file(s) reached firmly, derived, medium | {', '.join(ids)} | "
          f"{', '.join(sorted(prep.values()))} | |")
    print()
    print("## Coverage")
    print()
    print(f"The units above cover {len(ids)} criterion(s): {', '.join(ids)}. "
          f"The estate query that produced the scope reached {len(firm)} file(s) firmly "
          f"and {len(rows) - len(firm)} on a name alone; "
          + (estate.coverage_note(root).strip() or "") + ".")
    print()
    print("<!-- The class of each unit, how the work divides, and what each unit is "
          "called are judgement and are left blank. -->")
    return 0


def main(argv):
    if len(argv) < 2:
        print(__doc__.strip())
        return 2
    folder = pathlib.Path(argv[1])
    if not folder.is_dir():
        print(f"  {folder} is not a change folder")
        return 2
    return draft(folder) if "--draft" in argv else report(folder)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
