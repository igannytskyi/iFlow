package flow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/harness"
)

// realCase is what a real run left, kept to be judged again: the commit of
// the framework it ran on, the candidates laid over it in order, the tests
// run, and how the framework must read their run.
type realCase struct {
	About  string   `json:"about"`
	Commit string   `json:"commit"`
	Layers []string `json:"layers"`
	Tests  []string `json:"tests"`
	Failed []string `json:"failed"`
	NotRun []string `json:"not_run"`
}

// What real runs left — a real contract, real tests, on the framework's own
// repository as it stood — is judged again by the framework as it stands:
// free, and with what a real run does, not what a stub imagines.
func TestRealRunsAreReadAsTheyHappened(t *testing.T) {
	cases, _ := filepath.Glob(filepath.Join("testdata", "realruns", "*", "case.json"))
	if len(cases) == 0 {
		t.Fatal("no real run is kept")
	}
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in the framework's repository")
	}
	root := strings.TrimSpace(string(top))
	for _, file := range cases {
		dir := filepath.Dir(file)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var c realCase
			if err := json.Unmarshal(b, &c); err != nil {
				t.Fatal(err)
			}
			estate := t.TempDir()
			archive := exec.Command("sh", "-c", "git -C \"$1\" archive --format=tar \"$2\" | tar -x -C \"$3\"", "sh", root, c.Commit, estate)
			if out, err := archive.CombinedOutput(); err != nil {
				t.Skipf("the framework at %s cannot be read here (a shallow clone?): %s", c.Commit, out)
			}
			var layers []string
			for _, l := range c.Layers {
				abs, _ := filepath.Abs(filepath.Join(dir, l))
				layers = append(layers, abs)
			}
			ws, err := harness.Snapshot(estate, layers...)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(ws)
			prof, ok := detect(ws)
			if !ok {
				t.Fatal("the estate's tests cannot be run")
			}
			run := runTests(context.Background(), ws, prof, c.Tests)
			if !run.Reported {
				t.Fatalf("the run was not read from its report: %s", run.Why)
			}
			failed, notRun := run.failed(c.Tests), run.notRun(c.Tests)
			slices.Sort(failed)
			slices.Sort(notRun)
			want := slices.Sorted(slices.Values(c.Failed))
			if !slices.Equal(failed, want) || !slices.Equal(notRun, c.NotRun) && len(notRun)+len(c.NotRun) > 0 {
				t.Errorf("%s\nfailed %v, want %v\nnot run %v, want %v\n%s", c.About, failed, want, notRun, c.NotRun, lastLines(run.Out, 20))
			}
		})
	}
}
