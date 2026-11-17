package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// Chatter is a model that holds a conversation and calls tools: what an
// agent in a loop is made of.
type Chatter interface {
	Name() string
	Chat(ctx context.Context, msgs []model.Message, tools []model.Tool) (model.Message, model.Usage, error)
}

// Loop is this method's own agent: a model that calls tools, in a loop the
// harness owns. It runs on any model served over the chat-completions
// protocol. What it borrows from the agents that work — Claude Code, Aider,
// SWE-agent — is that the agent reads before it writes, edits rather than
// rewrites, runs the check itself and goes on until it passes; and that the
// harness, not the agent, decides when it is done: an agent that says it is
// done while the check fails is told so and goes on.
type Loop struct {
	Model Chatter
}

// Name implements Executor.
func (l Loop) Name() string { return "agent " + l.Model.Name() }

// Version implements Executor.
func (l Loop) Version() string { return "loop/2" }

const loopSystem = `You carry out one unit of work in a repository, with tools, until it is done.

How to work:
- Read before you change anything: read_file the files you will change and the ones they use; search for a name rather than guess where it is.
- Change the least that reaches the goal. Prefer edit_file, with old text copied exactly from read_file's output (without the line numbers) and enough of it to occur once. Write a whole file only when it is new or short.
- Change only files in the unit's scope; the harness refuses anything else. If the goal cannot be reached without changing a file outside the scope, say so and stop.
- Each write and edit answers whether the code still builds: mend what it names before anything else.
- After changing files, call run_check. Read what fails, fix it, and run it again. To see which test fails and why — a panic names no test — run_tests runs the tests matching a name under a folder, each with its output. Do not stop while it fails because of your files.
- Do not change tests to make them pass unless the unit is the one that writes them.
- Tests written for this change before you are not yours to change, and may be wrong. If the check fails in them — a test's own setup or fakes (a config missing what the code needs, a fake that does not do what it stands for), a panic whose cause is what a test handed in — and no change to your files can pass them, call cannot at once, naming the test file and the line and what is wrong there; that file is written again, and you are carried again after it.
- If the unit cannot be done as it is asked — what it relies on does not exist, what it must change is outside its scope — call cannot with why, at once; do not search on for what is not there.
- When the check passes, answer without calling a tool, in two sentences: what you changed and why. The harness runs the check once more; if it fails, you are told and go on.`

// idleNudge and idleStop are how many steps without a change an agent is
// told it has read enough, and stopped; maxFresh is how many calls that ask
// something new — a file not read, a test not run — count as finding out
// rather than idling, in a whole run.
const (
	idleNudge = 6
	idleStop  = 12
	maxFresh  = 15
)

// maxRead is the most a unit's agent may read, all its turns together.
func maxRead(t Task) int {
	if t.MaxRead > 0 {
		return t.MaxRead
	}
	return 250_000
}

// keepWhole is how many of the latest messages keep their tool answers
// whole; older answers are cut short, the way agents that run long keep
// their context from filling with files read long ago.
const keepWhole = 8

// Run implements Executor.
func (l Loop) Run(ctx context.Context, t Task, emit func(Event)) (Result, error) {
	if l.Model == nil {
		return Result{Terminal: FailedSubstrate}, errNoModel
	}
	ws := &workspace{dir: t.Workspace, scope: t.Scope, check: t.Check, first: t.First, judge: t.Judge, quick: t.Quick, focus: t.Focus, ctx: ctx}
	msgs := []model.Message{{Role: "system", Content: loopSystem}, {Role: "user", Content: prompt(t) + libraryNote(t.Workspace)}}
	// What the estate says is read once: from the agent's third turn on it
	// is sent as an index of what it held, and the agent reads what it
	// needs again, so that every turn does not carry every file.
	slim := msgs[1]
	slim.Content = prompt(Task{Goal: t.Goal, Scope: t.Scope, Check: t.Check, Context: index(t.Context)}) + libraryNote(t.Workspace)
	maxSteps := t.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 30
	}
	var res Result
	started := time.Now()
	end := func(terminal, summary string) (Result, error) {
		res.Terminal, res.Summary, res.Elapsed = terminal, summary, time.Since(started)
		emit(Event{At: time.Now(), Kind: Ended, Detail: terminal + ": " + summary, Usage: res.Usage})
		return res, nil
	}
	// A loop is the same call made again and again with nothing between:
	// reading a file again later — after other reading, or after its old
	// answer was cut from the conversation — is work, not a loop.
	last, again := "", 0
	// Reading is how an agent starts; reading on and on without changing
	// anything, while the check fails, is how a weak model goes round. It is
	// told so once, and stopped if it goes on.
	idle, nudged := 0, false
	asked, fresh := map[string]bool{}, 0
	refusedDone := 0
	for res.Steps < maxSteps {
		if ctx.Err() != nil {
			return end(Cancelled, "stopped")
		}
		if t.BudgetUSD > 0 && res.Usage.CostUSD >= t.BudgetUSD {
			return end(Exhausted, fmt.Sprintf("the budget of $%.2f was spent", t.BudgetUSD))
		}
		if read := res.Usage.In(); read >= maxRead(t) {
			return end(Exhausted, fmt.Sprintf("read %d tokens, the most one unit may; a unit that needs more is too large or cannot be done as it is asked", read))
		}
		sent := compact(msgs)
		if res.Turns >= 2 {
			sent[1] = slim
		}
		msg, u, err := l.Model.Chat(ctx, sent, specs())
		res.Usage = res.Usage.Add(u)
		res.Turns++
		emit(Event{At: time.Now(), Kind: Spent, Usage: u})
		if err != nil {
			if ctx.Err() != nil {
				return end(Cancelled, "stopped")
			}
			res.Elapsed = time.Since(started)
			res.Terminal, res.Summary = FailedSubstrate, err.Error()
			emit(Event{At: time.Now(), Kind: Ended, Detail: err.Error(), Usage: res.Usage})
			return res, err
		}
		msg.Role = "assistant"
		msgs = append(msgs, msg)
		if s := strings.TrimSpace(msg.Content); s != "" {
			emit(Event{At: time.Now(), Kind: Said, Detail: s})
		}
		if len(msg.ToolCalls) == 0 {
			// The agent says it is done; the harness sees for itself.
			if len(t.Check) == 0 {
				return end(Candidate, strings.TrimSpace(msg.Content))
			}
			ok, out, judged := ws.run()
			res.Steps++
			emit(Event{At: time.Now(), Kind: Called, Tool: "Check", Detail: strings.Join(t.Check, " ")})
			emit(Event{At: time.Now(), Kind: Got, Detail: tail(out, 6)})
			if missing := Unprovided(t); ok && len(missing) > 0 {
				if refusedDone++; refusedDone >= 3 {
					return end(Exhausted, "said it was done three times without what it was to provide: "+strings.Join(missing, ", "))
				}
				msgs = append(msgs, model.Message{Role: "user", Content: unprovidedSaid(missing)})
				continue
			}
			if ok {
				return end(Candidate, strings.TrimSpace(msg.Content))
			}
			if judged == "" {
				if outside := faultsOutside(out, t.Scope, ws.local()); len(outside) > 0 {
					return end(FailedTask, "the check fails in "+strings.Join(outside, ", ")+", which this unit may not change: "+faultsIn(out, outside))
				}
			}
			if refusedDone++; refusedDone >= 3 {
				return end(Exhausted, "said it was done three times while the check failed: "+tail(out, 8))
			}
			why := judged
			if why == "" {
				why = tail(out, 60)
			}
			msgs = append(msgs, model.Message{Role: "user", Content: "You are not done: the check `" + strings.Join(t.Check, " ") + "` fails.\n\n" + why + "\n\nFix it and run the check again."})
			continue
		}
		for _, call := range msg.ToolCalls {
			res.Steps++
			emit(Event{At: time.Now(), Kind: Called, Tool: call.Function.Name, Detail: clip(call.Function.Arguments, 240)})
			key := call.Function.Name + call.Function.Arguments
			if key == last {
				again++
			} else {
				last, again = key, 1
			}
			if again >= 3 {
				return end(Terminated, "the same call was made three times in a row — "+call.Function.Name)
			}
			ws.last.ran = false
			ws.calls++
			answer, kind, blocked := l.call(ws, call)
			if blocked {
				emit(Event{At: time.Now(), Kind: Said, Detail: answer})
				return end(FailedTask, answer) // the agent says it cannot be done as asked
			}
			switch {
			case strings.HasPrefix(answer, "written") || strings.HasPrefix(answer, "edited"):
				last = "" // the workspace changed: the same call now asks something new
				idle, nudged = 0, false
				asked = map[string]bool{}
			case kind == Got && !asked[key] && fresh < maxFresh:
				// Something not asked before since the last change — a file
				// not read, a test not run — is finding out, not going round:
				// it counts against the idle steps only once enough of it has
				// been done.
				asked[key] = true
				fresh++
			default:
				idle++
			}
			emit(Event{At: time.Now(), Kind: kind, Tool: call.Function.Name, Detail: clip(answer, 2000)})
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: answer})
			if idle >= idleStop {
				return end(Exhausted, fmt.Sprintf("read for %d steps without changing anything", idle))
			}
			// What fails only in files this unit may not change, no agent of
			// this unit can mend: the run says so now, so that what wrote
			// them is carried again, rather than spend its steps.
			if ws.last.ran && !ws.last.ok && ws.last.why == "" {
				if outside := faultsOutside(ws.last.out, t.Scope, ws.local()); len(outside) > 0 {
					return end(FailedTask, "the check fails in "+strings.Join(outside, ", ")+", which this unit may not change: "+faultsIn(ws.last.out, outside))
				}
			}
		}
		// After the turn's answers, never between them: the protocol wants
		// each call answered before anything else is said.
		if idle >= idleNudge && !nudged {
			nudged = true
			nudge := fmt.Sprintf("You have taken %d steps without changing anything. You have read enough: change the file that makes the check pass now, or say in one sentence why no change in your scope can.", idle)
			if f := firstFaults(ws.last.out, 3); !ws.last.ok && f != "" { // the check as it last ran
				nudge += "\n\nThe check fails here — change these lines first:\n" + f
			}
			msgs = append(msgs, model.Message{Role: "user", Content: nudge})
		}
	}
	return end(Exhausted, fmt.Sprintf("stopped after %d steps without finishing", res.Steps))
}

// libraryNote says where a Go repository's libraries can be read.
func libraryNote(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil || len(libraries()) == 0 {
		return ""
	}
	return "\n# Libraries\n\nThe source of the modules go.mod names can be read at " + libraries()[0] + "/<module>@<version>/, and the standard library's at " + libraries()[len(libraries())-1] + "/src/. Read a library's source before you use a name of it you are not sure of.\n"
}

// compact is the conversation as sent: the system, the unit and the latest
// turns whole, older tool answers cut to their first lines.
func compact(msgs []model.Message) []model.Message {
	out := make([]model.Message, len(msgs))
	copy(out, msgs)
	if len(msgs) <= keepWhole+2 {
		return out
	}
	for i := 2; i < len(out)-keepWhole; i++ {
		if out[i].Role == "tool" && len(out[i].Content) > 240 {
			out[i].Content = clip(out[i].Content, 160) + "\n[cut: this answer is old; call the tool again if you need it]"
		}
	}
	return out
}

// call carries out one tool call, and says what it answered, what kind of
// step that was, and whether the agent said its unit cannot be done.
func (l Loop) call(ws *workspace, call model.ToolCall) (string, Kind, bool) {
	t, ok := lookup(call.Function.Name)
	if !ok {
		return "there is no tool called " + call.Function.Name, Refused, false
	}
	args := map[string]string{}
	var raw map[string]any
	if call.Function.Arguments != "" && json.Unmarshal([]byte(repairArgs(call.Function.Arguments)), &raw) != nil {
		return "the arguments are not JSON", Refused, false
	}
	for k, v := range raw {
		args[k] = fmt.Sprint(v)
	}
	out, err := t.run(ws, args)
	if b, ok := err.(Blocked); ok {
		return b.Error(), Got, true
	}
	switch err.(type) {
	case nil:
		return out, Got, false
	case errRefused:
		return "refused: " + err.Error(), Refused, false
	default:
		return "error: " + err.Error(), Got, false
	}
}

// index is what the estate said, with each long section cut to its heading:
// the agent has read it whole once, and reads again what it needs.
func index(context string) string {
	if context == "" {
		return ""
	}
	var b strings.Builder
	for _, sec := range sections(context) {
		head, _, _ := strings.Cut(sec, "\n")
		if len(sec) <= 600 {
			b.WriteString(sec)
		} else {
			b.WriteString(head + "\n\n[shown to you at the start; read_file or search it again for what you need]\n\n")
		}
	}
	return b.String()
}

// sections splits Markdown at its headings, each section with its heading.
func sections(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == '\n') && i > start {
			out = append(out, s[start:i])
			start = i
		}
	}
	return append(out, s[start:])
}

// firstFaults is the first n lines of out that name a file, line and column
// as the place something is wrong.
func firstFaults(out string, n int) string {
	var got []string
	for _, l := range strings.Split(out, "\n") {
		if faultAt.MatchString(l) && len(got) < n {
			got = append(got, strings.TrimSpace(l))
		}
	}
	return strings.Join(got, "\n")
}
