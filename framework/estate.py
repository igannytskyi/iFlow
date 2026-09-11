#!/usr/bin/env python3
"""What the estate is, derived from it rather than described about it.

Usage: python3 framework/estate.py affects <path>...     what a change here reaches
       python3 framework/estate.py observability [<path>]  how well behaviour there is pinned
       python3 framework/estate.py freshness             what this was derived from, and when
       python3 framework/estate.py unknown               what it could not resolve
       python3 framework/estate.py contracts <dir>       what crosses between repositories
       python3 framework/estate.py observe <path> <cmd>   run it and see what actually ran
       python3 framework/estate.py inflight <dir>         what unfinished work already holds
       python3 framework/estate.py conflicts <dir> <path> who already holds these paths
       python3 framework/estate.py reachability <dir> <p>  who consumes this, and who cannot be reached

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
from collections import Counter, defaultdict

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


_TOUCHED = {}


def touched_map(repo):
    """When each file last moved, in one pass.

    Asking git once per file cost forty-five seconds on a repository of three
    thousand — two thousand nine hundred and thirty-two separate processes to
    answer one question. One walk of the history answers it for everything.
    """
    key = str(repo)
    if key in _TOUCHED:
        return _TOUCHED[key]
    r = subprocess.run(["git", "-C", str(repo), "log", "--format=@%h %cs",
                        "--name-only", "--no-merges"], capture_output=True, text=True)
    seen, commit, when = {}, "unversioned", ""
    for line in r.stdout.splitlines():
        if line.startswith("@"):
            parts = line[1:].split()
            commit, when = (parts + ["", ""])[:2]
        elif line.strip() and line not in seen:
            seen[line] = (commit, when)
    _TOUCHED[key] = seen
    return seen


def last_touched(repo, rel):
    """The commit that last moved this file. The invalidation key: a region is
    stale when what it was derived from is not what last touched it."""
    return touched_map(repo).get(rel, ("unversioned", ""))


CACHE = ".estate"


def cached(repo):
    p = repo / CACHE / "index.json"
    if not p.exists():
        return {}
    try:
        return json.loads(p.read_text())
    except json.JSONDecodeError:
        return {}


def refresh(repo=None):
    """Re-derive what moved and nothing else.

    This is what makes the index a cache rather than a corpus: it carries the
    commit each region was derived from, and a region whose source has not
    moved is not looked at again. A corpus would carry a date instead, and a
    date cannot tell you whether anything changed.
    """
    repo = repo or ROOT
    was, now, rederived = cached(repo), {}, []
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        commit, when = last_touched(repo, rel)
        if was.get(rel, {}).get("commit") == commit:
            now[rel] = was[rel]
            continue
        rederived.append(rel)
    index(repo)          # derives what moved and writes the cache back
    return rederived, sum(1 for _ in sources(repo))


# What is not the estate. A boundary nobody draws is a boundary that includes
# whatever happens to be on disk — the first run of this tool answered about
# another project's tests, and the second about copies of its own source held
# in changes still in flight.
OUTSIDE = {".git", "__pycache__", "node_modules", ".claude", "_bmad", "_bmad-output",
           ".venv", "venv", "dist", "build", "changes"}


# Files this index has no parser for. Counting them is not politeness: a zero
# over ground nobody looked at reads exactly like a zero over ground that was
# covered, and on a repository written in another language every answer here
# was a confident nothing.
UNREADABLE = {".ts", ".tsx", ".js", ".jsx", ".go", ".rs", ".java", ".kt", ".rb",
              ".php", ".cs", ".c", ".h", ".cpp", ".swift", ".scala", ".ex", ".exs"}


def unseen(repo=None):
    """What is here and cannot be read, by language."""
    repo = repo or ROOT
    counted = {}
    for p in repo.rglob("*"):
        if not p.is_file() or p.suffix not in UNREADABLE:
            continue
        if any(part in OUTSIDE or part.startswith("_bmad") for part in p.parts):
            continue
        counted[p.suffix] = counted.get(p.suffix, 0) + 1
    return counted


def coverage_note(repo=None):
    """One line, printed beside every answer, saying how much of the estate this
    answer is about."""
    repo = repo or ROOT
    read = sum(1 for _ in sources(repo))
    blind = unseen(repo)
    if not blind:
        return f"  this covers all {read} file(s) here"
    total = sum(blind.values())
    kinds = ", ".join(f"{n} {ext}" for ext, n in sorted(blind.items(), key=lambda kv: -kv[1])[:4])
    return (f"  this covers {read} file(s); {total} more are in languages this index "
            f"cannot read ({kinds}) and are not absent — they are unseen")


def sources(repo):
    for p in sorted(repo.rglob("*.py")):
        if any(part in OUTSIDE or part.startswith("_bmad") for part in p.parts):
            continue
        yield p


def index(repo, use_cache=True):
    """Symbols, the calls between them, and what could not be resolved.

    A definition is derived: the parser saw it. A call resolved to a unique name
    is *matched*, not derived — a name can collide, and saying so is the whole
    point of carrying confidence. A call through an attribute or a variable is
    resolved by nothing, and is counted rather than quietly dropped.
    """
    defines, calls, unresolved = defaultdict(list), [], []
    was = cached(repo) if use_cache else {}
    fresh = {}
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        commit, when = last_touched(repo, rel)
        keep = was.get(rel)
        if keep and keep.get("commit") == commit and "calls" in keep:
            # Derived once, and the source has not moved since. A cache that the
            # queries do not read is decoration: this one is read here.
            for name in keep["symbols"]:
                defines[name].append(rel)
            calls.extend((rel, n, k) for n, k in keep["calls"])
            unresolved.extend((rel, w) for w in keep["unresolved"])
            fresh[rel] = keep
            continue
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
                    # A method called on something whose type is unknown. Discarding
                    # these was worse than the over-claiming it replaced: on a real
                    # codebase they are the majority form, and throwing them away
                    # understated what a change reaches by most of it. They are kept
                    # and weighed instead — a weak edge reported is safer than a
                    # strong edge omitted, because what is not reported is what
                    # nobody re-tests.
                    calls.append((rel, f.attr, "attribute"))
                else:
                    unresolved.append((rel, "a call through something with no name"))
        for name in imported:
            calls.append((rel, name, "import"))
        fresh[rel] = {
            "commit": commit, "when": when,
            "symbols": sorted({n for n, w in defines.items() if rel in w}),
            "calls": [(n, k) for r, n, k in calls if r == rel],
            "unresolved": [w for r, w in unresolved if r == rel],
        }
    if use_cache and fresh:
        (repo / CACHE).mkdir(exist_ok=True)
        (repo / CACHE / "index.json").write_text(json.dumps(fresh, indent=1, sort_keys=True))
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
        if kind == "import":
            grade = DERIVED_PARTIAL
        elif kind == "attribute":
            grade = ("matched", "low")     # the receiver's type is unknown
        elif len(where) > 1:
            grade = MATCHED_WEAK           # several things answer to this name
        else:
            grade = MATCHED
        reached[rel].append((name, grade))
    rank = {"high": 0, "medium": 1, "low": 2}
    out = []
    for rel in sorted(reached):
        names = sorted({n for n, _ in reached[rel]})
        best = min((g for _, g in reached[rel]), key=lambda g: rank[g[1]])
        out.append({"file": rel, "through": names, "provenance": best[0],
                    "confidence": best[1]})
    return sorted(out, key=lambda r: (rank[r["confidence"]], r["file"]))


# What counts as a test. Looking for "/tests/" in the path is looking for one
# project's layout: a repository whose tests live at its root has none, and the
# answer comes back "nothing names this" when the truth is "I looked in the
# wrong place". A wrong shape of path is not evidence of an unwatched region.
#
# The opposite error costs more here. Reporting a region as watched when it is
# not tells a change it has something to be judged against when it has nothing,
# so a directory that ships as an importable package — a project's own testing
# library, not its tests — is not counted however it is named.
TEST_DIRS = {"tests", "spec", "specs", "__tests__"}
AMBIGUOUS_DIRS = {"test", "testing"}


def test_ground(region, repo=None):
    """Whether the region is itself where tests are kept.

    Asked of the whole estate, "nothing names this region" is true of almost
    every test fixture directory and says nothing anyone can act on: a change
    to a test is judged by the test it belongs to. Keeping the two populations
    apart is what leaves the answer readable — the question is about the code
    under test, and drowning it in four hundred fixture directories loses it.
    """
    return is_test(str(pathlib.PurePosixPath(region) / "x"), repo)


def is_test(rel, repo=None):
    parts = pathlib.PurePosixPath(rel).parts
    for i, d in enumerate(parts[:-1]):
        if d in TEST_DIRS:
            return True
        if d in AMBIGUOUS_DIRS and not (repo and
                (repo / pathlib.Path(*parts[:i + 1]) / "__init__.py").exists()):
            return True
    name = parts[-1]
    stem = name.split(".")[0]
    return stem.startswith("test_") or stem.endswith("_test") \
        or ".test." in name or ".spec." in name


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
                       if name in here and is_test(rel, repo)})
    if not here:
        blind = unseen(repo)
        return {"region": path, "verdict": "unknown",
                "why": ("nothing here is in a language this index can read; that is not "
                        f"an empty region, it is an unread one ({sum(blind.values())} "
                        f"file(s): {', '.join(sorted(blind))})") if blind else
                       "nothing is defined here that this index can see",
                "provenance": "derived", "confidence": "low"}
    named = {s for s in here if any(rel for rel, n, _ in calls
                                    if n == s and is_test(rel, repo))}
    share = len(named) / len(here)
    verdict = ("claimed" if share > 0.66 else
               "partly claimed" if named else "unclaimed")
    # 800 paths is a correct answer nobody reads. Name a few and say where the
    # rest are: which part of the estate watches this is the answerable question.
    shown = watchers[:12]
    where = Counter(str(pathlib.PurePosixPath(w).parent) for w in watchers)
    return {"region": path, "symbols": len(here), "named by a test": len(named),
            "watched by": shown + ([f"… and {len(watchers) - len(shown)} more, mostly in "
                                    + ", ".join(f"{d} {n}" for d, n in where.most_common(4))]
                                   if len(watchers) > len(shown) else []),
            "verdict": verdict,
            "means": {"claimed": "structure suggests it can be observed; run it to find out",
                      "partly claimed": "some of it is spoken for and the rest is not",
                      "unclaimed": "nothing names it, so a change here has nothing to be "
                                   "judged against and needs characterising first"}[verdict],
            "provenance": "matched", "confidence": "low",
            "caveat": "a test naming a symbol is not a test exercising it",
            # A verdict of unclaimed is a claim about the whole estate, and the
            # part of it this index cannot read may be exactly where the tests
            # are: on a twelve-service estate every test is in Go or C#. Saying
            # "nothing names this" without saying that is a confident zero.
            "read": coverage_note(repo).strip()}


def every_region(repo=None):
    """The same question asked of the whole estate at once.

    Asked one directory at a time, this answers "is this region watched", which
    is the question you ask when you already know where to look. The question
    before it is where in the estate nothing can judge a change, and that one is
    only answerable by asking everywhere and sorting by what it costs to be
    wrong there. A large unclaimed region is worse than a small one, so size
    orders the answer.

    A region here is a directory that directly holds code this index can read.
    Nesting is not summed: a parent that holds no files of its own is not a
    region, and one that does is answered for its own files only, because a
    verdict averaged over a subtree hides the part of it nothing watches.
    """
    repo = repo or ROOT
    defines, calls, _ = index(repo)
    where = defaultdict(set)                      # directory → symbols defined there
    for sym, files in defines.items():
        for f in files:
            where[str(pathlib.PurePosixPath(f).parent)].add(sym)
    named_anywhere = {n for rel, n, _ in calls if is_test(rel, repo)}
    rows = []
    for region, syms in where.items():
        named = syms & named_anywhere
        share = len(named) / len(syms)
        rows.append({"region": region, "symbols": len(syms), "named by a test": len(named),
                     "share": share, "is test ground": test_ground(region, repo),
                     "verdict": ("claimed" if share > 0.66 else
                                 "partly claimed" if named else "unclaimed")})
    order = {"unclaimed": 0, "partly claimed": 1, "claimed": 2}
    rows.sort(key=lambda r: (order[r["verdict"]], -r["symbols"]))
    return rows


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


def freshness(region=None, repo=None):
    """What each region was derived from, and whether that is still current.

    Not one answer for a repository: an index is never wholly fresh or wholly
    stale, and treating it as either is how a model comes to be trusted about
    ground that moved under it.
    """
    repo = repo or ROOT
    was = cached(repo)
    rows = []
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        if region and not rel.startswith(pathlib.Path(region).as_posix()):
            continue
        commit, when = last_touched(repo, rel)
        known = was.get(rel, {}).get("commit")
        rows.append({"region": rel, "derived from": known or "—", "last touched": commit,
                     "when": when,
                     "state": "fresh" if known == commit else
                              ("never derived" if known is None else "stale")})
    return rows


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
    # `get(` is not an HTTP verb. Reading it as one turned every environment
    # lookup and dictionary access in a real estate into a consumed contract —
    # PORT, timestamp, severity — and reported each as a dependency pointing
    # outside. A use is one only where what is passed looks like an address.
    (r'(?<!@)\b\w*\.?(?:get|post|put|patch|delete|request)\(\s*'
     r'["\']((?:[a-z]+:)?//[^"\']+|/[^"\']*)["\']', "http"),
    (r'\bsubscribe(?:s)?\(\s*["\']([^"\']+)["\']', "event"),
]

# Keys too generic to identify anything. A route of "/" is offered by most
# services that offer anything, and joining on it says only that both sides
# speak HTTP.
DEGENERATE = {"/", "", "/*", "/health", "/healthz", "/ping", "/metrics"}

# Ways one service reaches another that this index does not read. Naming them
# matters more than the edges it does find: an estate bound together by gRPC
# and reported as fifteen consumers pointing outside is not being described,
# it is being misdescribed.
OTHER_KINDS = [
    (r"\.proto\b", "protocol buffers"),
    (r"\b\w+ServiceStub\b|grpc\.", "gRPC"),
    (r"\b[A-Z_]+_SERVICE_ADDR\b", "service addresses from the environment"),
    (r"\bboto3\b|\bSQS\b|\bkafka\b|\bKafkaProducer\b", "queues and brokers"),
]


def other_kinds(where):
    """Contract kinds present in this estate that this index does not read."""
    found = {}
    base = pathlib.Path(where)
    for p in base.rglob("*"):
        if not p.is_file() or p.suffix in {".png", ".jpg", ".pdf", ".lock"}:
            continue
        if any(part in OUTSIDE or part.startswith("_bmad") for part in p.parts):
            continue
        try:
            text = p.read_text(errors="ignore")
        except OSError:
            continue
        for pattern, name in OTHER_KINDS:
            if re.search(pattern, text):
                found[name] = found.get(name, 0) + 1
    return found

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
                if k not in DEGENERATE:
                    declares.append({"kind": kind, "key": k, "file": rel})
        for line in text.splitlines():
            if line.strip() in decorated:
                continue
            for pattern, kind in USES:
                for key in re.findall(pattern, line):
                    k, host = normalise(kind, key)
                    if k not in DEGENERATE:
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


def reachability(where, region, telemetry=None):
    """Who consumes what this region offers, and which of them no change reaches.

    A consumer inside the estate can be changed with the offer. One outside it —
    an application already installed, another organization, a caller seen in
    traffic and belonging to no repository here — cannot. Knowing that before
    any work is done is what makes a contract change knowably unfinishable
    without a person, rather than a surprise at the end.
    """
    base = pathlib.Path(where)
    reps = [r for r in sorted(base.iterdir()) if r.is_dir()]
    known = {r.name for r in reps}
    edges, _, unmatched, surprises = contracts(where, telemetry)
    offers = []
    for r in reps:
        if region and not (r.name == region or str(r).endswith(region)):
            continue
        for d in facts(r)["declares"]:
            takers = [e["from"] for e in edges
                      if e["to"] == r.name and e["kind"] == d["kind"]
                      and e["key"] == d["key"]]
            outside = [s[0] for s in surprises
                       if s[1] == d["kind"] and s[2] == d["key"] and s[0] not in known]
            offers.append({"repo": r.name, **d, "taken by": takers,
                           "beyond reach": outside})
    return offers, unmatched


def main(argv):
    if len(argv) < 2:
        print(__doc__.strip())
        return 2
    cmd, args = argv[1], argv[2:]
    if cmd == "affects" and args:
        every = "--all" in args
        args = [a for a in args if a != "--all"]
        out = affects(args)
        # A list of two thousand files is not an area of effect. What a reader
        # needs is the shape: how much is firm, how much is a halo, and where it
        # concentrates. The full list is still there for anyone who wants it.
        strong = [r for r in out if r["confidence"] != "low"]
        shown = out if every else strong[:20]
        for row in shown:
            names = ", ".join(row["through"][:3])
            more = "" if len(row["through"]) <= 3 else f" and {len(row['through']) - 3} more"
            print(f"  {row['confidence']:>6}  {row['file']}  through {names}{more}")
        if not every and len(strong) > len(shown):
            print(f"  … and {len(strong) - len(shown)} more of the same strength "
                  f"(--all for every one)")
        if out:
            grades = {}
            for r in out:
                grades[r["confidence"]] = grades.get(r["confidence"], 0) + 1
            where = {}
            for r in out:
                top = r["file"].split("/")[0] + ("/" + r["file"].split("/")[1]
                                                 if "/" in r["file"][r["file"].find("/") + 1:]
                                                 else "")
                where[top] = where.get(top, 0) + 1
            spread = ", ".join(f"{k} {v}" for k, v in
                               sorted(where.items(), key=lambda kv: -kv[1])[:4])
            print(f"  shape: " + ", ".join(f"{n} {g}" for g, n in
                                           sorted(grades.items())) + f"; mostly in {spread}")
        named = named_by(args)
        for row in named:
            print(f"  {row['confidence']:>6}  {row['file']}  {row['how']}")
        direct = {r["file"] for r in out} | {r["file"] for r in named}
        far = transitive(direct | {pathlib.Path(a).as_posix() for a in args})
        ordered = sorted(far.items(), key=lambda kv: (kv[1], kv[0]))
        for f, hops in (ordered if every else ordered[:10]):
            print(f"     low  {f}  through {hops} import(s)")
        if not every and len(ordered) > 10:
            print(f"  … and {len(ordered) - 10} more through imports (--all for every one)")
        blind = sum(r["occurrences"] for r in unknown())
        print(f"  {len(out)} reached through code, {len(named)} that only name it as "
              f"text, {len(far)} further through imports.")
        print(f"  {blind} call(s) this index cannot resolve at all, so neither figure "
              f"is a floor or a ceiling — it is what one parser could see.")
        print(coverage_note())
        return 0
    if cmd == "observe" and len(args) >= 2:
        print(json.dumps(observe(args[0], args[1:]), indent=2))
        return 0
    if cmd == "observability":
        if args and args[0] != "--all":
            print(json.dumps(observability(args[0]), indent=2))
            return 0
        every = "--all" in args
        all_rows = every_region()
        rows = [r for r in all_rows if not r["is test ground"]]
        ground = [r for r in all_rows if r["is test ground"]]
        worst = [r for r in rows if r["verdict"] != "claimed"]
        for r in (rows if every else worst[:25]):
            print(f"  {r['verdict']:>14}  {r['region']}  "
                  f"{r['named by a test']}/{r['symbols']} symbol(s) named by a test")
        if not every and len(worst) > 25:
            print(f"  … and {len(worst) - 25} more that are not fully claimed "
                  f"(--all for every region)")
        tally = Counter(r["verdict"] for r in rows)
        syms = sum(r["symbols"] for r in rows)
        loose = sum(r["symbols"] for r in rows if r["verdict"] == "unclaimed")
        print(f"  {len(rows)} region(s): " + ", ".join(f"{tally[v]} {v}" for v in
              ("claimed", "partly claimed", "unclaimed") if tally[v]))
        if ground:
            print(f"  {len(ground)} further region(s) are themselves where tests are kept "
                  f"and are not counted — a change there is judged by the test it is part of")
        print(f"  {loose} of {syms} symbol(s) sit in regions nothing names, so a change "
              f"there has nothing to be judged against")
        print("  a test naming a symbol is not a test exercising it — this is what "
              "structure suggests, not what execution showed")
        print(coverage_note())
        return 0
    if cmd == "contracts" and args:
        tel = args[1] if len(args) > 1 else None
        edges, unconsumed, unmatched, surprises = contracts(args[0], tel)
        for e in edges:
            print(f"  {e['confidence']:>6}  {e['from']} → {e['to']}  "
                  f"{e['kind']} {e['key']}  ({e['how']})")
        blind_here = sum(unseen(pathlib.Path(args[0])).values())
        for u in unmatched:
            why = ("nothing the index can read offers this — it may be offered by one of "
                   f"the {blind_here} file(s) here it cannot read, or from outside the estate"
                   if blind_here else
                   "nothing in the estate offers this, so it is outside it or missing from it")
            print(f"     n/a  {u['consumer']} → ?  {u['kind']} {u['key']}  {why}")
        for u in unconsumed:
            print(f"     n/a  {u['offered by']} offers {u['kind']} {u['key']} and nothing "
                  f"in the estate consumes it")
        for s_ in surprises:
            print(f"    high  {s_[0]} → ?  {s_[1]} {s_[2]}  seen in traffic and predicted "
                  f"by nothing — the estate does not know about one side")
        print(f"  {len(edges)} edge(s) joined, {len(unmatched)} consumer(s) pointing outside, "
              f"{len(unconsumed)} offer(s) nobody takes, {len(surprises)} surprise(s)")
        others = other_kinds(args[0])
        if others:
            named = ", ".join(f"{k} ({n} file(s))" for k, n in
                              sorted(others.items(), key=lambda kv: -kv[1]))
            print(f"  and this estate is also bound together by {named}, which this index "
                  f"does not read — those services are not unconnected, they are unexamined")
        print(coverage_note(pathlib.Path(args[0])))
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
    if cmd == "refresh":
        rederived, total = refresh()
        for rel in rederived:
            print(f"  re-derived  {rel}")
        print(f"  {len(rederived)} of {total} region(s) had moved; the rest were not "
              f"looked at again")
        print(coverage_note())
        return 0
    if cmd == "reachability" and len(args) >= 2:
        tel = args[2] if len(args) > 2 else None
        offers, unmatched = reachability(args[0], args[1], tel)
        for o in offers:
            takers = ", ".join(o["taken by"]) or "nobody in the estate"
            print(f"  {o['repo']} offers {o['kind']} {o['key']} — taken by {takers}")
            for who in o["beyond reach"]:
                print(f"    beyond reach  {who}  belongs to no repository here, so no "
                      f"change reaches it")
        for u in unmatched:
            if u["consumer"] == args[1]:
                print(f"    beyond reach  {u['key']}  this region consumes something the "
                      f"estate does not offer")
        blocked = sum(len(o["beyond reach"]) for o in offers)
        print(f"  {len(offers)} offer(s); {blocked} consumer(s) no change can reach"
              if offers else "  this region offers nothing across a boundary")
        return 0
    if cmd == "freshness":
        rows = freshness(args[0] if args else None)
        for r in rows:
            if r["state"] != "fresh":
                print(f"  {r['state']:>13}  {r['region']}  last touched {r['last touched']} "
                      f"{r['when']}")
        fresh = sum(1 for r in rows if r["state"] == "fresh")
        print(f"  {fresh} of {len(rows)} region(s) current against what last touched them")
        print(coverage_note())
        return 0
    if cmd == "unknown":
        rows = unknown()
        for r in rows:
            print(f"  {r['occurrences']:>5}  {r['why']}")
        print(f"  {sum(r['occurrences'] for r in rows)} thing(s) this index cannot resolve "
              f"inside what it can read")
        print(coverage_note())
        return 0
    print(__doc__.strip())
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))