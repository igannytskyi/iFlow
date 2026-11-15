package measure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTouchpointsAreCountedAsTheBaselineCountsThem(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "changes", "c")
	os.MkdirAll(dir, 0o755)
	files := map[string]string{
		"00-intent.md":        "| Field | Value |\n|---|---|\n| Id | INT-001 |\n| Statement | carries pull request #42 |\n",
		"01-specification.md": "| Field | Value |\n|---|---|\n| Default class | C2 |\n",
		"03-admission.md":     "| Reviewed before admission | By | At | Note |\n|---|---|---|---|\n| yes | a person | today | |\n",
		"05-assurance.md": "| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |\n|---|---|---|---|---|---|---|---|\n| VE-001-01 | WU-001-01 | CR-001-01 | undecided | EV | settled | | |\n\n" +
			"| # | Standing | Ground | Raised against | To |\n|---|---|---|---|---|\n| 1 | open | a ground | area 5 | a person |\n",
	}
	for name, body := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	rows := ThroughMethod(root)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0]
	if r.Class != "C2" || r.PR != 42 || r.Touchpoints != 4 || r.Reading != 1 || r.Escalations != 1 || r.Left != 1 {
		t.Errorf("carried = %+v", r)
	}
}

func TestAClassIsWhatTheMessageCallsIt(t *testing.T) {
	for msg, want := range map[string]string{"Fix crash on empty name": "C2", "Bump deps": "C1*", "Add QUERY route": "C3", "Release 3.1": "unclassified"} {
		if got := called(msg); got != want {
			t.Errorf("called(%q) = %s, want %s", msg, got, want)
		}
	}
}
