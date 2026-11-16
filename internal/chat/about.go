package chat

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Budgets, in runes, of what the model is handed about a subject: enough to
// talk about it, never the whole estate.
const (
	aboutTask    = 40000
	aboutProject = 24000
)

// AboutTask is what a task has written about itself: its specification,
// its plan, its units, its record, and how each unit's last run ended.
func AboutTask(dir string) string {
	var b section
	b.add("The task's specification (change.md)", read(filepath.Join(dir, "change.md")), 16000)
	b.add("Its plan (plan.md)", read(filepath.Join(dir, "plan.md")), 8000)
	units, _ := filepath.Glob(filepath.Join(dir, "units", "*.md"))
	sort.Strings(units)
	for _, u := range units {
		b.add("Unit "+strings.TrimSuffix(filepath.Base(u), ".md"), read(u), 2500)
	}
	b.add("Its record (record.md), newest last", tailRunes(read(filepath.Join(dir, "record.md")), 6000), 6000)
	cands, _ := filepath.Glob(filepath.Join(dir, "candidates", "*", "trace.jsonl"))
	sort.Strings(cands)
	for _, c := range cands {
		b.add("How "+filepath.Base(filepath.Dir(c))+" ended (last steps of its trace)", tailLines(read(c), 6), 2000)
	}
	return clip(b.String(), aboutTask)
}

// AboutProject is what a project says about itself: its readme, the
// person's decisions, and its tasks, each named with where it stands.
func AboutProject(repo string, tasks []string) string {
	var b section
	b.add("The project", filepath.Base(repo), 200)
	for _, name := range []string{"README.md", "AGENTS.md"} {
		b.add(name, read(filepath.Join(repo, name)), 6000)
	}
	b.add("The person's decisions", read(filepath.Join(repo, "changes", "decisions.md")), 4000)
	b.add("Its tasks, newest first", strings.Join(tasks, "\n"), 8000)
	return clip(b.String(), aboutProject)
}

type section struct{ strings.Builder }

func (s *section) add(title, text string, limit int) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.WriteString("## " + title + "\n\n" + clip(text, limit) + "\n\n")
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n[… cut]"
}

func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "[…]\n" + string(r[len(r)-n:])
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
