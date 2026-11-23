package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/reader"
)

// The tools an agent is given, and nothing more: it reads anything in the
// workspace, writes only what the unit's scope names, and runs the check
// command and no other. The harness enforces this, not the agent.

type tool struct {
	name, about string
	params      string // JSON schema
	run         func(ws *workspace, args map[string]string) (string, error)
	// writes says a call of it that succeeds changes the workspace.
	writes bool
}

// workspace is where a run may act.
type workspace struct {
	dir       string
	scope     []string
	check     []string
	judge     func(passed bool, output string) (ok bool, why string)
	first     []string
	faults    func(string) []Fault
	libraries []string
	graph     Graph
	quick     func(files []string) []string
	focus     func(pattern, path string) []string
	ctx       context.Context

	// seen is what was read, and at which call; calls counts the calls.
	seen  map[string]readAt
	calls int
	// last is how the check went when it was last run.
	last struct {
		ran      bool
		ok       bool
		out, why string
	}
}

// readable resolves a path the agent may read: one inside the workspace,
// or a library's source.
func (w *workspace) readable(p string) (string, error) {
	if clean := filepath.Clean(p); filepath.IsAbs(clean) {
		for _, lib := range w.libraries {
			if clean == lib || strings.HasPrefix(clean, lib+string(filepath.Separator)) {
				return clean, nil
			}
		}
	}
	abs, _, err := w.within(p)
	return abs, err
}

// run runs the check in the workspace and judges it as the unit's task
// says: whether it passes, what it said, and why it does not pass where a
// judge says so.
func (w *workspace) run() (ok bool, out, why string) {
	ok, out = checkFirst(w.ctx, w.dir, w.first, w.check)
	if w.judge != nil {
		ok, why = w.judge(ok, out)
	}
	w.last.ran, w.last.ok, w.last.out, w.last.why = true, ok, out, why
	return ok, out, why
}

// Blocked is a unit its agent says cannot be done as it is asked: why, and
// the files it names as the cause.
type Blocked struct {
	Why   string
	Files []string
}

func (b Blocked) Error() string { return "cannot be done as asked: " + b.Why }

// errRefused marks a call the harness would not carry out.
type errRefused struct{ why string }

func (e errRefused) Error() string { return e.why }

// within resolves a path the agent names to one inside the workspace, or
// refuses it.
func (w *workspace) within(p string) (string, string, error) {
	clean := filepath.Clean(filepath.FromSlash(p))
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(w.dir, clean)
		if err != nil {
			return "", "", errRefused{"that path is outside the workspace"}
		}
		clean = rel
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errRefused{"that path is outside the workspace"}
	}
	return filepath.Join(w.dir, clean), filepath.ToSlash(clean), nil
}

// writable reports whether the unit's scope names a path.
func (w *workspace) writable(rel string) bool {
	return slices.ContainsFunc(w.scope, func(s string) bool {
		s = strings.TrimSuffix(filepath.ToSlash(s), "/")
		return rel == s || strings.HasPrefix(rel, s+"/")
	})
}

var tools = []tool{
	{"read_file", "Read a file in the repository, or of a library's source by its absolute path. A file of more than 250 lines answers with its outline — every declaration and its line — and from and to read the lines between them, at most 250 at a time.",
		`{"type":"object","properties":{"path":{"type":"string"},"from":{"type":"integer","description":"first line to read"},"to":{"type":"integer","description":"last line to read"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, err := w.readable(a["path"])
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(p)
			if errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("there is no %s.%s", a["path"], w.suggest(a["path"]))
			}
			if err != nil {
				return "", err
			}
			return w.read(a["path"], string(b), atoi(a["from"]), atoi(a["to"])), nil
		}, false},
	{"list_dir", "List a directory in the repository, or of a library's source by its absolute path.", `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, err := w.readable(a["path"])
			if err != nil {
				return "", err
			}
			entries, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			var out []string
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				out = append(out, name)
			}
			return strings.Join(out, "\n"), nil
		}, false},
	{"search", "Search the repository, or one folder of it — or of a library's source by its absolute path — for a regular expression; answers file:line: text, at most 40 lines.",
		`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string","description":"a folder to search under; the whole repository when left out"}},"required":["pattern"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			re, err := regexp.Compile(a["pattern"])
			if err != nil {
				return "", err
			}
			root := w.dir
			if a["path"] != "" {
				if root, err = w.readable(a["path"]); err != nil {
					return "", err
				}
			}
			var out []string
			_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil || len(out) >= searchLines {
					return nil
				}
				if d.IsDir() {
					if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "changes" {
						return filepath.SkipDir
					}
					return nil
				}
				f, err := os.Open(p)
				if err != nil {
					return nil
				}
				defer f.Close()
				sc := bufio.NewScanner(f)
				for n := 1; sc.Scan() && len(out) < searchLines; n++ {
					if re.Match(sc.Bytes()) {
						name := p // a library's file, by its absolute path
						if rel, err := filepath.Rel(w.dir, p); err == nil && !strings.HasPrefix(rel, "..") {
							name = rel
						}
						out = append(out, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(name), n, clip(strings.TrimSpace(sc.Text()), 160)))
					}
				}
				return nil
			})
			if len(out) == searchLines {
				out = append(out, "[more: narrow the pattern or the path]")
			}
			return strings.Join(out, "\n"), nil
		}, false},
	{"write_file", "Write a whole file. Only files in the unit's scope may be written. Answers whether the code still builds.", `{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, rel, err := w.within(a["path"])
			if err != nil {
				return "", err
			}
			if !w.writable(rel) {
				return "", errRefused{rel + " is not in the unit's scope"}
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(p, []byte(a["content"]), 0o644); err != nil {
				return "", err
			}
			return "written" + w.builds(rel), nil
		}, true},
	{"edit_file", "Replace one exact occurrence of old with new in a file in the unit's scope. Answers with the lines around the change, and whether the code still builds.", `{"type":"object","properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}},"required":["path","old","new"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, rel, err := w.within(a["path"])
			if err != nil {
				return "", err
			}
			if !w.writable(rel) {
				return "", errRefused{rel + " is not in the unit's scope"}
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			out, how, err := replaceOnce(string(b), a["old"], a["new"])
			if err != nil {
				return "", fmt.Errorf("%s: %w", rel, err)
			}
			if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
				return "", err
			}
			return "edited" + how + "; now:\n" + around(out, a["new"]) + w.builds(rel), nil
		}, true},
	{"cannot", "Say that the unit cannot be done as it is asked, and why: something it relies on does not exist, what it must change is outside its scope, it asks for what the estate cannot do. The unit goes back to be planned again with your reason. Say it as soon as you know — not after trying everything.",
		`{"type":"object","properties":{"why":{"type":"string"},"files":{"type":"array","items":{"type":"string"},"description":"the files that keep it from being done, where they are files: a test written before you, a file outside your scope that must change — such a file sends the change back to its specification, whose scope it is"}},"required":["why"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			var files []string
			for _, f := range strings.Split(a["files"], "\n") {
				if f = strings.TrimSpace(f); f != "" {
					files = append(files, f)
				}
			}
			return "", Blocked{Why: strings.TrimSpace(a["why"]), Files: files}
		}, false},
	{"run_check", "Run the unit's check — the repository's own tests or build — and see the result.", `{"type":"object","properties":{}}`,
		func(w *workspace, _ map[string]string) (string, error) {
			if len(w.check) == 0 {
				return "", errRefused{"there is no check command for this estate"}
			}
			ok, out, why := w.run()
			status := "passed"
			if !ok {
				status = "failed"
				if why != "" {
					status += ": " + why
				}
			}
			return status + "\n" + condense(out), nil
		}, false},
	{"find_definition", "Where a name — a function, type, method, constant — is defined in the repository as it stood before this change, with its first line: firmer than searching its text.",
		`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			if w.graph == nil {
				return "", errRefused{"the estate has no model of its code here; use search"}
			}
			return w.graph.Defined(strings.TrimSpace(a["name"]))
		}, false},
	{"find_callers", "Every place in the repository, as it stood before this change, that calls or uses a name, with the line: what changing it would touch.",
		`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			if w.graph == nil {
				return "", errRefused{"the estate has no model of its code here; use search"}
			}
			return w.graph.Callers(strings.TrimSpace(a["name"]))
		}, false},
	{"run_tests", "Run the tests whose names match a regular expression, under a folder of the repository, and see each pass or fail with its output — to find which test fails and why, faster than the whole check.",
		`{"type":"object","properties":{"pattern":{"type":"string","description":"a regular expression matching test names"},"path":{"type":"string","description":"the folder whose tests to run"}},"required":["pattern","path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			if w.focus == nil {
				return "", errRefused{"this estate has no way to run some of its tests; use run_check"}
			}
			if _, _, err := w.within(a["path"]); err != nil {
				return "", err
			}
			ok, out := check(w.ctx, w.dir, w.focus(a["pattern"], a["path"]))
			status := "passed"
			if !ok {
				status = "failed"
			}
			return status + "\n" + condense(out), nil
		}, false},
}

// suggest names the files in the workspace whose names are nearest a path
// that is not there: a wrong guess at a name is better answered with the
// right names than with the next guess.
func (w *workspace) suggest(path string) string {
	base := strings.ToLower(filepath.Base(path))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	type near struct {
		rel   string
		score int
	}
	var found []near
	_ = filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "changes" {
				return filepath.SkipDir
			}
			return nil
		}
		b := strings.ToLower(d.Name())
		s := strings.TrimSuffix(b, filepath.Ext(b))
		score := 0
		switch {
		case b == base:
			score = 3
		case s == stem:
			score = 2
		case len(stem) >= 3 && (strings.Contains(s, stem) || strings.Contains(stem, s)):
			score = 1
		}
		if score > 0 {
			rel, _ := filepath.Rel(w.dir, p)
			found = append(found, near{filepath.ToSlash(rel), score})
		}
		return nil
	})
	slices.SortFunc(found, func(a, b near) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return len(a.rel) - len(b.rel)
	})
	if len(found) == 0 {
		return " Use list_dir or search to find the right path."
	}
	var names []string
	for _, f := range found[:min(4, len(found))] {
		names = append(names, f.rel)
	}
	return " Did you mean: " + strings.Join(names, ", ") + "?"
}

// builds is what the fast check says of a file just written: nothing when
// it builds or there is no fast check, the faults when it does not.
func (w *workspace) builds(rel string) string {
	if w.quick == nil {
		return ""
	}
	cmd := w.quick([]string{rel})
	if len(cmd) == 0 {
		return ""
	}
	ok, out := check(w.ctx, w.dir, cmd)
	if ok {
		return "; it builds"
	}
	var said []string
	if w.faults != nil {
		for _, f := range w.faults(out) {
			if len(said) < 20 {
				said = append(said, f.Text)
			}
		}
	}
	if len(said) == 0 {
		said = []string{tail(out, 12)}
	}
	return "\n\nThe code does not build now — mend this before going on:\n" + strings.Join(said, "\n")
}

// around is the lines of s around the first line of what was put in, with
// their numbers.
func around(s, put string) string {
	first := strings.TrimSpace(strings.SplitN(strings.Trim(put, "\n"), "\n", 2)[0])
	lines := strings.Split(s, "\n")
	at := 0
	for i, l := range lines {
		if first != "" && strings.Contains(l, first) {
			at = i
			break
		}
	}
	n := strings.Count(strings.Trim(put, "\n"), "\n") + 1
	from, to := max(0, at-3), min(len(lines), at+n+3)
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%5d  %s\n", i+1, lines[i])
	}
	return strings.TrimRight(b.String(), "\n")
}

// condense keeps of a long output what an agent needs: how it began, every
// line that says what failed and where, and how it ended.
func condense(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) <= condenseWhole {
		return strings.Join(lines, "\n")
	}
	keep := make([]bool, len(lines))
	for i := range min(15, len(lines)) {
		keep[i] = true
	}
	for i := max(0, len(lines)-40); i < len(lines); i++ {
		keep[i] = true
	}
	notable := 0
	for i, l := range lines {
		if notable < 60 && failing.MatchString(l) {
			for j := i; j < min(len(lines), i+3); j++ { // what failed, and the lines that say how
				keep[j] = true
			}
			notable++
		}
	}
	var b strings.Builder
	skipped := 0
	for i, l := range lines {
		if !keep[i] {
			skipped++
			continue
		}
		if skipped > 0 {
			fmt.Fprintf(&b, "[%d lines]\n", skipped)
			skipped = 0
		}
		b.WriteString(l + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// condenseWhole is the longest output kept whole.
const condenseWhole = 120

// failing is a line that says what failed, or where.
var failing = regexp.MustCompile(`--- FAIL|^FAIL|panic:|^\s*[\w./-]+\.\w+:\d+|Error|error:|undefined|cannot |expected|got |want |assert`)

// repairArgs mends tool arguments a model sent almost as JSON: wrapped in a
// code fence or in prose, with a trailing comma, or as a JSON string of the
// object. What cannot be mended is left as it was.
func repairArgs(s string) string {
	if json.Valid([]byte(s)) {
		var str string
		if json.Unmarshal([]byte(s), &str) == nil && json.Valid([]byte(str)) {
			return str // the object sent as a string
		}
		return s
	}
	t := strings.TrimSpace(s)
	if i, j := strings.Index(t, "{"), strings.LastIndex(t, "}"); i >= 0 && j > i {
		t = t[i : j+1]
	}
	t = trailingComma.ReplaceAllString(t, "$1")
	if json.Valid([]byte(t)) {
		return t
	}
	return s
}

var trailingComma = regexp.MustCompile(`,\s*([}\]])`)

// replaceOnce replaces the one place old occurs in s with new. Where old
// does not occur exactly, it is matched line by line with the indentation
// and trailing space ignored — models copy code with its spacing changed
// more often than with its words changed — and the second value says so.
func replaceOnce(s, old, new string) (string, string, error) {
	if strings.TrimSpace(old) == "" {
		return "", "", errors.New("old text is empty; to write a new file use write_file")
	}
	switch n := strings.Count(s, old); {
	case n == 1:
		return strings.Replace(s, old, new, 1), "", nil
	case n > 1:
		return "", "", fmt.Errorf("old text occurs %d times; give more of it", n)
	}
	lines := strings.Split(s, "\n")
	want := strings.Split(strings.Trim(old, "\n"), "\n")
	at, found := -1, 0
	for i := 0; i+len(want) <= len(lines); i++ {
		same := true
		for j, w := range want {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(w) {
				same = false
				break
			}
		}
		if same {
			at, found = i, found+1
		}
	}
	switch found {
	case 0:
		return "", "", errors.New("old text not found; read the file again and copy it exactly")
	case 1:
	default:
		return "", "", fmt.Errorf("old text occurs %d times once spacing is ignored; give more of it", found)
	}
	repl := strings.Split(strings.Trim(new, "\n"), "\n")
	if strings.TrimSpace(new) == "" {
		repl = nil
	}
	out := append(append(slices.Clone(lines[:at]), repl...), lines[at+len(want):]...)
	return strings.Join(out, "\n"), " (matched with spacing ignored)", nil
}

// searchLines bounds what a search answers.
const searchLines = 40

// wholeLines is the longest file answered whole; a longer one answers with
// its outline, and is read a range at a time.
const wholeLines = 250

// read answers a read of a file: whole when it is short, its outline when
// it is long and no range is asked, the range asked otherwise; and, when
// the same lines were read a few steps ago and the file has not changed
// since, says so rather than hand them over again.
func (w *workspace) read(path, text string, from, to int) string {
	lines := strings.Split(text, "\n")
	if from == 0 && to == 0 && len(lines) > wholeLines {
		var b strings.Builder
		fmt.Fprintf(&b, "%s has %d lines; its outline — read a range with from and to:\n", path, len(lines))
		// What the file defines, as its language's reader sees it: each
		// definition at the line it begins.
		var starts []int
		for _, sym := range reader.Read(path, []byte(text)).Symbols {
			if !slices.Contains(starts, sym.Start) {
				starts = append(starts, sym.Start)
			}
		}
		slices.Sort(starts)
		for _, n := range starts {
			if n >= 1 && n <= len(lines) {
				fmt.Fprintf(&b, "%5d  %s\n", n, clip(lines[n-1], 140))
			}
		}
		if len(starts) == 0 {
			b.WriteString("(no definitions this program can read in it; read it a range at a time)\n")
		}
		return b.String()
	}
	if from <= 0 {
		from = 1
	}
	if to <= 0 || to > len(lines) {
		to = len(lines)
	}
	if to-from+1 > wholeLines {
		to = from + wholeLines - 1
	}
	if from > to {
		return fmt.Sprintf("%s has %d lines", path, len(lines))
	}
	key := fmt.Sprintf("%s:%d-%d", path, from, to)
	if w.seen == nil {
		w.seen = map[string]readAt{}
	}
	if was, ok := w.seen[key]; ok && was.text == text && w.calls-was.call <= recentReads {
		return fmt.Sprintf("lines %d to %d of %s are unchanged since you read them %d step(s) ago; they are above", from, to, path, w.calls-was.call)
	}
	w.seen[key] = readAt{text, w.calls}
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "%5d  %s\n", i, lines[i-1])
	}
	if to < len(lines) {
		fmt.Fprintf(&b, "[%d more lines]\n", len(lines)-to)
	}
	return b.String()
}

// recentReads is how many steps back a read is still in the conversation
// whole, so that reading it again hands over nothing new.
const recentReads = 6

type readAt struct {
	text string
	call int
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func numbered(s string) string {
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%5d  %s\n", i+1, l)
	}
	return b.String()
}

// specs is the tools as a model is told about them.
func specs() []model.Tool {
	out := make([]model.Tool, len(tools))
	for i, t := range tools {
		out[i].Type = "function"
		out[i].Function.Name = t.name
		out[i].Function.Description = t.about
		out[i].Function.Parameters = json.RawMessage(t.params)
	}
	return out
}

func lookup(name string) (tool, bool) {
	for _, t := range tools {
		if t.name == name {
			return t, true
		}
	}
	return tool{}, false
}
