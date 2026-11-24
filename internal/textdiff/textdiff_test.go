package textdiff

import (
	"strings"
	"testing"
)

// A change is said as the lines removed and added between what is kept,
// each hunk with its context and where it begins in each text.
func TestAChangeIsSaidLineByLine(t *testing.T) {
	a := "package a\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() {}\n"
	b := "package a\n\nfunc A() int {\n\treturn 2\n}\n\nfunc B() {}\n\nfunc C() {}\n"
	got, added, removed := Unified(a, b, 1)
	want := "@@ -3,3 +3,3 @@\n func A() int {\n-\treturn 1\n+\treturn 2\n }\n@@ -7,1 +7,3 @@\n func B() {}\n+\n+func C() {}\n"
	if got != want || added != 3 || removed != 1 {
		t.Errorf("diff:\n%s\nwant:\n%s\n+%d -%d", got, want, added, removed)
	}
	if d, add, _ := Unified("", "x\ny\n", 3); add != 2 || !strings.HasPrefix(d, "@@ -1,0 +1,2 @@") {
		t.Errorf("a new file: %q +%d", d, add)
	}
	if d, _, _ := Unified(a, a, 3); d != "" {
		t.Errorf("no change said: %q", d)
	}
}
