#!/usr/bin/env python3
"""Reading every other language with the grammar its own community maintains.

The alternative was a parser per language written here, which is how a small
tool becomes a large one and still trails what it copies. tree-sitter grammars
already exist for every language an estate is written in, and each one ships a
`tags.scm` — the query that names what a file defines and what it refers to,
the same query code hosts use for navigation. So the per-language work is not a
parser: it is a line in a table saying which grammar reads which extension.

Nothing here is required. The grammars are installed beside this file rather
than vendored into the repository, because a compiled grammar belongs to one
machine and a method has to travel. Without them this reader offers no
extensions at all and every one of those languages is reported unread, which is
what it was before — a missing reader must degrade to honesty, never to silence.
"""
import pathlib
import re
import sys

LIB = pathlib.Path(__file__).resolve().parent / "_lib"
NAME = "tree-sitter"

# extension → (module, the language function to ask it for). A grammar that
# offers several dialects names the one to use.
GRAMMARS = {
    ".go": ("tree_sitter_go", "language"),
    ".java": ("tree_sitter_java", "language"),
    ".cs": ("tree_sitter_c_sharp", "language"),
    ".rs": ("tree_sitter_rust", "language"),
    ".js": ("tree_sitter_javascript", "language"),
    ".jsx": ("tree_sitter_javascript", "language"),
    ".mjs": ("tree_sitter_javascript", "language"),
    ".cjs": ("tree_sitter_javascript", "language"),
    ".ts": ("tree_sitter_typescript", "language_typescript"),
    ".tsx": ("tree_sitter_typescript", "language_tsx"),
    ".rb": ("tree_sitter_ruby", "language"),
    ".php": ("tree_sitter_php", "language_php"),
    ".c": ("tree_sitter_c", "language"),
    ".h": ("tree_sitter_c", "language"),
    ".cpp": ("tree_sitter_cpp", "language"),
    ".cc": ("tree_sitter_cpp", "language"),
    ".cxx": ("tree_sitter_cpp", "language"),
    ".hpp": ("tree_sitter_cpp", "language"),
    ".hh": ("tree_sitter_cpp", "language"),
    ".kt": ("tree_sitter_kotlin", "language"),
    ".swift": ("tree_sitter_swift", "language"),
    ".scala": ("tree_sitter_scala", "language"),
}

_loaded = {}
_missing = set()


def _grammar(ext):
    """The parser and query for one extension, or None if it is not installed."""
    if ext in _loaded:
        return _loaded[ext]
    if ext in _missing or ext not in GRAMMARS:
        return None
    if LIB.is_dir() and str(LIB) not in sys.path:
        sys.path.insert(0, str(LIB))
    module, fn = GRAMMARS[ext]
    try:
        from tree_sitter import Language, Parser, Query
        mod = __import__(module)
        lang = Language(getattr(mod, fn)())
        tags = _tags(module)
        try:
            compiled = Query(lang, tags) if tags else None
        except Exception:
            compiled = None       # a query this version refuses is not a language lost
        _loaded[ext] = (Parser(lang), compiled)
    except Exception:
        # A grammar that is not installed, or one whose query this version of
        # tree-sitter refuses, is a language this reader does not read. It is
        # not a failure to report at the point of a query: it is a gap the
        # coverage note already states.
        _missing.add(ext)
        return None
    return _loaded[ext]


def _tags(module):
    """The grammar's own tags query, wherever it ships it.

    Not always beside the module it belongs to: a grammar whose name carries a
    hyphen ships its queries under the hyphenated spelling and its code under
    the underscored one, so the two are matched on the same normal form.
    """
    want = module.replace("-", "_")
    for base in sorted(LIB.iterdir()) if LIB.is_dir() else []:
        if base.is_dir() and base.name.replace("-", "_") == want:
            q = base / "queries" / "tags.scm"
            if q.exists():
                return q.read_text()
    return None


def extensions():
    """What this reader can read *here* — which is what is installed, not what
    the table above hopes for."""
    return {ext for ext in GRAMMARS if _grammar(ext) is not None}


# Reading a tree without knowing the language.
#
# A grammar's own `tags.scm` was the first plan and it is not enough: the
# TypeScript one covers only what TypeScript adds to JavaScript, so a plain
# function or a call matched nothing at all, and Kotlin ships no tags query
# whatever. What is consistent across grammars is not their queries — it is
# the names they give their nodes. A function declaration is called something
# containing "function" in every one of them, a call is called something
# containing "call" or "invocation", and the thing being named sits in a field
# called "name". That regularity is what is read here, so a language needs a
# grammar and nothing else.
DEFINES = ("function", "method", "class", "struct", "interface", "trait", "enum",
           "module", "constructor", "record", "protocol", "object_declaration",
           "type_alias", "namespace_definition", "package_declaration")
NOT_DEFINES = ("call", "invocation", "expression", "type", "parameter", "argument")
CALLS = ("call", "invocation", "object_creation", "new_expression")
IMPORTS = ("import", "use_declaration", "namespace_use_clause", "using_directive",
           "preproc_include", "include_statement")
# Ruby writes its imports as ordinary calls, and so do several others.
IMPORTING_CALLS = {"require", "require_relative", "load", "include_once", "import"}
IDENT = ("identifier", "name", "constant", "word")
# Words a grammar leaves in the callee position that name nothing.
# `require` and `import` are left out on purpose: where a grammar puts them in
# the callee position they are the import itself, and the call branch reads
# them as one.
KEYWORDS = {"new", "self", "this", "super", "return", "await", "yield",
            "typeof", "delete"}


def _name_of(node):
    """What a node names: the field a grammar sets aside for it, or the last
    identifier under it when the grammar sets none."""
    got = node.child_by_field_name("name")
    if got is not None:
        return got.text.decode("utf-8", "replace")
    last = None
    for child in node.children:
        if child.type.endswith(IDENT):
            last = child
    return last.text.decode("utf-8", "replace") if last is not None else None


def _callee(node):
    """The name a call reaches, and how firmly it is known.

    A bare name is one thing in the file's own scope. A name behind a dot, an
    arrow or a pair of colons is a member of something this reader cannot type,
    which is the same weak ground a Python attribute call stands on and is
    graded the same way.
    """
    target = (node.child_by_field_name("function")
              or node.child_by_field_name("method")
              or node.child_by_field_name("constructor")
              or node.child_by_field_name("type")
              or node.child_by_field_name("name"))
    if target is None:
        target = next((c for c in node.children if c.is_named), None)
    if target is None:
        return None, "name"
    text = target.text.decode("utf-8", "replace")
    # A grammar that keeps the receiver in its own field says the same thing a
    # separator says in the text: this call goes through something whose type
    # is not known here.
    through = any(node.child_by_field_name(f) is not None
                  for f in ("object", "receiver", "operand", "instance"))
    qualified = through or any(sep in text for sep in (".", "::", "->", "\\"))
    tail = re.split(r"\.|::|->|\\", text)[-1].strip()
    if not tail or not (tail[0].isalpha() or tail[0] == "_") or tail in KEYWORDS:
        return None, "name"
    return tail, ("attribute" if qualified else "name")


def _module(text):
    """The part of an import that can be matched against a file: its last
    segment. A path, a dotted package and a scoped namespace all end in the
    name of the thing being imported."""
    text = text.strip().strip(";").strip()
    for word in ("import", "use", "using", "require_relative", "require", "from",
                 "include", "#include"):
        if text.startswith(word + " "):
            text = text[len(word) + 1:]
    text = text.strip().strip('"\'`<>;')
    tail = re.split(r"[/\\.:]+", text.split(" as ")[0].split("{")[0])[-1]
    return tail.strip() or None


def read(rel, text):
    got = _grammar(pathlib.Path(rel).suffix)
    if got is None:
        return [], [], []
    parser, query = got
    defines, calls, unresolved = [], [], []
    try:
        tree = parser.parse(text.encode("utf-8", "replace"))
    except Exception:
        return [], [], ["file does not parse"]
    stack = [tree.root_node]
    while stack:
        node = stack.pop()
        stack.extend(node.children)
        kind = node.type
        if not node.is_named or node.child_count == 0:
            continue                    # a keyword is a token, not a statement
        if any(k in kind for k in CALLS):
            name, how = _callee(node)
            if name in IMPORTING_CALLS:
                arg = node.child_by_field_name("arguments")
                mod = _module(arg.text.decode("utf-8", "replace")) if arg is not None else None
                if mod:
                    calls.append((mod, "import"))
                continue
            if name:
                calls.append((name, how))
            else:
                unresolved.append("a call through something with no name")
        elif any(k in kind for k in IMPORTS):
            mod = _module(node.text.decode("utf-8", "replace"))
            if mod:
                calls.append((mod, "import"))
        elif any(k in kind for k in DEFINES) and not any(k in kind for k in NOT_DEFINES):
            name = _name_of(node)
            if name:
                defines.append(name)
    # The grammar's own tags query, where it has one, is asked as well: it knows
    # declarations peculiar to its language that no general rule catches.
    if query is not None:
        from tree_sitter import QueryCursor
        for _, caps in QueryCursor(query).matches(tree.root_node):
            names = caps.get("name") or []
            if not names:
                continue
            name = names[0].text.decode("utf-8", "replace").strip('"\'`')
            if any(k.startswith("definition.") for k in caps if k != "name"):
                defines.append(name)
    return list(dict.fromkeys(defines)), calls, unresolved
