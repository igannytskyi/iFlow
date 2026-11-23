package flow

import (
	"strings"
	"testing"
)

// Edits apart from each other merge; edits of the same lines do not.
func TestMerge3(t *testing.T) {
	t.Parallel()
	base := "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n"
	ours := "package greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n\nfunc Shout(n string) string { return Greet(n) + \"!\" }\n"
	theirs := "// Package greet greets.\npackage greet\n\nfunc Greet(n string) string { return \"Hello, \" + n }\n"
	m, clean, err := merge3([]byte(base), []byte(ours), []byte(theirs))
	if err != nil || !clean {
		t.Fatalf("edits apart did not merge: %v %v\n%s", clean, err, m)
	}
	if s := string(m); !strings.Contains(s, "// Package greet greets.") || !strings.Contains(s, "func Shout") {
		t.Errorf("the merge lost an edit:\n%s", s)
	}
	clash := "package greet\n\nfunc Greet(n string) string { return \"Hi, \" + n }\n"
	shout := "package greet\n\nfunc Greet(n string) string { return \"HELLO, \" + n }\n"
	if _, clean, err := merge3([]byte(base), []byte(shout), []byte(clash)); err != nil || clean {
		t.Errorf("edits of the same line merged: %v %v", clean, err)
	}
}
