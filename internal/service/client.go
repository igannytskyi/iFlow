package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// State is a run as a screen holds it: everything it was told, put
// together.
type State struct {
	protocol.RunDelta
}

// Apply puts what moved into the state: the steps and agent events told
// from where the screen had them, the rest as it now stands.
func (s *State) Apply(d protocol.RunDelta) {
	steps, agent := s.Steps, s.Agent
	s.RunDelta = d
	s.Steps = append(steps[:min(d.StepsFrom, len(steps))], d.Steps...)
	if agent == nil {
		agent = map[string][]protocol.Event{}
	}
	for u, events := range d.Agent {
		had := agent[u]
		agent[u] = append(had[:min(d.AgentFrom[u], len(had))], events...)
	}
	s.Agent, s.AgentFrom = agent, nil
}

// Client is a screen's side of the protocol: it calls the service, and holds
// every run it watches as it was told.
type Client struct {
	conn *protocol.Conn

	mu      sync.Mutex
	runs    map[string]*State
	chats   map[string]protocol.ChatState
	changed func(what string)
}

// ChatKey is what a client's changed is told when a conversation moved: the
// conversation's key, set apart from a run's id.
func ChatKey(r protocol.ChatRef) string { return "chat:" + key(r) }

// Dial connects to the service at socket and says which version it speaks;
// changed is called, on no goroutine of the caller's, whenever a watched
// run has moved.
func Dial(ctx context.Context, socket, client string, changed func(run string)) (*Client, error) {
	c := &Client{runs: map[string]*State{}, chats: map[string]protocol.ChatState{}, changed: changed}
	conn, err := protocol.Dial(ctx, socket, c.hear)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	var res protocol.InitializeResult
	if err := conn.Call(ctx, protocol.Initialize, protocol.InitializeParams{Version: protocol.Version, Client: client}, &res); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("the service at %s: %w", socket, err)
	}
	return c, nil
}

// Close ends the connection; the runs go on in the service.
func (c *Client) Close() error { return c.conn.Close() }

// Done is closed when the connection to the service ends.
func (c *Client) Done() <-chan struct{} { return c.conn.Done() }

func (c *Client) hear(_ context.Context, _ *protocol.Conn, method string, params json.RawMessage) (any, error) {
	if method == protocol.ChatChanged {
		var s protocol.ChatState
		if err := protocol.Decode(params, &s); err != nil {
			return nil, err
		}
		k := ChatKey(s.ChatRef)
		c.mu.Lock()
		c.chats[k] = s
		c.mu.Unlock()
		if c.changed != nil {
			c.changed(k)
		}
		return nil, nil
	}
	if method != protocol.Changed {
		return nil, &protocol.Error{Code: protocol.MethodNotFound, Message: method + " is not heard by this screen"}
	}
	var d protocol.RunDelta
	if err := protocol.Decode(params, &d); err != nil {
		return nil, err
	}
	c.mu.Lock()
	s := c.runs[d.Run]
	if s == nil {
		s = &State{}
		c.runs[d.Run] = s
	}
	s.Apply(d)
	c.mu.Unlock()
	if c.changed != nil {
		c.changed(d.Run)
	}
	return nil, nil
}

// Run is the run as this screen was last told it, and whether it was told
// anything of it.
func (c *Client) Run(id string) (State, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.runs[id]
	if !ok {
		return State{}, false
	}
	out := *s
	out.Steps = append([]protocol.Step(nil), s.Steps...)
	out.Agent = make(map[string][]protocol.Event, len(s.Agent))
	for u, ev := range s.Agent {
		out.Agent[u] = append([]protocol.Event(nil), ev...)
	}
	return out, true
}

// Runs is every run the service carries.
func (c *Client) Runs(ctx context.Context) ([]protocol.RunSummary, error) {
	var out []protocol.RunSummary
	err := c.conn.Call(ctx, protocol.Runs, nil, &out)
	return out, err
}

// Start starts a task on a project, and watches it.
func (c *Client) Start(ctx context.Context, p protocol.StartParams) (string, error) {
	return c.begin(ctx, protocol.Start, p)
}

// Resume carries a stopped change on, and watches it.
func (c *Client) Resume(ctx context.Context, p protocol.ResumeParams) (string, error) {
	return c.begin(ctx, protocol.Resume, p)
}

// Reopen carries a change again from its specification or its plan, and
// watches it.
func (c *Client) Reopen(ctx context.Context, p protocol.ReopenParams) (string, error) {
	return c.begin(ctx, protocol.Reopen, p)
}

func (c *Client) begin(ctx context.Context, method string, params any) (string, error) {
	var ref protocol.RunRef
	if err := c.conn.Call(ctx, method, params, &ref); err != nil {
		return "", err
	}
	return ref.Run, c.Watch(ctx, ref.Run)
}

// Watch asks to be told what moves in a run, from its start.
func (c *Client) Watch(ctx context.Context, run string) error {
	return c.conn.Call(ctx, protocol.Watch, protocol.RunRef{Run: run}, nil)
}

// Accept answers the question a run waits on, yes.
func (c *Client) Accept(ctx context.Context, run string) error {
	return c.conn.Call(ctx, protocol.Answer, protocol.AnswerParams{Run: run, Accept: true}, nil)
}

// Refuse answers it no, with why.
func (c *Client) Refuse(ctx context.Context, run, why string) error {
	return c.conn.Call(ctx, protocol.Answer, protocol.AnswerParams{Run: run, Why: why}, nil)
}

// Stop asks the service to stop a run.
func (c *Client) Stop(ctx context.Context, run string) error {
	return c.conn.Call(ctx, protocol.Stop, protocol.RunRef{Run: run}, nil)
}

// Projects is the projects the person works on, and their changes.
func (c *Client) Projects(ctx context.Context, also ...string) ([]protocol.Project, error) {
	var out []protocol.Project
	err := c.conn.Call(ctx, protocol.Projects, protocol.ProjectsParams{Also: also}, &out)
	return out, err
}

// Record is a change's record.
func (c *Client) Record(ctx context.Context, dir string) ([]protocol.Crossing, error) {
	var out []protocol.Crossing
	err := c.conn.Call(ctx, protocol.Record, protocol.ChangeRef{Dir: dir}, &out)
	return out, err
}

// Change is a change as its folder holds it.
func (c *Client) Change(ctx context.Context, dir string) (protocol.ChangeDoc, error) {
	var out protocol.ChangeDoc
	err := c.conn.Call(ctx, protocol.Get, protocol.ChangeRef{Dir: dir}, &out)
	return out, err
}

// Trace is what a candidate's agent did.
func (c *Client) Trace(ctx context.Context, dir, candidate string) ([]protocol.Event, error) {
	var out []protocol.Event
	err := c.conn.Call(ctx, protocol.Trace, protocol.TraceParams{Dir: dir, Candidate: candidate}, &out)
	return out, err
}

// Talk asks to be told how a conversation stands, now and as it moves.
func (c *Client) Talk(ctx context.Context, r protocol.ChatRef) error {
	return c.conn.Call(ctx, protocol.Listen, r, nil)
}

// Ask puts the person's words to the model in a conversation.
func (c *Client) Ask(ctx context.Context, r protocol.ChatRef, text string) error {
	return c.conn.Call(ctx, protocol.Ask, protocol.AskParams{ChatRef: r, Text: text}, nil)
}

// Chat is a conversation as this screen was last told it.
func (c *Client) Chat(r protocol.ChatRef) (protocol.ChatState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.chats[ChatKey(r)]
	return s, ok
}
