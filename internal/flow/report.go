package flow

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A test run is read from the report its runner writes for machines, never
// from what it writes for people: which tests ran, which passed, which
// failed, each by its name. A panic, a timeout, a failure in a goroutine,
// a path as the copy holds it — what a pattern over the runner's prose
// misses — the report states as the outcome of a named test.

// testOutcome is how one test ended, and what it said.
type testOutcome struct {
	Passed, Skipped bool
	Output          string
}

// testRun is a run of tests: whether the run as a whole passed, what it
// said for people, and, where its runner reports for machines, how each
// test ended.
type testRun struct {
	Passed   bool
	Out      string
	Took     time.Duration
	Reported bool                   // the runner reported each test
	Results  map[string]testOutcome // by test name, where Reported
	Unbuilt  string                 // what the build said, where the tests did not build and so did not run
	Why      string                 // why there is no report, where there is none
}

// testReport is a runner's report, read: how each test ended, and what the
// build said where the tests did not build.
type testReport struct {
	Results map[string]testOutcome
	Unbuilt string
}

// report is how a language's runner is asked for a report of the tests
// named, and how the report is read: the command, given the file it may
// write the report to, and the reader, given what the command wrote and
// the copy it ran in.
type report struct {
	cmd  func(names []string, file string) []string
	read func(out []byte, file, dir string) (testReport, error)
}

// runTests runs the tests named in dir as the estate's profile runs them,
// and reads how each ended from its runner's report where it writes one.
func runTests(ctx context.Context, dir string, prof profile, names []string) testRun {
	if prof.report == nil {
		ok, out, took := runIn(ctx, dir, resolve(dir, prof.only(names)))
		return testRun{Passed: ok, Out: out, Took: took, Why: prof.name + " writes no report this framework reads; the run is judged as a whole"}
	}
	f, err := os.CreateTemp("", "iflow-report-*")
	if err != nil {
		ok, out, took := runIn(ctx, dir, resolve(dir, prof.only(names)))
		return testRun{Passed: ok, Out: out, Took: took, Why: "no file for the report: " + err.Error()}
	}
	file := f.Name()
	_ = f.Close()
	defer os.Remove(file)
	ok, out, took := runIn(ctx, dir, resolve(dir, prof.report.cmd(names, file)))
	rep, err := prof.report.read([]byte(out), file, dir)
	if err != nil {
		return testRun{Passed: ok, Out: out, Took: took, Why: "its report could not be read: " + err.Error()}
	}
	text := out
	if prof.name == "go test" {
		text = goText([]byte(out)) // what go test says for people, out of its report
	}
	return testRun{Passed: ok, Out: text, Took: took, Reported: true, Results: rep.Results, Unbuilt: rep.Unbuilt}
}

// failed is those of names that did not pass: failed, or not run at all.
// Where the run was not reported test by test, it is every name, if the
// run failed.
func (t testRun) failed(names []string) []string {
	if !t.Reported {
		if t.Passed {
			return nil
		}
		return slices.Clone(names)
	}
	var out []string
	for _, n := range names {
		if o, ok := t.Results[n]; !ok || (!o.Passed && !o.Skipped) {
			out = append(out, n)
		}
	}
	return out
}

// notRun is those of names the report says nothing of: tests the runner
// did not find where they were written.
func (t testRun) notRun(names []string) []string {
	if !t.Reported || t.Unbuilt != "" {
		return nil // not run because they did not build: that is said by Unbuilt
	}
	var out []string
	for _, n := range names {
		if _, ok := t.Results[n]; !ok {
			out = append(out, n)
		}
	}
	return out
}

// passing is those of names that passed.
func (t testRun) passing(names []string) []string {
	if !t.Reported {
		if t.Passed {
			return slices.Clone(names)
		}
		return nil
	}
	var out []string
	for _, n := range names {
		if o := t.Results[n]; o.Passed {
			out = append(out, n)
		}
	}
	return out
}

// goEvent is one line of `go test -json`, as cmd/test2json writes it.
type goEvent struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

// goReport reads `go test -json`: each test's last pass, fail or skip, and
// what it said. A subtest is its parent's: the ledger names top-level tests.
func goReport(out []byte, _, _ string) (testReport, error) {
	res := map[string]testOutcome{}
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
			res[name] = testOutcome{Passed: e.Action == "pass", Skipped: e.Action == "skip"}
		}
	}
	if err := sc.Err(); err != nil {
		return testReport{}, err
	}
	if events == 0 && len(bytes.TrimSpace(out)) > 0 {
		return testReport{}, errors.New("go test wrote no report: " + lastLines(string(out), 3))
	}
	for name, o := range res {
		if b := said[name]; b != nil {
			o.Output = b.String()
			res[name] = o
		}
	}
	return testReport{Results: res, Unbuilt: unbuilt.String()}, nil
}

// goText is what `go test -json` would have said to people: its output
// lines in order, and what the go command said besides.
func goText(out []byte) string {
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

// junitReport reads a JUnit XML report from file, or from every report in
// the folder file names under dir: each case by its name, a parameter or
// a pair of brackets after it taken off, as the ledger names the test.
func junitReport(_ []byte, file, dir string) (testReport, error) {
	var files []string
	if st, err := os.Stat(file); err == nil && st.Size() > 0 {
		files = []string{file}
	} else if m, _ := filepath.Glob(filepath.Join(dir, "build", "test-results", "*", "*.xml")); len(m) > 0 {
		files = m // Gradle writes one report a test class, where it always does
	} else if m, _ := filepath.Glob(filepath.Join(dir, "target", "surefire-reports", "*.xml")); len(m) > 0 {
		files = m // and Maven
	}
	if len(files) == 0 {
		return testReport{}, errors.New("the runner wrote no JUnit report")
	}
	res := map[string]testOutcome{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return testReport{}, err
		}
		var s junitSuite
		if err := xml.Unmarshal(b, &s); err != nil {
			return testReport{}, fmt.Errorf("%s: %w", filepath.Base(f), err)
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
					o = testOutcome{Passed: !failed && c.Skipped == nil, Skipped: c.Skipped != nil && !failed}
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
	return testReport{Results: res}, nil
}
