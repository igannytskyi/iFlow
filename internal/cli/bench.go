package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/gate"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/models"
)

// suite is the reference tasks the framework is measured by.
type suite struct {
	About string      `json:"about"`
	Tasks []benchTask `json:"tasks"`
}

// benchTask is one reference task: an intent carried on a fresh copy of an
// estate, and what it should come to — landed, or holds when what it asks
// is true already.
type benchTask struct {
	Name      string `json:"name"`
	Estate    string `json:"estate"`
	Statement string `json:"statement"`
	Expect    string `json:"expect,omitempty"`
	// Commit, where given, makes the estate the framework's own repository
	// as it stood at that commit: a task the size of real work, on code that
	// does not move under it from one run to the next.
	Commit string `json:"commit,omitempty"`
}

// benchResult is one task carried: what it came to and what it took. One
// line of .estate/bench.jsonl.
type benchResult struct {
	At      string            `json:"at"`
	Commit  string            `json:"commit"`
	Label   string            `json:"label,omitempty"`
	Set     map[string]string `json:"set,omitempty"`
	Task    string            `json:"task"`
	Expect  string            `json:"expect"`
	Outcome string            `json:"outcome"`
	Why     string            `json:"why,omitempty"`
	OK      bool              `json:"ok"`
	USD     float64           `json:"usd"`
	In      int               `json:"in"`
	Out     int               `json:"out"`
	Calls   int               `json:"calls"`
	Seconds float64           `json:"seconds"`
	// Kept is where a task that did not come to what it should was left.
	Kept string `json:"kept,omitempty"`
	// Tape is where every answer the models gave was recorded; Replayed,
	// the tape a replayed run answered from instead.
	Tape     string `json:"tape,omitempty"`
	Replayed string `json:"replayed,omitempty"`
}

// tapeRun is what a run of the bench was made with, kept beside its tapes,
// so that a replay is made the same way.
type tapeRun struct {
	Commit string            `json:"commit"`
	Label  string            `json:"label,omitempty"`
	Set    map[string]string `json:"set,omitempty"`
	Tasks  []string          `json:"tasks"`
}

// Bench carries every reference task — or those named with --only — with
// every question answered yes, each on a fresh copy of its estate, and says
// how many came to what they should, at what cost. --set varies the run:
// review=N rounds of review, attempts=N one-shot answers before the agent,
// repeat=once to set a claim to repeat against its kept evidence. Results
// are appended to .estate/bench.jsonl under the framework, labelled with
// --label, so that variants can be set side by side.
func (app *App) Bench(ctx context.Context, args []string) int {
	home := gate.Home()
	file := filepath.Join(home, "examples", "bench.json")
	var only []string
	set := map[string]string{}
	label, parallel, replay := "", 2, ""
	for i := 0; i < len(args); i++ {
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch args[i] {
		case "--suite":
			file = next()
		case "--only":
			only = strings.Split(next(), ",")
		case "--label":
			label = next()
		case "--parallel":
			parallel, _ = strconv.Atoi(next())
		case "--replay":
			replay = next()
		case "--set":
			for _, kv := range strings.Split(next(), ",") {
				if k, v, ok := strings.Cut(kv, "="); ok {
					set[k] = v
				}
			}
		default:
			fmt.Fprintln(app.Out, "usage: iflow bench [--suite file] [--only a,b] [--set review=1,attempts=1,repeat=once] [--label name] [--parallel n] [--replay <tapes>]")
			return Misused
		}
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return app.fatal(err)
	}
	var s suite
	if err := json.Unmarshal(b, &s); err != nil {
		return app.fatal(fmt.Errorf("%s: %w", file, err))
	}
	// A replay is made as the run it replays was: its tasks, its settings,
	// and every answer from its tapes.
	if replay != "" {
		b, err := os.ReadFile(filepath.Join(replay, "run.json"))
		if err != nil {
			return app.fatal(fmt.Errorf("%s holds no recorded run: %w", replay, err))
		}
		var was tapeRun
		if err := json.Unmarshal(b, &was); err != nil {
			return app.fatal(err)
		}
		if len(only) == 0 {
			only = was.Tasks
		}
		set = was.Set
		if label == "" {
			label = "replay of " + filepath.Base(replay)
		}
	}
	var tasks []benchTask
	for _, t := range s.Tasks {
		if len(only) == 0 || slices.Contains(only, t.Name) {
			if t.Expect == "" {
				t.Expect = "landed"
			}
			tasks = append(tasks, t)
		}
	}
	cfg, problems := app.FlowConfig()
	if replay != "" {
		// Nothing is asked of a model: the tapes answer. No key is needed.
		cfg, problems = models.Replayed(cfg), ""
		for k := range set {
			if k == "author" || k == "reviewer" || k == "planner" {
				delete(set, k) // a model's answers are on the tape; which model gave them is not asked again
			}
		}
	}
	if problems != "" {
		fmt.Fprintln(app.Err, problems)
	}
	cfg.Person = "the bench, which answers yes"
	if err := vary(&cfg, set); err != nil {
		return app.fatal(err)
	}
	commit := gitHead(home)
	// Every answer is recorded, by task, beside what the run was made with.
	tapes := filepath.Join(home, ".estate", "tapes", time.Now().Format("2006-01-02T15-04-05")+"-"+strings.ReplaceAll(cmp.Or(label, "run"), " ", "-"))
	if replay == "" {
		names := make([]string, len(tasks))
		for i, t := range tasks {
			names[i] = t.Name
		}
		if err := os.MkdirAll(tapes, 0o755); err != nil {
			return app.fatal(err)
		}
		was, _ := json.MarshalIndent(tapeRun{Commit: commit, Label: label, Set: set, Tasks: names}, "", "  ")
		if err := os.WriteFile(filepath.Join(tapes, "run.json"), was, 0o644); err != nil {
			return app.fatal(err)
		}
		fmt.Fprintf(app.Out, "  every answer is recorded under %s\n", tapes)
	}
	results := make([]benchResult, len(tasks))
	sem := make(chan struct{}, max(parallel, 1))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			estate := filepath.Join(filepath.Dir(file), t.Estate)
			if t.Commit != "" {
				estate = home
			}
			var tape *model.Tape
			var err error
			path := filepath.Join(tapes, t.Name+".jsonl")
			if replay != "" {
				path = filepath.Join(replay, t.Name+".jsonl")
				tape, err = model.Replay(path)
			} else {
				tape, err = model.Record(path)
			}
			if err != nil {
				results[i] = benchResult{Task: t.Name, Outcome: "broken", Why: err.Error()}
				return
			}
			defer tape.Close()
			taped, err := models.Taped(cfg, tape)
			if err != nil {
				results[i] = benchResult{Task: t.Name, Outcome: "broken", Why: err.Error()}
				return
			}
			results[i] = benchOne(ctx, taped, estate, t)
			results[i].Commit, results[i].Label, results[i].Set = commit, label, set
			if replay != "" {
				results[i].Replayed = path
			} else {
				results[i].Tape = path
			}
			r := results[i]
			fmt.Fprintf(app.Out, "  %-18s %-8s %-6v $%.3f  %7d in  %6d out  %4.0fs  %s\n", r.Task, r.Outcome, r.OK, r.USD, r.In, r.Out, r.Seconds, clipLine(r.Why, 90))
			if r.Kept != "" {
				fmt.Fprintf(app.Out, "  %-18s kept at %s\n", "", r.Kept)
			}
		})
	}
	wg.Wait()
	if err := appendResults(filepath.Join(home, ".estate", "bench.jsonl"), results); err != nil {
		fmt.Fprintln(app.Err, err)
	}
	var ok int
	var usd, secs float64
	var in, out int
	for _, r := range results {
		if r.OK {
			ok++
		}
		usd, in, out, secs = usd+r.USD, in+r.In, out+r.Out, secs+r.Seconds
	}
	fmt.Fprintf(app.Out, "\n  %d of %d as expected; $%.3f, %d tokens in, %d out, %.0fs of work\n", ok, len(results), usd, in, out, secs)
	if ok < len(results) {
		return Failed
	}
	return OK
}

// vary sets what --set names on the run's configuration.
func vary(cfg *flow.Config, set map[string]string) error {
	for k, v := range set {
		switch k {
		case "author", "reviewer", "planner":
			// A role's model, as a role in the models' file names one:
			// provider:model.
			m, err := models.Load()
			if err != nil {
				return err
			}
			md, err := m.Model(v, "medium")
			if err != nil {
				return fmt.Errorf("%s=%s: %w", k, v, err)
			}
			switch k {
			case "author":
				cfg.Model = md
			case "reviewer":
				cfg.Reviewer = md
			default:
				cfg.Planner = md
			}
		case "review":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("review=%s: a number of rounds, one or more", v)
			}
			cfg.ReviewRounds = n
		case "attempts":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("attempts=%s: a number of answers, one or more", v)
			}
			cfg.Attempts = n
		case "repeat":
			if v != "once" && v != "twice" {
				return fmt.Errorf("repeat=%s: once or twice", v)
			}
			cfg.RepeatOnce = v == "once"
		default:
			return fmt.Errorf("--set %s: not something the bench varies (review, attempts, repeat, author, reviewer, planner)", k)
		}
	}
	return nil
}

// benchOne carries one task on a fresh copy of its estate.
func benchOne(ctx context.Context, cfg flow.Config, estate string, t benchTask) (res benchResult) {
	res = benchResult{At: time.Now().Format(time.RFC3339), Task: t.Name, Expect: t.Expect}
	dir, err := os.MkdirTemp("", "iflow-bench-"+t.Name+"-")
	if err != nil {
		res.Outcome, res.Why = "broken", err.Error()
		return res
	}
	repo := filepath.Join(dir, filepath.Base(estate))
	// A task that came to what it should leaves nothing; one that did not
	// is kept where it ran, its change folder and all, to be read.
	defer func() {
		if res.OK {
			os.RemoveAll(dir)
		} else {
			res.Kept = repo
		}
	}()
	if err := lay(estate, repo, t.Commit); err != nil {
		res.Outcome, res.Why = "broken", err.Error()
		return res
	}
	if exec.Command("git", "-C", repo, "init", "-q").Run() == nil {
		_ = exec.Command("git", "-C", repo, "add", "-A").Run()
		_ = exec.Command("git", "-C", repo, "-c", "user.name=bench", "-c", "user.email=bench@localhost", "commit", "-qm", "estate").Run()
	}
	var mu sync.Mutex
	answered := map[string]bool{}
	var run *flow.Run
	run = flow.New(cfg, repo, t.Statement, func() {
		if run == nil {
			return
		}
		v := run.View()
		if v.Question == "" {
			return
		}
		key := fmt.Sprint(v.Areas, v.Question, len(v.Steps))
		mu.Lock()
		defer mu.Unlock()
		if !answered[key] {
			answered[key] = true
			go run.Accept()
		}
	})
	began := time.Now()
	err = run.Start(ctx)
	// What the run said, step by step, is kept beside a run that did not
	// come to what it should: a run stopped before its change was written
	// leaves no other word of why.
	defer func() {
		if !res.OK {
			var b strings.Builder
			for _, st := range run.View().Steps {
				mark := " "
				if st.Bad {
					mark = "!"
				}
				fmt.Fprintf(&b, "%s %d  %s\n", mark, st.Area, st.Text)
			}
			_ = os.WriteFile(filepath.Join(dir, "steps.txt"), []byte(b.String()), 0o644)
		}
	}()
	res.Seconds = time.Since(began).Seconds()
	v := run.View()
	res.USD, res.In, res.Out, res.Calls = v.Spent.CostUSD, v.Spent.In(), v.Spent.Output, len(v.Costs)
	switch {
	case err == nil:
		res.Outcome = "landed"
	case errors.Is(err, flow.ErrHolds):
		res.Outcome = "holds"
	default:
		res.Outcome, res.Why = "stopped", err.Error()
	}
	res.OK = res.Outcome == t.Expect
	return res
}

func appendResults(path string, results []benchResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range results {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func clipLine(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// lay puts the estate at repo: a copy of the folder, or, for a commit, the
// repository at estate as it stood at that commit, its tracked files only.
func lay(estate, repo, commit string) error {
	if commit == "" {
		if out, err := exec.Command("cp", "-R", estate, repo).CombinedOutput(); err != nil {
			return errors.New(strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return err
	}
	archive := exec.Command("git", "-C", estate, "archive", "--format=tar", commit)
	untar := exec.Command("tar", "-x", "-C", repo)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	untar.Stdin = pipe
	var stderr strings.Builder
	archive.Stderr = &stderr
	if err := untar.Start(); err != nil {
		return err
	}
	if err := archive.Run(); err != nil {
		return fmt.Errorf("the estate at %s could not be read: %s", commit, strings.TrimSpace(stderr.String()))
	}
	return untar.Wait()
}
