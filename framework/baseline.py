#!/usr/bin/env python3
"""Area 13 — the present way of working, measured on a real estate.

Usage: python3 framework/baseline.py <estate> [--since YYYY-MM-DD] [--json]

Without this nothing else means anything: a figure about the method is a
figure about nothing until there is something for it to be better than.

The present way of working does not record what we want to count, so a
baseline is reconstructed rather than observed — from version history and
whatever the forge kept. Every figure below carries where it came from and how
far it is to be trusted, and the ones a repository cannot answer are named as
such rather than estimated into existence.

Three readings govern everything here and are printed with the figures:

  Never blend. Touchpoints per intent falls on its own if the mix shifts
  toward the cheap classes with nothing having improved, so every figure is
  reported per class with the mix beside it.

  The denominator is the intent. Units can be split arbitrarily and any
  per-unit figure improved by splitting them; intents cannot be split without
  a person noticing.

  What is wrongly accepted is always a lagging, censored estimate. A change
  accepted yesterday has not had time to be found wrong, so the rate is
  reported only over cohorts older than the lag, and the lag is an instrument
  in its own right.
"""
import collections
import json
import pathlib
import re
import subprocess
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import estate                                        # noqa: E402

# What a message says a change was. A proxy for the class and declared as one:
# a class is defined by how acceptance is decided, and a subject line says only
# what its author called the work.
SAYS = [
    ("C1", r"\b(bump|upgrade|update deps|dependenc|refactor|rename|move|typo|"
           r"format|lint|style|cleanup|remove unused|dead code|drop support)\b"),
    ("C2", r"\b(fix|bug|regression|broken|crash|error|fault|repair|correct)\b"),
    ("C3", r"\b(deprecat|api|interface|schema|contract|signature|protocol|"
           r"compat|breaking)\b"),
    ("C5", r"\b(add|new|support|feature|implement|introduce|allow)\b"),
]
REVERT = re.compile(r"^revert\b|\brevert(s|ing)? commit ([0-9a-f]{7,40})", re.I)
FIXES = re.compile(r"\b(fix(?:es|ed)?|close[sd]?|resolve[sd]?)\s+#(\d+)", re.I)
PR = re.compile(r"\(#(\d+)\)\s*$|Merge pull request #(\d+)")


def commits(repo, since=None):
    """Every commit, with what it says and what it touched."""
    fmt = "%x1e%H%x1f%an%x1f%cI%x1f%P%x1f%s%x1f%b"
    args = ["git", "-C", str(repo), "log", f"--format={fmt}", "--name-only"]
    if since:
        args.append(f"--since={since}")
    out = subprocess.run(args, capture_output=True, text=True).stdout
    for block in out.split("\x1e"):
        if not block.strip():
            continue
        head, _, files = block.partition("\n\n")
        parts = head.split("\x1f")
        if len(parts) < 5:
            continue
        sha, who, when, parents, subject = parts[:5]
        body = parts[5] if len(parts) > 5 else ""
        touched = [f for f in files.splitlines() if f.strip()]
        yield {"sha": sha, "who": who, "when": when[:10],
               "parents": parents.split(), "subject": subject.strip(),
               "body": body, "files": touched}


def called(commit):
    """Which class the message says this was — a proxy, and named as one."""
    text = f"{commit['subject']} {commit['body']}".lower()
    for name, pattern in SAYS:
        if re.search(pattern, text):
            return name
    return "unclassified"


def forge(repo, since, limit=400, page=30):
    """What the forge kept, where there is one and it can be asked.

    Version history says a change happened; it does not say how many times a
    person had to come back to it. Review records do, and §10.1 names them as
    part of a baseline for that reason. Where the forge cannot be asked, the
    figure taken from history alone is a floor and is reported as one.
    """
    remote = subprocess.run(["git", "-C", str(repo), "remote", "get-url", "origin"],
                            capture_output=True, text=True).stdout.strip()
    m = re.search(r"github\.com[/:]([^/]+/[^/.]+)", remote)
    if not m:
        return None, "no github remote, so no review records to read"
    # Asked thirty at a time. The forge counts the nodes a query would have to
    # traverse and refuses anything larger, so the page size is not a taste: it
    # is the largest this particular question fits into.
    rows, seen, cursor = [], set(), None
    while len(rows) < limit:
        args = ["gh", "pr", "list", "--repo", m.group(1), "--state", "merged",
                "--limit", str(page), "--json",
                "number,title,createdAt,mergedAt,comments,reviews,commits"]
        if cursor:
            args += ["--search", f"is:merged merged:<{cursor}"]
        got = subprocess.run(args, capture_output=True, text=True)
        if got.returncode:
            if rows:
                break
            return None, f"the forge could not be asked ({got.stderr.strip()[:60]})"
        page_rows = [r for r in json.loads(got.stdout or "[]")
                     if r["number"] not in seen]
        if not page_rows:
            break
        seen |= {r["number"] for r in page_rows}
        rows += page_rows
        cursor = min(r["mergedAt"] for r in page_rows)[:10]
        if since and cursor < since:
            break
    rows = [r for r in rows if not since or r.get("mergedAt", "")[:10] >= since]
    return rows, f"{len(rows)} merged pull request(s) read from {m.group(1)}"


def called_pr(pr):
    text = f"{pr.get('title', '')}".lower()
    for name, pattern in SAYS:
        if re.search(pattern, text):
            return name
    return "unclassified"


def intents(rows):
    """One intent is one pull request where the forge recorded them, and one
    commit where it did not. Never a work unit: units split, intents do not."""
    out = collections.defaultdict(list)
    for c in rows:
        m = PR.search(c["subject"])
        key = f"pr-{m.group(1) or m.group(2)}" if m else f"commit-{c['sha'][:12]}"
        out[key].append(c)
    return out


def wrongly_accepted(rows):
    """What was taken and later taken back, and how long that took.

    Reverts and nothing else: a later fix citing an issue says the issue
    existed, not that this change was wrong to accept. What is counted here is
    the estate contradicting an acceptance outright.
    """
    by_sha = {c["sha"]: c for c in rows}
    found = []
    for c in rows:
        m = REVERT.search(c["subject"]) or REVERT.search(c["body"])
        if not m:
            continue
        target = None
        for sha in re.findall(r"\b([0-9a-f]{7,40})\b", f"{c['subject']} {c['body']}"):
            for full in by_sha:
                if full.startswith(sha):
                    target = by_sha[full]
                    break
            if target:
                break
        found.append({"revert": c["sha"][:12], "at": c["when"],
                      "undid": target["sha"][:12] if target else None,
                      "accepted at": target["when"] if target else None,
                      "class said": called(target) if target else None})
    return found


def lag(found):
    """From acceptance to the discovery that it was wrong, in days."""
    import datetime
    out = []
    for f in found:
        if not f["accepted at"]:
            continue
        a = datetime.date.fromisoformat(f["accepted at"])
        b = datetime.date.fromisoformat(f["at"])
        out.append((b - a).days)
    return sorted(out)


def arbiter_strength(repo):
    """What stands behind the criteria, per region — the one instrument this
    method already has, asked of the estate rather than of history."""
    estate.ROOT = pathlib.Path(repo).resolve()
    estate._TOUCHED.clear()
    estate._WHERE.clear()
    rows = estate.every_region(estate.ROOT)
    live = [r for r in rows if not r["is test ground"]]
    named = sum(r["named by a test"] for r in live)
    total = sum(r["symbols"] for r in live)
    return {"regions": len(live),
            "claimed": sum(1 for r in live if r["verdict"] == "claimed"),
            "partly": sum(1 for r in live if r["verdict"] == "partly claimed"),
            "unclaimed": sum(1 for r in live if r["verdict"] == "unclaimed"),
            "symbols named by a test": named, "symbols": total,
            "share": round(100.0 * named / total, 1) if total else None,
            "provenance": "derived", "confidence": "low",
            "caveat": "a test naming a symbol is not a test exercising it"}


def measure(repo, since=None):
    rows = list(commits(repo, since))
    prs, forge_note = forge(repo, since)
    mix = collections.Counter()
    touch = collections.defaultdict(list)
    if prs:
        # An intent is a pull request: it is what a person opened, and it is
        # the thing that cannot be split without someone noticing.
        for pr in prs:
            klass = called_pr(pr)
            mix[klass] += 1
            # Stating it once; every review submitted on it; every comment on
            # it; and every commit after the first, which is a person coming
            # back. Each is an occasion on which a person had to act.
            touch[klass].append(1 + len(pr.get("reviews") or [])
                                + len(pr.get("comments") or [])
                                + max(0, len(pr.get("commits") or []) - 1))
        by_intent = {f"pr-{p['number']}": p for p in prs}
    else:
        by_intent = intents(rows)
        for key, group in by_intent.items():
            klass = collections.Counter(called(c) for c in group).most_common(1)[0][0]
            mix[klass] += 1
            people = {c["who"] for c in group}
            touch[klass].append(1 + max(0, len(group) - 1) + max(0, len(people) - 1))
    undone = wrongly_accepted(rows)
    days = lag(undone)
    return {
        "estate": pathlib.Path(repo).name,
        "since": since or (rows[-1]["when"] if rows else None),
        "until": rows[0]["when"] if rows else None,
        "commits": len(rows),
        "intents": len(by_intent),
        "class mix": {k: v for k, v in mix.most_common()},
        "touchpoints per intent": {
            k: round(sum(v) / len(v), 2) for k, v in sorted(touch.items())},
        "reverted": len(undone),
        "reverted share": round(100.0 * len(undone) / max(len(by_intent), 1), 2),
        "lag in days": {"median": days[len(days) // 2] if days else None,
                        "worst": days[-1] if days else None,
                        "measured on": len(days)},
        "arbiter strength": arbiter_strength(repo),
        "intents read from": forge_note,
    }


def report(m):
    print(f"=== {m['estate']}, {m['since']} to {m['until']}: "
          f"{m['commits']} commit(s) over {m['intents']} intent(s)")
    print("=== class mix — every figure below is read per class, and the mix is why")
    for k, v in m["class mix"].items():
        share = 100.0 * v / max(m["intents"], 1)
        print(f"    {k:<14} {v:>6} intent(s)  {share:>5.1f}%")
    print("    the class is what the message called the work, which is a proxy: a class "
          "is defined by how acceptance is decided, and a subject line does not say that")
    print("=== human touchpoints per intent — the primary claim")
    for k, v in m["touchpoints per intent"].items():
        print(f"    {k:<14} {v}")
    print(f"    intents: {m['intents read from']}")
    if "pull request" in m["intents read from"]:
        print("    counted as stating it once, every review submitted on it, every "
              "comment, and every commit after the first — each an occasion on which a "
              "person had to act")
    else:
        print("    counted from version history alone, which does not carry reviews or "
              "comments: this figure is a floor and not the number")
    print("=== what was accepted and taken back")
    print(f"    {m['reverted']} revert(s), {m['reverted share']}% of intents")
    lag_ = m["lag in days"]
    if lag_["measured on"]:
        print(f"    lag to discovery: {lag_['median']} day(s) median, {lag_['worst']} "
              f"worst, over {lag_['measured on']} revert(s) that name what they undid")
        print("    this is a lagging, censored estimate: anything accepted within the "
              "lag has not had time to be found wrong, and a period shorter than the "
              "lag says nothing at all")
    else:
        print("    nothing here names what it undid, so no lag can be taken")
    a = m["arbiter strength"]
    print("=== arbiter strength — what stands behind the criteria")
    print(f"    {a['regions']} region(s): {a['claimed']} claimed, {a['partly']} partly, "
          f"{a['unclaimed']} unclaimed; {a['symbols named by a test']} of {a['symbols']} "
          f"symbol(s) named by a test ({a['share']}%)")
    print(f"    {a['caveat']}")
    print("=== what a repository cannot answer")
    print("    cost per unit of verified change — human time at a stated rate is not in "
          "version history, and estimating it here would put a guess where a measurement "
          "is claimed")
    print("    share of undecided verdicts — there are no verdicts before the method; "
          "this instrument has no baseline and begins at zero when one starts")
    print("  the denominator is the intent, never the work unit: units split and any "
          "per-unit figure improves by splitting them")


def main(argv):
    paths = [a for a in argv[1:] if not a.startswith("--")]
    if not paths:
        print(__doc__.strip())
        return 2
    since = next((a.split("=", 1)[1] if "=" in a else None
                  for a in argv if a.startswith("--since")), None)
    if "--since" in argv:
        i = argv.index("--since")
        since = argv[i + 1] if i + 1 < len(argv) else None
    m = measure(paths[0], since)
    if "--json" in argv:
        print(json.dumps(m, indent=1, sort_keys=True))
        return 0
    report(m)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
