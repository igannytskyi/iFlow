package flow

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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
	// failed, how it names a test that failed — its last group the name.
	faults, failed *regexp.Regexp
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
		build: []string{"go", "build", "./..."}, vet: goVet, check: goBuild, format: gofmt,
		tests:  regexp.MustCompile(`(?m)^func (Test\w+)\(`),
		faults: regexp.MustCompile(`[\w./-]*_test\.go:\d+`),
		failed: regexp.MustCompile(`(?m)^\s*--- FAIL: (\w+)`),
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
		tests:  regexp.MustCompile(`(?m)^\s*(?:async\s+)?def (test_\w+)\(`),
		faults: regexp.MustCompile(`[\w./-]*(?:test_\w+|\w+_test)\.py:\d+|File "[^"]*(?:test_\w+|\w+_test)\.py", line \d+`),
		failed: regexp.MustCompile(`(?m)^(?:FAILED [^\s:]+::(?:\w+::)?(\w+)|(?:FAIL|ERROR): (\w+) \()`),
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
		tests:  regexp.MustCompile("(?m)\\b(?:test|it)\\s*\\(\\s*([\"'`])(.+?)[\"'`]"),
		faults: regexp.MustCompile(`[\w./-]*\.(?:test|spec)\.[cm]?[jt]sx?:\d+`),
		failed: regexp.MustCompile(`(?m)^\s*(?:✖|not ok \d+ -) (.+?)(?: \([\d.]+ms\))?$`),
	},
	{
		name: "cargo test", marks: []string{"Cargo.toml"}, exts: []string{".rs"},
		suite:  []string{"cargo", "test"},
		tests:  regexp.MustCompile(`(?m)#\[test\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?(?:async\s+)?fn (\w+)`),
		faults: regexp.MustCompile(`[\w./-]*\.rs:\d+`),
		failed: regexp.MustCompile(`(?m)^test (?:\S+::)?(\w+) \.\.\. FAILED`),
	},
	{
		name: "gradle test", marks: []string{"build.gradle", "build.gradle.kts", "pom.xml"}, exts: []string{".java", ".kt", ".kts"},
		tests:  regexp.MustCompile(`(?m)@Test\b[^{;]*?\b(?:void\s+|fun\s+)(\w+)\s*\(`),
		faults: regexp.MustCompile(`[\w./-]*Test\.(?:java|kt):\d+`),
		failed: regexp.MustCompile(`(?m)^\S+ > (\w+)\(\) FAILED`),
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
