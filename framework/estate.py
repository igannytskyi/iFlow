#!/usr/bin/env python3
"""What the estate is, derived from it rather than described about it.

Usage: python3 framework/estate.py affects <path>...     what a change here reaches
       python3 framework/estate.py observability <path>  how well behaviour there is pinned
       python3 framework/estate.py freshness             what this was derived from, and when
       python3 framework/estate.py unknown               what it could not resolve

Every answer carries where it came from and how far it is to be trusted. Nothing
here is maintained: it is derived on demand from the code as it stands, and a
region whose source has moved is re-derived before it is answered about.

What this is not: a description of the system kept beside it. The index is a
cache with an invalidation rule, never a corpus with a publication date.
"""
import ast
import json
import pathlib
import subprocess
import sys
from collections import defaultdict

ROOT = pathlib.Path(__file__).resolve().parent.parent

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
    """How well behaviour in a region can be pinned down at all.

    Answered from what exists, never guessed: an arbiter that reaches a symbol
    defined here is evidence the region can be observed. Nothing here says the
    observation is *good* — only that one exists.
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
    return {"region": path, "symbols": len(here), "watched by": watchers,
            "verdict": "observed" if watchers else "unobserved",
            "provenance": "matched", "confidence": "medium"}


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
        blind = sum(r["occurrences"] for r in unknown())
        print(f"  {len(out)} reached through code, {len(named)} that only name it as text.")
        print(f"  {blind} call(s) this index cannot resolve at all, so neither figure "
              f"is a floor or a ceiling — it is what one parser could see.")
        return 0
    if cmd == "observability" and args:
        r = observability(args[0])
        print(json.dumps(r, indent=2))
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
