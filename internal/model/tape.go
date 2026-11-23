package model

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// A tape is every answer the models gave in one run, by who asked — the
// author, the planner, a unit's agent — and the how-manieth time each
// asked. Recorded once against the models, it is replayed against the
// framework as it stands later: the same answers, at no cost and the same
// every time, so that a run that once failed is carried again to see
// whether the framework now holds — before anyone pays to find out.

// ErrOffTape is a question a replayed run asks that the tape has no answer
// for: the framework now asks something the run it was recorded in did not.
var ErrOffTape = errors.New("no answer on the tape")

type whoKey struct{}

// Within is ctx for the questions who asks: "author", "planner", "unit
// WU-001-02". A tape files every answer under who asked it.
func Within(ctx context.Context, who string) context.Context {
	return context.WithValue(ctx, whoKey{}, who)
}

func whoOf(ctx context.Context) string {
	if w, ok := ctx.Value(whoKey{}).(string); ok {
		return w
	}
	return "unnamed"
}

// Take is one answer on a tape.
type Take struct {
	Who     string   `json:"who"`
	N       int      `json:"n"`
	Answer  string   `json:"answer,omitempty"`
	Message *Message `json:"message,omitempty"`
	Usage   Usage    `json:"usage"`
	Err     string   `json:"error,omitempty"`
}

// Tape records answers to a file, or replays them from one.
type Tape struct {
	mu     sync.Mutex
	replay bool
	takes  map[string]Take
	count  map[string]int
	file   *os.File
}

// Record is a tape that writes every answer to path as it comes.
func Record(path string) (*Tape, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Tape{file: f, count: map[string]int{}}, nil
}

// Replay is a tape that answers from what path recorded.
func Replay(path string) (*Tape, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t := &Tape{replay: true, takes: map[string]Take{}, count: map[string]int{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		var tk Take
		if err := json.Unmarshal(sc.Bytes(), &tk); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		t.takes[key(tk.Who, tk.N)] = tk
	}
	return t, sc.Err()
}

// Close ends a recording.
func (t *Tape) Close() error {
	if t.file != nil {
		return t.file.Close()
	}
	return nil
}

func key(who string, n int) string { return fmt.Sprintf("%s#%d", who, n) }

// next is the number of who's next question.
func (t *Tape) next(who string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count[who]++
	return t.count[who]
}

// take answers who's n-th question from the tape, or records it from ask.
func (t *Tape) take(ctx context.Context, ask func() (Take, error)) (Take, error) {
	who := whoOf(ctx)
	n := t.next(who)
	if t.replay {
		t.mu.Lock()
		tk, ok := t.takes[key(who, n)]
		t.mu.Unlock()
		if !ok {
			return Take{}, fmt.Errorf("%w: %s, question %d", ErrOffTape, who, n)
		}
		if tk.Err != "" {
			return tk, errors.New(tk.Err)
		}
		return tk, nil
	}
	tk, err := ask()
	tk.Who, tk.N = who, n
	if err != nil {
		tk.Err = err.Error()
	}
	if b, merr := json.Marshal(tk); merr == nil {
		t.mu.Lock()
		_, _ = t.file.Write(append(b, '\n'))
		t.mu.Unlock()
	}
	return tk, err
}

// Taped is m, its answers recorded on or replayed from t. In replay m may
// be nil: nothing is asked of a model.
func (t *Tape) Taped(m Model, name string) Model {
	return taped{inner: m, tape: t, name: name}
}

// TapedChat is c, a model that holds conversations, its answers through t.
func (t *Tape) TapedChat(c Chatter) Chatter {
	return taped{chat: c, tape: t, name: c.Name()}
}

// taped is a model whose answers go through a tape.
type taped struct {
	inner Model
	chat  Chatter
	tape  *Tape
	name  string
}

func (m taped) Name() string { return m.name }

func (m taped) Ask(ctx context.Context, system, prompt string) (string, Usage, error) {
	tk, err := m.tape.take(ctx, func() (Take, error) {
		a, u, err := m.inner.Ask(ctx, system, prompt)
		return Take{Answer: a, Usage: u}, err
	})
	return tk.Answer, tk.Usage, err
}

func (m taped) AskShaped(ctx context.Context, system, prompt, name string, schema map[string]any) (string, Usage, error) {
	tk, err := m.tape.take(ctx, func() (Take, error) {
		s, ok := m.inner.(Shaped)
		if !ok {
			a, u, err := m.inner.Ask(ctx, system, prompt)
			return Take{Answer: a, Usage: u}, err
		}
		a, u, err := s.AskShaped(ctx, system, prompt, name, schema)
		return Take{Answer: a, Usage: u}, err
	})
	return tk.Answer, tk.Usage, err
}

// Chatter is a model that holds a conversation with tools.
type Chatter interface {
	Name() string
	Chat(ctx context.Context, msgs []Message, tools []Tool) (Message, Usage, error)
}

func (m taped) Chat(ctx context.Context, msgs []Message, tools []Tool) (Message, Usage, error) {
	tk, err := m.tape.take(ctx, func() (Take, error) {
		c := m.chat
		if c == nil {
			c, _ = m.inner.(Chatter)
		}
		if c == nil {
			return Take{}, fmt.Errorf("%s holds no conversation", m.name)
		}
		msg, u, err := c.Chat(ctx, msgs, tools)
		return Take{Message: &msg, Usage: u}, err
	})
	if tk.Message == nil {
		return Message{}, tk.Usage, err
	}
	return *tk.Message, tk.Usage, err
}
