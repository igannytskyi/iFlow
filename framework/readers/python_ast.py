#!/usr/bin/env python3
"""Reading Python with the parser that ships with Python.

This reader is the floor: it needs nothing installed, so an estate written in
Python is never unread. It sees one thing the general reader does not — a call
on a receiver whose type is unknown — and that form is the majority on real
code, so it stays the reader for Python rather than being replaced by the
general one.
"""
import ast

EXTENSIONS = {".py"}
NAME = "python"


def read(rel, text):
    """One file: what it defines, what it calls, what it imports, and what this
    could not resolve at all."""
    defines, calls, unresolved = [], [], []
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return [], [], ["file does not parse"]
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
            defines.append(node.name)
        elif isinstance(node, ast.Call):
            f = node.func
            if isinstance(f, ast.Name):
                calls.append((f.id, "name"))
            elif isinstance(f, ast.Attribute):
                # A method called on something whose type is unknown. Discarding
                # these was worse than the over-claiming it replaced: on a real
                # codebase they are the majority form, and throwing them away
                # understated what a change reaches by most of it. They are kept
                # and weighed instead — a weak edge reported is safer than a
                # strong edge omitted, because what is not reported is what
                # nobody re-tests.
                calls.append((f.attr, "attribute"))
            else:
                unresolved.append("a call through something with no name")
    calls.extend((n, "import") for n in sorted(imported))
    return defines, calls, unresolved
