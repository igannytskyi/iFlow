#!/usr/bin/env python3
"""Arbiter for the readers — what a file is read as, in every language.

CR-018-01  every reader answers in one shape, whatever the language, so that
           what is derived from one can be graded beside what is derived from
           another
CR-018-02  a language this install claims to read yields what an index needs
           from it: what the file defines, what it imports, and what it calls.
           A language claimed and not checked here is named rather than assumed
CR-018-03  nothing is claimed that cannot be read, and asking for a language
           nobody reads returns nothing rather than failing
CR-018-04  the derivation says which readers produced an answer, so a cache
           cannot serve what one reader concluded as what another would
CR-018-05  a file with no extension is read as what its first line says it is,
           and one that says nothing is read by nobody
CR-018-06  a module is named by what it is: a path names the file at its end, a
           dotted name names the last thing in it, and a statement naming
           several modules yields all of them
"""
import pathlib
import sys
import tempfile

from harness import ROOT

sys.path.insert(0, str(ROOT / "framework"))
import readers                                       # noqa: E402
from readers import python_ast, treesitter           # noqa: E402

TESTS = {
    "CR-018-01": "direct",
    "CR-018-02": ("proxy", "one small file per language stands for every file written in "
                           "it; what is established is that the grammar is wired up and "
                           "yields all three kinds, not that it reads every construct"),
    "CR-018-03": "direct",
    "CR-018-04": "direct",
    "CR-018-05": "direct",
    "CR-018-06": "direct",
}

KINDS = {"name", "attribute", "typed", "import"}

# One file per language, written the way that language is written. Where a
# sample is missing for something this install reads, CR-018-02 says so.
SAMPLES = {
    ".py": "import os\n\n\nclass Thing:\n    def run(self):\n        return helper(os.sep)\n",
    ".go": 'package m\n\nimport "net/http"\n\nfunc Run() { helper(); http.Get("/x") }\n',
    ".java": "import com.example.Other;\n\nclass Thing { void run() { helper(); } }\n",
    ".cs": "using System.Linq;\n\npublic class Thing { public void Run() { Helper.Do(); } }\n",
    ".ts": 'import { Other } from "./other";\n\nexport class Thing { run() { helper(Other); } }\n',
    ".tsx": 'import { Other } from "./other";\n\nexport function Thing() { return helper(Other); }\n',
    ".js": 'const other = require("./other");\n\nfunction thing() { return helper(other); }\n',
    ".jsx": 'import { Other } from "./other";\n\nexport function Thing() { return helper(Other); }\n',
    ".rs": "use crate::other::Other;\n\npub fn run() { helper(Other::new()); }\n",
    ".rb": "require 'other'\n\nclass Thing\n  def run\n    helper\n  end\nend\n",
    ".php": "<?php\nuse App\\Other;\n\nclass Thing { function run() { helper(); } }\n",
    ".c": '#include "other.h"\n\nint run(void) { return helper(); }\n',
    ".cpp": '#include "other.h"\n\nclass Thing { public: int run() { return helper(); } };\n',
    ".kt": "import other.Other\n\nclass Thing { fun run() { helper() } }\n",
    ".scala": "import other.Other\n\nclass Thing { def run() = helper() }\n",
    ".swift": "import Other\n\nclass Thing { func run() { helper() } }\n",
    ".sh": "source ./other.sh\n\nrun() { helper; }\n",
    ".ps1": '. "$PSScriptRoot\\other.ps1"\n\nfunction Run-Thing { Invoke-Helper }\n',
}


def cr_018_01():
    for ext, src in sorted(SAMPLES.items()):
        if ext not in readers.by_extension():
            continue
        answer = readers.read("sample" + ext, src)
        if not isinstance(answer, tuple) or len(answer) != 3:
            return f"{ext} was not answered in three parts"
        defines, calls, unresolved = answer
        if not all(isinstance(x, list) for x in answer):
            return f"{ext} answered with something other than lists"
        if any(not isinstance(n, str) for n in defines):
            return f"{ext} named a definition with something that is not a name"
        for entry in calls:
            if not (isinstance(entry, tuple) and len(entry) == 2):
                return f"{ext} reported a call in a shape the index cannot read"
            if entry[1] not in KINDS:
                return f"{ext} reported a call of kind {entry[1]!r}, which is not one of {KINDS}"
    return None


def cr_018_02():
    read_here = set(readers.by_extension())
    unchecked = sorted(read_here - set(SAMPLES))
    for ext in sorted(read_here & set(SAMPLES)):
        defines, calls, _ = readers.read("sample" + ext, SAMPLES[ext])
        kinds = {k for _, k in calls}
        if not defines:
            return f"{ext} is claimed and a file written in it defines nothing"
        if "import" not in kinds:
            return f"{ext} is claimed and an import written in it is not read as one"
        if not (kinds & {"name", "attribute", "typed"}):
            return f"{ext} is claimed and a call written in it is not read as one"
    if unchecked:
        print(f"        ({len(unchecked)} language(s) read here and not checked by this "
              f"arbiter: {' '.join(unchecked)})")
    return None


def cr_018_03():
    claimed = readers.by_extension()
    for ext, (module, _fn) in treesitter.GRAMMARS.items():
        if ext in claimed and treesitter._grammar(ext) is None:
            return f"{ext} is claimed and no grammar for it loads"
    defines, calls, unresolved = readers.read("thing.unknown-language", "whatever this is\n")
    if defines or calls or unresolved:
        return "a language nobody reads was answered for anyway"
    if readers.claims("thing.unknown-language") is not None:
        return "a language nobody reads was claimed by a reader"
    return None


def cr_018_04():
    before = readers.derivation()
    if before != readers.derivation():
        return "the derivation is not stable when nothing changes"
    original = pathlib.Path(python_ast.__file__).read_text()
    try:
        pathlib.Path(python_ast.__file__).write_text(original + "\n# a reader changed\n")
        if readers.derivation() == before:
            return "the derivation did not change when a reader did"
    finally:
        pathlib.Path(python_ast.__file__).write_text(original)
    if readers.derivation() != before:
        return "the derivation did not come back when the reader did"
    return None


def cr_018_05():
    if ".sh" not in readers.by_extension():
        return None
    with tempfile.TemporaryDirectory() as tmp:
        d = pathlib.Path(tmp)
        (d / "deploy").write_text("#!/usr/bin/env bash\nsource ./other.sh\nrun() { helper; }\n")
        (d / "notes").write_text("this is not a program\n")
        if readers.claims(d / "deploy") is None:
            return "a script that says what it is on its first line was read by nobody"
        if readers.claims(d / "notes") is not None:
            return "a file that says nothing about itself was claimed anyway"
        defines, calls, _ = readers.read("deploy", (d / "deploy").read_text())
        if "run" not in defines:
            return "a script named by its first line was claimed and then not read"
    return None


def cr_018_06():
    """Three ways this has been wrong, each found on a real estate.

    A path split on every separator at once named the extension rather than the
    file. A dotted module read as a path named nothing. A grouped import read
    by its last string lost every other dependency the file had.
    """
    for text, want in (
            ('"./lib/common.sh"', "common"),      # a path names the file at its end
            ('"Az.Storage"', "Storage"),          # a dotted name names the last thing in it
            ('"net/http"', "http"),
            ('"../utils/helpers.js"', "helpers"),
    ):
        got = treesitter._module(text)
        if want not in got:
            return f"{text} was read as {got}, which does not name {want}"
    if not reads(".go"):
        return None
    grouped = 'package m\n\nimport (\n\t"fmt"\n\t"x/one"\n\t"x/two"\n)\n\nfunc Go() {}\n'
    modules = {n for n, kind in treesitter.read("x.go", grouped)[1] if kind == "import"}
    for want in ("fmt", "one", "two"):
        if want not in modules:
            return f"a grouped import naming three modules yielded {sorted(modules)}"
    return None


def reads(ext):
    return ext in readers.by_extension()


def main():
    failures = []
    for name, fn in (("CR-018-01", cr_018_01), ("CR-018-02", cr_018_02),
                     ("CR-018-03", cr_018_03), ("CR-018-04", cr_018_04),
                     ("CR-018-05", cr_018_05), ("CR-018-06", cr_018_06)):
        problem = fn()
        print(f"  {'FAIL' if problem else 'ok  '}  {name}" + (f"  — {problem}" if problem else ""))
        if problem:
            failures.append(name)
    print(f"  {'passed' if not failures else str(len(failures)) + ' failed'}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
