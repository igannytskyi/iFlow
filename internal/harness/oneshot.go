package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
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

Then, last, one line:
=== SUMMARY: <what you did, in one sentence>

Write only files you are allowed to change. Write nothing outside these blocks.`

// written is one file an agent wrote.
type written struct{ path, content string }

// parseFiles reads an answer in the form oneshotSystem asks for.
func parseFiles(answer string) (files []written, summary string) {
	lines := strings.Split(answer, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(line, "=== FILE:"):
			p := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "=== FILE:")), "`")
			var body []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "=== END"; i++ {
				body = append(body, lines[i])
			}
			// A fence around the content is the model's habit, not the file.
			if len(body) > 0 && strings.HasPrefix(strings.TrimSpace(body[0]), "```") {
				body = body[1:]
				if n := len(body); n > 0 && strings.TrimSpace(body[n-1]) == "```" {
					body = body[:n-1]
				}
			}
			files = append(files, written{p, strings.Join(body, "\n") + "\n"})
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
		files, summary := parseFiles(answer)
		if len(files) == 0 {
			said = "Your answer held no file in the form asked for."
			emit(Event{At: time.Now(), Kind: Refused, Detail: said})
			continue
		}
		last = nil
		for _, f := range files {
			if !allowed(f.path, t.Scope) {
				emit(Event{At: time.Now(), Kind: Refused, Tool: "Write", Detail: f.path + " is outside what the unit may change"})
				continue
			}
			dst := filepath.Join(t.Workspace, filepath.FromSlash(f.path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return res, err
			}
			if err := os.WriteFile(dst, []byte(f.content), 0o644); err != nil {
				return res, err
			}
			res.Steps++
			last = append(last, f)
			emit(Event{At: time.Now(), Kind: Called, Tool: "Write", Detail: f.path})
		}
		if len(last) == 0 {
			said = "Every file you wrote is outside what you may change."
			continue
		}
		res.Summary = summary
		if len(t.Check) == 0 {
			res.Terminal = Candidate
			break
		}
		ok, out := check(ctx, t.Workspace, t.Check)
		res.Steps++
		emit(Event{At: time.Now(), Kind: Called, Tool: "Check", Detail: strings.Join(t.Check, " ")})
		emit(Event{At: time.Now(), Kind: Got, Detail: tail(out, 6)})
		if ok {
			res.Terminal = Candidate
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
			b.WriteString("=== FILE: " + f.path + "\n" + f.content + "=== END\n")
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
	c.Dir = dir
	out, err := c.CombinedOutput()
	return err == nil, string(out)
}

// tail is the last n lines of what a command said, where failures are.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
