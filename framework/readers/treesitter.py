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
    # Scripts are code an estate runs and nothing else reads: a deploy step, a
    # migration, the thing a pipeline calls. Left unread they are the part of
    # an estate that changes silently.
    ".sh": ("tree_sitter_bash", "language"),
    ".bash": ("tree_sitter_bash", "language"),
    ".zsh": ("tree_sitter_bash", "language"),
    ".ps1": ("tree_sitter_powershell", "language"),
    ".psm1": ("tree_sitter_powershell", "language"),
    ".psd1": ("tree_sitter_powershell", "language"),
}

# A shell script is often written with no extension at all — `bin/deploy`,
# `scripts/release`. What it is is written on its first line.
SHEBANG = {"sh": ".sh", "bash": ".sh", "zsh": ".sh", "dash": ".sh", "ksh": ".sh",
           "python": ".py", "python3": ".py", "ruby": ".rb", "node": ".js",
           "pwsh": ".ps1"}


def shebang(text):
    """The extension a file without one would have had, read off its first line."""
    first = text[:200].splitlines()[0] if text[:200].strip() else ""
    if not first.startswith("#!"):
        return None
    words = first[2:].replace("/", " ").split()
    for word in reversed(words):
        if word in SHEBANG:
            return SHEBANG[word]
    return None

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
CALLS = ("call", "invocation", "object_creation", "new_expression", "command")
# A node type that merely contains one of those words as a part: the name of a
# call is not a call, and its argument list is not one either.
NOT_CALLS = ("name", "argument", "element", "sep", "list", "chain", "clause",
             "body", "parameter", "declarator", "substitution")
# What a type says about where it came from. A class naming its parent depends
# on it as plainly as a call does, and in framework code it is often the only
# dependency there is: nothing calls a Rails model, it inherits from one.
INHERITS = ("superclass", "heritage", "extends", "implements", "base_list",
            "inherit", "super_interfaces", "supertypes", "derive", "impl_item")
IMPORTS = ("import", "use_declaration", "namespace_use_clause", "using_directive",
           "preproc_include", "include_statement")
# Ruby writes its imports as ordinary calls, and so do several others.
IMPORTING_CALLS = {"require", "require_relative", "load", "include_once", "import",
                   "source", ".", "import-module", "using", "dofile"}
IDENT = ("identifier", "name", "constant", "word")
# Words a grammar leaves in the callee position that name nothing.
# `require` and `import` are left out on purpose: where a grammar puts them in
# the callee position they are the import itself, and the call branch reads
# them as one.
KEYWORDS = {"new", "self", "this", "super", "return", "await", "yield",
            "typeof", "delete"}
# The words a grammar leaves inside an inheritance clause that name no type.
INHERIT_WORDS = {"extends", "implements", "impl", "for", "where", "class",
                 "interface", "public", "private", "protected", "abstract",
                 "sealed", "final", "static", "override", "open", "data"}


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
        target = next((c for c in node.children if c.is_named
                       and c.type not in ("comment",)), None)
    if target is None:
        return None, "name", ""
    raw = target.text.decode("utf-8", "replace").strip()
    if raw in (".", "&") or target.type.endswith("operator"):
        # A shell dot-sources a file with an operator where other languages
        # write a keyword. What it runs is the next thing along.
        after = [c for c in node.children if c.is_named and c is not target]
        raw = after[0].text.decode("utf-8", "replace").strip() if after else raw
        return raw.strip('"\'`'), "name", "."
    text = raw
    # A grammar that keeps the receiver in its own field says the same thing a
    # separator says in the text: this call goes through something whose type
    # is not known here.
    through = any(node.child_by_field_name(f) is not None
                  for f in ("object", "receiver", "operand", "instance"))
    qualified = through or any(sep in text for sep in (".", "::", "->", "\\"))
    if target.type.startswith("command_name") or node.type == "command":
        qualified = False        # a shell names what it runs; nothing is behind it
    tail = text if node.type == "command" else re.split(r"\.|::|->|\\", text)[-1]
    tail = tail.strip().strip('"\'`')
    if not tail or not (tail[0].isalpha() or tail[0] == "_") or tail in KEYWORDS:
        return None, "name", raw
    return tail, ("attribute" if qualified else "name"), raw


def _argument(node, callee):
    """What an import was given. Grammars name that child every way there is —
    a field called arguments, one called argument, a list of elements, or
    simply whatever follows the word."""
    for field in ("arguments", "argument", "value", "name"):
        got = node.child_by_field_name(field)
        if got is not None and got.text.decode("utf-8", "replace").strip() != callee:
            return got.text.decode("utf-8", "replace")
    rest = [c for c in node.children if c.is_named
            and c.text.decode("utf-8", "replace").strip() != callee]
    return rest[-1].text.decode("utf-8", "replace") if rest else ""


def _module_text(node):
    """The part of an import statement that names what is imported.

    Every language writes the module as a string or as the last dotted path in
    the statement, and everything else in there names symbols.
    """
    strings = []
    stack = [node]
    while stack:
        here = stack.pop()
        if "string" in here.type and "content" not in here.type:
            strings.append(here.text.decode("utf-8", "replace"))
        stack.extend(here.children)
    if strings:
        return strings[-1]
    return node.text.decode("utf-8", "replace")


def _module(text):
    """The part of an import that can be matched against a file.

    A path and a dotted module end differently: `./lib/common.sh` names the
    file `common` and `Az.Storage` names the module `Storage`. Splitting both
    on every separator at once made the first one name the extension.
    """
    text = text.strip().strip(";").strip()
    for word in ("import", "use", "using", "require_relative", "require", "from",
                 "include", "#include", "source", "Import-Module"):
        if text.lower().startswith(word.lower() + " "):
            text = text[len(word) + 1:]
    text = text.strip().strip('"\'`<>;,()').split(" as ")[0].split("{")[0].strip()
    text = text.strip('"\'`')
    if not text:
        return None
    if any(sep in text for sep in ("/", "\\")) or text.startswith((".", "~", "$")):
        parts = [p for p in re.split(r"[/\\]+", text) if p not in ("", ".", "..")]
        tail = parts[-1] if parts else ""
        tail = tail.rsplit(".", 1)[0] if "." in tail[1:] else tail
        return [p for p in (tail, parts[-2] if len(parts) > 1 else None) if p]
    # `use crate::search::Searcher` names a type inside a module, and the file
    # is as likely to be named for one as for the other. Both are offered; a
    # name that matches nothing costs nothing.
    parts = [p for p in re.split(r"[.:]+", text) if p]
    if not parts:
        return []
    return [p for p in (parts[-1], parts[-2] if len(parts) > 1 else None) if p]


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
        kind = node.type
        if not node.is_named or node.child_count == 0:
            continue                    # a keyword is a token, not a statement
        if any(k in kind for k in IMPORTS):
            # Read the statement whole and do not descend into it. Its parts
            # are named imports and clauses, and each of those carries the name
            # of a *symbol*: read as modules they filled the import graph with
            # class names, and a graph joined on class names joins nothing.
            for mod in _module(_module_text(node)):
                calls.append((mod, "import"))
            continue
        stack.extend(node.children)
        if any(k in kind for k in CALLS) and not any(k in kind for k in NOT_CALLS):
            name, how, raw = _callee(node)
            if (name or "").lower() in IMPORTING_CALLS or raw in IMPORTING_CALLS:
                for mod in _module(_argument(node, raw)):
                    calls.append((mod, "import"))
                continue
            if name:
                calls.append((name, how))
            else:
                unresolved.append("a call through something with no name")
        elif any(k in kind for k in INHERITS):
            # Only the header. Rust writes the whole implementation inside the
            # node that names the trait, and reading all of it would call every
            # word in the body a type this file inherits from.
            body = [c.start_byte for c in node.children
                    if any(w in c.type for w in ("body", "block", "declaration_list"))]
            head = node.text[:(min(body) - node.start_byte)] if body else node.text
            for word in re.findall(r"[A-Za-z_][A-Za-z0-9_]*",
                                   head.decode("utf-8", "replace")):
                if word not in KEYWORDS and word not in INHERIT_WORDS:
                    calls.append((word, "name"))

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
