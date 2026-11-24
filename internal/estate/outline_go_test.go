package estate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Go type is handed whole — a struct with its fields — where its first
// line would have left an agent to guess what it holds.
func TestAGoTypeIsShownWhole(t *testing.T) {
	f := filepath.Join(t.TempDir(), "meta.go")
	src := "package meta\n\n// Specification is a change to be made.\ntype Specification struct {\n\tSchema int `json:\"schema\"`\n\tName   string\n}\n\nfunc (s Specification) Valid() bool { return true }\n"
	if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got := goType(f, "Specification")
	for _, want := range []string{"type Specification struct", "Schema int", "Name   string"} {
		if !strings.Contains(got, want) {
			t.Errorf("the type lacks %q:\n%s", want, got)
		}
	}
	if goType(f, "Missing") != "" {
		t.Error("a type the file does not declare was shown")
	}
	if o := GoOutline([]byte(src)); !strings.Contains(o, "Schema int") || !strings.Contains(o, "func (s Specification) Valid() bool") {
		t.Errorf("the outline lacks the fields or the method:\n%s", o)
	}
}
