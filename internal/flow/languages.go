package flow

import (
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
	// faults is how a run of its tests names the line of a test file;
	// failed, how it names a test that failed — its last group the name;
	// noneRan, how it says it ran no test at all.
	faults, failed, noneRan *regexp.Regexp
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
		build: []string{"go", "build", "./..."}, vet: goVet, check: goBuild, format: gofmt, declares: goDeclares,
		fault: regexp.MustCompile(`^\s*(?:# |vet: )?([\w./-]+\.go):\d+:\d+:`), missing: goMissing, libraries: goLibraries,
		tests:   regexp.MustCompile(`(?m)^func (Test\w+)\(`),
		faults:  regexp.MustCompile(`[\w./-]*_test\.go:\d+`),
		failed:  regexp.MustCompile(`(?m)^\s*--- FAIL: (\w+)`),
		noneRan: regexp.MustCompile(`\[no tests to run\]|\[no test files\]`),
	},
	{
		name: "pytest", marks: []string{"pyproject.toml", "pytest.ini", "setup.py"}, exts: []string{".py"},
		suite: []string{"python3", "-m", "pytest", "-q"},
		only: func(n []string) []string {
			return []string{"python3", "-m", "pytest", "-q", "-k", strings.Join(n, " or ")}
		},
		focus: func(pattern, path string) []string {
			return []string{"python3", "-m", "pytest", "-q", "-k", pattern, path}
		},
		check: func(files []string) []string {
			return append([]string{"python3", "-m", "py_compile"}, files...)
		},
		fault:   regexp.MustCompile(`File "([^"]+\.py)", line \d+`),
		tests:   regexp.MustCompile(`(?m)^\s*(?:async\s+)?def (test_\w+)\(`),
		faults:  regexp.MustCompile(`[\w./-]*(?:test_\w+|\w+_test)\.py:\d+|File "[^"]*(?:test_\w+|\w+_test)\.py", line \d+`),
		failed:  regexp.MustCompile(`(?m)^(?:FAILED [^\s:]+::(?:\w+::)?(\w+)|(?:FAIL|ERROR): (\w+) \()`),
		noneRan: regexp.MustCompile(`(?m)^Ran 0 tests|no tests ran`),
	},
	{
		name: "node --test", marks: []string{"package.json"}, exts: []string{".js", ".cjs", ".mjs", ".ts", ".tsx", ".jsx"},
		suite: []string{"node", "--test"},
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
		check: func(files []string) []string {
			if len(files) != 1 || slices.Contains([]string{".ts", ".tsx", ".jsx"}, strings.ToLower(filepath.Ext(files[0]))) {
				return nil // node checks one script at a time, and none it would have to compile
			}
			return []string{"node", "--check", files[0]}
		},
		fault:   regexp.MustCompile(`([^\s()]+\.[cm]?[jt]sx?):\d+(?::\d+)?\b`),
		tests:   regexp.MustCompile("(?m)\\b(?:test|it)\\s*\\(\\s*([\"'`])(.+?)[\"'`]"),
		faults:  regexp.MustCompile(`[\w./-]*\.(?:test|spec)\.[cm]?[jt]sx?:\d+`),
		failed:  regexp.MustCompile(`(?m)^\s*(?:✖|not ok \d+ -) (.+?)(?: \([\d.]+ms\))?$`),
		noneRan: regexp.MustCompile(`(?m)^\S*\s*tests 0\s*$`),
	},
	{
		name: "cargo test", marks: []string{"Cargo.toml"}, exts: []string{".rs"},
		suite:   []string{"cargo", "test"},
		tests:   regexp.MustCompile(`(?m)#\[test\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?(?:async\s+)?fn (\w+)`),
		faults:  regexp.MustCompile(`[\w./-]*\.rs:\d+`),
		failed:  regexp.MustCompile(`(?m)^test (?:\S+::)?(\w+) \.\.\. FAILED`),
		noneRan: regexp.MustCompile(`(?m)^running 0 tests`),
	},
	{
		name: "gradle test", marks: []string{"build.gradle", "build.gradle.kts", "pom.xml"}, exts: []string{".java", ".kt", ".kts"},
		tests:   regexp.MustCompile(`(?m)@Test\b[^{;]*?\b(?:void\s+|fun\s+)(\w+)\s*\(`),
		faults:  regexp.MustCompile(`[\w./-]*Test\.(?:java|kt):\d+`),
		failed:  regexp.MustCompile(`(?m)^\S+ > (\w+)\(\) FAILED`),
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

// testFaults is the lines of a test run's output that say where in a test
// file it failed, in any language the framework knows, the first few.
func testFaults(out string) string {
	var at []string
	for _, line := range strings.Split(out, "\n") {
		if len(at) == 6 {
			break
		}
		if slices.ContainsFunc(languages, func(l language) bool { return l.faults != nil && l.faults.MatchString(line) }) {
			at = append(at, strings.TrimSpace(line))
		}
	}
	return strings.Join(at, "\n")
}

// failedTests is the names of the tests a run says failed, in any language
// the framework knows.
func failedTests(out string) []string {
	var names []string
	for _, l := range languages {
		if l.failed == nil {
			continue
		}
		for _, m := range l.failed.FindAllStringSubmatch(out, -1) {
			for i := len(m) - 1; i > 0; i-- {
				if m[i] != "" {
					names = append(names, m[i])
					break
				}
			}
		}
	}
	return names
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
	ast.Inspect(f, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.FuncDecl:
			names[d.Name.Name] = true
		case *ast.TypeSpec:
			names[d.Name.Name] = true
		case *ast.ValueSpec:
			for _, id := range d.Names {
				names[id.Name] = true
			}
		case *ast.Field: // of a struct, or a method of an interface
			for _, id := range d.Names {
				names[id.Name] = true
			}
		}
		return true
	})
	return names, nil
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
		if l, ok := languageOfFile(p.File); ok && l.declares != nil {
			if names, err := l.declares(src); err != nil || !names[p.Name] {
				out = append(out, p.Name)
			}
			continue
		}
		if p.Kind == meta.Field {
			continue
		}
		if !slices.ContainsFunc(reader.Read(p.File, src).Symbols, func(s reader.Symbol) bool { return s.Name == p.Name }) {
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
