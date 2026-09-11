#!/usr/bin/env python3
"""Arbiter for the cross-repository contract join.

CR-012-01  an edge is joined only when both ends are in the estate, and says on
           what basis it was joined
CR-012-02  a consumer pointing at something no repository offers is reported as
           pointing outside the estate rather than as an absence of dependency
CR-012-03  traffic nobody predicted is reported as a gap in the estate
CR-012-04  observing an edge raises what is claimed for it
CR-012-07  a route declared in test ground is a stand-in, not an offer, and is
           read as evidence that the repository depends on it
CR-012-08  offers declared in a language this index cannot parse are read as
           offers rather than left out of the estate
CR-012-09  one repository declaring a route twice is one offer, and a key that
           several repositories offer keeps all of them
CR-012-10  what is shaped like a route and is not one — a dotted module path, a
           snippet in documentation — does not become a contract
CR-012-11  two spellings of one route are one key, and a call that lands inside a
           route a repository offers is joined to it rather than called absent
CR-012-12  a call a repository makes to its own routes is not reported as
           crossing a boundary, and routes assembled at import time are reported
           as unresolved rather than as offering nothing
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
    "CR-012-07": "direct",
    "CR-012-08": ("proxy", "one route form per language stands for the many ways each "
                  "framework writes a route; what is tested is that an unparsed language "
                  "is read at all, not that every form of it is recognised"),
    "CR-012-09": "direct",
    "CR-012-10": "direct",
    "CR-012-11": "direct",
    "CR-012-12": "direct",
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


def cr_012_07():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "orders").mkdir()
        (d / "orders" / "app.py").write_text('@app.route("/orders")\ndef create(): ...\n')
        (d / "orders" / "tests").mkdir()
        (d / "orders" / "tests" / "fake_payment.py").write_text(
            '@app.route("/paymentAuth", methods=["POST"])\ndef auth(): ...\n')
        (d / "payment").mkdir()
        (d / "payment" / "api.py").write_text('@app.route("/paymentAuth")\ndef auth(): ...\n')
        out = run(d)
        if "payment offers http /paymentAuth and nothing" in out:
            return "a stand-in in test ground was not read as a dependency on the real one"
        if "orders → payment" not in out or "stand-in" not in out:
            return "the repository keeping the stand-in was not joined to the one offering it"
        if "orders offers http /paymentAuth" in out:
            return "a route faked for a test was reported as an offer of the repository"
    return None


def cr_012_08():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "gateway").mkdir()
        (d / "gateway" / "main.go").write_text(
            'func main() {\n\tr.HandleFunc(baseUrl+"/orders", handler)\n}\n')
        (d / "probe").mkdir()
        (d / "probe" / "check.py").write_text(
            'def hit(): requests.get("http://gateway/orders")\n')
        out = run(d)
        if "probe → gateway" not in out:
            return "an offer declared in a language this index cannot parse was not read"
    return None


def cr_012_09():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "front").mkdir()
        (d / "front" / "api.py").write_text(
            '@app.route("/login", methods=["GET"])\ndef form(): ...\n\n'
            '@app.route("/login", methods=["POST"])\ndef submit(): ...\n')
        (d / "users").mkdir()
        (d / "users" / "api.py").write_text('@app.route("/login")\ndef login(): ...\n')
        (d / "probe").mkdir()
        (d / "probe" / "check.py").write_text('def hit(): requests.get("/login")\n')
        out = run(d)
        if "offered by front, users" not in out:
            return "a key offered by two repositories did not name both"
        reach = subprocess.run(
            [sys.executable, str(ESTATE), "reachability", str(d), "users"],
            capture_output=True, text=True, cwd=ROOT).stdout
        if "taken by probe" not in reach:
            return "a repository offering a contested key was told nobody consumes it"
        if reach.count("users offers http /login") != 1:
            return "one route declared twice was reported as two offers"
    return None


def cr_012_10():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "svc").mkdir()
        (d / "svc" / "tests").mkdir()
        (d / "svc" / "tests" / "test_it.py").write_text(
            '@mock.patch("svc.clusters.cluster.get_local_nodes")\ndef t(): ...\n')
        (d / "svc" / "docs").mkdir()
        (d / "svc" / "docs" / "onboarding.go").write_text('r.GET("/foo", handler)\n')
        (d / "caller").mkdir()
        (d / "caller" / "c.py").write_text('def hit(): requests.get("/foo")\n')
        out = run(d)
        if "cluster" in out:
            return "a dotted module path in a decorator was read as a route"
        if "→ svc" in out:
            return "a route in a documentation snippet was read as an offer"
        if "not read" not in out:
            return "documentation and examples were skipped without saying so"
    return None


def cr_012_11():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "api").mkdir()
        (d / "api" / "app.py").write_text(
            '@app.route("/api/orders/<order_id>")\ndef one(): ...\n')
        (d / "web").mkdir()
        (d / "web" / "client.py").write_text(
            'def one(): requests.get("/api/Orders/{id}")\n'
            'def all(): requests.get("/api/orders/7/items")\n')
        out = run(d)
        if out.count("web → api") < 2:
            return f"a route written in two spellings was not joined: {out}"
        if "covers it" not in out and "cover it" not in out:
            return "a call landing inside an offered route did not say what covered it"
        # The other direction: the caller's literals match nothing but
        # themselves, or a route of parameters swallows every call in the estate.
        (d / "api" / "subs.py").write_text(
            '@app.route("/<org>/<project>/subscriptions")\ndef subs(): ...\n')
        (d / "web" / "unrelated.py").write_text(
            'def go(): requests.get("/user/orgs")\n')
        out = run(d)
        if "/user/orgs" in out and "web → api" in out.split("/user/orgs")[0].splitlines()[-1]:
            return "a call was joined to a route that matches it only through parameters"
    return None


def cr_012_12():
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "site").mkdir()
        (d / "site" / "app.py").write_text(
            '@app.route("/internal/status")\ndef st(): ...\n'
            'def check(): requests.get("/internal/status")\n')
        (d / "site" / "urls.py").write_text(
            're_path(r"^projectconfigs/$", View.as_view()),\n')
        out = run(d)
        if "cross no boundary" not in out:
            return "a call to a route the repository offers itself was not recognised"
        if "pointing outside" not in out or "1 consumer(s) pointing outside" in out:
            return "an internal call was still counted as pointing outside the estate"
        if "assembled at import time" not in out:
            return "routes declared as fragments were not reported as unresolved"
    return None


def main():
    failures = []
    for name, fn in (("CR-012-01", cr_012_01), ("CR-012-02", cr_012_02),
                     ("CR-012-03", cr_012_03), ("CR-012-04", cr_012_04), ("CR-012-05", cr_012_05),
                     ("CR-012-06", cr_012_06), ("CR-012-07", cr_012_07),
                     ("CR-012-08", cr_012_08), ("CR-012-09", cr_012_09),
                     ("CR-012-10", cr_012_10), ("CR-012-11", cr_012_11),
                     ("CR-012-12", cr_012_12)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
