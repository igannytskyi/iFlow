package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assurance(t *testing.T, root, change, body string) {
	t.Helper()
	dir := filepath.Join(root, "changes", change)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "05-assurance.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const raised = "| # | Standing | Ground | Raised against | To |\n|---|---|---|---|---|\n| 1 | open | R7 cannot see what the arbiter reads | area 5 | a person |\n| 2 | open | R7 again, from another side | area 5 | a person |\n"

func settles(refs ...string) string {
	s := "| Escalation | Resolved by | Note |\n|---|---|---|\n"
	for _, r := range refs {
		s += "| " + r + " | this change | |\n"
	}
	return s
}

func TestADebtIsSettledOnceAndOnlyIfOwed(t *testing.T) {
	root := t.TempDir()
	assurance(t, root, "a", raised)
	assurance(t, root, "b", settles("a#1"))
	assurance(t, root, "c", settles("a#1", "a#9"))
	l, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Owed) != 1 || l.Owed[0].Ref != "a#2" {
		t.Errorf("owed = %+v", l.Owed)
	}
	joined := strings.Join(l.Problems, "\n")
	if !strings.Contains(joined, "a#1 is settled twice") || !strings.Contains(joined, "a#9, which was never owed") {
		t.Errorf("R15 not found: %v", l.Problems)
	}
}

func TestOneDefectNamedTwiceIsShown(t *testing.T) {
	root := t.TempDir()
	assurance(t, root, "a", raised)
	l, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if s := l.Shared(); len(s) != 1 || s[0].Rule != "R7" || len(s[0].Refs) != 2 {
		t.Errorf("shared = %+v", s)
	}
}
