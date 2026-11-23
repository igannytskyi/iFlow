package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
