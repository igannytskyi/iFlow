// Package testrun reads a test runner's report for machines: which tests
// ran, which passed, which failed, which did not build — each by its name.
// What a runner writes for people is never read for this: a panic, a
// timeout, a failure in a goroutine, a path as a copy holds it are where a
// pattern over prose misses, and the report states them as the outcome of
// a named test. The flow reads a run by it, and the gate repeats a claim by
// it: a run's result is its tests' outcomes, not its words.
package testrun

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Outcome is how one test ended, and what it said.
type Outcome struct {
	Passed, Skipped bool
	Output          string
}

// Report is a runner's report, read: how each test ended, and what the
// build said where the tests did not build.
type Report struct {
	Results map[string]Outcome
	Unbuilt string
}

// goEvent is one line of `go test -json`, as cmd/test2json writes it.
type goEvent struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

// ReadGo reads `go test -json`: each test's last pass, fail or skip, and
// what it said. A subtest is its parent's: the ledger names top-level tests.
func ReadGo(out []byte, _, _ string) (Report, error) {
	res := map[string]Outcome{}
	said := map[string]*strings.Builder{}
	var unbuilt strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	events := 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue // what the go command says outside the report
		}
		var e goEvent
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		events++
		switch {
		case e.Action == "build-output":
			unbuilt.WriteString(e.Output)
			continue
		case e.Test == "":
			continue
		}
		name, _, _ := strings.Cut(e.Test, "/")
		if said[name] == nil {
			said[name] = &strings.Builder{}
		}
		switch e.Action {
		case "output":
			said[name].WriteString(e.Output)
		case "pass", "fail", "skip":
			if e.Test != name {
				continue // a subtest: its parent's outcome stands for it
			}
			res[name] = Outcome{Passed: e.Action == "pass", Skipped: e.Action == "skip"}
		}
	}
	if err := sc.Err(); err != nil {
		return Report{}, err
	}
	if events == 0 && len(bytes.TrimSpace(out)) > 0 {
		return Report{}, errors.New("go test wrote no report: " + lastLines(string(out), 3))
	}
	for name, o := range res {
		if b := said[name]; b != nil {
			o.Output = b.String()
			res[name] = o
		}
	}
	return Report{Results: res, Unbuilt: unbuilt.String()}, nil
}

// GoText is what `go test -json` would have said to people: its output
// lines in order, and what the go command said besides.
func GoText(out []byte) string {
	var b strings.Builder
	for _, line := range strings.Split(string(out), "\n") {
		var e goEvent
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &e) == nil {
			if e.Action == "output" || e.Action == "build-output" {
				b.WriteString(e.Output)
			}
			continue
		}
		if line != "" {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// junitCase is one <testcase> of a JUnit XML report.
type junitCase struct {
	Name    string     `xml:"name,attr"`
	Failure *junitText `xml:"failure"`
	Error   *junitText `xml:"error"`
	Skipped *junitText `xml:"skipped"`
	Out     string     `xml:"system-out"`
	Err     string     `xml:"system-err"`
}

// junitText is a failure, an error or a skip: its message and its text.
type junitText struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func (t *junitText) String() string {
	if t == nil {
		return ""
	}
	return strings.TrimSpace(t.Message + "\n" + t.Text)
}

// junitSuite is a <testsuite> or <testsuites>: its cases and the suites
// within it, as deep as a runner nests them.
type junitSuite struct {
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

// ReadJUnit reads a JUnit XML report from file, or from every report in
// the folder file names under dir: each case by its name, a parameter or
// a pair of brackets after it taken off, as the ledger names the test.
func ReadJUnit(_ []byte, file, dir string) (Report, error) {
	var files []string
	if st, err := os.Stat(file); err == nil && st.Size() > 0 {
		files = []string{file}
	} else if m, _ := filepath.Glob(filepath.Join(dir, "build", "test-results", "*", "*.xml")); len(m) > 0 {
		files = m // Gradle writes one report a test class, where it always does
	} else if m, _ := filepath.Glob(filepath.Join(dir, "target", "surefire-reports", "*.xml")); len(m) > 0 {
		files = m // and Maven
	}
	if len(files) == 0 {
		return Report{}, errors.New("the runner wrote no JUnit report")
	}
	res := map[string]Outcome{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return Report{}, err
		}
		var s junitSuite
		if err := xml.Unmarshal(b, &s); err != nil {
			return Report{}, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		var walk func(junitSuite)
		walk = func(s junitSuite) {
			for _, c := range s.Cases {
				name := c.Name
				if i := strings.IndexAny(name, "[("); i > 0 {
					name = name[:i] // test_x[1], testX(): the test the ledger names
				}
				o, seen := res[name]
				failed := c.Failure != nil || c.Error != nil
				if !seen {
					o = Outcome{Passed: !failed && c.Skipped == nil, Skipped: c.Skipped != nil && !failed}
				} else if failed {
					o.Passed, o.Skipped = false, false // one case of it failing fails the test
				}
				o.Output += c.Failure.String() + c.Error.String() + c.Out + c.Err
				res[name] = o
			}
			for _, sub := range s.Suites {
				walk(sub)
			}
		}
		walk(s)
	}
	return Report{Results: res}, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
