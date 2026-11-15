package harness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// ClaudeCode delegates a unit to the Claude Code program, headless, confined
// to the workspace, its edits accepted and its shell limited to the check
// command. Its own loop does the work; this reads its stream of steps.
type ClaudeCode struct {
	Binary string
	Model  string
}

// Name implements Executor.
func (c ClaudeCode) Name() string { return "Claude Code" }

func (c ClaudeCode) binary() string {
	if c.Binary != "" {
		return c.Binary
	}
	return "claude"
}

// Version implements Executor.
func (c ClaudeCode) Version() string {
	out, err := exec.Command(c.binary(), "--version").Output()
	if err != nil {
		return "unknown"
	}
	v := strings.TrimSpace(string(out))
	if c.Model != "" {
		v += " / " + c.Model
	}
	return v
}

const confined = `You carry out one unit of work in a repository copied for you. Change only the files in scope. Do not add or change tests unless the unit says it is the one that writes them. When you are done, say in two sentences what you changed and why.`

// Run implements Executor.
func (c ClaudeCode) Run(ctx context.Context, t Task, emit func(Event)) (Result, error) {
	allowed := []string{"Read", "Edit", "Write", "Glob", "Grep", "LS"}
	if len(t.Check) > 0 {
		// The check however the agent spells it: by its path, by its name,
		// or as ./name from the workspace.
		name := filepath.Base(t.Check[0])
		allowed = append(allowed, "Bash("+t.Check[0]+":*)", "Bash("+name+":*)", "Bash(./"+name+":*)")
	}
	args := []string{"-p", "--output-format", "stream-json", "--verbose",
		"--permission-mode", "acceptEdits", "--append-system-prompt", confined,
		"--allowedTools", strings.Join(allowed, ",")}
	if t.BudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprint(t.BudgetUSD))
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	cmd := exec.CommandContext(ctx, c.binary(), args...)
	cmd.Dir = t.Workspace
	cmd.Stdin = strings.NewReader(prompt(t))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Terminal: FailedSubstrate}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{Terminal: FailedSubstrate}, fmt.Errorf("harness: %s: %w", c.binary(), err)
	}
	res := Result{Terminal: FailedSubstrate}
	started := time.Now()
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		c.read(sc.Bytes(), &res, emit)
	}
	werr := cmd.Wait()
	switch {
	case ctx.Err() != nil:
		res.Terminal = Cancelled
	case werr != nil && res.Terminal == FailedSubstrate:
		return res, fmt.Errorf("harness: %s ended: %w: %s", c.binary(), werr, strings.TrimSpace(stderr.String()))
	}
	res.Elapsed = time.Since(started)
	emit(Event{At: time.Now(), Kind: Ended, Detail: res.Terminal, Usage: res.Usage})
	return res, nil
}

type messageUsage struct {
	Input       int `json:"input_tokens"`
	CacheRead   int `json:"cache_read_input_tokens"`
	CacheCreate int `json:"cache_creation_input_tokens"`
	Output      int `json:"output_tokens"`
}

func (m messageUsage) of(model_ string) model.Usage {
	return model.Usage{Model: model_, Input: m.Input, CacheRead: m.CacheRead, CacheWrite: m.CacheCreate, Output: m.Output}
}

// read turns one line of the program's stream into steps.
func (c ClaudeCode) read(line []byte, res *Result, emit func(Event)) {
	var ev struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Model   string `json:"model"`
		Message struct {
			Model   string       `json:"model"`
			Usage   messageUsage `json:"usage"`
			Content []struct {
				Type    string          `json:"type"`
				Text    string          `json:"text"`
				Name    string          `json:"name"`
				Input   json.RawMessage `json:"input"`
				Content json.RawMessage `json:"content"`
				IsError bool            `json:"is_error"`
			} `json:"content"`
		} `json:"message"`
		Result     string                     `json:"result"`
		Cost       float64                    `json:"total_cost_usd"`
		Turns      int                        `json:"num_turns"`
		IsError    bool                       `json:"is_error"`
		Usage      messageUsage               `json:"usage"`
		ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	}
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	now := time.Now()
	switch ev.Type {
	case "system":
		if ev.Subtype == "init" && ev.Model != "" {
			res.Usage.Model = strings.TrimSuffix(ev.Model, "[1m]")
		}
	case "assistant":
		// What this model turn cost rides on its first step.
		turn := ev.Message.Usage.of(ev.Message.Model)
		res.Turns++
		for _, b := range ev.Message.Content {
			var e Event
			switch b.Type {
			case "text":
				s := strings.TrimSpace(b.Text)
				if s == "" {
					continue
				}
				e = Event{At: now, Kind: Said, Detail: s}
			case "tool_use":
				res.Steps++
				e = Event{At: now, Kind: Called, Tool: b.Name, Detail: brief(b.Input)}
			default:
				continue
			}
			e.Usage, turn = turn, model.Usage{}
			emit(e)
		}
		if !turn.Zero() {
			emit(Event{At: now, Kind: Spent, Usage: turn})
		}
	case "user":
		for _, b := range ev.Message.Content {
			if b.Type == "tool_result" {
				kind := Got
				if b.IsError {
					kind = Refused
				}
				emit(Event{At: now, Kind: kind, Detail: text(b.Content)})
			}
		}
	case "result":
		res.Summary = ev.Result
		name := res.Usage.Model
		if len(ev.ModelUsage) > 0 {
			var names []string
			for k := range ev.ModelUsage {
				names = append(names, strings.TrimSuffix(k, "[1m]"))
			}
			slices.Sort(names)
			name = strings.Join(names, ", ")
		}
		res.Usage = ev.Usage.of(name)
		res.Usage.CostUSD = ev.Cost
		switch {
		case ev.Subtype == "success" && !ev.IsError:
			res.Terminal = Candidate
		case strings.Contains(ev.Subtype, "max_budget") || strings.Contains(ev.Subtype, "max_turns"):
			res.Terminal = Exhausted
		default:
			res.Terminal = FailedTask
		}
	}
}

// brief is a tool's input, cut to one readable line.
func brief(raw json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return clip(string(raw), 240)
	}
	for _, k := range []string{"file_path", "path", "command", "pattern"} {
		if v, ok := m[k].(string); ok {
			return clip(v, 240)
		}
	}
	return clip(string(raw), 240)
}

// text is a tool result's content, which arrives as a string or as blocks.
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return clip(s, 2000)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			parts = append(parts, b.Text)
		}
		return clip(strings.Join(parts, "\n"), 2000)
	}
	return clip(string(raw), 2000)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " …"
}

// prompt is the task as the agent reads it.
func prompt(t Task) string {
	var b strings.Builder
	b.WriteString("# The unit\n\n" + t.Goal + "\n\n# Scope — the only files you may change\n\n")
	for _, s := range t.Scope {
		b.WriteString("- " + s + "\n")
	}
	if len(t.Check) > 0 {
		b.WriteString("\n# How to see where you stand\n\nRun `" + strings.Join(t.Check, " ") + "`. It is the estate's own suite; it is not what will judge you.\n")
	}
	if t.Context != "" {
		b.WriteString("\n# What the estate says about the scope\n\n" + t.Context + "\n")
	}
	return b.String()
}

var errNoModel = errors.New("harness: no model configured")
