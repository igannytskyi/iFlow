// Package models is which model does each piece of work: kept in the user's
// configuration directory, chosen in the workbench or by hand, and turned
// into what a run is carried out with.
package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
)

// Config is which model does each piece of work, kept in the user's
// configuration directory so that the workbench reads it however it was
// started. A key is never kept here: it is read from the environment or,
// on a Mac, from the keychain, where only its owner puts it.
type Config struct {
	Providers map[string]Provider `json:"providers"`
	// Roles name a model for each kind of work as provider:model —
	// "openrouter:z-ai/glm-5.3-flash", "claude-code:sonnet".
	Roles Roles `json:"roles"`
	// Effort is how hard each role's model thinks: low, medium or high.
	Effort Roles `json:"effort"`

	FloorPricing bool `json:"floor_pricing"`
}

// Roles is one value for each kind of work.
type Roles struct {
	Author   string `json:"author"`
	Reviewer string `json:"reviewer"`
	Planner  string `json:"planner"`
	Executor string `json:"executor"`
	// Stronger takes a unit's last attempt when the executor's failed; none
	// when empty.
	Stronger string `json:"executor_stronger,omitempty"`
}

// Provider is a server speaking the OpenAI chat protocol, and where its key
// is found.
type Provider struct {
	Base string `json:"base"`
	// KeyEnv is the variable holding the key; Keychain the keychain item,
	// by service, read where the variable is not set.
	KeyEnv   string `json:"key_env,omitempty"`
	Keychain string `json:"keychain,omitempty"`
	// Prices, per model, for a server that does not say what an answer
	// cost: a million tokens in, from cache, out.
	Prices map[string]model.Price `json:"prices,omitempty"`
}

// fileOverride is where SetFile points File(); empty, the default.
var fileOverride string

// SetFile points the choice of models at a path of the caller's choosing —
// the seam the tests read and write models.json through. SetFile("")
// restores the default behaviour, the path in the user's configuration
// directory.
func SetFile(path string) { fileOverride = path }

// ModelsFile is where the choice of models is kept.
func File() string {
	if fileOverride != "" {
		return fileOverride
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "iflow", "models.json")
}

// Default is what a first start writes: every role on GLM 5.3 Flash
// through OpenRouter, and DeepSeek beside it to compare, both by one key.
func Default() Config {
	return Config{
		Providers: map[string]Provider{
			"openrouter": {Base: "https://openrouter.ai/api/v1", KeyEnv: "OPENROUTER_API_KEY", Keychain: "iflow.openrouter"},
			"deepseek": {Base: "https://api.deepseek.com/v1", KeyEnv: "DEEPSEEK_API_KEY", Keychain: "iflow.deepseek",
				Prices: map[string]model.Price{"deepseek-chat": {In: 0.30, Cached: 0.03, Out: 1.20}}},
		},
		Roles: Roles{Author: "openrouter:z-ai/glm-5.3-flash", Reviewer: "openrouter:z-ai/glm-5.3-flash",
			Planner: "openrouter:z-ai/glm-5.3-flash", Executor: "openrouter:z-ai/glm-5.3-flash", Stronger: "openrouter:z-ai/glm-5.3"},
		Effort: Roles{Author: "medium", Reviewer: "medium", Planner: "medium", Executor: "low"},
	}
}

// Load reads the choice of models, writing the default on a first start.
func Load() (Config, error) {
	path := File()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		m := Default()
		_ = m.Save()
		return m, nil
	}
	if err != nil {
		return Config{}, err
	}
	var m Config
	if err := json.Unmarshal(b, &m); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Model is the model a role names, ready to be asked.
func (m Config) Model(spec, effort string) (model.Model, error) {
	provider, name, ok := strings.Cut(spec, ":")
	if !ok || name == "" {
		return nil, fmt.Errorf("%q names no model: write it provider:model", spec)
	}
	if provider == "claude-code" {
		return model.ClaudeCode{Model: name, Effort: effort}, nil
	}
	p, ok := m.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("%q names provider %q, which %s does not describe", spec, provider, File())
	}
	key, err := p.key()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", provider, err)
	}
	return model.OpenAI{BaseURL: p.Base, Key: key, Model: name, Provider: provider, Effort: effort,
		Price: p.Prices[name], Floor: m.FloorPricing}, nil
}

// key is the provider's key, from the environment or the keychain.
func (p Provider) key() (string, error) {
	if p.KeyEnv != "" {
		if k := strings.TrimSpace(os.Getenv(p.KeyEnv)); k != "" {
			return k, nil
		}
	}
	if p.Keychain != "" {
		out, err := exec.Command("security", "find-generic-password", "-s", p.Keychain, "-w").Output()
		if k := strings.TrimSpace(string(out)); err == nil && k != "" {
			return k, nil
		}
	}
	return "", fmt.Errorf("no key: set %s, or put it in the keychain as %s", p.KeyEnv, p.Keychain)
}

// Describe writes which model does what and whether its key is found —
// never the key.
func (m Config) Describe(w io.Writer) {
	fmt.Fprintf(w, "models, from %s\n\n", File())
	for _, r := range []struct{ role, spec, effort string }{
		{"author", m.Roles.Author, m.Effort.Author}, {"reviewer", m.Roles.Reviewer, m.Effort.Reviewer},
		{"planner", m.Roles.Planner, m.Effort.Planner}, {"executor", m.Roles.Executor, m.Effort.Executor},
		{"agent", m.Roles.Stronger, m.Effort.Executor}, // the executor's second rung, where set
	} {
		if r.spec == "" {
			continue
		}
		state := "ready"
		if _, err := m.Model(r.spec, r.effort); err != nil {
			state = err.Error()
		}
		fmt.Fprintf(w, "  %-9s %-36s effort %-7s %s\n", r.role, r.spec, r.effort, state)
	}
}

// Save writes the choice of models where Load reads it.
func (m Config) Save() error {
	path := File()
	if path == "" {
		return errors.New("no configuration directory")
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(path, append(out, '\n'), 0o644)
}

// Flow fills in what base is carried out with: the model of each role, from
// the choice and from the environment —
//
//	IFLOW_MODEL           every role on this model, as provider:model
//	IFLOW_EXECUTOR=agent  units handed to Claude Code's own agent
//
// A role whose model cannot be had — no key, an unknown provider — falls
// back to Claude Code, and the second value says why.
func (m Config) Flow(base flow.Config) (flow.Config, string) {
	roles := m.Roles
	if all := os.Getenv("IFLOW_MODEL"); all != "" {
		roles = Roles{all, all, all, all, ""}
	}
	var problems []string
	pick := func(role, spec, effort string) model.Model {
		md, err := m.Model(spec, effort)
		if err != nil {
			problems = append(problems, role+": "+err.Error()+" — Claude Code (sonnet) does it instead")
			return model.ClaudeCode{Model: "sonnet", Effort: effort}
		}
		return md
	}
	base.Model = pick("author", roles.Author, m.Effort.Author)
	base.Reviewer = pick("reviewer", roles.Reviewer, m.Effort.Reviewer)
	base.Planner = pick("planner", roles.Planner, m.Effort.Planner)
	cheap := pick("executor", roles.Executor, m.Effort.Executor)
	agent := cheap
	if roles.Stronger != "" {
		agent = pick("stronger executor", roles.Stronger, m.Effort.Executor)
	}
	// A unit goes first to one question of the cheap model; what that cannot
	// carry goes to an agent that reads, runs the check and tries again.
	attempts := base.Attempts
	if attempts <= 0 {
		attempts = 2
	}
	base.Executor = harness.Ladder{First: harness.Oneshot{Model: cheap, Attempts: attempts}, Then: Agent(agent)}
	if os.Getenv("IFLOW_EXECUTOR") == "agent" {
		base.Executor = harness.ClaudeCode{Model: "sonnet"}
	}
	return base, strings.Join(problems, "\n")
}

// Agent is the executor that works with tools on a model: Claude Code's own
// agent for a model it runs, this method's loop for any other.
func Agent(m model.Model) harness.Executor {
	switch md := m.(type) {
	case model.ClaudeCode:
		return harness.ClaudeCode{Model: md.Model}
	case harness.Chatter:
		return harness.Loop{Model: md}
	}
	return harness.Oneshot{Model: m}
}

// Choice is a model one can pick for a role, with what it costs.
type Choice struct {
	Spec, Label, Price string
}

// Catalogue is the models offered for picking: prices as OpenRouter listed
// them on 2026-11-15, per million tokens in and out; Claude Code runs on a
// subscription and its price is what the same work would cost by the API.
var Catalogue = []Choice{
	{"openrouter:z-ai/glm-5.3-flash", "GLM 5.3 Flash", "$0.04 / $0.08"},
	{"openrouter:z-ai/glm-5.3", "GLM 5.3", "$0.08 / $3.60"},
	{"openrouter:deepseek/deepseek-v4.1-flash", "DeepSeek V4.1 Flash", "$0.05 / $0.20"},
	{"claude-code:haiku", "Claude Haiku", "subscription"},
	{"claude-code:sonnet", "Claude Sonnet", "subscription"},
}

// Try asks each model named a one-word question, once, and says what the
// answer cost — the proof that a key works, at the price of a few tokens.
func (m Config) Try(ctx context.Context, w io.Writer) int {
	fmt.Fprintln(w)
	seen := map[string]bool{}
	failed := false
	for _, spec := range []string{m.Roles.Author, m.Roles.Reviewer, m.Roles.Planner, m.Roles.Executor, m.Roles.Stronger} {
		if spec == "" || seen[spec] {
			continue
		}
		seen[spec] = true
		md, err := m.Model(spec, "low")
		if err != nil {
			fmt.Fprintf(w, "  %-36s %v\n", spec, err)
			failed = true
			continue
		}
		began := time.Now()
		answer, u, err := md.Ask(ctx, "Answer with one word.", "Say OK.")
		if err != nil {
			fmt.Fprintf(w, "  %-36s %v\n", spec, err)
			failed = true
			continue
		}
		fmt.Fprintf(w, "  %-36s answered %q in %s: %d tokens in, %d from cache, %d out, $%.6f\n",
			spec, strings.TrimSpace(answer), time.Since(began).Round(time.Millisecond), u.Input, u.CacheRead, u.Output, u.CostUSD)
	}
	if failed {
		return 1
	}
	return 0
}

// Taped is cfg with every model it asks going through t: its answers
// recorded on the tape, or replayed from it. An executor that is not the
// framework's own — Claude Code's agent — cannot be taped, and is said.
func Taped(cfg flow.Config, t *model.Tape) (flow.Config, error) {
	tape := func(m model.Model) model.Model {
		if m == nil {
			return nil
		}
		return t.Taped(m, m.Name())
	}
	cfg.Model, cfg.Reviewer, cfg.Planner = tape(cfg.Model), tape(cfg.Reviewer), tape(cfg.Planner)
	var executor func(harness.Executor) (harness.Executor, error)
	executor = func(e harness.Executor) (harness.Executor, error) {
		switch x := e.(type) {
		case harness.Ladder:
			first, err := executor(x.First)
			if err != nil {
				return nil, err
			}
			then, err := executor(x.Then)
			if err != nil {
				return nil, err
			}
			return harness.Ladder{First: first, Then: then}, nil
		case harness.Oneshot:
			x.Model, x.Stronger = tape(x.Model), tape(x.Stronger)
			return x, nil
		case harness.Loop:
			return harness.Loop{Model: t.TapedChat(x.Model)}, nil
		case nil:
			return nil, nil
		}
		return nil, fmt.Errorf("%s cannot be taped: its model is asked outside the framework", e.Name())
	}
	ex, err := executor(cfg.Executor)
	if err != nil {
		return cfg, err
	}
	cfg.Executor = ex
	return cfg, nil
}

// Replayed is base made as Flow makes it, its models stand-ins that answer
// nothing: a run whose every answer comes from a tape asks none of them,
// and needs no key.
func Replayed(base flow.Config) flow.Config {
	m := replayed{}
	base.Model, base.Reviewer, base.Planner = m, m, m
	attempts := base.Attempts
	if attempts <= 0 {
		attempts = 2
	}
	base.Executor = harness.Ladder{First: harness.Oneshot{Model: m, Attempts: attempts}, Then: harness.Loop{Model: m}}
	return base
}

// replayed is a model asked nothing: in a replay, the tape answers.
type replayed struct{}

func (replayed) Name() string { return "replayed" }
func (replayed) Ask(context.Context, string, string) (string, model.Usage, error) {
	return "", model.Usage{}, errors.New("a replayed run asks no model")
}
func (replayed) Chat(context.Context, []model.Message, []model.Tool) (model.Message, model.Usage, error) {
	return model.Message{}, model.Usage{}, errors.New("a replayed run asks no model")
}
