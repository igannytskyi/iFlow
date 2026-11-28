package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/gate"
)

// The reference suite reads, and every task names an estate that is there
// and asks for something.
func TestTheReferenceSuiteIsWhole(t *testing.T) {
	file := filepath.Join(gate.Home(), "examples", "bench.json")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var s suite
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, task := range s.Tasks {
		if names[task.Name] {
			t.Errorf("%s is named twice", task.Name)
		}
		names[task.Name] = true
		if task.Statement == "" {
			t.Errorf("%s asks for nothing", task.Name)
		}
		if st, err := os.Stat(filepath.Join(filepath.Dir(file), task.Estate)); err != nil || !st.IsDir() {
			t.Errorf("%s: no estate %s", task.Name, task.Estate)
		}
		if task.Expect != "" && task.Expect != "landed" && task.Expect != "holds" {
			t.Errorf("%s expects %q", task.Name, task.Expect)
		}
	}
}

// What --set varies lands on the configuration; anything else is refused.
func TestTheBenchVariesWhatItSays(t *testing.T) {
	var cfg flow.Config
	if err := vary(&cfg, map[string]string{"review": "1", "attempts": "1", "repeat": "once"}); err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewRounds != 1 || cfg.Attempts != 1 || !cfg.RepeatOnce {
		t.Errorf("varied %+v", cfg)
	}
	// The executor's two rungs, each on its own model.
	cfg.Executor = harness.Ladder{First: harness.Oneshot{Attempts: 2}, Then: harness.Oneshot{}}
	if err := vary(&cfg, map[string]string{"executor": "claude-code:haiku", "stronger": "claude-code:sonnet"}); err != nil {
		t.Fatal(err)
	}
	if l, ok := cfg.Executor.(harness.Ladder); !ok || l.First.(harness.Oneshot).Model.Name() != "claude-code/haiku" || l.First.(harness.Oneshot).Attempts != 2 || l.Then != (harness.ClaudeCode{Model: "sonnet"}) {
		t.Errorf("the executor varied to %+v", cfg.Executor)
	}
	for _, bad := range []map[string]string{{"review": "0"}, {"repeat": "thrice"}, {"speed": "fast"}} {
		if err := vary(&cfg, bad); err == nil {
			t.Errorf("%v was taken", bad)
		}
	}
}

// A task at a commit is laid from the repository as it stood then: its
// tracked files, and nothing written since.
func TestATaskAtACommitIsLaidAsItStood(t *testing.T) {
	src := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", src, "-c", "user.name=t", "-c", "user.email=t@localhost"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "one")
	out, err := exec.Command("git", "-C", src, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "later.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := lay(src, repo, strings.TrimSpace(string(out))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "a.go")); err != nil {
		t.Error("the file of the commit was not laid")
	}
	if _, err := os.Stat(filepath.Join(repo, "later.go")); err == nil {
		t.Error("a file written after the commit was laid")
	}
}
