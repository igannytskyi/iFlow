package flow

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/reader"
)

// language is everything the framework knows of one language, in one
// place: how an estate written in it is recognised, how its tests are run
// — all, by name, under a folder — how its code is built and one file of
// it checked, its one form, how a test is declared in it, and how a run of
// its tests names the line of a test file it failed at. A field left empty
// is something the language does not have, or the framework does not yet
// know of it; nothing is guessed in its place.
type language struct {
	name string
	// marks are files at an estate's root whose presence says it is in
	// this language; exts are the extensions of its source files.
	marks, exts []string
	// suite runs every test; only, the tests named; focus, the tests
	// matching a pattern under a folder, each said to pass or fail.
	suite []string
	only  func(names []string) []string
	focus func(pattern, path string) []string
	// build builds the whole estate, running nothing; vet builds the tests
	// of the files named, running none; check checks the files named, in a
	// moment, running nothing; format writes them in the one form.
	build  []string
	vet    func(files []string) []string
	check  func(files []string) []string
	format func(files []string) []string
	// tests is how a test is declared: its last group is the test's name.
	tests *regexp.Regexp
	// noneRan, for a runner that writes no report, is how it says it ran
	// no test at all; which tests failed is read from the report alone.
	noneRan *regexp.Regexp
	// fault is how its build and its runner name the line of a file they
	// blame — its first group the file; missing, whether such a line says a
	// name is missing from one of the packages named, the ones this work
	// writes into.
	fault   *regexp.Regexp
	missing func(line string, packages map[string]bool) bool
	// libraries is the folders of the source of the libraries an estate in
	// it uses, which an agent may read.
	libraries func() []string
	// declares, where the language has a parser of its own here, is every
	// name a file of it declares, fields among them; where it has none,
	// the estate's reader says what is declared, and a field it cannot see.
	declares func(src []byte) (map[string]bool, error)
	// declare is, by extension, how provisions are written into a file of
	// it without behaviour: the contract, which the framework declares
	// wherever a file's extension has a declarer.
	declare map[string]declarer
	// report, where the runner writes one for machines, runs the tests
	// named and reads how each ended: what every step that must know which
	// test failed reads, in place of the runner's prose.
	report *report
}

// languages is every language the framework knows, in the order an
// estate's marks are tried.
var languages = []language{
	{
		name: "go test", marks: []string{"go.mod"}, exts: []string{".go"},
		suite: []string{"go", "test", "-count=1", "./..."},
		only: func(n []string) []string {
			return []string{"go", "test", "-count=1", "-run", "^(" + strings.Join(n, "|") + ")$", "./..."}
		},
		focus: func(pattern, path string) []string {
			return []string{"go", "test", "-count=1", "-v", "-run", pattern, "./" + strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./") + "/..."}
		},
		report: &report{cmd: func(n []string, _ string) []string {
			return []string{"go", "test", "-json", "-count=1", "-run", "^(" + strings.Join(n, "|") + ")$", "./..."}
		}, read: goReport},
		build: []string{"go", "build", "./..."}, vet: goVet, check: goBuild, format: gofmt, declares: goDeclares, declare: map[string]declarer{".go": goDeclare},
		fault: regexp.MustCompile(`^\s*(?:# |vet: )?([\w./-]+\.go):\d+:\d+:`), missing: goMissing, libraries: goLibraries,
		tests:   regexp.MustCompile(`(?m)^func ((?:Test|Fuzz)\w*)\(`), // what -run runs: tests, and fuzz tests on their seeds
		noneRan: regexp.MustCompile(`\[no tests to run\]|\[no test files\]`),
	},
	{
		name: "pytest", marks: []string{"pyproject.toml", "pytest.ini", "setup.py"}, exts: []string{".py"},
		declare: map[string]declarer{".py": pyDeclare}, declares: pyDeclares,
		suite: []string{"python3", "-m", "pytest", "-q"},
		only: func(n []string) []string {
			return []string{"python3", "-m", "pytest", "-q", "-k", strings.Join(n, " or ")}
		},
		focus: func(pattern, path string) []string {
			return []string{"python3", "-m", "pytest", "-q", "-k", pattern, path}
		},
		report: &report{cmd: func(n []string, file string) []string {
			return []string{"python3", "-m", "pytest", "-q", "-p", "no:cacheprovider", "-k", strings.Join(n, " or "), "--junitxml=" + file}
		}, read: junitReport},
		check: func(files []string) []string {
			return append([]string{"python3", "-m", "py_compile"}, files...)
		},
		fault:   regexp.MustCompile(`File "([^"]+\.py)", line \d+`),
		tests:   regexp.MustCompile(`(?m)^\s*(?:async\s+)?def (test_\w+)\(`),
		noneRan: regexp.MustCompile(`(?m)^Ran 0 tests|no tests ran`),
	},
	{
		name: "node --test", marks: []string{"package.json"}, exts: []string{".js", ".cjs", ".mjs", ".ts", ".tsx", ".jsx"},
		declare: map[string]declarer{".js": jsDeclare, ".cjs": jsDeclare, ".mjs": jsDeclare},
		suite:   []string{"node", "--test"},
		only: func(n []string) []string {
			out := []string{"node", "--test"}
			for _, name := range n {
				out = append(out, "--test-name-pattern="+regexp.QuoteMeta(name))
			}
			return out
		},
		focus: func(pattern, path string) []string {
			return []string{"node", "--test", "--test-name-pattern=" + pattern, path}
		},
		report: &report{cmd: func(n []string, file string) []string {
			out := []string{"node", "--test", "--test-reporter=spec", "--test-reporter-destination=stdout", "--test-reporter=junit", "--test-reporter-destination=" + file}
			for _, name := range n {
				out = append(out, "--test-name-pattern="+regexp.QuoteMeta(name))
			}
			return out
		}, read: junitReport},
		check: func(files []string) []string {
			if len(files) != 1 || slices.Contains([]string{".ts", ".tsx", ".jsx"}, strings.ToLower(filepath.Ext(files[0]))) {
				return nil // node checks one script at a time, and none it would have to compile
			}
			return []string{"node", "--check", files[0]}
		},
		fault:   regexp.MustCompile(`([^\s()]+\.[cm]?[jt]sx?):\d+(?::\d+)?\b`),
		tests:   regexp.MustCompile("(?m)\\b(?:test|it)\\s*\\(\\s*([\"'`])(.+?)[\"'`]"),
		noneRan: regexp.MustCompile(`(?m)^\S*\s*tests 0\s*$`),
	},
	{
		name: "cargo test", marks: []string{"Cargo.toml"}, exts: []string{".rs"},
		suite:   []string{"cargo", "test"},
		tests:   regexp.MustCompile(`(?m)#\[test\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?(?:async\s+)?fn (\w+)`),
		noneRan: regexp.MustCompile(`(?m)^running 0 tests`),
	},
	{
		name: "gradle test", marks: []string{"build.gradle", "build.gradle.kts", "pom.xml"}, exts: []string{".java", ".kt", ".kts"},
		tests:   regexp.MustCompile(`(?m)@Test\b[^{;]*?\b(?:void\s+|fun\s+)(\w+)\s*\(`),
		noneRan: regexp.MustCompile(`No tests found`),
	},
}

// languageOfEstate is the language an estate's marks say it is in.
func languageOfEstate(repo string) (language, bool) {
	for _, l := range languages {
		for _, m := range l.marks {
			if _, err := os.Stat(filepath.Join(repo, m)); err == nil {
				return l, true
			}
		}
	}
	return language{}, false
}

// languageOfFile is the language a file is written in, by its extension.
func languageOfFile(file string) (language, bool) {
	ext := strings.ToLower(filepath.Ext(file))
	for _, l := range languages {
		if slices.Contains(l.exts, ext) {
			return l, true
		}
	}
	return language{}, false
}

// quickCheck is the check of the files named in the language of the first
// of them that has one, of that language's files among them.
func quickCheck(files []string) []string {
	for _, f := range files {
		l, ok := languageOfFile(f)
		if !ok || l.check == nil {
			continue
		}
		var mine []string
		for _, g := range files {
			if lg, ok := languageOfFile(g); ok && lg.name == l.name {
				mine = append(mine, g)
			}
		}
		return l.check(mine)
	}
	return nil
}

// testsIn is the names of the tests a file declares, read as its language
// declares them.
func testsIn(file, content string) []string {
	l, ok := languageOfFile(file)
	if !ok || l.tests == nil {
		return nil
	}
	var out []string
	for _, m := range l.tests.FindAllStringSubmatch(content, -1) {
		out = append(out, m[len(m)-1])
	}
	return out
}

// goDeclares is every name a Go file declares: its functions and methods,
// its types, the fields and methods of its structs and interfaces, its
// variables and constants.
func goDeclares(src []byte) (map[string]bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	fieldsOf := func(fl *ast.FieldList) {
		for _, fd := range fl.List {
			for _, id := range fd.Names {
				names[id.Name] = true
			}
		}
	}
	// What the file declares is what its package does: its top-level
	// declarations, and the fields and methods of the types among them —
	// never a parameter, never a function's own variable.
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			names[d.Name.Name] = true
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					names[spec.Name.Name] = true
					switch t := spec.Type.(type) {
					case *ast.StructType:
						fieldsOf(t.Fields)
					case *ast.InterfaceType:
						fieldsOf(t.Methods)
					}
				case *ast.ValueSpec:
					for _, id := range spec.Names {
						names[id.Name] = true
					}
				}
			}
		}
	}
	return names, nil
}

// declaredIn is every name a file declares, as its language's parser reads
// it, or the estate's reader where it has none — and whether fields are
// among what was read.
func declaredIn(file string, src []byte) (names map[string]bool, fields bool) {
	if l, ok := languageOfFile(file); ok && l.declares != nil {
		if names, err := l.declares(src); err == nil {
			return names, true
		}
	}
	names = map[string]bool{}
	for _, s := range reader.Read(file, src).Symbols {
		names[s.Name] = true
	}
	return names, false
}

// undeclared is what the provisions say is declared that the files in dir
// do not declare: read by the language's own parser where it has one, by
// the estate's reader where not. A field a reader cannot see is not said
// missing — it cannot be told either way.
func undeclared(dir string, provides []meta.Provision) []string {
	var out []string
	for _, p := range provides {
		if p.Kind == meta.Described {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p.File)))
		if err != nil {
			out = append(out, p.Name)
			continue
		}
		names, fields := declaredIn(p.File, src)
		if p.Kind == meta.Field && !fields {
			continue // a field its reader cannot see, neither said there nor missing
		}
		if !names[p.Name] {
			out = append(out, p.Name)
		}
	}
	return out
}

// goMissing says a Go build's line says a name is missing that this work
// could declare: one of no package, or of a package it writes into. A name
// missing from a library — key.NameN, a method of widget.Editor — is no
// name any unit can declare: it is a mistake in the file that uses it.
func goMissing(line string, packages map[string]bool) bool {
	for _, re := range []*regexp.Regexp{goUndefined, goNoMember} {
		if m := re.FindStringSubmatch(line); m != nil {
			pkg := strings.Join(m[1:], "")
			return pkg == "" || packages[pkg]
		}
	}
	return strings.Contains(line, "has no field or method")
}

var (
	goUndefined = regexp.MustCompile(`undefined: (?:(\w+)\.)?\w+`)
	goNoMember  = regexp.MustCompile(`\(type \*?(?:(\w+)\.)?\w+ has no field or method|unknown field \w+ in struct literal of type \*?(?:(\w+)\.)?\w+`)
)

// goLibraries is where the source of the modules a Go estate uses is kept,
// and the standard library's.
var goLibraries = sync.OnceValue(func() []string {
	out, err := exec.Command("go", "env", "GOMODCACHE", "GOROOT").Output()
	if err != nil {
		return nil
	}
	var dirs []string
	for _, l := range strings.Fields(string(out)) {
		dirs = append(dirs, filepath.Clean(l))
	}
	return dirs
})

// faultsOf is how the language of a unit's scope reads a check's output:
// each line that blames a file, and whether it says a name is missing that
// the unit's packages are to declare.
func faultsOf(l language, scope []string) func(string) []harness.Fault {
	if l.fault == nil {
		return nil
	}
	packages := map[string]bool{}
	for _, p := range scope {
		packages[filepath.Base(filepath.Dir(filepath.FromSlash(p)))] = true
	}
	return func(out string) []harness.Fault {
		var fs []harness.Fault
		for _, line := range strings.Split(out, "\n") {
			m := l.fault.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			fs = append(fs, harness.Fault{File: m[1], Text: strings.TrimSpace(line), Missing: l.missing != nil && l.missing(line, packages)})
		}
		return fs
	}
}

// scopeLanguage is the language of the first file of a scope that is in one.
func scopeLanguage(scope []string) (language, bool) {
	for _, f := range scope {
		if l, ok := languageOfFile(f); ok {
			return l, true
		}
	}
	return language{}, false
}

// nothingRan says a run of the tests named shows that none of them ran:
// none is named in what it said, and it says, in a language's words, that
// it ran nothing.
func nothingRan(out string, names []string) bool {
	if slices.ContainsFunc(names, func(n string) bool { return strings.Contains(out, n) }) {
		return false
	}
	return slices.ContainsFunc(languages, func(l language) bool { return l.noneRan != nil && l.noneRan.MatchString(out) })
}

// pyNames lists, one a line, every name a Python module declares: its
// functions, classes and assignments, and within classes their methods,
// attributes and what their methods assign to self.
const pyNames = `import ast, sys
tree = ast.parse(sys.stdin.read())
out = set()
def targets(t):
    if isinstance(t, ast.Name): out.add(t.id)
    elif isinstance(t, ast.Attribute) and isinstance(t.value, ast.Name) and t.value.id == "self": out.add(t.attr)
    elif isinstance(t, (ast.Tuple, ast.List)):
        for e in t.elts: targets(e)
def walk(body, top):
    for n in body:
        if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            out.add(n.name)
            if isinstance(n, ast.ClassDef): walk(n.body, False)
            else:
                for m in ast.walk(n):
                    if isinstance(m, (ast.Assign, ast.AnnAssign, ast.AugAssign)):
                        for t in (m.targets if isinstance(m, ast.Assign) else [m.target]):
                            if isinstance(t, ast.Attribute): targets(t)
        elif isinstance(n, ast.Assign):
            for t in n.targets: targets(t)
        elif isinstance(n, ast.AnnAssign): targets(n.target)
walk(tree.body, True)
print("\n".join(sorted(out)))`

// pyDeclares is every name a Python file declares, by Python's own parser.
func pyDeclares(src []byte) (map[string]bool, error) {
	cmd := exec.Command("python3", "-I", "-c", pyNames)
	cmd.Stdin = bytes.NewReader(src)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, n := range strings.Fields(string(out)) {
		names[n] = true
	}
	return names, nil
}
