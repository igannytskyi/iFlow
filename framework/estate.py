#!/usr/bin/env python3
"""What the estate is, derived from it rather than described about it.

Usage: python3 framework/estate.py affects <path>...     what a change here reaches
       python3 framework/estate.py observability <path>  how well behaviour there is pinned
       python3 framework/estate.py freshness             what this was derived from, and when
       python3 framework/estate.py unknown               what it could not resolve
       python3 framework/estate.py contracts <dir>       what crosses between repositories
       python3 framework/estate.py observe <path> <cmd>   run it and see what actually ran
       python3 framework/estate.py inflight <dir>         what unfinished work already holds
       python3 framework/estate.py conflicts <dir> <path> who already holds these paths

Every answer carries where it came from and how far it is to be trusted. Nothing
here is maintained: it is derived on demand from the code as it stands, and a
region whose source has moved is re-derived before it is answered about.

What this is not: a description of the system kept beside it. The index is a
cache with an invalidation rule, never a corpus with a publication date.
"""
import ast
import json
import pathlib
import re
import subprocess
import sys
from collections import defaultdict

# What is looked at is where this is run, not where this file happens to live.
# The first version took its own location, which meant a tool for looking at
# estates could only ever look at the one it was kept in.
ROOT = pathlib.Path.cwd().resolve()
SELF = pathlib.Path(__file__).resolve().parent.parent

# How far a statement is to be trusted, and why.
DERIVED_TOTAL = ("derived", "high")       # the parser saw it and could not be wrong
DERIVED_PARTIAL = ("derived", "medium")   # the parser saw it, within known blind spots
MATCHED = ("matched", "medium")           # resolved by name, which can collide
MATCHED_WEAK = ("matched", "low")         # resolved by a name several things answer to


def repos(where=None):
    """Every repository in the estate. Here, one — and the seam is named so that
    the day there are hundreds, nothing else changes."""
    return [where or ROOT]


def head(repo):
    r = subprocess.run(["git", "-C", str(repo), "rev-parse", "--short", "HEAD"],
                       capture_output=True, text=True)
    return r.stdout.strip() or "unversioned"


# What is not the estate. A boundary nobody draws is a boundary that includes
# whatever happens to be on disk — the first run of this tool answered about
# another project's tests, and the second about copies of its own source held
# in changes still in flight.
OUTSIDE = {".git", "__pycache__", "node_modules", ".claude", "_bmad", "_bmad-output",
           ".venv", "venv", "dist", "build", "changes"}


def sources(repo):
    for p in sorted(repo.rglob("*.py")):
        if any(part in OUTSIDE or part.startswith("_bmad") for part in p.parts):
            continue
        yield p


def index(repo):
    """Symbols, the calls between them, and what could not be resolved.

    A definition is derived: the parser saw it. A call resolved to a unique name
    is *matched*, not derived — a name can collide, and saying so is the whole
    point of carrying confidence. A call through an attribute or a variable is
    resolved by nothing, and is counted rather than quietly dropped.
    """
    defines, calls, unresolved = defaultdict(list), [], []
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        try:
            tree = ast.parse(path.read_text())
        except SyntaxError:
            unresolved.append((rel, "file does not parse"))
            continue
        imported = set()
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                mod = getattr(node, "module", None) or ""
                for a in node.names:
                    imported.add(a.name)
                    if mod:
                        imported.add(mod.split(".")[-1])
        for node in ast.walk(tree):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                defines[node.name].append(rel)
            elif isinstance(node, ast.Call):
                f = node.func
                if isinstance(f, ast.Name):
                    calls.append((rel, f.id, "name"))
                elif isinstance(f, ast.Attribute):
                    # An attribute call names a method on something whose type this
                    # index does not know. Treating it as resolved was the first
                    # version's mistake: it is a blind spot, and saying so is the
                    # only honest thing to do with it.
                    unresolved.append((rel, "a call through an attribute, whose "
                                            "receiver this index cannot type"))
                else:
                    unresolved.append((rel, "a call through something with no name"))
        for name in imported:
            calls.append((rel, name, "import"))
    return defines, calls, unresolved


def named_by(paths, repo=None):
    """Files that name these paths as text rather than calling into them.

    The strongest dependency in this repository is invisible to a parser: the
    arbiters run the gate as a process, addressing it by a path in a string.
    A code graph sees nothing there. Finding it needs a different kind of look,
    and what it finds is weaker — a mention is not a call — so it is labelled
    as what it is rather than folded in with the rest.
    """
    repo = repo or ROOT
    wanted = {pathlib.Path(p).as_posix() for p in paths}
    stems = {pathlib.Path(p).name for p in wanted}
    out = []
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        if rel in wanted:
            continue
        text = path.read_text()
        hit = sorted({w for w in wanted if w in text} | {t for t in stems if t in text})
        if hit:
            out.append({"file": rel, "through": hit, "provenance": "matched",
                        "confidence": "low", "how": "named as text, not called"})
    return out


def importers(repo=None):
    """Which file imports which, as a map. Derived: an import is written down."""
    repo = repo or ROOT
    by_module = {}
    for path in sources(repo):
        by_module[path.stem] = path.relative_to(repo).as_posix()
    edges = defaultdict(set)
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        try:
            tree = ast.parse(path.read_text())
        except SyntaxError:
            continue
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                mod = getattr(node, "module", None) or ""
                names = [mod.split(".")[-1]] if mod else [a.name.split(".")[0]
                                                          for a in node.names]
                for n in names:
                    if n in by_module and by_module[n] != rel:
                        edges[by_module[n]].add(rel)
    return edges


def transitive(seeds, repo=None):
    """Reach is not one hop. Reporting only the files that touch a change
    directly understates the blast radius, and understating it is the dangerous
    direction: what is not reported is what nobody re-tests."""
    edges = importers(repo)
    seen, frontier, hops = set(seeds), list(seeds), {s: 0 for s in seeds}
    while frontier:
        cur = frontier.pop()
        for nxt in edges.get(cur, ()):
            if nxt not in seen:
                seen.add(nxt)
                hops[nxt] = hops[cur] + 1
                frontier.append(nxt)
    return {f: h for f, h in hops.items() if f not in seeds}


def affects(paths, repo=None):
    """What a change to these paths reaches, and how far to trust each answer."""
    repo = repo or ROOT
    defines, calls, _ = index(repo)
    wanted = {pathlib.Path(p).as_posix() for p in paths}
    changed_symbols = {s for s, where in defines.items()
                       if any(w in wanted or w.startswith(tuple(wanted)) for w in where)}
    reached = defaultdict(list)
    for rel, name, kind in calls:
        if name not in changed_symbols or rel in wanted:
            continue
        where = defines.get(name, [])
        if len(where) > 1:
            # A name several things answer to resolves nothing. Counting it as
            # reach is how the first version turned a change to one file into
            # fifty-eight, every one of them a coincidence of naming.
            continue
        reached[rel].append((name, DERIVED_PARTIAL if kind == "import" else MATCHED))
    out = []
    for rel in sorted(reached):
        names = sorted({n for n, _ in reached[rel]})
        prov, conf = min((p for _, p in reached[rel]), key=lambda p: p[1])
        out.append({"file": rel, "through": names, "provenance": prov, "confidence": conf})
    return out


def observability(path, repo=None):
    """How well behaviour in a region can be pinned down — as far as structure
    can say, which is not far.

    A test that names a symbol defined here has *claimed* the region, and a
    claim is not an observation. What settles it is running something and
    seeing which lines execute, which is `observe` below. This half proposes;
    that half confirms.
    """
    repo = repo or ROOT
    defines, calls, _ = index(repo)
    here = {s for s, where in defines.items()
            if any(w.startswith(pathlib.Path(path).as_posix()) for w in where)}
    watchers = sorted({rel for rel, name, _ in calls
                       if name in here and "/tests/" in rel})
    if not here:
        return {"region": path, "verdict": "unknown",
                "why": "nothing is defined here that this index can see",
                "provenance": "derived", "confidence": "low"}
    named = {s for s in here if any(rel for rel, n, _ in calls
                                    if n == s and "/tests/" in rel)}
    share = len(named) / len(here)
    verdict = ("claimed" if share > 0.66 else
               "partly claimed" if named else "unclaimed")
    return {"region": path, "symbols": len(here), "named by a test": len(named),
            "watched by": watchers, "verdict": verdict,
            "means": {"claimed": "structure suggests it can be observed; run it to find out",
                      "partly claimed": "some of it is spoken for and the rest is not",
                      "unclaimed": "nothing names it, so a change here has nothing to be "
                                   "judged against and needs characterising first"}[verdict],
            "provenance": "matched", "confidence": "low",
            "caveat": "a test naming a symbol is not a test exercising it"}


def observe(path, command, repo=None):
    """Run something and see which lines in the region actually executed.

    This is the half that settles it. A region nothing executes cannot be
    changed under any class that rests on comparing behaviour, and knowing
    that before the work starts is the point of asking.
    """
    import trace as tracemod
    repo = repo or ROOT
    target = (repo / path).resolve()
    lines = set()
    if target.is_file():
        src = target.read_text().splitlines()
        for i, ln in enumerate(src, start=1):
            t = ln.strip()
            if t and not t.startswith("#") and not t.startswith('"""'):
                lines.add(i)
    tracer = tracemod.Trace(count=1, trace=0)
    argv, old = list(command), sys.argv
    try:
        sys.argv = argv
        tracer.run(compile(pathlib.Path(argv[0]).read_text(), argv[0], "exec"))
    except SystemExit:
        pass
    finally:
        sys.argv = old
    executed = {ln for (f, ln), n in tracer.results().counts.items()
                if pathlib.Path(f).resolve() == target and n}
    if not lines:
        return {"region": path, "verdict": "unknown",
                "why": "nothing here that could execute"}
    share = len(executed) / len(lines)
    return {"region": path, "lines": len(lines), "executed": len(executed),
            "share": round(share, 2),
            "verdict": "observed" if share > 0.66 else
                       "partly observed" if executed else "unobserved",
            "by": " ".join(command),
            "provenance": "derived", "confidence": "high",
            "caveat": "what one run reached, not what could be reached"}


def freshness():
    return [{"repo": r.name, "derived from": head(r), "files": sum(1 for _ in sources(r))}
            for r in repos()]


def unknown(repo=None):
    """What the index could not resolve. Named, because a blind spot nobody
    states is indistinguishable from ground that has been covered."""
    repo = repo or ROOT
    _, _, un = index(repo)
    counted = defaultdict(int)
    for rel, why in un:
        counted[why] += 1
    return [{"why": w, "occurrences": n} for w, n in sorted(counted.items())]


# ---------------------------------------------------------------- contracts

# A contract is what one repository offers and another consumes: a route, an
# event, a queue. Finding both ends is the part nobody ships, and it is the
# reason a change in one repository can break another with nothing in either
# saying so.
#
# The extraction below is deliberately small. Recognising routes across every
# web framework is a solved problem taken off the shelf; what is built here is
# the *join*, and it treats extraction as an input it can be given rather than
# as work it must do.

DECLARES = [
    (r'@\w+\.(?:route|get|post|put|patch|delete)\(\s*["\']([^"\']+)["\']', "http"),
    (r'\bpublish(?:es)?\(\s*["\']([^"\']+)["\']', "event"),
]
USES = [
    (r'(?<!@)\b\w*\.?(?:get|post|put|patch|delete|request)\(\s*["\']([^"\']+)["\']', "http"),
    (r'\bsubscribe(?:s)?\(\s*["\']([^"\']+)["\']', "event"),
]

URL = re.compile(r'^(?:[a-z]+:)?//(?P<host>[^/]+)(?P<path>/.*)$')


def normalise(kind, key):
    """A consumer writes a URL and a producer writes a path. Joining them means
    saying so: the host is not part of the contract, it is a hint about who
    offers it — and a useful one, because when it agrees with the repository
    the key matched, two independent things point the same way."""
    if kind != "http":
        return key, None
    m = URL.match(key)
    if m:
        return m.group("path"), m.group("host")
    return key, None


def facts(repo):
    """What this repository offers, and what it consumes from elsewhere."""
    declares, uses = [], []
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        text = path.read_text()
        decorated = {ln.strip() for ln in text.splitlines() if ln.lstrip().startswith("@")}
        for pattern, kind in DECLARES:
            for key in re.findall(pattern, text):
                k, _ = normalise(kind, key)
                declares.append({"kind": kind, "key": k, "file": rel})
        for line in text.splitlines():
            if line.strip() in decorated:
                continue
            for pattern, kind in USES:
                for key in re.findall(pattern, line):
                    k, host = normalise(kind, key)
                    uses.append({"kind": kind, "key": k, "file": rel, "host": host})
    return {"repo": repo.name, "declares": declares, "uses": uses}


def observed(path):
    """Runtime, if anyone is collecting it: lines of `consumer kind key`.

    Static analysis proposes an edge; telemetry confirms it. A proposal nobody
    has ever seen exercised is a guess, and a call nobody could have predicted
    is a gap in the estate rather than an absence of one.
    """
    if not path:
        return set()
    out = set()
    for line in pathlib.Path(path).read_text().splitlines():
        parts = line.split()
        if len(parts) >= 3 and not line.startswith("#"):
            out.add((parts[0], parts[1], parts[2]))
    return out


def contracts(where, telemetry=None):
    all_facts = [facts(r) for r in sorted(pathlib.Path(where).iterdir()) if r.is_dir()]
    seen = observed(telemetry)
    offered = {}
    for f in all_facts:
        for d in f["declares"]:
            offered.setdefault((d["kind"], d["key"]), []).append((f["repo"], d["file"]))
    edges, unconsumed, unmatched = [], [], []
    consumed = set()
    for f in all_facts:
        for u in f["uses"]:
            k = (u["kind"], u["key"])
            producers = [p for p in offered.get(k, []) if p[0] != f["repo"]]
            if not producers:
                unmatched.append({"consumer": f["repo"], "file": u["file"], **u})
                continue
            consumed.add(k)
            confirmed = (f["repo"], u["kind"], u["key"]) in seen
            agrees = u.get("host") and u["host"] == producers[0][0]
            if confirmed:
                conf, how = "high", "seen in traffic"
            elif len(producers) > 1:
                conf, how = "low", "several repositories offer this key"
            elif agrees:
                conf, how = "medium", "the key matches and the address names the same repository"
            else:
                conf, how = "medium", "one repository offers this key"
            edges.append({
                "from": f["repo"], "to": producers[0][0], "kind": u["kind"], "key": u["key"],
                "provenance": "observed" if confirmed else "matched",
                "confidence": conf, "how": how})
    for k, where_offered in offered.items():
        if k not in consumed:
            unconsumed.append({"kind": k[0], "key": k[1],
                               "offered by": where_offered[0][0],
                               "file": where_offered[0][1]})
    surprises = [s for s in seen
                 if not any(e["from"] == s[0] and e["kind"] == s[1] and e["key"] == s[2]
                            for e in edges)]
    return edges, unconsumed, unmatched, surprises


# ------------------------------------------------------------- in flight

# An estate described only by what has landed describes the past. Two units can
# each be sound against the code as it stands and unsound against each other,
# and the moment to find that out is before either is started. Which means the
# model must carry work that has not landed yet.
#
# This also removes a record: a conflict was written into the admission artefact
# and nothing could check it. Now it is derived, and the writing down is a
# report rather than a claim.


def unfinished(where):
    """Every unit that has been planned and has not entered, with what it holds."""
    out = []
    base = pathlib.Path(where)
    for folder in sorted(base.iterdir()) if base.is_dir() else []:
        if not folder.is_dir() or not (folder / "02-plan.md").exists():
            continue
        landed = set()
        land = folder / "06-landing.md"
        if land.exists():
            for line in land.read_text().splitlines():
                cells = [c.strip() for c in line.strip("|").split("|")]
                if len(cells) > 5 and cells[1].startswith("WU-") and cells[4]:
                    landed.add(cells[1])
        for line in (folder / "02-plan.md").read_text().splitlines():
            cells = [c.strip() for c in line.strip("|").split("|")]
            if len(cells) > 4 and cells[0].startswith("WU-") and cells[0] not in landed:
                out.append({"change": folder.name, "unit": cells[0],
                            "holds": [p for p in cells[3].split() if p and p != "|"],
                            "provenance": "attested", "confidence": "low",
                            "how": "the plan says so; a scope is declared, not derived"})
    return out


def conflicts(where, paths):
    """Who already holds these paths, and on what footing.

    A declared scope is what someone wrote down. What that scope *reaches* is
    computed. An intersection on the second is worth more than on the first,
    and saying which is the difference between a warning and a fact.
    """
    wanted = {pathlib.Path(p).as_posix() for p in paths}
    reached = {r["file"] for r in affects(sorted(wanted))} | {
        r["file"] for r in named_by(sorted(wanted))}
    out = []
    for unit in unfinished(where):
        held = set(unit["holds"])
        direct = {h for h in held if any(h == w or w.startswith(h) or h.startswith(w)
                                         for w in wanted)}
        indirect = {h for h in held if any(f == h or f.startswith(h) for f in reached)}
        if direct:
            out.append({**unit, "on": sorted(direct), "confidence": "low",
                        "how": "both declare the same region, and a declaration is "
                               "what someone wrote down"})
        elif indirect:
            out.append({**unit, "on": sorted(indirect), "confidence": "medium",
                        "provenance": "matched",
                        "how": "what this change reaches runs into what that unit holds"})
    return out


def main(argv):
    if len(argv) < 2:
        print(__doc__.strip())
        return 2
    cmd, args = argv[1], argv[2:]
    if cmd == "affects" and args:
        out = affects(args)
        for row in out:
            print(f"  {row['confidence']:>6}  {row['file']}  through {', '.join(row['through'])}")
        named = named_by(args)
        for row in named:
            print(f"  {row['confidence']:>6}  {row['file']}  {row['how']}")
        direct = {r["file"] for r in out} | {r["file"] for r in named}
        far = transitive(direct | {pathlib.Path(a).as_posix() for a in args})
        for f, hops in sorted(far.items(), key=lambda kv: (kv[1], kv[0])):
            print(f"     low  {f}  through {hops} import(s)")
        blind = sum(r["occurrences"] for r in unknown())
        print(f"  {len(out)} reached through code, {len(named)} that only name it as "
              f"text, {len(far)} further through imports.")
        print(f"  {blind} call(s) this index cannot resolve at all, so neither figure "
              f"is a floor or a ceiling — it is what one parser could see.")
        return 0
    if cmd == "observe" and len(args) >= 2:
        print(json.dumps(observe(args[0], args[1:]), indent=2))
        return 0
    if cmd == "observability" and args:
        r = observability(args[0])
        print(json.dumps(r, indent=2))
        return 0
    if cmd == "contracts" and args:
        tel = args[1] if len(args) > 1 else None
        edges, unconsumed, unmatched, surprises = contracts(args[0], tel)
        for e in edges:
            print(f"  {e['confidence']:>6}  {e['from']} → {e['to']}  "
                  f"{e['kind']} {e['key']}  ({e['how']})")
        for u in unmatched:
            print(f"     n/a  {u['consumer']} → ?  {u['kind']} {u['key']}  "
                  f"nothing in the estate offers this, so it is outside it or missing from it")
        for u in unconsumed:
            print(f"     n/a  {u['offered by']} offers {u['kind']} {u['key']} and nothing "
                  f"in the estate consumes it")
        for s_ in surprises:
            print(f"    high  {s_[0]} → ?  {s_[1]} {s_[2]}  seen in traffic and predicted "
                  f"by nothing — the estate does not know about one side")
        print(f"  {len(edges)} edge(s) joined, {len(unmatched)} consumer(s) pointing outside, "
              f"{len(unconsumed)} offer(s) nobody takes, {len(surprises)} surprise(s)")
        return 0
    if cmd == "inflight" and args:
        rows = unfinished(args[0])
        for r in rows:
            print(f"  {r['change']}/{r['unit']}  holds {', '.join(r['holds']) or '—'}")
        print(f"  {len(rows)} unit(s) planned and not entered")
        return 0
    if cmd == "conflicts" and len(args) >= 2:
        rows = conflicts(args[0], args[1:])
        for r in rows:
            print(f"  {r['confidence']:>6}  {r['change']}/{r['unit']}  on "
                  f"{', '.join(r['on'])}  ({r['how']})")
        print(f"  {len(rows)} unfinished unit(s) already hold this ground"
              if rows else "  nothing unfinished holds this ground")
        return 0
    if cmd == "freshness":
        for r in freshness():
            print(f"  {r['repo']}  derived from {r['derived from']}  {r['files']} files")
        return 0
    if cmd == "unknown":
        rows = unknown()
        for r in rows:
            print(f"  {r['occurrences']:>5}  {r['why']}")
        print(f"  {sum(r['occurrences'] for r in rows)} thing(s) this index cannot resolve")
        return 0
    print(__doc__.strip())
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))