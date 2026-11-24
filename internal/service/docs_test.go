package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// What a candidate changed is set against how each file stood before the
// change: as the admission kept it, as absent where it kept that, and, where
// it kept nothing, as the estate holds it — until the change has landed,
// when how the file stood is not known and is not guessed.
func TestADiffIsSetAgainstThePriorState(t *testing.T) {
	estate := t.TempDir()
	dir := filepath.Join(estate, "changes", "x")
	write := func(rel, text string) {
		t.Helper()
		p := filepath.Join(estate, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := "changes/x/candidates/CA-001-01/files/"
	write("changes/x/baseline/kept.go.before", "a\nb\n")
	write(files+"kept.go", "a\nc\n")
	write("changes/x/baseline/made.go.absent", "")
	write(files+"made.go", "new\n")
	write("other.go", "one\n")
	write(files+"other.go", "one\ntwo\n")
	write(files+"CHANGES", "kept.go made.go other.go\n")

	byPath := func() map[string]protocol.FileDiff {
		t.Helper()
		got, err := diff(protocol.TraceParams{Dir: dir, Candidate: "CA-001-01"})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]protocol.FileDiff{}
		for _, f := range got {
			out[f.Path] = f
		}
		return out
	}
	got := byPath()
	if len(got) != 3 {
		t.Fatalf("the diff names %d files, not 3: %v", len(got), got)
	}
	if f := got["kept.go"]; f.Added != 1 || f.Removed != 1 || f.New || !strings.Contains(f.Unified, "-b\n+c") {
		t.Errorf("a file the admission kept is not set against it: %+v", f)
	}
	if f := got["made.go"]; !f.New || f.Added != 1 || f.Removed != 0 {
		t.Errorf("a file the change made is not new: %+v", f)
	}
	if f := got["other.go"]; f.Added != 1 || f.Removed != 0 || f.Unrecorded {
		t.Errorf("a file kept nowhere is not set against the estate: %+v", f)
	}

	write("changes/x/06-landing.json", "{}")
	if f := byPath()["other.go"]; !f.Unrecorded || f.Unified != "" {
		t.Errorf("once landed, a file kept nowhere is set against what the change made: %+v", f)
	}

	if _, err := diff(protocol.TraceParams{Dir: dir, Candidate: "../x"}); err == nil {
		t.Error("a candidate named by a path was read")
	}
}
