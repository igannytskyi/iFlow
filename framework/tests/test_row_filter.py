#!/usr/bin/env python3
"""Arbiter for SPEC-002. Run: python3 framework/tests/test_row_filter.py

CR-002-01  a row a person wrote is judged, whatever punctuation its text contains
CR-002-02  an unfilled template row is still not mistaken for data
CR-002-03  a change built from the current shapes passes the gate
"""
import importlib.util
import sys
import tempfile

from harness import GATE, TEMPLATES, build_change, gate


def _gate_module():
    spec = importlib.util.spec_from_file_location("gate", GATE)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def cr_002_01():
    m = _gate_module()
    head = "| Id | Criterion | Procedure |\n|---|---|---|\n"
    written = {
        "two-sided bound": "| CR-9 | depth stays > 0 and < 100 | machine |",
        "comparison only": "| CR-9 | latency < 200 ms | machine |",
        "arrow in prose": "| CR-9 | the old shape -> the new one | machine |",
    }
    bad = [name for name, row in written.items()
           if len(m.real(m.tables(head + row + "\n")[0][1])) != 1]
    return bad and f"written rows dropped: {', '.join(bad)}" or None


def cr_002_02():
    m = _gate_module()
    leaked = []
    for t in sorted(TEMPLATES.glob("*.md")):
        for _, rows in m.tables(t.read_text()):
            for r in m.real(rows):
                joined = " ".join(v for v in r.values() if v)
                if "<" in joined and ">" in joined:
                    leaked.append(f"{t.name}: {joined[:40]}")
    return leaked and f"template rows kept as data: {leaked}" or None


def cr_002_03():
    with tempfile.TemporaryDirectory() as tmp:
        out = gate(build_change(tmp + "/c"))
        return "FAIL" in out and f"a change built from the current shapes does not pass:\n{out}" or None


def main():
    failures = []
    for name, fn in (("CR-002-01", cr_002_01), ("CR-002-02", cr_002_02), ("CR-002-03", cr_002_03)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
