package harness

import (
	"os"
	"path/filepath"
	"testing"
)

// A copy of the estate leaves out the estate's own changes — what will
// judge the work — and keeps a changes folder deeper down, which is part of
// the estate its tests read.
func TestASnapshotKeepsAnExamplesChanges(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"changes/c/01-specification.md", "examples/e/changes/x/record.md", "a.go"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws, err := Snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ws)
	if _, err := os.Stat(filepath.Join(ws, "changes")); err == nil {
		t.Error("the estate's own changes were copied to where an agent works")
	}
	if _, err := os.Stat(filepath.Join(ws, "examples/e/changes/x/record.md")); err != nil {
		t.Error("an example's changes, which the estate's tests read, were left out")
	}
}
