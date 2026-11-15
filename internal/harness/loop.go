package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// Loop is this method's own harness: a model that calls tools, in a loop the
// harness owns. It runs on any model served over the chat-completions
// protocol, so an open model on a local server does the work as well as a
// hosted one, and what bounds the run is decided here rather than by the
// model's own judgement of when it is done.
type Loop struct {
	Model model.OpenAI
}

// Name implements Executor.
func (l Loop) Name() string { return "iflow loop" }

// Version implements Executor.
func (l Loop) Version() string { return "loop/1 on " + l.Model.Model }

const loopSystem = `You carry out one unit of work in a repository, using the tools you are given. Read before you change anything. Change only files in the unit's scope; the harness refuses anything else. Run the check when you think you are done. When the unit is done, answer without calling a tool, in two sentences: what you changed and why.`

// Run implements Executor.
func (l Loop) Run(ctx context.Context, t Task, emit func(Event)) (Result, error) {
	if l.Model.Model == "" {
		return Result{Terminal: FailedSubstrate}, errNoModel
	}
	ws := &workspace{dir: t.Workspace, scope: t.Scope, check: t.Check, ctx: ctx}
	msgs := []model.Message{{Role: "system", Content: loopSystem}, {Role: "user", Content: prompt(t)}}
	maxSteps := t.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 40
	}
	var res Result
	started := time.Now()
	done := func() Result { res.Elapsed = time.Since(started); return res }
	repeats := map[string]int{}
	for res.Steps < maxSteps {
		if ctx.Err() != nil {
			res.Terminal = Cancelled
			break
		}
		msg, u, err := l.Model.Chat(ctx, msgs, specs())
		res.Usage = res.Usage.Add(u)
		res.Turns++
		emit(Event{At: time.Now(), Kind: Spent, Usage: u})
		if err != nil {
			res.Terminal = FailedSubstrate
			emit(Event{At: time.Now(), Kind: Ended, Detail: err.Error(), Usage: res.Usage})
			return done(), err
		}
		msg.Role = "assistant"
		msgs = append(msgs, msg)
		if s := strings.TrimSpace(msg.Content); s != "" {
			emit(Event{At: time.Now(), Kind: Said, Detail: s})
		}
		if len(msg.ToolCalls) == 0 {
			res.Terminal, res.Summary = Candidate, strings.TrimSpace(msg.Content)
			break
		}
		for _, call := range msg.ToolCalls {
			res.Steps++
			emit(Event{At: time.Now(), Kind: Called, Tool: call.Function.Name, Detail: clip(call.Function.Arguments, 240)})
			// The same call made three times is a loop, not progress: the
			// run stops rather than paying for a fourth.
			key := call.Function.Name + call.Function.Arguments
			if repeats[key]++; repeats[key] >= 3 {
				res.Terminal = Terminated
				res.Summary = "stopped: the same call was made three times — " + call.Function.Name
				emit(Event{At: time.Now(), Kind: Ended, Detail: res.Summary, Usage: res.Usage})
				return done(), nil
			}
			answer, kind := l.call(ws, call)
			emit(Event{At: time.Now(), Kind: kind, Tool: call.Function.Name, Detail: clip(answer, 2000)})
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: answer})
		}
	}
	if res.Terminal == "" {
		res.Terminal = Exhausted
		res.Summary = fmt.Sprintf("stopped after %d steps without finishing", res.Steps)
	}
	emit(Event{At: time.Now(), Kind: Ended, Detail: res.Terminal, Usage: res.Usage})
	return done(), nil
}

func (l Loop) call(ws *workspace, call model.ToolCall) (string, Kind) {
	t, ok := lookup(call.Function.Name)
	if !ok {
		return "there is no tool called " + call.Function.Name, Refused
	}
	args := map[string]string{}
	var raw map[string]any
	if call.Function.Arguments != "" && json.Unmarshal([]byte(call.Function.Arguments), &raw) != nil {
		return "the arguments are not JSON", Refused
	}
	for k, v := range raw {
		args[k] = fmt.Sprint(v)
	}
	out, err := t.run(ws, args)
	switch err.(type) {
	case nil:
		return out, Got
	case errRefused:
		return "refused: " + err.Error(), Refused
	default:
		return "error: " + err.Error(), Got
	}
}
