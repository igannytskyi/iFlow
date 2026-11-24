// Package service carries runs for every screen that asks: the window, a
// terminal, another program. It holds the runs; the screens hold nothing
// but what they were told, and speak to it by the protocol alone. A run
// goes on when the screen that started it closes, and every screen that
// watches it is told what moved.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/protocol"
)

// Run is a run the service carries; *flow.Run is one.
type Run interface {
	Start(ctx context.Context) error
	View() flow.View
	Accept()
	Refuse(why string)
}

// Maker makes the runs the service carries, each telling changed when it
// moves.
type Maker interface {
	New(project, statement string, settles []string, changed func()) (Run, error)
	Resume(dir string, settles []string, changed func()) (Run, error)
	Reopen(dir string, from int, notes string, settles []string, changed func()) (Run, error)
}

// Config is what a service is made with.
type Config struct {
	// Maker makes the runs it carries.
	Maker Maker
	// Person is who stops a run when a screen asks it to.
	Person string
	// Projects is the folders of the projects the person works on.
	Projects func() []string
	// Model is who a conversation beside the work talks with; nil leaves
	// the service without conversations.
	Model func() model.Model
	// KeepFile is where a service asked to shut down keeping its runs
	// leaves them, and where the next one finds them to carry on.
	KeepFile string
}

// Service is the runs being carried, and who watches each, and the
// conversations beside them.
type Service struct {
	maker    Maker
	person   string
	ctx      context.Context
	projects func() []string
	model    func() model.Model
	talk     talk

	since    time.Time
	keepFile string
	quit     context.CancelFunc

	mu   sync.Mutex
	n    int
	runs map[string]*entry
	// order is the runs' ids in the order they were started.
	order []string
}

// New is a service made with cfg. Its runs end when ctx does.
func New(ctx context.Context, cfg Config) *Service {
	return &Service{maker: cfg.Maker, person: cfg.Person, projects: cfg.Projects, model: cfg.Model, ctx: ctx, runs: map[string]*entry{},
		since: time.Now(), keepFile: cfg.KeepFile}
}

// entry is one run, and the screens watching it.
type entry struct {
	id, project string
	run         Run
	cancel      context.CancelFunc

	mu       sync.Mutex
	started  bool
	finished bool
	stopped  bool // asked to stop: by a screen, or by the service ending
	err      error
	watchers map[*protocol.Conn]*watcher
}

// watcher is one screen watching a run: what it was told so far, and a
// signal, holding at most one, that there is more.
type watcher struct {
	conn  *protocol.Conn
	more  chan struct{}
	steps int
	agent map[string]int
	stop  chan struct{}
}

// changed tells every watcher there is more; a watcher already told waits
// for nothing more to be said twice.
func (e *entry) changed() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, w := range e.watchers {
		select {
		case w.more <- struct{}{}:
		default:
		}
	}
}

// start makes a run with make and carries it, under a new id.
func (s *Service) start(project string, make func(changed func()) (Run, error)) (protocol.RunRef, error) {
	s.mu.Lock()
	s.n++
	e := &entry{id: fmt.Sprintf("run-%d", s.n), project: project, watchers: map[*protocol.Conn]*watcher{}}
	s.mu.Unlock()
	run, err := make(e.changed)
	if err != nil {
		return protocol.RunRef{}, err
	}
	ctx, cancel := context.WithCancel(s.ctx)
	e.run, e.cancel = run, cancel
	s.mu.Lock()
	s.runs[e.id] = e
	s.order = append(s.order, e.id)
	s.mu.Unlock()
	e.mu.Lock()
	e.started = true
	e.mu.Unlock()
	go func() {
		err := run.Start(ctx)
		e.mu.Lock()
		e.finished, e.err = true, err
		e.mu.Unlock()
		e.changed()
	}()
	return protocol.RunRef{Run: e.id}, nil
}

func (s *Service) entry(id string) (*entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.runs[id]
	if !ok {
		return nil, &protocol.Error{Code: protocol.Refused, Message: "no run " + id + " is carried here"}
	}
	return e, nil
}

// Handle answers one call of a screen.
func (s *Service) Handle(ctx context.Context, c *protocol.Conn, method string, params json.RawMessage) (any, error) {
	switch method {
	case protocol.Initialize:
		var p protocol.InitializeParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		// Answered in every version: a screen of another version reads who
		// the service is, and replaces it where it is the older.
		return s.identity(), nil
	case protocol.Shutdown:
		var p protocol.ShutdownParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return s.shutdown(p.Keep), nil
	case protocol.Runs:
		return s.summaries(), nil
	case protocol.Projects:
		var p protocol.ProjectsParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		var paths []string
		if s.projects != nil {
			paths = s.projects()
		}
		for _, a := range p.Also {
			if !slices.Contains(paths, a) {
				paths = append(paths, a)
			}
		}
		return Catalog(paths), nil
	case protocol.Record:
		var p protocol.ChangeRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return record(p.Dir)
	case protocol.Get:
		var p protocol.ChangeRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return document(p.Dir)
	case protocol.Trace:
		var p protocol.TraceParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return trace(p)
	case protocol.Diff:
		var p protocol.TraceParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return diff(p)
	case protocol.Ask:
		var p protocol.AskParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return nil, s.ask(p)
	case protocol.Listen:
		var p protocol.ChatRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		s.talk.watch(p, c)
		return nil, nil
	case protocol.Start:
		var p protocol.StartParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		if p.Project == "" || p.Statement == "" {
			return nil, &protocol.Error{Code: protocol.InvalidParams, Message: "a run is started on a project, with a statement"}
		}
		return s.start(p.Project, func(changed func()) (Run, error) {
			return s.maker.New(p.Project, p.Statement, p.Settles, changed)
		})
	case protocol.Resume:
		var p protocol.ResumeParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return s.start(projectOf(p.Dir), func(changed func()) (Run, error) { return s.maker.Resume(p.Dir, p.Settles, changed) })
	case protocol.Reopen:
		var p protocol.ReopenParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		return s.start(projectOf(p.Dir), func(changed func()) (Run, error) {
			return s.maker.Reopen(p.Dir, p.From, p.Notes, p.Settles, changed)
		})
	case protocol.Answer:
		var p protocol.AnswerParams
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		e, err := s.entry(p.Run)
		if err != nil {
			return nil, err
		}
		if e.run.View().Question == "" {
			return nil, &protocol.Error{Code: protocol.Refused, Message: p.Run + " is not waiting for an answer"}
		}
		if as, ok := e.run.(interface{ AcceptAs(string) }); ok && p.Accept && p.By != "" {
			as.AcceptAs(p.By)
		} else if p.Accept {
			e.run.Accept()
		} else {
			e.run.Refuse(p.Why)
		}
		return nil, nil
	case protocol.Stop:
		var p protocol.RunRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		e, err := s.entry(p.Run)
		if err != nil {
			return nil, err
		}
		// A run that has a folder is stopped as any run is, by the file the
		// run watches, so that its record says who stopped it; one that has
		// none yet has nothing to record.
		e.mu.Lock()
		e.stopped = true
		e.mu.Unlock()
		if dir := e.run.View().Dir; dir != "" && flow.Running(dir) {
			return nil, flow.Stop(dir, s.person)
		}
		e.cancel()
		return nil, nil
	case protocol.Watch:
		var p protocol.RunRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		e, err := s.entry(p.Run)
		if err != nil {
			return nil, err
		}
		s.watch(e, c)
		return nil, nil
	case protocol.Unwatch:
		var p protocol.RunRef
		if err := protocol.Decode(params, &p); err != nil {
			return nil, err
		}
		e, err := s.entry(p.Run)
		if err != nil {
			return nil, err
		}
		e.unwatch(c)
		return nil, nil
	}
	return nil, &protocol.Error{Code: protocol.MethodNotFound, Message: method + " is no method of this service"}
}

// watch tells c what moved in e, from the start, and on every move until c
// stops watching or goes.
func (s *Service) watch(e *entry, c *protocol.Conn) {
	e.mu.Lock()
	if _, ok := e.watchers[c]; ok {
		e.mu.Unlock()
		return
	}
	w := &watcher{conn: c, more: make(chan struct{}, 1), agent: map[string]int{}, stop: make(chan struct{})}
	e.watchers[c] = w
	e.mu.Unlock()
	w.more <- struct{}{} // what there is already
	go func() {
		defer e.unwatch(c)
		for {
			select {
			case <-w.more:
			case <-w.stop:
				return
			case <-c.Done():
				return
			}
			d, done := e.delta(w)
			if err := c.Notify(protocol.Changed, d); err != nil {
				return
			}
			if done {
				return // nothing will move again
			}
		}
	}()
}

func (e *entry) unwatch(c *protocol.Conn) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if w, ok := e.watchers[c]; ok {
		close(w.stop)
		delete(e.watchers, c)
	}
}

// delta is what moved in e since w was last told, and whether the run has
// ended, so that w will be told nothing more.
func (e *entry) delta(w *watcher) (protocol.RunDelta, bool) {
	v := e.run.View()
	e.mu.Lock()
	finished, err, stopped := e.finished, e.err, e.stopped
	e.mu.Unlock()
	d := protocol.RunDelta{Run: e.id, Project: e.project, Statement: v.Statement, Slug: v.Slug, Dir: v.Dir,
		StepsFrom: w.steps, Question: v.Question, Reviewed: v.Reviewed, Spent: usage(v),
		Finished: finished}
	if err != nil {
		d.Error = err.Error()
	}
	for i, a := range v.Areas {
		d.Areas[i] = protocol.Status(a)
	}
	if finished {
		d.Outcome = outcomeOf(d.Areas, err, stopped)
	}
	for _, st := range v.Steps[min(w.steps, len(v.Steps)):] {
		d.Steps = append(d.Steps, protocol.Step{At: st.At, Area: st.Area, Text: st.Text, Object: st.Object, Bad: st.Bad})
	}
	w.steps = len(v.Steps)
	units := make([]string, 0, len(v.Agent))
	for u := range v.Agent {
		units = append(units, u)
	}
	slices.Sort(units)
	for _, u := range units {
		events := v.Agent[u]
		from := min(w.agent[u], len(events))
		if from == len(events) {
			continue
		}
		if d.Agent == nil {
			d.Agent, d.AgentFrom = map[string][]protocol.Event{}, map[string]int{}
		}
		d.AgentFrom[u] = from
		for _, ev := range events[from:] {
			d.Agent[u] = append(d.Agent[u], protocol.Event{At: ev.At, Kind: string(ev.Kind), Tool: ev.Tool, Detail: ev.Detail})
		}
		w.agent[u] = len(events)
	}
	return d, finished
}

func usage(v flow.View) protocol.Usage {
	return protocol.Usage{Input: v.Spent.Input, CacheRead: v.Spent.CacheRead, CacheWrite: v.Spent.CacheWrite,
		Output: v.Spent.Output, CostUSD: v.Spent.CostUSD}
}

// summaries is every run carried here, in the order they were started.
func (s *Service) summaries() []protocol.RunSummary {
	s.mu.Lock()
	ids := slices.Clone(s.order)
	s.mu.Unlock()
	out := make([]protocol.RunSummary, 0, len(ids))
	for _, id := range ids {
		e, err := s.entry(id)
		if err != nil {
			continue
		}
		v := e.run.View()
		e.mu.Lock()
		sum := protocol.RunSummary{Run: id, Project: e.project, Statement: v.Statement, Dir: v.Dir, Question: v.Question, Finished: e.finished}
		if e.err != nil {
			sum.Error = e.err.Error()
		}
		e.mu.Unlock()
		out = append(out, sum)
	}
	return out
}

// ErrRunning is a service already listening where this one would.
var ErrRunning = errors.New("a service is listening already")

// Listen listens at socket for screens, taking the place of a socket a
// service that ended left behind, never that of one listening.
func Listen(ctx context.Context, socket string) (net.Listener, error) {
	if c, err := protocol.Dial(ctx, socket, nil); err == nil {
		_ = c.Close()
		return nil, ErrRunning
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(socket)
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}
	// Only the person who runs the service may speak to it.
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = l.Close()
		return nil, err
	}
	return l, nil
}

// Serve speaks the protocol with every screen that connects to l, until ctx
// ends.
func (s *Service) Serve(ctx context.Context, l net.Listener) error {
	ctx = s.serveUntil(ctx)
	s.carryOnKept()
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()
	for {
		nc, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c := protocol.NewConn(nc, s.Handle)
		go func() {
			select {
			case <-c.Done():
			case <-ctx.Done():
				_ = c.Close()
			}
		}()
	}
}

// outcomeOf is how a run that ended, with err, came out.
func outcomeOf(areas [7]protocol.Status, err error, stopped bool) protocol.Outcome {
	switch {
	case errors.Is(err, flow.ErrHolds):
		return protocol.EndHolds
	case err == nil && areas[6] == protocol.Done:
		return protocol.EndLanded
	case stopped || errors.Is(err, context.Canceled):
		return protocol.EndStopped
	}
	return protocol.EndFailed
}
