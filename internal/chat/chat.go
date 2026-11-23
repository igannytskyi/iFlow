// Package chat is a conversation with a model beside the work: about the
// project as a whole, or about one task, with what that task has written in
// front of the model. Each conversation is its own, kept on disk where what
// it is about is kept, and goes on where it was left.
package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/model"
)

// File is where a task's conversation is kept, in the task's folder; the
// project's is kept under the changes folder.
const File = "chat.jsonl"

// Message is one turn.
type Message struct {
	At    time.Time   `json:"at"`
	Role  string      `json:"role"` // "person" or "model"
	Text  string      `json:"text"`
	Usage model.Usage `json:"usage,omitzero"`
	Err   bool        `json:"err,omitempty"`
	// Sources are the pages a search of the web found that the answer
	// cites.
	Sources []model.Citation `json:"sources,omitempty"`
}

// Thread is one conversation. Its methods may be called from any goroutine.
type Thread struct {
	// Path is the file the conversation is kept in.
	Path string
	// About gathers what the model is handed about the subject, read anew
	// for each question so that it is never stale.
	About func() string
	// Root, where given, is the repository the model may read while it
	// answers — files, folders, a search — and never change.
	Root string

	mu      sync.Mutex
	msgs    []Message
	waiting bool
	partial strings.Builder // the answer so far, while it is written
}

// Partial is the answer being written now, as far as it has come.
func (t *Thread) Partial() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.partial.String()
}

// streamer is a model that hands over its answer as it writes it.
type streamer interface {
	Stream(ctx context.Context, msgs []model.Message, tools []model.Tool, said func(string)) (model.Message, model.Usage, error)
}

// Open reads the conversation kept at path, or starts one.
func Open(path string, about func() string) *Thread {
	t := &Thread{Path: path, About: about}
	f, err := os.Open(path)
	if err != nil {
		return t
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var m Message
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			t.msgs = append(t.msgs, m)
		}
	}
	return t
}

// Messages is the conversation so far.
func (t *Thread) Messages() []Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.msgs)
}

// Waiting reports whether a question is out.
func (t *Thread) Waiting() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.waiting
}

// Cost is what the conversation has spent.
func (t *Thread) Cost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var c float64
	for _, m := range t.msgs {
		c += m.Usage.CostUSD
	}
	return c
}

// ErrBusy is asking while the last question is still out.
var ErrBusy = errors.New("chat: the last question has not been answered")

const system = `You are talking with the person who runs this project through iFlow, a method that carries software changes from intent to landing with one-shot agents. Answer plainly and briefly, in the language the person writes in. You are handed what the project or task says about itself; ground your answers in it and say when it does not say. You cannot change files or run anything; when what the person wants is a change, say so and offer the words of an intent for it.`

// historyRunes bounds how much of the conversation is handed back with each
// question; the oldest turns go first.
const historyRunes = 24000

// Ask puts the person's text to the model and keeps both, calling changed
// whenever the thread changes: a piece of the answer as it is written, and
// the answer once it has come. It returns at once; the model is asked in
// its own goroutine.
func (t *Thread) Ask(ctx context.Context, m model.Model, text string, changed func()) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	t.mu.Lock()
	if t.waiting {
		t.mu.Unlock()
		return ErrBusy
	}
	t.waiting = true
	q := Message{At: time.Now(), Role: "person", Text: text}
	t.msgs = append(t.msgs, q)
	history := slices.Clone(t.msgs)
	t.mu.Unlock()
	t.keep(q)
	go func() {
		about := ""
		if t.About != nil {
			about = t.About()
		}
		said := func(piece string) {
			t.mu.Lock()
			t.partial.WriteString(piece)
			t.mu.Unlock()
			if changed != nil {
				changed() // a new frame shows the piece
			}
		}
		answer, sources, u, err := ask(ctx, m, about, t.Root, history, said)
		a := Message{At: time.Now(), Role: "model", Text: strings.TrimSpace(answer), Usage: u, Sources: sources}
		if err != nil {
			a.Text, a.Err = err.Error(), true
		}
		t.mu.Lock()
		t.msgs = append(t.msgs, a)
		t.waiting = false
		t.partial.Reset()
		t.mu.Unlock()
		t.keep(a)
		if changed != nil {
			changed()
		}
	}()
	return nil
}

// searcher is a model whose server can search the web for it.
type searcher interface{ Searches() bool }

// searches says whether m may search the web while it answers.
func searches(m model.Model) bool {
	s, ok := m.(searcher)
	return ok && s.Searches()
}

// ask hands the model the subject and the conversation: as turns, to a
// model that holds a conversation, and as one prompt to one that does not.
// A model whose server searches the web may search it, and what it cites is
// answered beside its words.
func ask(ctx context.Context, m model.Model, about, root string, history []Message, said func(string)) (string, []model.Citation, model.Usage, error) {
	sys := system
	var web []model.Tool
	if searches(m) {
		sys += " You have a tool that searches the web, and you use it when the question asks for what the project's own documents cannot say — a library's current version, how something is done now; say which pages you rely on." +
			" An earlier answer in this conversation may say you cannot search the web: that is no longer so."
		web = []model.Tool{model.WebSearch()}
	}
	if about != "" {
		sys += "\n\n# What you are handed\n\n" + about
	}
	history = recent(history)
	if c, ok := m.(harness.Chatter); ok {
		msgs := []model.Message{{Role: "system", Content: sys}}
		for _, h := range history {
			if h.Err {
				continue
			}
			role := "user"
			if h.Role == "model" {
				role = "assistant"
			}
			msgs = append(msgs, model.Message{Role: role, Content: h.Text})
		}
		turn := func(msgs []model.Message, tools []model.Tool) (model.Message, model.Usage, error) {
			if s, ok := m.(streamer); ok {
				return s.Stream(ctx, msgs, tools, said)
			}
			return c.Chat(ctx, msgs, tools)
		}
		if root == "" {
			r, u, err := turn(msgs, web)
			return r.Content, model.Sources(r.Annotations), u, err
		}
		return look(turn, harness.NewReader(ctx, root), msgs, web)
	}
	var b strings.Builder
	for _, h := range history[:len(history)-1] {
		if !h.Err {
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", h.Role, h.Text)
		}
	}
	prompt := history[len(history)-1].Text
	if b.Len() > 0 {
		prompt = "# The conversation so far\n\n" + b.String() + "# The person now\n\n" + prompt
	}
	answer, u, err := m.Ask(ctx, sys, prompt)
	return answer, nil, u, err
}

// lookRounds bounds how many times the model may read before it answers.
const lookRounds = 12

// look lets the model read the repository before it answers: each turn
// that calls a tool is answered and the model asked again, until it
// answers in words or has read as much as it may.
func look(turn func([]model.Message, []model.Tool) (model.Message, model.Usage, error), r harness.Reader, msgs []model.Message, web []model.Tool) (string, []model.Citation, model.Usage, error) {
	msgs[0].Content += "\n\nYou may read the repository with the tools you are given — a file, a folder, a search — before you answer; read what the question needs and no more."
	var total model.Usage
	var cited []model.Annotation
	tools := append(r.Tools(), web...)
	for round := 0; ; round++ {
		if round == lookRounds {
			tools = nil // enough reading: answer with what has been read
		}
		reply, u, err := turn(msgs, tools)
		total = total.Add(u)
		cited = append(cited, reply.Annotations...)
		if err != nil || len(reply.ToolCalls) == 0 {
			return reply.Content, model.Sources(cited), total, err
		}
		reply.Role, reply.Annotations = "assistant", nil // what the server attached is not sent back to it
		msgs = append(msgs, reply)
		for _, call := range reply.ToolCalls {
			msgs = append(msgs, model.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: r.Call(call)})
		}
	}
}

// recent is the latest turns that fit the bound, the last always.
func recent(h []Message) []Message {
	n := 0
	for i := len(h) - 1; i >= 0; i-- {
		n += len([]rune(h[i].Text))
		if n > historyRunes && i < len(h)-1 {
			return h[i+1:]
		}
	}
	return h
}

func (t *Thread) keep(m Message) {
	if t.Path == "" {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(t.Path), 0o755)
	f, err := os.OpenFile(t.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
