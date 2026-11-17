package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/igannytskyi/iflow/internal/flow"
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
	for _, bad := range []map[string]string{{"review": "0"}, {"repeat": "thrice"}, {"speed": "fast"}} {
		if err := vary(&cfg, bad); err == nil {
			t.Errorf("%v was taken", bad)
		}
	}
}
