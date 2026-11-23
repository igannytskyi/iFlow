package harness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/model"
)

// The files a unit may change are in every turn as they stand, never cut:
// whole while they fit, by their top-level lines past that; a file not yet
// written is not shown.
func TestTheFilesAUnitChangesAreAlwaysInView(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "feed.go"), []byte("package ui\n\nfunc feed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	long := "package ui\n\nfunc long() {\n" + strings.Repeat("\tx := 1\n", 20000) + "}\n"
	if err := os.WriteFile(filepath.Join(dir, "long.go"), []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	got := yours(dir, []string{"feed.go", "long.go", "new.go"}, 12000)
	if !strings.Contains(got, "func feed() {}") || !strings.Contains(got, "3: func long() {") || strings.Contains(got, "x := 1") || strings.Contains(got, "new.go") {
		t.Errorf("not the files as they stand, within the bound:\n%.600s", got)
	}
	if yours(dir, []string{"new.go"}, 12000) != "" {
		t.Error("a file not yet written was shown")
	}
}

// What an agent read stays whole while the turn fits its bound: it is cut,
// oldest first, only as far as the turn does not fit.
func TestOldAnswersAreCutOnlyBeyondTheTurnsBound(t *testing.T) {
	answer := strings.Repeat("x", 4*8_000) // about 8k tokens
	msgs := []model.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "u"}}
	for range 8 {
		msgs = append(msgs, model.Message{Role: "assistant", Content: "a"}, model.Message{Role: "tool", Content: answer})
	}
	for _, m := range compact(msgs) {
		if strings.Contains(m.Content, "[cut:") {
			t.Fatal("an answer was cut from a turn of about 64k tokens")
		}
	}
	for range 8 {
		msgs = append(msgs, model.Message{Role: "assistant", Content: "a"}, model.Message{Role: "tool", Content: answer})
	}
	sent, size, cut := compact(msgs), 0, 0
	for i, m := range sent {
		size += tokens(m.Content)
		if strings.Contains(m.Content, "[cut:") {
			cut++
			if i > 2+2*cut {
				t.Errorf("answer %d was cut before an older one", i)
			}
		}
	}
	if size > turnTokens || cut == 0 || cut > 5 {
		t.Errorf("a turn of about 128k tokens came to %d, with %d answers cut", size, cut)
	}
}

// A one-shot agent that cannot do its unit names the files why.
func TestAOneShotCannotNamesItsFiles(t *testing.T) {
	answer := "=== CANNOT: the hook lives in remote.go\n=== FILES: internal/ui/remote.go, `internal/ui/feed_test.go`\n"
	if why, ok := cannot(answer); !ok || why != "the hook lives in remote.go" {
		t.Errorf("cannot was not read: %q", why)
	}
	if got := cannotFiles(answer); len(got) != 2 || got[0] != "internal/ui/remote.go" || got[1] != "internal/ui/feed_test.go" {
		t.Errorf("the files were not read: %q", got)
	}
}

// What an agent blames is what the framework can act on: a test it must
// pass, by name; a file of the repository, by its path. Anything else is
// refused, to be said again — never guessed at.
func TestCannotBlamesOnlyWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "ui", "remote.go"), []byte("package ui\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := &workspace{dir: dir, tests: []string{"TestFeed", "TestStrip"}}
	tool, _ := lookup("cannot")
	_, err := tool.run(ws, map[string]string{"why": "the hook lives in remote.go", "tests": "TestFeed", "files": "./internal/ui/remote.go"})
	var b Blocked
	if !errors.As(err, &b) || len(b.Tests) != 1 || b.Tests[0] != "TestFeed" || len(b.Files) != 1 || b.Files[0] != "internal/ui/remote.go" {
		t.Fatalf("what was blamed rightly was not taken: %v", err)
	}
	for _, args := range []map[string]string{
		{"why": "w", "tests": "TestNowhere"},
		{"why": "w", "files": "remote.go"},
		{"why": "w", "files": "/etc/passwd"},
	} {
		if _, err := tool.run(ws, args); !errors.As(err, new(errRefused)) {
			t.Errorf("%v was taken: %v", args, err)
		}
	}
}
