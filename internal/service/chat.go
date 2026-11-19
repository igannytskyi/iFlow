package service

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/igannytskyi/iflow/internal/chat"
	"github.com/igannytskyi/iflow/internal/protocol"
)

// talk is the conversations the service holds, each about a task or a
// project, and the screens watching each.
type talk struct {
	mu       sync.Mutex
	threads  map[string]*chat.Thread
	watchers map[string]map[*protocol.Conn]bool
}

// key is where a conversation is kept: the task's folder, or the project's
// changes folder.
func key(r protocol.ChatRef) string {
	if r.Dir != "" {
		return r.Dir
	}
	return filepath.Join(r.Project, "changes")
}

// thread is the conversation r names, read from disk the first time.
func (t *talk) thread(r protocol.ChatRef) *chat.Thread {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key(r)
	if th, ok := t.threads[k]; ok {
		return th
	}
	about := func() string { return chat.AboutProject(r.Project, nil) }
	if r.Dir != "" {
		about = func() string { return chat.AboutTask(r.Dir) }
	}
	th := chat.Open(filepath.Join(k, chat.File), about)
	th.Root = r.Project
	if t.threads == nil {
		t.threads, t.watchers = map[string]*chat.Thread{}, map[string]map[*protocol.Conn]bool{}
	}
	t.threads[k] = th
	return th
}

// state is a conversation as it stands.
func state(r protocol.ChatRef, th *chat.Thread) protocol.ChatState {
	s := protocol.ChatState{ChatRef: r, Partial: th.Partial(), Waiting: th.Waiting(), CostUSD: th.Cost(),
		Messages: []protocol.ChatMessage{}}
	for _, m := range th.Messages() {
		s.Messages = append(s.Messages, protocol.ChatMessage{At: m.At, Role: m.Role, Text: m.Text, CostUSD: m.Usage.CostUSD, Err: m.Err})
	}
	return s
}

// told tells every screen watching r how it stands now.
func (t *talk) told(r protocol.ChatRef) {
	th := t.thread(r)
	s := state(r, th)
	t.mu.Lock()
	var conns []*protocol.Conn
	for c := range t.watchers[key(r)] {
		conns = append(conns, c)
	}
	t.mu.Unlock()
	for _, c := range conns {
		if c.Notify(protocol.ChatChanged, s) != nil {
			t.mu.Lock()
			delete(t.watchers[key(r)], c)
			t.mu.Unlock()
		}
	}
}

// watch tells c how r stands, now and on every move.
func (t *talk) watch(r protocol.ChatRef, c *protocol.Conn) {
	t.thread(r)
	t.mu.Lock()
	k := key(r)
	if t.watchers[k] == nil {
		t.watchers[k] = map[*protocol.Conn]bool{}
	}
	t.watchers[k][c] = true
	t.mu.Unlock()
	t.told(r)
}

// ask puts text to the model in the conversation r, telling who watches as
// the answer is written.
func (s *Service) ask(p protocol.AskParams) error {
	if s.model == nil {
		return &protocol.Error{Code: protocol.Refused, Message: "no model is configured to talk with"}
	}
	m := s.model()
	if m == nil {
		return &protocol.Error{Code: protocol.Refused, Message: "no model is configured to talk with"}
	}
	th := s.talk.thread(p.ChatRef)
	if err := th.Ask(s.ctx, m, p.Text, func() { s.talk.told(p.ChatRef) }); err != nil {
		return fmt.Errorf("the model could not be asked: %w", err)
	}
	s.talk.told(p.ChatRef)
	return nil
}
