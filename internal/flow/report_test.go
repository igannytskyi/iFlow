package flow

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A Go run is read from go test's own report: a test that passes, one that
// fails, one whose goroutine panics, and one not found, each by its name.
func TestAGoRunIsReadFromItsReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for f, body := range map[string]string{
		"go.mod": "module example.com/greet\n\ngo 1.22\n",
		"greet_test.go": `package greet

import (
	"testing"
	"time"
)

func TestPasses(t *testing.T) {}

func TestFails(t *testing.T) { t.Fatal("wrong greeting") }

func TestSub(t *testing.T) {
	t.Parallel()
	t.Run("inner", func(t *testing.T) {})
}

func TestPanicsFirst(t *testing.T) { panic("not yet: Shout") }

func TestReachedAfterAPanic(t *testing.T) {}

func TestPanicsInAGoroutine(t *testing.T) {
	t.Parallel()
	go func() { panic("boom") }()
	time.Sleep(time.Second)
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prof := profile{language: languages[0]}
	names := []string{"TestPanicsFirst", "TestPasses", "TestFails", "TestSub", "TestReachedAfterAPanic", "TestPanicsInAGoroutine", "TestNowhere"}
	run := runTests(context.Background(), dir, prof, names)
	if !run.Reported {
		t.Fatalf("the run was not read from its report: %s\n%s", run.Why, run.Out)
	}
	// A panic ends the binary; the tests after it are run again, not taken
	// to be missing.
	if got := run.passing(names); !slices.Equal(got, []string{"TestPasses", "TestSub", "TestReachedAfterAPanic"}) {
		t.Errorf("passing %q", got)
	}
	failed := run.failed(names)
	if !slices.Contains(failed, "TestFails") || !slices.Contains(failed, "TestNowhere") || slices.Contains(failed, "TestPasses") {
		t.Errorf("failed %q", failed)
	}
	// A goroutine's panic ends the binary: the test running then, and every
	// test not reached, did not pass — none is taken to have.
	if slices.Contains(run.passing(names), "TestPanicsInAGoroutine") {
		t.Error("the test whose goroutine panicked was taken to pass")
	}
	if !slices.Contains(failed, "TestPanicsFirst") {
		t.Errorf("the test that panicked is not failed: %q", failed)
	}
	if got := run.notRun(names); !slices.Equal(got, []string{"TestNowhere"}) {
		t.Errorf("not run %q", got)
	}
	if o := run.Results["TestFails"]; o.Passed || !contains(o.Output, "wrong greeting") {
		t.Errorf("what the failing test said was not kept: %+v", o)
	}

	// Tests that do not build did not run: the report says the build's
	// words, and no test is taken to be missing.
	if err := os.WriteFile(filepath.Join(dir, "greet_test.go"), []byte("package greet\n\nimport \"testing\"\n\nfunc TestPasses(t *testing.T) { undefinedThing() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run = runTests(context.Background(), dir, prof, []string{"TestPasses"})
	if !run.Reported || !contains(run.Unbuilt, "undefined: undefinedThing") || len(run.notRun([]string{"TestPasses"})) > 0 || len(run.failed([]string{"TestPasses"})) != 1 {
		t.Errorf("a build that failed was not read as such: %+v", run)
	}
}

// A JUnit report — pytest's, node's nested suites — is read case by case,
// a case's parameters taken off its name.
func TestAJUnitReportIsReadCaseByCase(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "report.xml")
	xml := `<?xml version="1.0"?>
<testsuites>
  <testsuite name="pytest">
    <testcase classname="tests.test_greet" name="test_greets"/>
    <testcase classname="tests.test_greet" name="test_shouts[1]"/>
    <testcase classname="tests.test_greet" name="test_shouts[2]"><failure message="assert">AssertionError: HELLO</failure></testcase>
    <testcase classname="tests.test_greet" name="test_later"><skipped message="not yet"/></testcase>
  </testsuite>
  <testsuite name="node">
    <testsuite name="greeting">
      <testcase name="greet names the person"/>
      <testcase name="says goodbye"><error message="TypeError">boom</error></testcase>
    </testsuite>
  </testsuite>
</testsuites>`
	if err := os.WriteFile(file, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := junitReport(nil, file, "")
	if err != nil {
		t.Fatal(err)
	}
	res := rep.Results
	run := testRun{Reported: true, Results: res}
	names := []string{"test_greets", "test_shouts", "test_later", "greet names the person", "says goodbye"}
	if got := run.failed(names); !slices.Equal(got, []string{"test_shouts", "says goodbye"}) {
		t.Errorf("failed %q", got)
	}
	if !res["test_later"].Skipped || !contains(res["test_shouts"].Output, "AssertionError") {
		t.Errorf("read %+v", res)
	}
	if _, err := junitReport(nil, filepath.Join(t.TempDir(), "none.xml"), t.TempDir()); err == nil {
		t.Error("a report that was not written was read")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
