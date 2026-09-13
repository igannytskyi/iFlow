#!/usr/bin/env python3
"""What the estate model is worth, measured rather than asserted.

Usage: python3 framework/measure.py <estate>...        over these repositories
       python3 framework/measure.py <estate>... --json  the same, as data

Two figures, both of them about the index rather than about any estate:

**Reach** — the share of files that define anything and are reached by
something else. Comparable between two indexes, and nothing more: this
representation is never complete, and what it is measured by is freshness.

**Quotation** — the share of reported call sites whose named line contains the
call. This is the figure that matters when an answer is handed to someone to
act on: a quotation off by a line sends a reader to the wrong place and says
nothing about being wrong.

It was a sampling run by hand across borrowed repositories, cited as evidence
and repeatable by nobody. A measurement cited as evidence is a command.
"""
import json
import pathlib
import random
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import estate                                        # noqa: E402

SAMPLE = 40
SEED = 3


def quotation(repo, ext=None):
    """Of the call sites this index reports, how many name the line the call
    is on."""
    random.seed(SEED)
    estate.ROOT = repo
    estate._TOUCHED.clear()
    estate._WHERE.clear()
    defines, _, _ = estate.index(repo)
    names = [n for n, files in defines.items()
             if len(n) > 3 and (ext is None or any(f.endswith(ext) for f in files))]
    right = seen = 0
    for name in random.sample(names, min(SAMPLE, len(names))) if names else []:
        for site in estate.call_sites(name, repo)[:5]:
            if site["kind"] not in ("name", "attribute"):
                continue
            if ext and not site["file"].endswith(ext):
                continue
            seen += 1
            line = estate.excerpt(repo, site["file"], site["line"], site["line"])
            if line and name in line[0][1]:
                right += 1
    return {"sites": seen, "named rightly": right,
            "share": round(100.0 * right / seen, 1) if seen else None}


def languages(repo):
    """Which languages this estate is written in, by what defines anything."""
    estate.ROOT = repo
    defines, _, _ = estate.index(repo)
    found = {}
    for files in defines.values():
        for f in files:
            suffix = pathlib.PurePosixPath(f).suffix
            found[suffix] = found.get(suffix, 0) + 1
    return [ext for ext, n in sorted(found.items(), key=lambda kv: -kv[1]) if n >= 20]


def over(paths):
    out = []
    for path in paths:
        repo = pathlib.Path(path).resolve()
        estate.ROOT = repo
        estate._TOUCHED.clear()
        estate._WHERE.clear()
        reach = estate.coverage(repo)
        row = {"estate": repo.name,
               "reach": reach["coverage"],
               "reach counting conventions": reach["counting what a convention reaches"],
               "quotation": quotation(repo),
               "by language": {ext: quotation(repo, ext) for ext in languages(repo)}}
        out.append(row)
    return out


def main(argv):
    paths = [a for a in argv[1:] if not a.startswith("--")]
    if not paths:
        print(__doc__.strip())
        return 2
    rows = over(paths)
    if "--json" in argv:
        print(json.dumps(rows, indent=1, sort_keys=True))
        return 0
    print(f"{'estate':<22}{'reach':>8}{'with conventions':>18}{'quotation':>12}")
    for r in rows:
        q = r["quotation"]["share"]
        print(f"{r['estate']:<22}{r['reach']:>7}%{r['reach counting conventions']:>17}%"
              f"{(f'{q}%' if q is not None else '—'):>12}")
        for ext, fig in r["by language"].items():
            if fig["share"] is not None:
                print(f"    {ext:<18}{'':>7} {'':>17}{fig['share']:>11}% "
                      f"of {fig['sites']} site(s)")
    print("  reach is for comparing one index with another, never for judging this one: "
          "a representation of an estate is never complete and is measured by freshness")
    print("  quotation is what an answer handed to someone to act on rests on")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
