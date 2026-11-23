package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A command run in the sandbox writes under a home of its own, never the
// person's, and still finds the Go toolchain's caches where they were.
func TestACommandWritesUnderAHomeOfItsOwn(t *testing.T) {
	cmd := exec.Command("sh", "-c", `mkdir -p "$HOME/Library/Application Support/iflow" && echo x > "$HOME/Library/Application Support/iflow/models.json" && echo "$HOME"`)
	cmd.Env = Env()
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	home := strings.TrimSpace(string(out))
	mine, _ := os.UserHomeDir()
	if home == mine || !strings.Contains(filepath.Base(home), "iflow-home-") {
		t.Errorf("the command's home is %s, not one of its own", home)
	}
	if _, err := exec.LookPath("go"); err == nil {
		want, _ := exec.Command("go", "env", "GOMODCACHE").Output()
		got := exec.Command("go", "env", "GOMODCACHE")
		got.Env = Env()
		have, _ := got.Output()
		if strings.TrimSpace(string(have)) != strings.TrimSpace(string(want)) {
			t.Errorf("the module cache moved: %s, not %s", have, want)
		}
	}
}
