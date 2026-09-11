#!/usr/bin/env python3
"""Reading Python with the parser that ships with Python.

This reader is the floor: it needs nothing installed, so an estate written in
Python is never unread.

It also does the one thing that separates a reported dependency from a guess.
A call on a receiver — `session.mount(...)` — names a method and nothing else,
and a method name belongs to every file that happens to define one. Reported
that way it reaches all of them, and all but one of those reaches is false.
Where the receiver's type can be read off the file itself — it was assigned
from a constructor, it is annotated, it is `self` inside a class — the call is
reported as reaching that type instead, once and firmly.
"""
import ast

EXTENSIONS = {".py"}
NAME = "python"


def _bindings(tree):
    """Names in this file whose type the file itself states.

    Nothing here is inference across files: an assignment from a constructor,
    an annotation, a parameter's declared type. What cannot be read locally is
    left unknown rather than guessed at.
    """
    bound = {}

    def note(target, value):
        if isinstance(target, ast.Name) and isinstance(value, ast.Call):
            fn = value.func
            if isinstance(fn, ast.Name):
                bound[target.id] = fn.id
            elif isinstance(fn, ast.Attribute):
                bound[target.id] = fn.attr

    def annotation(name, node):
        if isinstance(node, ast.Name):
            bound[name] = node.id
        elif isinstance(node, ast.Attribute):
            bound[name] = node.attr
        elif isinstance(node, ast.Subscript):     # Optional[Thing], list[Thing]
            annotation(name, node.slice)

    for node in ast.walk(tree):
        if isinstance(node, ast.Assign):
            for t in node.targets:
                note(t, node.value)
        elif isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name):
            annotation(node.target.id, node.annotation)
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            args = node.args
            for a in list(args.args) + list(args.posonlyargs) + list(args.kwonlyargs):
                if a.annotation is not None:
                    annotation(a.arg, a.annotation)
    return bound


def _enclosing(tree):
    """Which class each line of the file sits in, so that `self` has a type."""
    span = {}
    for node in ast.walk(tree):
        if isinstance(node, ast.ClassDef):
            for line in range(node.lineno, (node.end_lineno or node.lineno) + 1):
                span.setdefault(line, node.name)
    return span


def read(rel, text):
    """One file: what it defines, what it calls, what it imports, and what this
    could not resolve at all.

    Each definition carries the lines it spans and each call the line it sits
    on, because an answer that names a file sends a reader to look for the
    thing, and an answer that names the lines hands it over.
    """
    defines, calls, unresolved = [], [], []
    try:
        tree = ast.parse(text)
    except SyntaxError:
        return [], [], ["file does not parse"]
    # Each import on its own line: attributing them all to the first one was
    # cheap and quoted the wrong line four times in five.
    imported = {}
    for node in ast.walk(tree):
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            mod = getattr(node, "module", None) or ""
            for a in node.names:
                imported.setdefault(a.name, node.lineno)
                if mod:
                    imported.setdefault(mod.split(".")[-1], node.lineno)
    bound, inside = _bindings(tree), _enclosing(tree)
    known = {n for n in imported} | {n.split(".")[-1] for n in imported}
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            defines.append((node.name, node.lineno, node.end_lineno or node.lineno))
            for base in getattr(node, "bases", []):
                # What a class inherits from is a dependency written down, and
                # in framework code it is often the only one there is.
                if isinstance(base, ast.Name):
                    calls.append((base.id, "name", base.lineno))
                elif isinstance(base, ast.Attribute):
                    calls.append((base.attr, "name", base.lineno))
        elif isinstance(node, ast.Call):
            f = node.func
            if isinstance(f, ast.Name):
                calls.append((f.id, "name", node.lineno))
            elif isinstance(f, ast.Attribute):
                receiver = f.value
                kind = None
                if isinstance(receiver, ast.Name):
                    if receiver.id == "self":
                        kind = inside.get(node.lineno)
                    elif receiver.id in bound:
                        kind = bound[receiver.id]
                    elif receiver.id in known or receiver.id[:1].isupper():
                        kind = receiver.id
                if kind:
                    # The receiver's type is stated in this file, so the call
                    # reaches that type firmly. The method name is kept beside
                    # it rather than dropped: a method inherited from a parent
                    # in another file is reached by the name and by nothing
                    # else, and deleting evidence to raise a figure is the one
                    # move this whole method exists to prevent.
                    calls.append((kind, "typed", node.lineno))
                    calls.append((f.attr, "attribute", node.lineno))
                else:
                    # A call on a receiver whose type is unknown. Discarding
                    # these was worse than the over-claiming it replaced: on a
                    # real codebase they are the majority form, and throwing
                    # them away understated what a change reaches by most of
                    # it. They are kept and weighed instead — a weak edge
                    # reported is safer than a strong edge omitted, because
                    # what is not reported is what nobody re-tests.
                    calls.append((f.attr, "attribute", node.lineno))
            else:
                unresolved.append("a call through something with no name")
    calls.extend((n, "import", line) for n, line in sorted(imported.items()))
    return defines, calls, unresolved
