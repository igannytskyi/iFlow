package flow

import (
	"context"
	"os"
	"slices"
	"time"

	"github.com/igannytskyi/iflow/internal/testrun"
)

// A test run is read from the report its runner writes for machines, never
// from what it writes for people: which tests ran, which passed, which
// failed, each by its name. A panic, a timeout, a failure in a goroutine,
// a path as the copy holds it — what a pattern over the runner's prose
// misses — the report states as the outcome of a named test.

// testRun is a run of tests: whether the run as a whole passed, what it
// said for people, and, where its runner reports for machines, how each
// test ended.
type testRun struct {
	Passed   bool
	Out      string
	Took     time.Duration
	Reported bool                       // the runner reported each test
	Results  map[string]testrun.Outcome // by test name, where Reported
	Unbuilt  string                     // what the build said, where the tests did not build and so did not run
	Cmd      []string                   // the command run, as it was run, where the runner reported
	Raw      string                     // what it wrote, its report among it, where the runner reported
	Why      string                     // why there is no report, where there is none
}

// report is how a language's runner is asked for a report of the tests
// named, and how the report is read: the command, given the file it may
// write the report to, and the reader, given what the command wrote and
// the copy it ran in.
type report struct {
	cmd  func(names []string, file string) []string
	read func(out []byte, file, dir string) (testrun.Report, error)
}

// runTests runs the tests named in dir as the estate's profile runs them,
// and reads how each ended from its runner's report where it writes one.
func runTests(ctx context.Context, dir string, prof profile, names []string) testRun {
	run := runOnce(ctx, dir, prof, names)
	// A test that panics ends the run where it stands: the tests not
	// reached were not run, not missing. They are run again, without the
	// ones that ended, until each has an outcome or a run reaches none.
	for run.Reported && run.Unbuilt == "" {
		left := run.notRun(names)
		if len(left) == 0 || len(left) == len(names) && len(run.Results) == 0 {
			break
		}
		more := runOnce(ctx, dir, prof, left)
		if !more.Reported || len(more.Results) == 0 {
			// The run ended reaching none of them: a goroutine's panic, which
			// no test is named for. Each is run alone; one whose run fails is
			// the one that failed, one whose run passes was found nowhere.
			for _, n := range left {
				one := runOnce(ctx, dir, prof, []string{n})
				switch {
				case len(one.Results) > 0:
					for k, o := range one.Results {
						run.Results[k] = o
					}
				case !one.Passed:
					run.Results[n] = testrun.Outcome{Output: one.Out}
				}
				run.Took += one.Took
			}
			break
		}
		for n, o := range more.Results {
			run.Results[n] = o
		}
		run.Out += more.Out
		run.Took += more.Took
		run.Passed = run.Passed && more.Passed
	}
	return run
}

// runOnce is one run of the tests named.
func runOnce(ctx context.Context, dir string, prof profile, names []string) testRun {
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
	cmd := prof.report.cmd(names, file) // as a claim names it: run wherever its tool is
	ok, out, took := runIn(ctx, dir, resolve(dir, cmd))
	rep, err := prof.report.read([]byte(out), file, dir)
	if err != nil {
		return testRun{Passed: ok, Out: out, Took: took, Why: "its report could not be read: " + err.Error()}
	}
	text := out
	if prof.name == "go test" {
		text = testrun.GoText([]byte(out)) // what go test says for people, out of its report
	}
	return testRun{Passed: ok, Out: text, Took: took, Reported: true, Results: rep.Results, Unbuilt: rep.Unbuilt, Cmd: cmd, Raw: out}
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
