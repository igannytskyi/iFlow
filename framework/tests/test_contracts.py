#!/usr/bin/env python3
"""Arbiter for the cross-repository contract join.

CR-012-01  an edge is joined only when both ends are in the estate, and says on
           what basis it was joined
CR-012-02  a consumer pointing at something no repository offers is reported as
           pointing outside the estate rather than as an absence of dependency
CR-012-03  traffic nobody predicted is reported as a gap in the estate
CR-012-04  observing an edge raises what is claimed for it
"""
import pathlib
import subprocess
import sys
import tempfile

from harness import ROOT

ESTATE = ROOT / "framework" / "estate.py"

TESTS = {
    "CR-012-01": "direct",
    "CR-012-02": "direct",
    "CR-012-03": "direct",
    "CR-012-04": "direct",
    "CR-012-05": "direct",
    "CR-012-06": "direct",
}


def build_estate(where, telemetry=""):
    """Two repositories that talk to each other, one that talks outside, and
    optionally what was actually seen. Constructed, never borrowed."""
    d = pathlib.Path(where)
    (d / "orders-api").mkdir(parents=True)
    (d / "orders-api" / "app.py").write_text(
        '@app.route("/orders")\ndef create(): ...\n')
    (d / "billing").mkdir(parents=True)
    (d / "billing" / "client.py").write_text(
        'def charge(): requests.post("http://orders-api/orders", json={})\n')
    (d / "mobile-bff").mkdir(parents=True)
    (d / "mobile-bff" / "gw.py").write_text(
        'def quote(): requests.get("http://pricing-v1/quote")\n')
    seen = d / "seen.txt"
    seen.write_text(telemetry)
    return d, seen


def run(where, telemetry=None):
    args = [sys.executable, str(ESTATE), "contracts", str(where)]
    if telemetry:
        args.append(str(telemetry))
    return subprocess.run(args, capture_output=True, text=True, cwd=ROOT).stdout


def cr_012_01():
    with tempfile.TemporaryDirectory() as tmp:
        d, _ = build_estate(tmp)
        out = run(d)
        if "billing → orders-api" not in out:
            return "an edge between two repositories in the estate was not joined"
        if "the address names the same repository" not in out:
            return "the edge was joined without saying on what basis"
    return None


def cr_012_02():
    with tempfile.TemporaryDirectory() as tmp:
        d, _ = build_estate(tmp)
        out = run(d)
        if "/quote" not in out or "outside it or missing from it" not in out:
            return "a consumer pointing at nothing in the estate was not reported"
    return None


def cr_012_03():
    with tempfile.TemporaryDirectory() as tmp:
        d, seen = build_estate(tmp, "analytics event order.shipped\n")
        out = run(d, seen)
        if "predicted by nothing" not in out or "analytics" not in out:
            return "traffic nobody predicted was not reported as a gap in the estate"
    return None


def cr_012_04():
    with tempfile.TemporaryDirectory() as tmp:
        d, seen = build_estate(tmp)
        before = [l for l in run(d).splitlines() if "billing → orders-api" in l][0]
        seen.write_text("billing http /orders\n")
        after = [l for l in run(d, seen).splitlines() if "billing → orders-api" in l][0]
        if "medium" not in before:
            return f"an unobserved edge was not claimed cautiously: {before.strip()}"
        if "high" not in after or "seen in traffic" not in after:
            return f"observing an edge did not raise what is claimed for it: {after.strip()}"
    return None


def cr_012_05():
    """A key too generic to identify anything does not join two services."""
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "one").mkdir()
        (d / "one" / "a.py").write_text('@app.route("/")\ndef root(): ...\n')
        (d / "two").mkdir()
        (d / "two" / "b.py").write_text('def hit(): requests.get("http://one/")\n')
        if "one → " in run(d) or "two → " in run(d):
            return "two services were joined on a key that identifies nothing"
    return None


def cr_012_06():
    """An estate bound by something this index does not read says so."""
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "svc").mkdir()
        (d / "svc" / "client.py").write_text(
            "stub = RecommendationServiceStub(channel)\n")
        (d / "svc" / "api.proto").write_text("service Recommendation {}\n")
        out = run(d)
        if "does not read" not in out:
            return "a contract kind this index cannot read was not named"
        if "not unconnected, they are unexamined" not in out:
            return "silence about an unread contract kind was not distinguished from absence"
    return None


def main():
    failures = []
    for name, fn in (("CR-012-01", cr_012_01), ("CR-012-02", cr_012_02),
                     ("CR-012-03", cr_012_03), ("CR-012-04", cr_012_04), ("CR-012-05", cr_012_05),
                     ("CR-012-06", cr_012_06)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
