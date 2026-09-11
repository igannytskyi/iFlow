#!/usr/bin/env python3
"""Who can read what.

The index does not know how to parse anything. It asks whoever can read a file
to say what that file defines, what it refers to and what it imports, and it
grades every answer the same way regardless of which reader produced it.

The seam matters more than the readers behind it. What is unread is computed
from what no installed reader covers, so installing one moves files out of the
unseen count and removing one moves them back — the estate never claims to have
read what nothing read.
"""
import hashlib
import pathlib

from . import python_ast, treesitter

READERS = [python_ast, treesitter]


def by_extension():
    """extension → the reader that claims it. The first claim wins, so the
    reader that needs nothing installed is asked first."""
    claimed = {}
    for reader in READERS:
        got = reader.EXTENSIONS if hasattr(reader, "EXTENSIONS") else reader.extensions()
        for ext in got:
            claimed.setdefault(ext, reader)
    return claimed


_peeked = {}


def claims(path):
    """Which reader takes this file.

    The extension decides it, except where there is none: a deploy script, a
    release step, the thing a pipeline runs are written as `bin/deploy` with
    the language on the first line. Reading that line is the difference
    between an estate whose scripts are part of it and one where they are not.
    """
    path = pathlib.Path(path)
    known = by_extension()
    if path.suffix in known:
        return known[path.suffix]
    if path.suffix:
        return None
    key = str(path)
    if key not in _peeked:
        try:
            with open(path, "r", errors="replace") as f:
                alias = treesitter.shebang(f.read(200))
        except OSError:
            alias = None
        _peeked[key] = alias
    alias = _peeked[key]
    return known.get(alias) if alias else None


def readable(path):
    return claims(path) is not None


def read(rel, text):
    """What this file defines, refers to and imports — or nothing at all, if no
    reader here can read it."""
    known = by_extension()
    suffix = pathlib.Path(rel).suffix
    reader = known.get(suffix)
    if reader is None and not suffix:
        alias = treesitter.shebang(text)
        reader, rel = known.get(alias), rel + (alias or "")
    if reader is None:
        return [], [], []
    return reader.read(rel, text)


def derivation():
    """A short name for *how* things are being read right now.

    The cache asks whether the source moved. That is only half the question: an
    answer derived by one reader is not the answer another would give, and
    installing a grammar changes every file it now covers. Without this the
    index went on serving what a regular expression had concluded about a file
    a parser could read, and nothing said so.
    """
    h = hashlib.sha256()
    for reader in READERS:
        h.update(pathlib.Path(reader.__file__).read_bytes())
    for ext, reader in sorted(by_extension().items()):
        h.update(f"{ext}:{reader.NAME}".encode())
    return h.hexdigest()[:12]


def who():
    """Which reader is doing what, for an answer that has to say where it came
    from."""
    out = {}
    for ext, reader in sorted(by_extension().items()):
        out.setdefault(reader.NAME, []).append(ext)
    return out
