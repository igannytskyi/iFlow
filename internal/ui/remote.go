package ui

import (
	"context"
	"errors"

	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
	"github.com/igannytskyi/iflow/internal/service"
)

// remoteRun is a run the service carries, as the window was last told it.
// The window holds it as any screen does: by its id, through the protocol.
type remoteRun struct {
	ctx    context.Context
	client *service.Client
	id     string
	// failed is what the service said when the window asked it something,
	// for the window to show.
	failed func(error)
}

// View is the run as the window was last told it.
func (r *remoteRun) View() flow.View {
	s, _ := r.client.Run(r.id)
	return viewOf(s)
}

// Accept answers the question the run waits on, yes. The window does not
// wait for the service to take it: what the run does next it is told.
func (r *remoteRun) Accept() { go r.do(func() error { return r.client.Accept(r.ctx, r.id) }) }

// Refuse answers it no, with why.
func (r *remoteRun) Refuse(why string) {
	go r.do(func() error { return r.client.Refuse(r.ctx, r.id, why) })
}

// Stop asks the service to stop the run.
func (r *remoteRun) Stop() { go r.do(func() error { return r.client.Stop(r.ctx, r.id) }) }

func (r *remoteRun) do(call func() error) {
	if err := call(); err != nil && r.failed != nil {
		r.failed(err)
	}
}

// viewOf is a run as the service told it, in the flow's terms, which the
// window draws.
func viewOf(s service.State) flow.View {
	v := flow.View{Repo: s.Project, Statement: s.Statement, Slug: s.Slug, Dir: s.Dir, Question: s.Question,
		Reviewed: s.Reviewed, Finished: s.Finished,
		Spent: model.Usage{Input: s.Spent.Input, CacheRead: s.Spent.CacheRead, CacheWrite: s.Spent.CacheWrite,
			Output: s.Spent.Output, CostUSD: s.Spent.CostUSD}}
	for i, a := range s.Areas {
		v.Areas[i] = flow.Status(a)
	}
	for _, st := range s.Steps {
		v.Steps = append(v.Steps, flow.Step{At: st.At, Area: st.Area, Text: st.Text, Object: st.Object, Bad: st.Bad})
	}
	if len(s.Agent) > 0 {
		v.Agent = make(map[string][]harness.Event, len(s.Agent))
		for u, events := range s.Agent {
			for _, e := range events {
				v.Agent[u] = append(v.Agent[u], harness.Event{At: e.At, Kind: harness.Kind(e.Kind), Tool: e.Tool, Detail: e.Detail})
			}
		}
	}
	return v
}

// begin starts a run in the service by call, and gives it to p.
func (u *UI) begin(p *project, area int, call func(ctx context.Context) (string, error)) error {
	if u.client == nil {
		return errNoService(u.serviceErr)
	}
	id, err := call(u.ctx)
	if err != nil {
		return err
	}
	p.run = &remoteRun{ctx: u.ctx, client: u.client, id: id, failed: func(err error) {
		p.failed.Store(new(err.Error()))
		u.invalidate()
	}}
	p.live, p.seen, p.item, p.area, p.selected = nil, 0, nil, area, runKey
	p.failed.Store(nil)
	u.runsMu.Lock()
	if u.runOf == nil {
		u.runOf = map[string]*project{}
	}
	u.runOf[id] = p
	u.runsMu.Unlock()
	return nil
}

// moved is told, on the client's goroutine, that a run moved: the window is
// drawn again, and a run that ended has its project read again, for what it
// wrote last.
func (u *UI) moved(run string) {
	u.runsMu.Lock()
	p := u.runOf[run]
	u.runsMu.Unlock()
	if p != nil && u.client != nil {
		if s, ok := u.client.Run(run); ok && s.Finished {
			p.stale.Store(true)
		}
	}
	u.invalidate()
}

// settlesOf is the escalations a run is to close: the one named, if any.
func settlesOf(ref string) []string {
	if ref == "" {
		return nil
	}
	return []string{ref}
}

// errNoService is why nothing can be carried from the window: the service
// that carries runs could not be reached.
func errNoService(why string) error {
	return errors.New("the service that carries runs could not be reached (" + why + "), so nothing can be carried from here")
}
