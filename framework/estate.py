#!/usr/bin/env python3
"""What the estate is, derived from it rather than described about it.

Usage: python3 framework/estate.py affects <path>...     what a change here reaches
       python3 framework/estate.py observability [<path>]  how well behaviour there is pinned
       python3 framework/estate.py freshness             what this was derived from, and when
       python3 framework/estate.py unknown               what it could not resolve
       python3 framework/estate.py readers               which languages are read here
       python3 framework/estate.py coverage              how much of this the index reaches
       python3 framework/estate.py context <symbol|path>  the code to change, and what it touches
       python3 framework/estate.py context --task "..."   where a task's words land, to choose from
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
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys
from collections import Counter, defaultdict

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import readers                                    # noqa: E402
from readers import python_ast                    # noqa: E402

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
_WHERE = {}


def history(repo, prefix=""):
    """One walk of one repository's history: path → the commit that last moved it.

    Asking git once per file cost forty-five seconds on a repository of three
    thousand — two thousand nine hundred and thirty-two separate processes to
    answer one question. One walk answers it for everything.
    """
    r = subprocess.run(["git", "-C", str(repo), "log", "--format=@%h %cs",
                        "--name-only", "--no-merges"], capture_output=True, text=True)
    seen, commit, when = {}, "unversioned", ""
    for line in r.stdout.splitlines():
        if line.startswith("@"):
            parts = line[1:].split()
            commit, when = (parts + ["", ""])[:2]
        elif line.strip() and prefix + line not in seen:
            seen[prefix + line] = (commit, when)
    return seen


def touched_map(repo):
    """When each file last moved — across every repository under here.

    An estate of nine services is nine repositories in one directory, and that
    directory is not itself a repository. Walking only its own history found
    none, gave every file the same non-answer, and the non-answer compared
    equal to itself: the index reported all of it current against a history it
    had never read, which is the corpus with a publication date this is meant
    not to be.
    """
    key = str(repo)
    if key in _TOUCHED:
        return _TOUCHED[key]
    seen = history(repo)
    if not seen:
        for child in sorted(repo.iterdir()) if repo.is_dir() else []:
            if child.is_dir() and (child / ".git").exists():
                seen.update(history(child, child.name + "/"))
    _TOUCHED[key] = seen
    return seen


def last_touched(repo, rel):
    """The commit that last moved this file. Where history covers it, this is
    both the invalidation key and what an admission record cites."""
    return touched_map(repo).get(rel, ("unversioned", ""))


def stamp(repo, rel):
    """What a region was derived from, in a form that changes when it does.

    The contents, always. A commit answers a different question — which change
    to cite for this region — and it answers the invalidation question wrongly
    while anyone is working: a file edited and not yet committed is still at
    the commit that last touched it, so the index went on serving line numbers
    for the file as it used to be. Every answer this thing gives is about a
    working tree, and a working tree is what it must be keyed on.
    """
    commit, when = last_touched(repo, rel)
    try:
        digest = hashlib.sha1((repo / rel).read_bytes()).hexdigest()[:12]
    except OSError:
        return "unversioned", ""
    return f"{commit}+{digest}" if commit != "unversioned" else "contents:" + digest, when


CACHE = ".estate"


def home(repo):
    """Where derived material for this repository is kept.

    By default beside the repository it belongs to, because that is where the
    thing it is derived from lives and where an adopter will look for it. An
    estate of many repositories usually wants them together instead — under the
    change the answers were taken for — and says so with IFLOW_ESTATE, which
    names one directory holding a cache per repository.
    """
    root = os.environ.get("IFLOW_ESTATE")
    if not root:
        return repo / CACHE
    base = pathlib.Path(root).expanduser().resolve() / "cache"
    here = pathlib.Path(os.environ.get("IFLOW_ESTATE_ROOT", "")).expanduser()
    try:
        name = repo.resolve().relative_to(here.resolve()).as_posix() if here else repo.name
    except ValueError:
        name = repo.name
    return base / (name or repo.name)


def cached(repo):
    p = home(repo) / "index.json"
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
        commit, when = stamp(repo, rel)
        if was.get(rel, {}).get("commit") == commit \
                and was.get(rel, {}).get("read by") == readers.derivation():
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
           ".venv", "venv", "dist", "build", "changes",
           # Libraries installed beside the code are not the code. The grammars
           # this index reads with are themselves written in the languages it
           # reads, and counting them made the method's own estate four times
           # its size, most of it somebody else's.
           "_lib", "vendor", "third_party", "site-packages"}


# Files this index has no parser for. Counting them is not politeness: a zero
# over ground nobody looked at reads exactly like a zero over ground that was
# covered, and on a repository written in another language every answer here
# was a confident nothing.
UNREADABLE = {".ts", ".tsx", ".js", ".jsx", ".go", ".rs", ".java", ".kt", ".rb",
              ".php", ".cs", ".c", ".h", ".cpp", ".swift", ".scala", ".ex", ".exs"}


def unseen(repo=None):
    """What is here and nothing installed can read, by language.

    Derived from the readers rather than declared: installing a grammar moves
    files out of this count and removing one moves them back. A list written
    here instead would go on calling a language unread after it became readable.
    """
    repo = repo or ROOT
    known = readers.by_extension()
    counted = {}
    for p in repo.rglob("*"):
        if not p.is_file() or p.suffix in known or p.suffix not in UNREADABLE:
            continue
        if outside(p, repo):
            continue
        counted[p.suffix] = counted.get(p.suffix, 0) + 1
    return counted


# Ground nobody wrote. A generated file is real code and is not the estate's
# code: nobody edits it, a change to it is a change to whatever produced it, and
# reading it costs more than everything hand-written put together — three
# generated parsers were twenty of the twenty-seven seconds one estate took.
# It is skipped and counted, never skipped and forgotten.
GENERATED_NAMES = (".pb.go", "_pb2.py", "_pb2_grpc.py", ".g.dart", ".generated.",
                   ".designer.cs", ".freezed.dart", "_generated.go", ".pb.cc",
                   ".pb.h", "parser.c")
GENERATED_BANNER = re.compile(
    r"(?i)(@generated|code generated by|autogenerated|auto-generated|"
    r"automatically generated|do not edit|generated by the protocol buffer)")


_GENERATED = {}


def generated(path):
    """Whether a file says something else wrote it.

    The name says so for the conventions a toolchain fixed; otherwise the file
    says so itself, in the first lines, where every generator writes it.
    """
    if str(path) in _GENERATED:
        return _GENERATED[str(path)]
    verdict = _generated(path)
    _GENERATED[str(path)] = verdict
    return verdict


def _generated(path):
    name = path.name
    if any(mark in name for mark in GENERATED_NAMES):
        return True
    try:
        with open(path, "r", errors="replace") as f:
            return bool(GENERATED_BANNER.search(f.read(600)))
    except OSError:
        return False


def outside(p, base):
    """Whether this file is outside what is being looked at.

    The names are matched against the path *inside* the estate, never the whole
    path: an estate kept under a directory called `changes` is not a folder of
    changes, and matching the ancestors made every file in it vanish — nought
    of nought regions, reported as an estate with nothing in it.
    """
    try:
        parts = p.relative_to(base).parts
    except ValueError:
        parts = p.parts
    return any(part in OUTSIDE or part.startswith("_bmad") for part in parts)


def machine_written(repo=None):
    """How many files here were written by something other than a person."""
    repo = repo or ROOT
    return sum(1 for p in repo.rglob("*")
               if p.is_file() and not outside(p, repo) and readers.readable(p)
               and generated(p))


def coverage_note(repo=None):
    """One line, printed beside every answer, saying how much of the estate this
    answer is about."""
    repo = repo or ROOT
    read = sum(1 for _ in sources(repo))
    blind = unseen(repo)
    made = machine_written(repo)
    tail = ("" if not made else
            f"; {made} more say they were generated and are not read — a change to "
            f"one of those is a change to whatever produced it")
    if not blind:
        return f"  this covers all {read} file(s) here{tail}"
    total = sum(blind.values())
    kinds = ", ".join(f"{n} {ext}" for ext, n in sorted(blind.items(), key=lambda kv: -kv[1])[:4])
    return (f"  this covers {read} file(s); {total} more are in languages this index "
            f"cannot read ({kinds}) and are not absent — they are unseen{tail}")


def sources(repo):
    """Every file some installed reader can read. Which files those are is not
    a constant here: it is whatever is installed beside the readers."""
    for p in sorted(repo.rglob("*")):
        if not p.is_file() or outside(p, repo) or not readers.readable(p):
            continue
        if generated(p):
            continue
        yield p


def unread_sources(repo):
    """Files in languages this index cannot parse, which is where most of an
    estate's offers are declared."""
    known = readers.by_extension()
    for p in sorted(repo.rglob("*")):
        if not p.is_file() or p.suffix in known or p.suffix not in UNREADABLE:
            continue
        if outside(p, repo):
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
    how = readers.derivation()
    was = cached(repo) if use_cache else {}
    fresh = {}
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        commit, when = stamp(repo, rel)
        keep = was.get(rel)
        if keep and keep.get("commit") == commit and "calls" in keep \
                and keep.get("read by") == how:
            # Derived once, and the source has not moved since. A cache that the
            # queries do not read is decoration: this one is read here.
            for name, _start, _end in keep["symbols"]:
                defines[name].append(rel)
            calls.extend((rel, n, k) for n, k, _line in keep["calls"])
            unresolved.extend((rel, w) for w in keep["unresolved"])
            fresh[rel] = keep
            continue
        try:
            text = path.read_text(errors="replace")
        except OSError:
            unresolved.append((rel, "file could not be read"))
            continue
        got, refs, could_not = readers.read(rel, text)
        for name, _start, _end in got:
            defines[name].append(rel)
        calls.extend((rel, n, k) for n, k, _line in refs)
        unresolved.extend((rel, w) for w in could_not)
        # What this file holds is what the reader just returned. Asking the
        # whole index for it instead — every symbol, every call, filtered by
        # this one file — cost a pass over the estate per file, which is
        # nothing on a hundred files and does not finish on twenty thousand.
        fresh[rel] = {
            "commit": commit, "when": when, "read by": how,
            "symbols": [list(g) for g in got],
            "calls": [list(c) for c in refs],
            "unresolved": list(could_not),
        }
    if use_cache and fresh:
        home(repo).mkdir(parents=True, exist_ok=True)
        (home(repo) / "index.json").write_text(json.dumps(fresh, indent=1, sort_keys=True))
    _WHERE[str(repo)] = fresh
    return defines, calls, unresolved


def positions(repo=None):
    """Where in each file the definitions and the calls sit.

    The index answers which files are involved; this answers which lines. An
    answer that names a file sends a reader to look for the thing; an answer
    that names the lines hands it over, and that difference is most of what a
    context is for.
    """
    repo = (repo or ROOT).resolve()
    if str(repo) not in _WHERE:
        index(repo)
    return _WHERE.get(str(repo), {})


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


def importers(repo=None, graded=False):
    """Which file imports which, as a map. Derived: an import is written down.

    Read from the index rather than parsed again here, which is what lets reach
    close over an estate written in several languages: an import is an import
    whoever read the file.
    """
    repo = repo or ROOT
    # A module is a file in some languages and a directory in others. Matching
    # only file names found none of Go's imports, where the thing imported is
    # the package and the package is the folder.
    by_module = defaultdict(set)
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        by_module[path.stem].add(rel)
        by_module[path.parent.name].add(rel)
        # And every tail of the path it sits on, so that an import naming a
        # place — a namespace, a package, a directory — reaches what is there
        # rather than everything of that name anywhere.
        parts = pathlib.PurePosixPath(rel).parts
        for i in range(len(parts) - 1):
            by_module["/".join(parts[i:-1])].add(rel)
            by_module["/".join(parts[i:-1] + (path.stem,))].add(rel)
    def resolve(name):
        """Everywhere a name could mean, longest tail first.

        `using eShop.ClientApp.Models.Catalog` names a namespace whose root is
        nowhere on disk — the directory is `src/ClientApp/Models/Catalog`. The
        longest suffix that matches is the most specific place it can mean, and
        the shorter ones are the places it might.
        """
        out = []
        if name in by_module:
            out.append((name, by_module[name]))
        parts = name.split("/")
        for i in range(1, len(parts)):
            tail = "/".join(parts[i:])
            if tail in by_module:
                out.append((tail, by_module[tail]))
        return out

    # An import statement is one destination and several guesses at it. Both go
    # into the graph: the most specific tail that names somewhere here is the
    # firm edge, the coarser ones are weak, and which of them an answer uses is
    # the answer's business. Dropping the weak ones here — as this did for one
    # release — narrows the graph itself to make one query tidier, and a graph
    # narrowed to suit a query is no longer a graph of the estate.
    edges, firm = defaultdict(set), defaultdict(set)
    index(repo)
    for rel, spot in positions(repo).items():
        by_line = defaultdict(list)
        for name, kind, line in spot.get("calls", []):
            if kind == "import":
                by_line[line].append(name)
        for line, names in by_line.items():
            best = None
            for name in sorted(names, key=lambda n: -n.count("/")):
                for matched, targets in resolve(name):
                    for target in targets:
                        if target == rel:
                            continue
                        edges[target].add(rel)
                        if best is None or matched == best:
                            best = matched
                            firm[target].add(rel)
    return (edges, firm) if graded else edges


def transitive(seeds, repo=None, edges=None):
    """Reach is not one hop. Reporting only the files that touch a change
    directly understates the blast radius, and understating it is the dangerous
    direction: what is not reported is what nobody re-tests.

    Asked over the firm half of the import graph instead, the same walk answers
    a narrower question — where a caller certainly reaches — which is what
    telling two same-named definitions apart needs.
    """
    edges = importers(repo) if edges is None else edges
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
    # Two files can share a method name and have nothing to do with each other,
    # and on a real estate most pairs are exactly that: between half and two
    # thirds of everything a name match reports joins files with no import
    # between them, directly or through any chain. Code that is never imported
    # is code that cannot be called, so the import graph is what separates a
    # dependency from a coincidence of vocabulary. It does not delete the
    # coincidences — something loaded by a framework is reached by no import
    # either — it says which is which.
    linked = set(transitive(set(wanted), repo))
    reached = defaultdict(list)
    for rel, name, kind in calls:
        if name not in changed_symbols or rel in wanted:
            continue
        where = defines.get(name, [])
        here = rel in linked
        if kind == "import":
            grade = DERIVED_PARTIAL
        elif kind == "typed":
            # The file said what the receiver is — an assignment from a
            # constructor, an annotation, `self` inside a class. That is read
            # off this file rather than guessed across the estate, so it is the
            # firmest thing here short of an import.
            grade = ("derived", "high" if here and len(where) == 1 else "medium")
        elif kind == "attribute":
            grade = ("matched", "low")     # the receiver's type is unknown
        elif len(where) > 1:
            grade = MATCHED_WEAK           # several things answer to this name
        elif here:
            grade = MATCHED
        else:
            grade = MATCHED_WEAK           # nothing imports it: the name may be all
        reached[rel].append((name, grade))
    rank = {"high": 0, "medium": 1, "low": 2}
    out = []
    for rel in sorted(reached):
        names = sorted({n for n, _ in reached[rel]})
        best = min((g for _, g in reached[rel]), key=lambda g: rank[g[1]])
        out.append({"file": rel, "through": names, "provenance": best[0],
                    "confidence": best[1], "imports it": rel in linked})
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


# Ground that shows what code looks like rather than running it. A getting
# started page carries a Go snippet with `r.GET("/foo")` in it, and read as a
# declaration it makes the service offer `/foo` to anyone who asks — and
# something in the estate always asks. What is written to be read is not what
# is deployed.
# Fixtures and mocks were here once and did not belong: a fake of another
# service is not an illustration of one, it is the plainest evidence that this
# repository calls it. Dropping them lost every stand-in a real estate keeps.
ILLUSTRATION = {"docs", "doc", "examples", "example", "samples", "sample",
                "snippets", "gettingstarteddocs", "templates"}


def illustration(rel):
    return any(part.lower() in ILLUSTRATION
               for part in pathlib.PurePosixPath(rel).parts[:-1])


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


ENTRY_NAMES = {"main", "__main__", "index", "app", "cli", "server", "program",
               "setup", "manage", "conftest"}

# Directories a framework loads from by name. Nothing imports a Rails model or
# a migration: the framework finds the file because of where it sits. That is
# not the same as nothing reaching it, and it is not the same as something
# reaching it either — it is a third thing, and it is named as one.
LOADED_BY_NAME = {"migrate", "migrations", "initializers", "models", "controllers",
                  "jobs", "mailers", "channels", "helpers", "policies", "tasks",
                  "middleware", "plugins", "hooks", "seeds", "steps"}


def named_in_text(repo, targets):
    """Which files name these ones as text rather than reaching them in code.

    A Sphinx configuration says `pygments_style = "flask_theme_support.FlaskyStyle"`
    and a framework loads what that string names. The edge is real, the reader
    is a regular expression, and both facts are reported.
    """
    if not targets:
        return {}
    want = {t: re.compile(r"\b" + re.escape(pathlib.PurePosixPath(t).stem) + r"\b")
            for t in targets if len(pathlib.PurePosixPath(t).stem) >= 5}
    found = defaultdict(set)
    for path in sources(repo):
        rel = path.relative_to(repo).as_posix()
        try:
            text = path.read_text(errors="ignore")
        except OSError:
            continue
        for target, pattern in want.items():
            if target != rel and pattern.search(text):
                found[target].add(rel)
    return found


# ------------------------------------------------------------------ context
#
# Every other answer here is shaped for a decision: what may be admitted, what
# is judged by nothing, what crosses a boundary. This one is shaped for work.
# An agent asked to fix something needs the code, not a list of paths to go and
# read — the round trips it saves are the whole point — and it needs to be told
# which part of what it was handed is firm.
#
# What that changes about the answer: the source is verbatim and carries its
# line numbers, the firm ground comes first, and everything is cut to a budget
# that is stated rather than silently exceeded.


def where_defined(name, repo=None):
    """Every place a name is defined, with the lines it spans.

    Places, not sightings. A Java class and its two constructors carry the same
    name and sit inside each other, and the grammar's own query finds the class
    a second time at the line it is named on: four entries, one definition, the
    class quoted four times. What is inside something already listed is part of
    it.
    """
    repo = repo or ROOT
    out = []
    for rel, spot in positions(repo).items():
        for symbol, start, end in spot.get("symbols", []):
            if symbol == name:
                out.append((rel, start, end))
    kept = []
    for rel, start, end in sorted(out, key=lambda t: (t[0], t[1], -t[2])):
        if any(r == rel and s <= start and end <= e for r, s, e in kept):
            continue
        kept.append((rel, start, end))
    return kept


def excerpt(repo, rel, start, end, limit=120):
    """The lines themselves, numbered as they are in the file."""
    try:
        lines = (repo / rel).read_text(errors="replace").splitlines()
    except OSError:
        return []
    start = max(1, start)
    end = min(len(lines), end)
    if end - start + 1 > limit:
        end = start + limit - 1
    return [(n, lines[n - 1]) for n in range(start, end + 1)]


def call_sites(name, repo=None):
    """Who calls this name, on which line, and how firmly it is known."""
    repo = repo or ROOT
    defines, _, _ = index(repo)
    where = defines.get(name, [])
    out = []
    for rel, spot in positions(repo).items():
        for called, kind, line in spot.get("calls", []):
            if called != name:
                continue
            if rel in where:
                # A call in the file that defines it resolves to nothing else.
                # Left out, the answer says nobody calls what the file calls
                # eight times on the next page.
                #
                # Except a typed one: `self.register(...)` inside the class is
                # read as reaching the class, which is right for a blast radius
                # and wrong to quote — what is on that line is a call to a
                # method, and showing it as a use of the class misleads whoever
                # reads it.
                if kind == "typed":
                    continue
                out.append({"file": rel, "line": line, "kind": kind,
                            "confidence": "high", "here": True})
                continue
            firm = ("high" if kind == "typed" and len(where) == 1 else
                    "medium" if kind == "import" or (kind == "name" and len(where) == 1)
                    else "low")
            out.append({"file": rel, "line": line, "kind": kind, "confidence": firm,
                        "here": False})
    rank = {"high": 0, "medium": 1, "low": 2}
    # One line is one place to look, however many times the name appears on it.
    best = {}
    for r in out:
        key = (r["file"], r["line"])
        if key not in best or rank[r["confidence"]] < rank[best[key]["confidence"]]:
            best[key] = r
    return sorted(best.values(), key=lambda r: (rank[r["confidence"]], r["file"], r["line"]))


def callees(rel, span=None, repo=None):
    """What this calls that is defined somewhere else here.

    Bounded by the lines asked about. Asked for a file, a file's worth; asked
    for one function, that function's — otherwise a question about six lines
    is answered with everything the module happens to touch.
    """
    repo = repo or ROOT
    defines, _, _ = index(repo)
    # Which files this one can reach at all. A method name matched in a file
    # nothing here imports is a coincidence of vocabulary, and quoting it sends
    # a reader somewhere the code never goes.
    edges = defaultdict(set)
    for target, who in importers(repo).items():
        for w in who:
            edges[w].add(target)
    seen, frontier = {rel}, [rel]
    while frontier:
        for nxt in edges.get(frontier.pop(), ()):
            if nxt not in seen:
                seen.add(nxt)
                frontier.append(nxt)
    out = {}
    for called, kind, line in positions(repo).get(rel, {}).get("calls", []):
        if span and not (span[0] <= line <= span[1]):
            continue
        for target in defines.get(called, ()):
            if target == rel:
                continue
            firm = ("high" if kind == "typed" and len(defines[called]) == 1 else
                    "medium" if kind == "import" or (kind == "name" and
                                                     len(defines[called]) == 1) else "low")
            if target not in seen:
                firm = "low"
            best = out.get((target, called))
            rank = {"high": 0, "medium": 1, "low": 2}
            if best is None or rank[firm] < rank[best["confidence"]]:
                out[(target, called)] = {"file": target, "name": called, "line": line,
                                         "confidence": firm, "reachable": target in seen}
    rank = {"high": 0, "medium": 1, "low": 2}
    return sorted(out.values(), key=lambda r: (rank[r["confidence"]], r["file"]))


def judged_by(rel, name=None, repo=None):
    """What would judge a change here: the tests that name what it defines."""
    repo = repo or ROOT
    spots = positions(repo)
    mine = {n for n, _s, _e in spots.get(rel, {}).get("symbols", [])}
    if name:
        mine &= {name}
    out = defaultdict(set)
    for other, spot in spots.items():
        if other == rel or not is_test(other, repo):
            continue
        for called, _kind, line in spot.get("calls", []):
            if called in mine:
                out[other].add(called)
    return {k: sorted(v) for k, v in sorted(out.items())}


# A task is not a query. What a person says about a bug names the behaviour —
# "the cart ignores the currency" — and the code may call none of it that. So
# what is done here is deliberately shallow and deliberately loud: the words of
# the task are matched against what the estate defines, the matches are offered
# as a place to start, and every word that placed nothing is named. A word that
# names nothing means one of two things, and both are worth saying: the code
# calls it something else, or the ground it names is not read here.
#
# Understanding the task is not this thing's job. It has no model and cannot
# have one; whoever asked has both. What it owes is a faithful expansion of
# whatever it is pointed at, and an honest account of what it could not place.

STOPWORDS = {
    "the", "a", "an", "and", "or", "not", "but", "if", "then", "when", "where",
    "this", "that", "these", "those", "it", "its", "is", "are", "was", "were",
    "be", "been", "being", "to", "of", "in", "on", "for", "with", "from", "by",
    "as", "at", "into", "about", "after", "before", "should", "must", "can",
    "will", "would", "we", "i", "you", "they", "he", "she", "our", "my",
    "fix", "bug", "issue", "problem", "add", "make", "change", "update",
    "support", "feature", "need", "needs", "want", "please", "instead",
    "does", "do", "did", "doing", "have", "has", "had", "get", "gets", "set",
    "new", "old", "use", "used", "using", "so", "now", "also", "only", "some",
    "all", "any", "every", "there", "here", "out", "up", "down", "over",
}


def words_of(statement):
    """The words in a task that could name something in an estate.

    Anything quoted, anything shaped like a path or a dotted name, and every
    other word that is not grammar. Longer words first, because a long word
    that matches is worth more than a short one that matches often.
    """
    strong = set(re.findall(r'"([^"]+)"|`([^`]+)`', statement))
    strong = {a or b for a, b in strong}
    strong |= {m for m in re.findall(r"[A-Za-z_][\w./-]*[./][\w./-]+", statement)}
    plain = [w for w in re.findall(r"[A-Za-z_][A-Za-z0-9_]*", statement)
             if len(w) >= 3 and w.lower() not in STOPWORDS]
    ordered = sorted(strong, key=len, reverse=True) + \
        sorted({w for w in plain if w not in strong}, key=len, reverse=True)
    return ordered


def stem(word):
    """A word with its ending taken off, roughly.

    A task says `streamed` and the code says `stream_with_context`. This is not
    morphology, it is four suffixes, and what it finds is reported as a stem
    match rather than as a name — a weaker thing, said to be weaker.
    """
    low = word.lower()
    for suffix in ("ing", "ed", "es", "s"):
        if low.endswith(suffix) and len(low) - len(suffix) >= 3:
            return low[: -len(suffix)]
    return low


def split_name(name):
    """A symbol as the words it is made of: CamelCase, snake_case, kebab."""
    parts = re.findall(r"[A-Z]+(?![a-z])|[A-Z][a-z0-9]*|[a-z0-9]+", name)
    return {p.lower() for p in parts if len(p) >= 3}


def placed(statement, repo=None):
    """Where each word of a task lands, and which words land nowhere.

    Three ways to land, and each says which it was: the word is a symbol's
    name, it is one of the words a symbol's name is made of, or it is part of
    a path. Nothing here weighs one word against another by meaning — that is
    not derivable from text, and pretending otherwise would put a guess where
    the answer says "derived".
    """
    repo = repo or ROOT
    defines, _, _ = index(repo)
    spots = positions(repo)
    by_word = defaultdict(set)
    for name in defines:
        for part in split_name(name):
            by_word[part].add(name)
    found, missing = [], []
    for word in words_of(statement):
        low = word.lower()
        exact = [n for n in defines if n.lower() == low]
        partial = sorted(by_word.get(low, set()) - set(exact))
        inpath = sorted(rel for rel in spots if low in rel.lower())
        by_stem = sorted(by_word.get(stem(low), set())) if stem(low) != low else []
        if exact:
            found.append({"word": word, "how": "is the name of", "symbols": sorted(exact),
                          "files": [], "weight": 3})
        elif partial:
            found.append({"word": word, "how": "is part of the name of",
                          "symbols": partial[:40], "files": [], "weight": 2})
        elif inpath:
            found.append({"word": word, "how": "is part of the path of", "symbols": [],
                          "files": inpath[:40], "weight": 1})
        elif by_stem:
            found.append({"word": word, "how": "shares a stem with the name of",
                          "symbols": by_stem[:40], "files": [], "weight": 1})
        else:
            missing.append(word)
    return found, missing


def survey(statement, repo=None, repo_limit=40):
    """Everywhere the words of a task land, and everywhere they do not.

    This used to rank the places and offer the best of them. It was wrong, and
    it was wrong in a way no tuning reaches: on one estate three words of a
    task fell on a local method in an admin filter and it outscored the class
    the task was plainly about. Choosing between them means understanding the
    sentence, and nothing here understands anything.

    What is derivable is where each word lands and how — and that is what this
    returns, in full. Whoever asked has a model and can choose; this hands them
    the material to choose from, and expands whatever they choose.
    """
    repo = repo or ROOT
    found, missing = placed(statement, repo)
    out = []
    for hit in found:
        places = []
        for name in hit["symbols"]:
            for rel, start, end in where_defined(name, repo):
                places.append({"file": rel, "name": name, "line": start,
                               "test ground": is_test(rel, repo)})
        for rel in hit["files"]:
            places.append({"file": rel, "name": None, "line": 1,
                           "test ground": is_test(rel, repo)})
        out.append({"word": hit["word"], "how": hit["how"],
                    "places": places[:repo_limit], "more": max(0, len(places) - repo_limit)})
    return out, missing


def context(target, repo=None, budget=120000, want_all=False):
    """What an agent needs to change one thing, in one answer.

    The target is a symbol, a file, or a file and a symbol in it. What comes
    back is the definition itself, the places that call into it with the lines
    around them, what it calls that lives here, and what would judge a change
    to it — ordered so that what is firm arrives first and what is a guess
    arrives last, and cut to a budget that is named rather than silently
    exceeded.
    """
    repo = (repo or ROOT).resolve()
    spots = positions(repo)
    name, rel = None, None
    text = str(target)
    if (repo / text).exists():
        rel = pathlib.Path(text).as_posix()
    elif ":" in text:
        rel, name = text.rsplit(":", 1)
        rel = rel if (repo / rel).exists() else None
    else:
        name = text
    if name:
        subjects = [(f, s, e) for f, s, e in where_defined(name, repo)
                    if rel is None or f == rel]
    else:
        subjects = [(rel, s, e) for _n, s, e in spots.get(rel, {}).get("symbols", [])]
        subjects = [(rel, 1, 10 ** 9)] if not subjects else [
            (rel, min(s for _f, s, _e in subjects), max(e for _f, _s, e in subjects))]
    callers = call_sites(name, repo) if name else [
        {"file": r["file"], "line": 0, "kind": "name", "confidence": r["confidence"]}
        for r in affects([rel], repo)]
    # Where a name is defined in several places, the import graph usually says
    # which one a caller means: a file that can reach exactly one of them means
    # that one. Without this every caller of an ambiguous name is reported as a
    # guess, which on a .NET estate is every caller there is.
    if len(subjects) > 1:
        _all, firm = importers(repo, graded=True)
        reach = {f: set(transitive({f}, repo, edges=firm)) for f, _s, _e in subjects}
        for who in callers:
            candidates = [f for f, files in reach.items() if who["file"] in files]
            if len(candidates) == 1:
                who["reaches"] = candidates[0]
                if who["confidence"] == "low":
                    who["confidence"] = "medium"
        rank = {"high": 0, "medium": 1, "low": 2}
        callers.sort(key=lambda r: (rank[r["confidence"]], r["file"], r["line"]))
    return {
        "target": text,
        "subjects": subjects,
        "callers": callers,
        "calls out": (callees(subjects[0][0], (subjects[0][1], subjects[0][2]), repo)
                      if subjects else []),
        "judged by": judged_by(subjects[0][0], name, repo) if subjects else {},
        "provenance": "derived", "confidence": "medium",
        "caveat": "what is firm is marked; the rest is a name that matched, and a name "
                  "can belong to something else",
    }


def coverage(repo=None):
    """The share of files something else in this estate is known to reach.

    Borrowed whole from how CodeGraph states its own: of the files that define
    anything, how many have at least one resolved cross-file dependent —
    something that imports them, calls into them, or refers to them by name.
    It is the one figure that can be compared between two indexes, and until
    now this one had never taken it.

    The residual is not a defect to hide. A file nothing reaches is either
    genuinely unreached — an entry point, dead ground — or reached by something
    no static reader follows: reflection, a container, a framework convention,
    a name assembled at run time. Both are reported, and neither is flattered
    by narrowing what counts as a file.
    """
    repo = repo or ROOT
    defines, calls, _ = index(repo)
    where = defaultdict(set)
    for name, files in defines.items():
        for f in files:
            where[f].add(name)
    reached = defaultdict(set)                 # file → the files that reach it
    for rel, name, _kind in calls:
        for target in defines.get(name, ()):
            if target != rel:
                reached[target].add(rel)
    # An import is a dependent as much as a call is, and in several languages
    # it is the only one visible: a file imported for its side effects, a
    # package imported whole. Counting only names matched left a fifth of
    # Python unreached that its own imports reach.
    for target, who in importers(repo).items():
        reached[target] |= who
    bearing = sorted(where)
    covered = [f for f in bearing if reached.get(f)]
    rest = sorted(set(bearing) - set(covered))
    # What is left is not one thing. A test is reached by a runner that finds
    # it by name, an entry point is reached by whoever starts the program, and
    # neither is written down anywhere a reader could follow. Saying which is
    # which is the difference between a frontier and a defect.
    by_runner = [f for f in rest if is_test(f, repo)]
    left = [f for f in rest if f not in by_runner]
    entries = [f for f in left if pathlib.PurePosixPath(f).stem in ENTRY_NAMES
               or "main" in where.get(f, ())]
    left = [f for f in left if f not in entries]
    loaded = [f for f in left
              if any(part in LOADED_BY_NAME for part in pathlib.PurePosixPath(f).parts[:-1])]
    left = [f for f in left if f not in loaded]
    as_text = named_in_text(repo, left)
    nothing = [f for f in left if f not in as_text]
    blind = unseen(repo)
    convention = len(covered) + len(by_runner) + len(entries) + len(loaded) + len(as_text)
    return {"files that define anything": len(bearing),
            "reached by something else": len(covered),
            "coverage": round(100.0 * len(covered) / len(bearing), 1) if bearing else 0.0,
            "counting what a convention reaches":
                round(100.0 * convention / len(bearing), 1) if bearing else 0.0,
            "reached by a runner": len(by_runner),
            "entry points": len(entries),
            "loaded by name": len(loaded),
            "named only as text": len(as_text),
            "unreached": nothing,
            "unread files": sum(blind.values()),
            "provenance": "derived", "confidence": "medium",
            "caveat": "a name matched is not a call resolved; this counts what one "
                      "reader could see, and what nothing reaches may be reached by "
                      "reflection, a container or a framework convention"}


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
        commit, when = stamp(repo, rel)
        known = was.get(rel, {}).get("commit")
        rows.append({"region": rel, "derived from": known or "—", "last touched": commit,
                     "when": when,
                     "state": "fresh" if known == commit else
                              ("never derived" if known is None else "stale"),
                     "outside history": commit.startswith("contents:")})
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

# The consuming side of an estate is written in the same languages the offering
# side is, and reading only one of them halves every answer: twenty-three offers
# and no consumers is not an estate nobody calls, it is an estate half read.
# A service rarely writes the whole path at the point of call — it keeps a base
# and joins a fragment to it — so the base is read as what it is: evidence that
# this repository calls that path.
# A route table declares fragments, not paths: Django writes `^projectconfigs/$`
# in one module and joins it to `^api/0/relays/` in another, at import time.
# Read as a path, the fragment joins to nothing and, worse, invites a join on a
# tail — `/settings` is a fragment of half the estate. They are read and counted
# so that the gap is stated, and are not offered for joining.
DECLARES_FRAGMENT = [
    (r'\b(?:re_path|path|url)\(\s*r?["\']\^?([^"\']+)["\']', "http"),
]
BASE_PATTERN = 1          # the entry below that reads a base kept in a field

USES_TEXT = [
    (r'(?:GetFromJsonAsync|PostAsJsonAsync|PutAsJsonAsync|PatchAsJsonAsync|'
     r'GetStringAsync|GetAsync|PostAsync|PutAsync|DeleteAsync)\s*(?:<[^>]*>)?\s*\(\s*'
     r'\$?"((?:[a-z]+:)?//[^"]+|/[^"]*)"', "http"),                      # .NET
    (r'\b\w*(?:BaseUrl|BaseAddress|baseUrl|Endpoint|endpoint)\w*\s*=\s*'
     r'\$?"((?:[a-z]+:)?//[^"]+|/?[a-zA-Z][\w-]*(?:/[^"]*)+)"', "http"),  # a base kept in a field
    (r'\bfetch\(\s*[`"\']((?:[a-z]+:)?//[^`"\']+|/[^`"\']*)', "http"),   # browsers
    (r'\baxios\.(?:get|post|put|patch|delete)\(\s*[`"\']'
     r'((?:[a-z]+:)?//[^`"\']+|/[^`"\']*)', "http"),
    (r'\bhttp\.(?:Get|Post|NewRequest)\([^)]*?"((?:[a-z]+:)?//[^"]+|/[^"]*)"', "http"),  # Go
]

# Keys too generic to identify anything. A route of "/" is offered by most
# services that offer anything, and joining on it says only that both sides
# speak HTTP.
# What an estate offers is usually declared in a language this index cannot
# parse. Reading only Python meant that on a nine-repository estate every
# consumer was reported as pointing outside it while the services they call
# sat in the next directory — a correct statement about the index presented as
# a statement about the estate. A route declaration is a shape a regular
# expression can find without understanding the language around it, so the
# offer side is read textually. It is weaker than a parse and is graded as
# such: what it finds is a declaration that looks like a route, not a route
# proven to exist.
DECLARES_TEXT = [
    # A route is rarely a bare literal: it is a base path joined to one. The
    # first version of this required the string to sit right after the bracket,
    # and on a real service every route was written `baseUrl+"/cart"`, so it
    # found none of them and reported the estate as offering nothing.
    (r'@(?:Request|Get|Post|Put|Patch|Delete)Mapping\(\s*(?:value\s*=\s*)?'
     r'"([^"]+)"', "http"),                                  # Spring
    (r'@Path\(\s*"([^"]+)"\s*\)', "http"),                   # JAX-RS
    (r'\[(?:HttpGet|HttpPost|HttpPut|HttpDelete|Route)\(\s*"([^"]+)"', "http"),  # ASP.NET
    (r'\.Map(?:Get|Post|Put|Patch|Delete|Forwarder)\(\s*"([^"]+)"', "http"),  # minimal APIs
    (r'\.(?:HandleFunc|Handle|Path)\(\s*(?:[\w.()]+\s*\+\s*)?"(/[^"]*)"',
     "http"),                                                # net/http, gorilla/mux
    (r'\b(?:r|router|app|e|g)\.(?:GET|POST|PUT|PATCH|DELETE)\(\s*'
     r'(?:[\w.()]+\s*\+\s*)?"(/[^"]*)"', "http"),            # gin, echo
    (r'\b(?:app|router)\.(?:get|post|put|patch|delete|all)\(\s*'
     r'["\'](/[^"\']*)["\']', "http"),                       # express
]

DEGENERATE = {"/", "", "/*", "/health", "/healthz", "/ping", "/metrics"}

# A route is declared relative to the group it is mapped on, and the group is
# named once at the top of the file. Reading the route alone gives `/items`,
# which belongs to no service in particular and joins to nothing.
GROUPS = [r'\.MapGroup\(\s*"([^"]+)"', r'\bAPIRouter\(\s*prefix\s*=\s*["\']([^"\']+)["\']',
          r'\bBlueprint\([^)]*url_prefix\s*=\s*["\']([^"\']+)["\']']

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
        if outside(p, base):
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


# One path parameter, five spellings: {id}, :id, <custid>, <int:pk>, %s. They
# are the same contract written in five frameworks, and comparing them as text
# reports a front end and the service behind it as two unrelated things.
PARAM = re.compile(r'\{[^/}]*\}|<[^/>]*>|\([^/)]*\)|:[A-Za-z_][A-Za-z0-9_]*'
                   r'|%[sd]|\$\{[^/}]*\}')


def normalise(kind, key):
    """A consumer writes a URL and a producer writes a path. Joining them means
    saying so: the host is not part of the contract, it is a hint about who
    offers it — and a useful one, because when it agrees with the repository
    the key matched, two independent things point the same way."""
    if kind != "http":
        return key, None
    # `@mock.patch("snuba.clusters.cluster.get_local_nodes")` is a decorator that
    # takes a dotted module path, and it is shaped exactly like a route
    # declaration. Read as one, a single estate contributed one thousand seven
    # hundred imaginary routes. A route has a separator in it; a module path
    # does not.
    if "/" not in key or key.lstrip().startswith("."):
        return None, None
    host = None
    m = URL.match(key)
    if m:
        key, host = m.group("path"), m.group("host")
    # Parameters first: `{brandId?}` is an optional parameter, not a query
    # string, and cutting at the question mark left half a parameter behind.
    key = PARAM.sub("{}", key)
    key = key.split("?")[0]                      # a query string is not the contract
    # Routers match paths without regard to case, so two spellings of one route
    # are one contract. Holding them apart reported a caller of /api/Orders as
    # pointing outside an estate that offers /api/orders.
    key = key.lower()
    if not key.startswith("/"):
        key = "/" + key                          # `api/catalog/items` is that path
    if len(key) > 1:
        key = key.rstrip("*").rstrip("/") or "/"
    return key, host


def facts(repo):
    """What this repository offers, and what it consumes from elsewhere."""
    declares, uses, stands_in, fragments, shown = [], [], [], [], 0
    # One pass over everything, not one over what a parser reads and another
    # over what it does not. A route is declared in text whoever parses the
    # file, and splitting the scan by that meant a language stopped being read
    # for contracts the moment it started being read for symbols.
    for path in sorted(set(sources(repo)) | set(unread_sources(repo))):
        rel = path.relative_to(repo).as_posix()
        if illustration(rel):
            shown += 1
            continue
        try:
            text = path.read_text(errors="ignore")
        except OSError:
            continue
        native = path.suffix in python_ast.EXTENSIONS
        if native:
            decorated = {ln.strip() for ln in text.splitlines()
                         if ln.lstrip().startswith("@")}
            for pattern, kind in DECLARES_FRAGMENT:
                for key in re.findall(pattern, text):
                    k, _ = normalise(kind, key)
                    if k and k not in DEGENERATE:
                        fragments.append({"kind": kind, "key": k, "file": rel})
            for pattern, kind in DECLARES:
                for key in re.findall(pattern, text):
                    k, _ = normalise(kind, key)
                    if k and k not in DEGENERATE:
                        # A route declared inside test ground is a stand-in for
                        # a service this repository talks to, not something this
                        # repository offers. Read as an offer it joins every
                        # consumer of it to the wrong repository, and an edge to
                        # the wrong service is worse than no edge: it is
                        # believed, and it points at people who cannot act on it.
                        (stands_in if is_test(rel, repo) else declares).append(
                            {"kind": kind, "key": k, "file": rel})
            for line in text.splitlines():
                if line.strip() in decorated:
                    continue
                for pattern, kind in USES:
                    for key in re.findall(pattern, line):
                        k, host = normalise(kind, key)
                        if k and k not in DEGENERATE:
                            uses.append({"kind": kind, "key": k, "file": rel,
                                         "host": host,
                                         "base": key.rstrip('"\'').endswith("/"),
                                         "in test": is_test(rel, repo)})
        for n, (pattern, kind) in enumerate(USES_TEXT):
            for line in text.splitlines():
                for key in re.findall(pattern, line):
                    k, host = normalise(kind, key)
                    if k and k not in DEGENERATE:
                        uses.append({"kind": kind, "key": k, "file": rel, "host": host,
                                     "read": "text", "in test": is_test(rel, repo),
                                     "base": n == BASE_PATTERN or key.endswith("/")})
        # A forwarder is both ends at once: it offers a path and says which
        # service the call is handed to, which is the one thing a key never says.
        for local, host, remote in re.findall(
                r'\.MapForwarder\(\s*"([^"]+)"\s*,\s*"([^"]+)"\s*,\s*"([^"]+)"', text):
            k, _ = normalise("http", remote)
            if k and k not in DEGENERATE:
                uses.append({"kind": "http", "key": k, "file": rel, "in test": False,
                             "host": host.split("//")[-1], "read": "text"})
        prefixes = {m for pat in GROUPS for m in re.findall(pat, text)}
        for pattern, kind in DECLARES_TEXT:
            for key in re.findall(pattern, text):
                for full in ({key} if not prefixes or key.startswith(("http", "//"))
                             else {p.rstrip("/") + "/" + key.lstrip("/") for p in prefixes}):
                    k, _ = normalise(kind, full)
                    if not k or k in DEGENERATE:
                        continue
                    (stands_in if is_test(rel, repo) else declares).append(
                        {"kind": kind, "key": k, "file": rel, "read": "text"})
    return {"repo": repo.name, "declares": declares, "uses": uses,
            "stands in for": stands_in, "fragments": fragments,
            "illustrations": shown}


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


def covers(offered, consumed, consumed_is_base=False):
    """Whether the offered route covers the call, the way a router would.

    Matching is directional. A route is a pattern and a call is a path: the
    pattern's parameters match whatever the caller put there, and the caller's
    literals match nothing but themselves. Letting both sides wildcard made
    `/{}/{}/subscriptions` cover `/user/orgs` and joined twenty-six unrelated
    calls to one service.

    The one case where a call may be shorter than the route it reaches is a
    base — a prefix the caller keeps and joins fragments to. A base is visible
    in the source: it ends in a separator, or it is held in a field named for
    one. Then, and only then, the comparison runs the other way, and it runs on
    literals alone.
    """
    o = [p for p in offered.strip("/").split("/") if p]
    c = [p for p in consumed.strip("/").split("/") if p]
    if not any(seg != "{}" for seg in o):
        # A route that is nothing but parameters — `/{customerId}` — matches
        # every call of one segment. It is degenerate for the same reason `/`
        # is: matching everything says nothing about who offers what.
        return False
    if len(o) <= len(c):
        if any(x != y and x != "{}" for x, y in zip(o, c)):
            return False
        rest = c[len(o):]
        return len(o) >= 2 or all(seg == "{}" for seg in rest)
    if not consumed_is_base:
        return False
    return len(c) >= 2 and all(x == y for x, y in zip(c, o))


def contracts(where, telemetry=None):
    all_facts = [facts(r) for r in sorted(pathlib.Path(where).iterdir()) if r.is_dir()]
    seen = observed(telemetry)
    offered = {}
    for f in all_facts:
        for d in f["declares"]:
            offered.setdefault((d["kind"], d["key"]), []).append((f["repo"], d["file"]))
    edges, unconsumed, unmatched = [], [], []
    consumed, internal = set(), 0
    for f in all_facts:
        for u in f["uses"]:
            k = (u["kind"], u["key"])
            # Two handlers for one route in one repository are one offer. Counting
            # the declarations rather than the repositories made a service that
            # answers GET and POST on the same path look like two services
            # claiming it, and downgraded a sound edge to an ambiguous one.
            producers = sorted({p[0] for p in offered.get(k, [])} - {f["repo"]})
            near = None
            if not producers:
                # No exact key. The router the estate actually runs matches a
                # prefix, so this looks for the same thing rather than calling
                # the dependency absent.
                candidates = [(ok, sorted({p[0] for p in who} - {f["repo"]}))
                              for (okind, ok), who in offered.items()
                              if okind == u["kind"]
                              and covers(ok, u["key"], u.get("base", False))]
                candidates = [(ok, reps) for ok, reps in candidates if reps]
                if candidates:
                    near, producers = min(candidates, key=lambda c: len(c[0]))
                    near = (near, len(candidates))
            if not producers:
                if any(p[0] == f["repo"] for p in offered.get(k, [])) or any(
                        covers(ok, u["key"], u.get("base", False))
                        and f["repo"] in {p[0] for p in who}
                        for (okind, ok), who in offered.items() if okind == u["kind"]):
                    # A repository calling a route it offers itself is not a
                    # contract crossing a boundary, and reporting it as pointing
                    # outside the estate buried the calls that really do.
                    internal += 1
                    continue
                unmatched.append({"consumer": f["repo"], "file": u["file"], **u})
                continue
            consumed.add(k if near is None else (u["kind"], near[0]))
            confirmed = (f["repo"], u["kind"], u["key"]) in seen
            agrees = u.get("host") and u["host"] == producers[0]
            if confirmed:
                conf, how = "high", "seen in traffic"
            elif near is not None:
                route, many = near
                conf, how = "low", (
                    f"no repository offers this key; {producers[0]} offers {many} route(s) "
                    f"that cover it, {route} among them" if many > 1 else
                    f"no repository offers this key; the route {route} covers it")
            elif len(producers) > 1:
                conf, how = "low", ("this key is offered by " + ", ".join(producers)
                                    + ", and which one it reaches is not derivable here")
            elif agrees:
                conf, how = "medium", "the key matches and the address names the same repository"
            else:
                conf, how = "medium", "one repository offers this key"
            if u.get("in test") and not confirmed:
                # A URL written in a test is as likely to name a fixture as a
                # service: a unit test hitting its own application on /settings
                # was joined to another repository that happens to offer one.
                conf = "low"
                how += "; the call is made from the consumer's own test ground"
            edges.append({
                "from": f["repo"], "to": producers[0], "kind": u["kind"], "key": u["key"],
                # Which of several offerers a call reaches is not decidable from
                # a key. Naming one and forgetting the rest makes every other
                # offerer of it look untouched by a change nobody can rule out.
                "among": producers,
                "provenance": "observed" if confirmed else "matched",
                "confidence": conf, "how": how})
    # A stand-in is thrown away as an offer and kept as evidence of the reverse:
    # nobody writes a fake of a service they do not call. It is the weakest
    # ground for an edge and says so, but a dependency stated nowhere else is
    # worth more reported weakly than dropped.
    fakes, already = [], set()
    for f in all_facts:
        for d in f["stands in for"]:
            k = (d["kind"], d["key"])
            if (f["repo"], k) in already:       # one fake written twice is one fake
                continue
            already.add((f["repo"], k))
            producers = sorted({p[0] for p in offered.get(k, [])} - {f["repo"]})
            row = {"repo": f["repo"], "to": producers[0] if producers else None, **d}
            fakes.append(row)
            if producers:
                consumed.add(k)
                edges.append({
                    "from": f["repo"], "to": producers[0], "kind": d["kind"],
                    "key": d["key"], "among": producers, "provenance": "matched",
                    "confidence": "low",
                    "how": "this repository keeps a stand-in for it in its own tests"})
    for k, where_offered in offered.items():
        if k not in consumed:
            unconsumed.append({"kind": k[0], "key": k[1],
                               "offered by": where_offered[0][0],
                               "file": where_offered[0][1]})
    strength = {"high": 0, "medium": 1, "low": 2}
    best = {}
    for e in edges:
        k = (e["from"], e["to"], e["kind"], e["key"])
        if k not in best or strength[e["confidence"]] < strength[best[k]["confidence"]]:
            best[k] = e
    edges = list(best.values())
    seen_un, once = set(), []
    for u in unmatched:                      # the same call written twice is one gap
        k = (u["consumer"], u["kind"], u["key"])
        if k not in seen_un:
            seen_un.add(k)
            once.append(u)
    unmatched = once
    surprises = [s for s in seen
                 if not any(e["from"] == s[0] and e["kind"] == s[1] and e["key"] == s[2]
                            for e in edges)]
    loose = [{"repo": f["repo"], **d} for f in all_facts for d in f["fragments"]]
    shown = sum(f["illustrations"] for f in all_facts)
    return edges, unconsumed, unmatched, surprises, fakes, internal, loose, shown


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
    edges, _, unmatched, surprises, *_rest = contracts(where, telemetry)
    offers = []
    for r in reps:
        if region and not (r.name == region or str(r).endswith(region)):
            continue
        declared, once = set(), []
        for d in facts(r)["declares"]:       # one route declared by two handlers
            if (d["kind"], d["key"]) in declared:
                continue
            declared.add((d["kind"], d["key"]))
            once.append(d)
        for d in once:
            takers = sorted({e["from"] for e in edges
                              if r.name in e.get("among", [e["to"]])
                              and e["kind"] == d["kind"] and e["key"] == d["key"]})
            shared = sorted({o for e in edges for o in e.get("among", [])
                             if e["kind"] == d["kind"] and e["key"] == d["key"]
                             and len(e.get("among", [])) > 1} - {r.name})
            outside = [s[0] for s in surprises
                       if s[1] == d["kind"] and s[2] == d["key"] and s[0] not in known]
            offers.append({"repo": r.name, **d, "taken by": takers,
                           "also offered by": shared, "beyond reach": outside})
    return offers, unmatched


def outline(repo, rel, start, end):
    """What is inside something too long to quote: its parts, and where they are.

    A class of fifteen hundred lines quoted from the top gives a page of
    docstring. What a reader needs from something that size is the shape of it
    and the line to jump to.
    """
    inner = [(name, line) for name, line, _e in
             positions(repo).get(rel, {}).get("symbols", [])
             if start < line <= end]
    return sorted(inner, key=lambda t: t[1])


def render_context(c, budget, every):
    """Print what was found, firmest first, and stop when the budget is gone.

    The order is the whole design: what a reader is handed first is what it
    will act on, and a halo of maybes at the top of an answer is worse than no
    answer at all.
    """
    out, spent = [], 0

    def say(line):
        nonlocal spent
        spent += len(line) + 1
        out.append(line)

    repo = ROOT
    if len(c["subjects"]) > 1:
        say(f"    ({len(c['subjects'])} places define this name; all of them are below, "
            f"because which one a caller means is not derivable from the name)")
    # The budget, not a constant, decides how much of anything is quoted: what
    # an answer should contain follows from the work it is for, and a figure
    # picked here to keep answers tidy is a filter nobody asked for.
    limit = 10 ** 9 if every else max(120, budget // 400)
    for rel, start, end in c["subjects"]:
        say(f"=== {rel}:{start}-{end}")
        for n, line in excerpt(repo, rel, start, end, limit=limit):
            say(f"{n:>6}  {line}")
        if end - start + 1 > limit:
            parts = outline(repo, rel, start, end)
            say(f"    … {end - start + 1 - limit} more line(s) not quoted; what is in them:")
            for name, line in parts:
                say(f"      {rel}:{line}  {name}")
    # Thirty call sites in one file say the same thing thirty times. A few from
    # each of many files is the answer to "who uses this"; the rest is a count
    # and the file to open.
    per_file, skipped, crowded = defaultdict(int), 0, defaultdict(int)
    for who in c["callers"]:
        room = spent < budget * 0.75 or every
        per_file_cap = 10 ** 9 if every else max(3, budget // 20000)
        if not room or per_file[who["file"]] >= per_file_cap:
            skipped += 1
            crowded[who["file"]] += 1
            continue
        per_file[who["file"]] += 1
        where = "in the same file" if who.get("here") else who["confidence"]
        if who.get("reaches"):
            where += f", and of the {len(c['subjects'])} it can reach only {who['reaches']}"
        say(f"--- called from {who['file']}:{who['line']}  ({where})")
        for n, line in excerpt(repo, who["file"], who["line"] - 2, who["line"] + 2):
            say(f"{n:>6}  {line}")
    if skipped:
        worst = ", ".join(f"{f} ({n} more)" for f, n in
                          sorted(crowded.items(), key=lambda kv: -kv[1])[:5])
        say(f"    … and {skipped} call site(s) not quoted, in {len(crowded)} file(s): "
            f"{worst}{' …' if len(crowded) > 5 else ''}")
    firm_out = [r for r in c["calls out"] if r["confidence"] != "low"]
    loose_out = len(c["calls out"]) - len(firm_out)
    if firm_out or loose_out:
        say("--- what it calls that lives here")
        for r in (c["calls out"] if every else firm_out)[:max(40, budget // 2000)]:
            say(f"    {r['confidence']:>6}  {r['file']}  {r['name']} (line {r['line']})")
        if loose_out and not every:
            say(f"    … and {loose_out} more that share a name with something here and "
                f"import nothing that leads to it (--all to see them)")
    if c["judged by"]:
        say("--- what would judge a change here")
        for test, names in list(c["judged by"].items())[:10]:
            say(f"    {test}  names {', '.join(names[:4])}")
    else:
        say("--- nothing here names what this defines: a change to it is judged by "
            "nothing that can be found by reading")
    print("\n".join(out))
    return spent


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
            note = "" if row.get("imports it", True) else "  (no import reaches it)"
            print(f"  {row['confidence']:>6}  {row['file']}  through {names}{more}{note}")
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
            stranger = sum(1 for r in out if not r.get("imports it", True))
            print(f"  shape: " + ", ".join(f"{n} {g}" for g, n in
                                           sorted(grades.items())) + f"; mostly in {spread}")
            if stranger:
                print(f"  {stranger} of them share a name with it and import nothing that "
                      f"leads to it — a coincidence of vocabulary unless a framework "
                      f"loads them")
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
        if not all_rows:
            blind = unseen()
            print("  nothing here is in a language this index can read: that is not an "
                  "estate with no regions, it is one this cannot see" if blind else
                  "  nothing is defined here that this index can see")
            print(coverage_note())
            return 0
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
        edges, unconsumed, unmatched, surprises, fakes, internal, loose, shown = \
            contracts(args[0], tel)
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
        if shown:
            print(f"  {shown} file(s) of documentation, examples and fixtures were not "
                  f"read: a route in a snippet is written to be read, not deployed")
        if internal:
            print(f"  {internal} call(s) go to routes the calling repository offers "
                  f"itself and cross no boundary")
        if loose:
            byrepo = Counter(f["repo"] for f in loose)
            print(f"  {len(loose)} route(s) are declared as fragments assembled at import "
                  f"time (" + ", ".join(f"{r} {n}" for r, n in byrepo.most_common(4))
                  + "); this index does not resolve them into paths, so what they offer "
                  f"is unread rather than absent")
        loose = [f for f in fakes if not f["to"]]
        for f in loose:
            print(f"     low  {f['repo']} → ?  {f['kind']} {f['key']}  a stand-in for it is "
                  f"kept in this repository's own tests, so it depends on it, and nothing "
                  f"the index can read offers it")
        if fakes:
            byrepo = Counter(f["repo"] for f in fakes)
            print(f"  {len(fakes)} route(s) declared inside test ground are stand-ins for "
                  f"services these repositories talk to, not offers of their own ("
                  + ", ".join(f"{r} {n}" for r, n in byrepo.most_common()) + ")")
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
            also = ("" if not o["also offered by"] else
                    f"; {', '.join(o['also offered by'])} offer(s) the same key, so a "
                    f"consumer of it may be reaching them instead")
            print(f"  {o['repo']} offers {o['kind']} {o['key']} — taken by {takers}{also}")
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
        blind_hist = sum(1 for r in rows if r["outside history"])
        print(f"  {fresh} of {len(rows)} region(s) current against what last touched them")
        if blind_hist:
            print(f"  {blind_hist} of them sit outside any history this can read and "
                  f"are keyed on their contents instead: a change is still noticed, "
                  f"but nothing here can say which commit it came from")
        print(coverage_note())
        return 0
    if cmd == "coverage":
        c = coverage()
        for f in c["unreached"][:15]:
            print(f"  unreached  {f}")
        if len(c["unreached"]) > 15:
            print(f"  … and {len(c['unreached']) - 15} more nothing here reaches")
        print(f"  {c['reached by something else']} of {c['files that define anything']} "
              f"file(s) that define anything are reached by something else here — "
              f"{c['coverage']}%")
        for how, n in (("test ground a runner finds by name", c["reached by a runner"]),
                       ("an entry point", c["entry points"]),
                       ("where a framework loads by name", c["loaded by name"]),
                       ("named as text and nowhere else", c["named only as text"])):
            if n:
                print(f"  {n} more are {how}")
        print(f"  counting those, {c['counting what a convention reaches']}%")
        print(f"  {c['caveat']}")
        print(coverage_note())
        return 0
    if cmd == "context" and args:
        every = "--all" in args
        rest = [a for a in args if a != "--all"]
        budget, task, kept = 120000, None, []
        skip = False
        for i, a in enumerate(rest):
            if skip:
                skip = False
                continue
            if a.startswith("--budget="):
                budget = int(a.split("=", 1)[1])
            elif a.startswith("--task="):
                task = a.split("=", 1)[1]
            elif a == "--task" and i + 1 < len(rest):
                task, skip = rest[i + 1], True
            else:
                kept.append(a)
        rest = kept
        if task is None and rest and " " in rest[0]:
            task, rest = rest[0], rest[1:]      # a sentence is a task, not a symbol
        if task is not None:
            found, missing = survey(task)
            print("=== where the words of this task land")
            for hit in found:
                print(f"    {hit['word']}  {hit['how']}:")
                for place in hit["places"]:
                    mark = "  (test ground)" if place["test ground"] else ""
                    named = f":{place['name']}" if place["name"] else ""
                    print(f"        {place['file']}{named}  line {place['line']}{mark}")
                if hit["more"]:
                    print(f"        … and {hit['more']} more place(s)")
            if missing:
                print(f"    placed nothing: {', '.join(missing)} — either this estate "
                      f"calls them something else, or the ground they name is not read "
                      f"here")
            if not found:
                print("    nothing in this task names anything here: it is about ground "
                      "this index cannot see, or about behaviour nobody named after it")
            print("  — these are places, not an answer. Which of them the task is about "
                  "is not derivable from the words, and choosing wrongly here costs more "
                  "than choosing slowly: ask again naming the ones you mean, as "
                  "`context <file>:<symbol> …`, and each will be expanded in full.")
            print(coverage_note())
            return 0
        share = budget // max(len(rest), 1)
        spent = 0
        for one in rest:
            c = context(one, budget=share, want_all=every)
            spent += render_context(c, share, every)
        print(f"  — {spent} character(s) of a {budget} budget"
              + (f", over {len(rest)} target(s)" if len(rest) > 1 else "")
              + f"; {c['caveat']}")
        print(coverage_note())
        return 0
    if cmd == "readers":
        for who, exts in sorted(readers.who().items()):
            print(f"  {who:>12}  {' '.join(sorted(exts))}")
        blind = sorted(UNREADABLE - set(readers.by_extension()))
        print(f"  {len(readers.by_extension())} extension(s) read here; "
              f"{len(blind)} known and unread ({' '.join(blind)})")
        print("  a file with no extension is read by what its first line says it is")
        print("  a language nothing here reads is reported unseen, never absent — "
              "install a grammar beside the readers and it moves into the count above")
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