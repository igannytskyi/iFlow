package harness

import (
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
	answer := strings.Repeat("x", 4*4_000) // about 4k tokens
	msgs := []model.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "u"}}
	for range 8 {
		msgs = append(msgs, model.Message{Role: "assistant", Content: "a"}, model.Message{Role: "tool", Content: answer})
	}
	for _, m := range compact(msgs) {
		if strings.Contains(m.Content, "[cut:") {
			t.Fatal("an answer was cut from a turn of about 32k tokens")
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
	if size > turnTokens || cut == 0 || cut > 6 {
		t.Errorf("a turn of about 64k tokens came to %d, with %d answers cut", size, cut)
	}
}
