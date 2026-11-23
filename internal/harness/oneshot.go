package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/sandbox"
)

// Oneshot carries out a task the way the method wants every agent to work:
// one question to a model that has been handed everything it needs and
// nothing else, answered with whole files, after which the agent is gone.
// The harness, not the agent, writes the files and runs the check; where the
// check fails, a new agent is asked, handed the same and what the last one
// wrote and what the check said — never a conversation.
type Oneshot struct {
	Model model.Model
	// Attempts is how many agents may try, three when zero.
	Attempts int
	// Stronger, where given, takes the last attempt when the ones before it
	// failed the check: the cheap model does what it can, and only what it
	// cannot is paid for at the stronger one's price.
	Stronger model.Model
}

// Name implements Executor.
func (o Oneshot) Name() string {
	if o.Stronger != nil {
		return "one-shot " + o.Model.Name() + ", then " + o.Stronger.Name()
	}
	return "one-shot " + o.Model.Name()
}

// Version implements Executor.
func (o Oneshot) Version() string { return "1" }

const oneshotSystem = `You change a software project to reach one goal. You are given everything you need: you cannot look at anything else, run anything or ask anything.

Answer only in this form. For each file you write, the whole file, from its first line to its last:

=== FILE: <path>
<the complete content of the file>
=== END

To change part of a file that exists — and always for one longer than 150 lines, which you must never write whole — use instead, once per file, with as many SEARCH/REPLACE pairs as needed:

=== EDIT: <path>
<<<<<<< SEARCH
<lines copied exactly from the file as you were given it, enough of them to occur once>
=======
<the lines that take their place>
>>>>>>> REPLACE
=== END

Then, last, one line:
=== SUMMARY: <what you did, in one sentence>

If the unit cannot be done as it is asked — something it relies on does not exist, or what it must change is outside what you may change — write no file, only:
=== CANNOT: <why, in one sentence>
=== FILES: <the files that keep it from being done, comma-separated: a test written before you, a file outside what you may change that must change>

Write only files you are allowed to change. Write nothing outside these blocks.`

// written is one file an agent wrote: whole, or as edits to what is there.
type written struct {
	path, content string
	edits         [][2]string // search, replace
	// cut says the block never reached its === END: the answer stopped
	// in the middle of it, at the most a model may write at once.
	cut bool
}

// text is the file as the agent wrote it, for the next agent to read.
func (w written) text() string {
	if w.edits == nil {
		return "=== FILE: " + w.path + "\n" + w.content + "=== END\n"
	}
	var b strings.Builder
	b.WriteString("=== EDIT: " + w.path + "\n")
	for _, e := range w.edits {
		b.WriteString("<<<<<<< SEARCH\n" + e[0] + "\n=======\n" + e[1] + "\n>>>>>>> REPLACE\n")
	}
	return b.String() + "=== END\n"
}

// apply writes what an agent wrote into the workspace: a whole file as it
// is, an edit by finding each search once and putting its replacement in
// its place. A whole file that shortens one already there to under half is
// refused: an agent shown an outline rewrites what it never saw.
func (w written) apply(dst string) error {
	if w.cut {
		return fmt.Errorf("%s was cut off: your answer stopped before its === END, at the most you may write at once; write it in parts — a shorter file, or EDIT blocks — so that each block ends", w.path)
	}
	before, err := os.ReadFile(dst)
	if w.edits == nil {
		if markers.MatchString(w.content) {
			return fmt.Errorf("%s was written whole with SEARCH/REPLACE markers inside it; a FILE block holds the file itself, an EDIT block the changes", w.path)
		}
		if err == nil && strings.Count(string(before), "\n") > 150 && len(w.content) < len(before)/2 {
			return fmt.Errorf("%s was written whole and lost more than half of what it held; change it with EDIT blocks instead", w.path)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(w.content), 0o644)
	}
	if err != nil {
		return fmt.Errorf("%s does not exist, so it cannot be edited; write it whole with a FILE block", w.path)
	}
	text := string(before)
	for _, e := range w.edits {
		out, _, err := replaceOnce(text, e[0], e[1])
		if err != nil {
			return fmt.Errorf("an edit of %s: %v:\n%s", w.path, err, e[0])
		}
		text = out
	}
	if markers.MatchString(text) {
		return fmt.Errorf("an edit of %s left SEARCH/REPLACE markers in it", w.path)
	}
	return os.WriteFile(dst, []byte(text), 0o644)
}

// markers are the lines of an edit block, which never belong in a file.
var markers = regexp.MustCompile(`(?m)^(?:<<<<<<< SEARCH|>>>>>>> REPLACE)\s*$`)

// header is a line that begins a block of an answer: within another block,
// it says that block was never ended.
func header(line string) bool {
	l := strings.TrimSpace(line)
	return strings.HasPrefix(l, "=== FILE:") || strings.HasPrefix(l, "=== EDIT:") || strings.HasPrefix(l, "=== SUMMARY:")
}

// parseFiles reads an answer in the form oneshotSystem asks for.
func parseFiles(answer string) (files []written, summary string) {
	lines := strings.Split(answer, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(line, "=== FILE:"):
			p := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "=== FILE:")), "`")
			var body []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "=== END" && !header(lines[i]); i++ {
				body = append(body, lines[i])
			}
			cut := i >= len(lines) || header(lines[i])
			if cut {
				i-- // the next block, if any, begins here
			}
			// A fence around the content is the model's habit, not the file.
			if len(body) > 0 && strings.HasPrefix(strings.TrimSpace(body[0]), "```") {
				body = body[1:]
				if n := len(body); n > 0 && strings.TrimSpace(body[n-1]) == "```" {
					body = body[:n-1]
				}
			}
			files = append(files, written{path: p, content: strings.Join(body, "\n") + "\n", cut: cut})
		case strings.HasPrefix(line, "=== EDIT:"):
			w := written{path: strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "=== EDIT:")), "`"), edits: [][2]string{}}
			var search, replace []string
			mode := ""
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "=== END" && !header(lines[i]); i++ {
				switch strings.TrimSpace(lines[i]) {
				case "<<<<<<< SEARCH":
					mode, search, replace = "search", nil, nil
				case "=======":
					if mode == "search" {
						mode = "replace"
					} else if mode == "replace" {
						replace = append(replace, lines[i])
					}
				case ">>>>>>> REPLACE":
					w.edits = append(w.edits, [2]string{strings.Join(search, "\n"), strings.Join(replace, "\n")})
					mode = ""
				default:
					switch mode {
					case "search":
						search = append(search, lines[i])
					case "replace":
						replace = append(replace, lines[i])
					}
				}
			}
			if w.cut = i >= len(lines) || header(lines[i]); w.cut {
				i--
			}
			files = append(files, w)
		case strings.HasPrefix(line, "=== SUMMARY:"):
			summary = strings.TrimSpace(strings.TrimPrefix(line, "=== SUMMARY:"))
		}
	}
	return files, summary
}

// Run implements Executor.
func (o Oneshot) Run(ctx context.Context, t Task, emit func(Event)) (Result, error) {
	began := time.Now()
	attempts := o.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	res := Result{Terminal: Exhausted}
	var last []written
	var said string
	for n := 1; n <= attempts; n++ {
		if res.Usage.CostUSD >= t.BudgetUSD && t.BudgetUSD > 0 {
			res.Summary = fmt.Sprintf("the budget of $%.2f was spent before attempt %d", t.BudgetUSD, n)
			break
		}
		prompt := o.prompt(t, last, said)
		m := o.Model
		if o.Stronger != nil && n == attempts && n > 1 {
			m = o.Stronger
		}
		emit(Event{At: time.Now(), Kind: Said, Detail: fmt.Sprintf("agent %d (%s) handed %d tokens", n, m.Name(), (len(prompt)+3)/4)})
		answer, u, err := m.Ask(ctx, oneshotSystem, prompt)
		res.Turns++
		res.Usage = res.Usage.Add(u)
		emit(Event{At: time.Now(), Kind: Spent, Detail: fmt.Sprintf("agent %d answered", n), Usage: u})
		if err != nil {
			if ctx.Err() != nil {
				res.Terminal, res.Summary = Cancelled, "stopped"
				break
			}
			res.Terminal, res.Summary = FailedSubstrate, err.Error()
			break
		}
		if why, ok := cannot(answer); ok {
			res.Terminal, res.Summary, res.Cannot = FailedTask, why, why
			for _, f := range cannotFiles(answer) {
				// Only a file of the repository, by its path from its root, is
				// blamed: anything else is not a file the framework can act on.
				f = filepath.ToSlash(filepath.Clean(f))
				if _, err := os.Stat(filepath.Join(t.Workspace, filepath.FromSlash(f))); err == nil && !filepath.IsAbs(f) && !strings.HasPrefix(f, "..") {
					res.Blames = append(res.Blames, f)
				}
			}
			break
		}
		files, summary := parseFiles(answer)
		said = ""
		if len(files) == 0 {
			said = "Your answer held no file in the form asked for."
			emit(Event{At: time.Now(), Kind: Refused, Detail: said})
			continue
		}
		last = nil
		for _, f := range files {
			if !allowed(f.path, t.Scope) && !slices.Contains(res.Widened, filepath.ToSlash(filepath.Clean(f.path))) {
				// Outside its scope: given it where the task can widen it.
				why := "outside what the unit may change"
				if t.Widen != nil {
					if why = t.Widen(filepath.ToSlash(filepath.Clean(f.path))); why == "" {
						res.Widened = append(res.Widened, filepath.ToSlash(filepath.Clean(f.path)))
					}
				}
				if why != "" {
					emit(Event{At: time.Now(), Kind: Refused, Tool: "Write", Detail: f.path + " is " + why})
					continue
				}
			}
			dst := filepath.Join(t.Workspace, filepath.FromSlash(f.path))
			if err := f.apply(dst); err != nil {
				emit(Event{At: time.Now(), Kind: Refused, Tool: "Write", Detail: err.Error()})
				said += err.Error() + "\n"
				continue
			}
			res.Steps++
			last = append(last, f)
			emit(Event{At: time.Now(), Kind: Called, Tool: "Write", Detail: f.path})
		}
		if len(last) == 0 {
			if said == "" {
				said = "Every file you wrote is outside what you may change."
			}
			continue
		}
		res.Summary = summary
		if len(t.Check) == 0 {
			res.Terminal = Candidate
			break
		}
		ok, out := checkFirst(ctx, t.Workspace, t.First, t.Check)
		judged := ""
		if t.Judge != nil {
			ok, judged = t.Judge(ok, out)
		}
		res.Steps++
		emit(Event{At: time.Now(), Kind: Called, Tool: "Check", Detail: strings.Join(t.Check, " ")})
		emit(Event{At: time.Now(), Kind: Got, Detail: tail(out, 6)})
		if missing := unprovided(t); ok && len(missing) > 0 {
			said = unprovidedSaid(missing)
			res.Summary = "it left out what it was to provide: " + strings.Join(missing, ", ")
			continue
		}
		if ok {
			res.Terminal = Candidate
			break
		}
		if judged != "" {
			said = judged
			res.Summary = "the check still failed after " + fmt.Sprint(n) + " agent(s)"
			continue
		}
		if outside := faultsOutside(faults(t, out), append(slices.Clone(t.Scope), res.Widened...)); len(outside) > 0 {
			// What fails is in files this unit may not change — tests
			// written before it, a unit before it — and no agent of this
			// unit can mend it: say so now rather than spend the attempts.
			res.Terminal, res.Blames = FailedTask, outside
			res.Summary = "the check fails in " + strings.Join(outside, ", ") + ", which this unit may not change; what wrote it must be carried again: " + faultsIn(out, outside)
			break
		}
		said = "The check `" + strings.Join(t.Check, " ") + "` failed after your files were written:\n\n" + tail(out, 60)
		res.Summary = "the check still failed after " + fmt.Sprint(n) + " agent(s)"
	}
	res.Elapsed = time.Since(began)
	emit(Event{At: time.Now(), Kind: Ended, Detail: res.Terminal, Usage: res.Usage})
	return res, nil
}

// prompt is what one agent is handed: the goal, what it may change, the
// pack the framework gathered, and, for any agent after the first, what the
// one before wrote and why it was not enough.
func (o Oneshot) prompt(t Task, last []written, said string) string {
	var b strings.Builder
	b.WriteString("# Goal\n\n" + t.Goal + "\n\n# You may change only\n\n")
	for _, s := range t.Scope {
		b.WriteString("- " + s + "\n")
	}
	if t.Context != "" {
		b.WriteString("\n" + t.Context + "\n")
	}
	if len(last) > 0 {
		b.WriteString("\n# What the previous agent wrote\n\n")
		for _, f := range last {
			b.WriteString(f.text())
		}
	}
	if said != "" {
		b.WriteString("\n# Why it was not enough\n\n" + said + "\n")
	}
	return b.String()
}

func allowed(p string, scope []string) bool {
	p = filepath.ToSlash(filepath.Clean(p))
	if strings.HasPrefix(p, "../") || filepath.IsAbs(p) {
		return false
	}
	return slices.ContainsFunc(scope, func(s string) bool {
		s = strings.TrimSuffix(s, "/")
		return p == s || strings.HasPrefix(p, s+"/")
	})
}

func check(ctx context.Context, dir string, cmd []string) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Env = sandbox.Env()
	c.Dir = dir
	out, err := c.CombinedOutput()
	return err == nil, string(out)
}

// checkFirst runs first, where given, and the check only once it passes:
// what fails fast is said fast.
func checkFirst(ctx context.Context, dir string, first, cmd []string) (bool, string) {
	if len(first) > 0 {
		if ok, out := check(ctx, dir, first); !ok {
			return false, out
		}
	}
	return check(ctx, dir, cmd)
}

// tail is the last n lines of what a command said, where failures are.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// faultsOutside is every file a check's faults blame that is outside the
// scope, when they blame nothing inside it. A name another file finds
// missing is no fault of that file when it is what this unit was to
// declare; a name missing from a library is.
func faultsOutside(fs []Fault, scope []string) []string {
	var outside []string
	for _, f := range fs {
		p := strings.TrimPrefix(f.File, "./")
		if allowed(p, scope) || f.Missing {
			return nil
		}
		if !slices.Contains(outside, p) {
			outside = append(outside, p)
		}
	}
	return outside
}

// faultsIn is what a check said against the files it blames: those lines,
// whole, which are what whoever wrote the files needs to mend them — not the
// tail of the output, which is the runner's summary.
func faultsIn(out string, files []string) string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		for _, f := range files {
			if strings.Contains(l, f) {
				lines = append(lines, strings.TrimSpace(l))
				break
			}
		}
		if len(lines) == 40 {
			break
		}
	}
	if len(lines) == 0 {
		return tail(out, 8)
	}
	return strings.Join(lines, "\n")
}

// cannot is why an agent says its unit cannot be done as asked, if it says so.
func cannot(answer string) (string, bool) {
	for _, l := range strings.Split(answer, "\n") {
		if why, ok := strings.CutPrefix(strings.TrimSpace(l), "=== CANNOT:"); ok {
			return strings.TrimSpace(why), true
		}
	}
	return "", false
}

// cannotFiles is the files an agent names as keeping its unit from being
// done, where it says it cannot be.
func cannotFiles(answer string) []string {
	var out []string
	for _, l := range strings.Split(answer, "\n") {
		if list, ok := strings.CutPrefix(strings.TrimSpace(l), "=== FILES:"); ok {
			for _, f := range strings.Split(list, ",") {
				if f = strings.Trim(strings.TrimSpace(f), "`"); f != "" {
					out = append(out, f)
				}
			}
		}
	}
	return out
}
