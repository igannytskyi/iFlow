#!/usr/bin/env python3
"""Arbiter for SPEC-002. Run: python3 tests/test_row_filter.py

CR-002-01  a row a person wrote is judged, whatever punctuation it contains
CR-002-02  an unfilled template row is still not mistaken for data
CR-002-03  the gate's verdict on every existing change is unchanged
"""
import importlib.util
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent


def load_gate():
    spec = importlib.util.spec_from_file_location("gate", ROOT / "tools" / "check.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def rows_of(gate, table_text):
    return gate.tables(table_text)[0][1]


def cr_002_01(gate):
    """Written rows survive, whatever punctuation they carry."""
    written = [
        ("two-sided bound", "| CR-9 | Queue depth stays > 0 and < 100 under load | machine | test-run | always |"),
        ("comparison only", "| CR-9 | Latency < 200 ms at p99 | machine | test-run | always |"),
        ("arrow in prose", "| CR-9 | The old shape -> the new shape, with no gap | machine | test-run | always |"),
    ]
    head = "| Id | Criterion | Procedure | Required evidence | Threshold |\n|---|---|---|---|---|\n"
    bad = []
    for name, line in written:
        rows = rows_of(gate, head + line + "\n")
        if len(gate.real(rows)) != 1:
            bad.append(name)
    return bad and f"written rows dropped: {', '.join(bad)}" or None


def cr_002_02(gate):
    """Unfilled template rows are still dropped."""
    leaked = []
    for t in sorted((ROOT / "templates").glob("*.md")):
        for _, rows in gate.tables(t.read_text()):
            for r in gate.real(rows):
                joined = " ".join(v for v in r.values() if v)
                if "<" in joined and ">" in joined:
                    leaked.append(f"{t.name}: {joined[:50]}")
    return leaked and f"template rows kept as data: {leaked}" or None


def cr_002_03(_gate):
    """Every existing change still passes the gate."""
    broken = []
    for folder in sorted((ROOT / "changes").iterdir()):
        if not folder.is_dir():
            continue
        out = subprocess.run([sys.executable, str(ROOT / "tools" / "check.py"), str(folder)],
                             capture_output=True, text=True)
        if out.returncode != 0:
            broken.append(f"{folder.name}: {out.stdout.strip().splitlines()[-1]}")
    return broken and f"gate verdict moved: {broken}" or None


def main():
    gate = load_gate()
    failures = []
    for name, fn in (("CR-002-01", cr_002_01), ("CR-002-02", cr_002_02), ("CR-002-03", cr_002_03)):
        problem = fn(gate)
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
