// Package model is how the method asks a language model something: one
// question, one answer, and what it cost. Which model answers is a choice
// made outside the method and recorded beside every answer it gives.
package model

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Usage is what one answer, or one run, cost: which model, the tokens it
// read — fresh, from cache, and written to cache — the tokens it wrote, and
// the money, as the provider prices it.
type Usage struct {
	Model      string  `json:"model,omitempty"`
	Input      int     `json:"input"`
	CacheRead  int     `json:"cache_read"`
	CacheWrite int     `json:"cache_write"`
	Output     int     `json:"output"`
	CostUSD    float64 `json:"cost_usd"`
}

// In is every token read, cached or not.
func (u Usage) In() int { return u.Input + u.CacheRead + u.CacheWrite }

// Zero reports whether nothing was spent.
func (u Usage) Zero() bool { return u.In() == 0 && u.Output == 0 && u.CostUSD == 0 }

// Add returns the sum of two usages; the model is kept where both name the
// same one, and both are named where they differ.
func (u Usage) Add(o Usage) Usage {
	// Each model named once, compared whole: one model's name may hold
	// another's (glm-5.3 and glm-5.3-flash).
	names := slices.DeleteFunc(strings.Split(u.Model, ", "), func(n string) bool { return n == "" })
	for _, n := range strings.Split(o.Model, ", ") {
		if n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	m := strings.Join(names, ", ")
	return Usage{m, u.Input + o.Input, u.CacheRead + o.CacheRead, u.CacheWrite + o.CacheWrite, u.Output + o.Output, u.CostUSD + o.CostUSD}
}

// Model answers one question.
type Model interface {
	// Name says which model answered, as a record cites it.
	Name() string
	Ask(ctx context.Context, system, prompt string) (string, Usage, error)
}

// ClaudeCode asks the Claude Code program, headless, with its tools off: a
// question answered, not a task carried out.
type ClaudeCode struct {
	// Binary is the program, "claude" when empty.
	Binary string
	// Model names a model the program accepts, its default when empty.
	Model string
	// Effort is how hard the model thinks before it answers — low, medium,
	// high — which is most of what an answer costs; the program's default
	// when empty.
	Effort string
}

// Name implements Model.
func (c ClaudeCode) Name() string {
	if c.Model != "" {
		return "claude-code/" + c.Model
	}
	return "claude-code"
}

func (c ClaudeCode) binary() string {
	if c.Binary != "" {
		return c.Binary
	}
	return "claude"
}

// Ask implements Model.
func (c ClaudeCode) Ask(ctx context.Context, system, prompt string) (string, Usage, error) {
	// Asked from an empty folder with nothing but project settings, the
	// program reads no instructions, memory or tools of anyone's: the model
	// is handed the question and nothing more, which is several thousand
	// tokens less on every call.
	args := []string{"-p", "--output-format", "json", "--system-prompt", system, "--tools", "",
		"--setting-sources", "project", "--strict-mcp-config", "--disable-slash-commands", "--no-session-persistence"}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	if c.Effort != "" {
		args = append(args, "--effort", c.Effort)
	}
	empty, err := os.MkdirTemp("", "iflow-ask-")
	if err != nil {
		return "", Usage{}, err
	}
	defer os.RemoveAll(empty)
	cmd := exec.CommandContext(ctx, c.binary(), args...)
	cmd.Dir = empty
	cmd.Stdin = strings.NewReader(prompt)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return "", Usage{}, fmt.Errorf("model: %s: %w: %s", c.binary(), err, strings.TrimSpace(errb.String()))
	}
	var r struct {
		Result     string                     `json:"result"`
		IsError    bool                       `json:"is_error"`
		Cost       float64                    `json:"total_cost_usd"`
		ModelUsage map[string]json.RawMessage `json:"modelUsage"`
		Usage      struct {
			Input       int `json:"input_tokens"`
			CacheRead   int `json:"cache_read_input_tokens"`
			CacheCreate int `json:"cache_creation_input_tokens"`
			Output      int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		return "", Usage{}, fmt.Errorf("model: unreadable answer from %s: %w", c.binary(), err)
	}
	u := Usage{Model: modelOf(r.ModelUsage, c.Name()), Input: r.Usage.Input, CacheRead: r.Usage.CacheRead,
		CacheWrite: r.Usage.CacheCreate, Output: r.Usage.Output, CostUSD: r.Cost}
	if r.IsError {
		return "", u, fmt.Errorf("model: %s answered with an error: %s", c.binary(), r.Result)
	}
	return r.Result, u, nil
}

// modelOf is which model a Claude Code run says answered, without the
// context-window mark it adds; fallback where it says none.
func modelOf(usage map[string]json.RawMessage, fallback string) string {
	var names []string
	for k := range usage {
		names = append(names, strings.TrimSuffix(k, "[1m]"))
	}
	if len(names) == 0 {
		return fallback
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// OpenAI asks any server speaking the OpenAI chat-completions protocol: a
// local Ollama, vLLM or LM Studio, or a hosted one.
type OpenAI struct {
	// BaseURL ends before /chat/completions, e.g. http://localhost:11434/v1.
	BaseURL string
	Key     string
	Model   string
	// Provider names who serves the model, as a record cites it: openrouter,
	// deepseek, ollama.
	Provider string
	// Effort is how hard a reasoning model thinks — low, medium, high —
	// where the server takes it; the model's default when empty.
	Effort string
	// Price is what a million tokens cost, for a server that does not say
	// what an answer cost; zero where it is free or says so itself.
	Price Price
	// MaxTokens bounds an answer, thinking included; 32000 when zero. A
	// server's own default is often far less, and an answer cut there ends
	// in the middle of its JSON.
	MaxTokens int
	Client    *http.Client
}

// Price is the cost of a million tokens: read fresh, read from cache, and
// written.
type Price struct {
	In, Cached, Out float64
}

// Name implements Model.
func (o OpenAI) Name() string {
	if o.Provider != "" {
		return o.Provider + "/" + o.Model
	}
	return o.Model
}

func (o OpenAI) openRouter() bool { return strings.Contains(o.BaseURL, "openrouter.ai") }

// Message is one turn of a conversation in the chat-completions protocol.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	// Annotations are what the server attached to the answer: the pages a
	// search it ran found, each cited. They are read, never sent back.
	Annotations []Annotation `json:"annotations,omitempty"`
}

// Annotation is one thing a server attached to an answer.
type Annotation struct {
	Type string `json:"type"` // url_citation for a page cited
	// URLCitation is the page, as servers nest it; some write its fields
	// beside Type instead, and Source reads either.
	URLCitation *Citation `json:"url_citation,omitempty"`
	Citation
}

// Citation is a page an answer cites.
type Citation struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

// Source is the page an annotation cites, if it cites one.
func (a Annotation) Source() (Citation, bool) {
	if a.Type != "url_citation" {
		return Citation{}, false
	}
	if a.URLCitation != nil && a.URLCitation.URL != "" {
		return *a.URLCitation, true
	}
	return a.Citation, a.URL != ""
}

// Sources is every page the annotations cite, each once, in order.
func Sources(as []Annotation) []Citation {
	var out []Citation
	for _, a := range as {
		if c, ok := a.Source(); ok && !slices.ContainsFunc(out, func(o Citation) bool { return o.URL == c.URL }) {
			out = append(out, c)
		}
	}
	return out
}

// ToolCall is a model asking for a tool to be run.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Tool describes a tool a model may call: a function the caller runs, or,
// of a type the server names, one the server runs itself.
type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function,omitzero"`
	// Parameters configure a tool the server runs.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// WebSearch is OpenRouter's search of the web, run by the server: native
// search for the models that have it, Exa's for the rest. What it finds
// comes back as the answer's annotations.
func WebSearch() Tool {
	return Tool{Type: "openrouter:web_search", Parameters: map[string]any{"max_results": 5}}
}

// served says whether a tool is one the server runs.
func (t Tool) served() bool { return strings.HasPrefix(t.Type, "openrouter:") }

// Searches says whether the server can search the web for this model.
func (o OpenAI) Searches() bool { return o.openRouter() }

// Ask implements Model.
func (o OpenAI) Ask(ctx context.Context, system, prompt string) (string, Usage, error) {
	msgs := []Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}}
	// A question that asks for one JSON object is asked in the server's JSON
	// mode, where the server holds the model to well-formed JSON rather than
	// leaving a broken brace to be found afterwards.
	if strings.Contains(system, "one JSON object") {
		m, u, err := o.chat(ctx, msgs, nil, map[string]any{"type": "json_object"})
		return m.Content, u, err
	}
	m, u, err := o.Chat(ctx, msgs, nil)
	return m.Content, u, err
}

// Chat sends a conversation, with the tools the model may call, and returns
// the model's next turn.
func (o OpenAI) Chat(ctx context.Context, msgs []Message, tools []Tool) (Message, Usage, error) {
	return o.chat(ctx, msgs, tools, nil)
}

func (o OpenAI) chat(ctx context.Context, msgs []Message, tools []Tool, format map[string]any) (Message, Usage, error) {
	body, err := o.body(msgs, tools, format, false)
	if err != nil {
		return Message{}, Usage{}, err
	}
	raw, err := o.post(ctx, body)
	if err != nil {
		return Message{}, Usage{}, err
	}
	var r struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage served `json:"usage"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Message{}, Usage{}, fmt.Errorf("model: unreadable answer: %w", err)
	}
	if len(r.Choices) == 0 {
		return Message{}, Usage{}, errors.New("model: the answer held no choice")
	}
	return r.Choices[0].Message, o.usage(r.Usage), nil
}

// served is what a server says an answer used.
type served struct {
	Prompt     int      `json:"prompt_tokens"`
	Completion int      `json:"completion_tokens"`
	Cost       *float64 `json:"cost"`
	Details    struct {
		Cached int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CacheHit int `json:"prompt_cache_hit_tokens"` // DeepSeek's own name for it
}

// usage is what an answer cost: as the server says, or by the price given.
func (o OpenAI) usage(s served) Usage {
	cached := max(s.Details.Cached, s.CacheHit)
	u := Usage{Model: o.Name(), Input: s.Prompt - cached, CacheRead: cached, Output: s.Completion}
	if s.Cost != nil {
		u.CostUSD = *s.Cost
	} else {
		u.CostUSD = (float64(u.Input)*o.Price.In + float64(cached)*o.Price.Cached + float64(u.Output)*o.Price.Out) / 1e6
	}
	return u
}

// body is a request for the model's next turn.
func (o OpenAI) body(msgs []Message, tools []Tool, format map[string]any, stream bool) ([]byte, error) {
	req := map[string]any{"model": o.Model, "messages": msgs, "max_tokens": cmp.Or(o.MaxTokens, 32000)}
	if format != nil {
		req["response_format"] = format
	}
	if !o.openRouter() {
		// A tool only OpenRouter runs is not offered to a server that would
		// not run it.
		tools = slices.DeleteFunc(slices.Clone(tools), Tool.served)
	}
	if len(tools) > 0 {
		req["tools"] = tools
	}
	if stream {
		req["stream"] = true
		req["stream_options"] = map[string]any{"include_usage": true}
	}
	if o.openRouter() {
		// OpenRouter says what an answer cost when asked to account for it.
		req["usage"] = map[string]any{"include": true}
		if o.Effort != "" {
			req["reasoning"] = map[string]any{"effort": o.Effort}
		}
	} else if o.Effort != "" {
		req["reasoning_effort"] = o.Effort
	}
	return json.Marshal(req)
}

// retries is how many times a request a server turned away for now — too
// many requests, overloaded, a gateway that timed out — is sent again.
const retries = 5

// post sends a request, again after a pause where the server says to try
// later, and returns the body of the answer.
func (o OpenAI) post(ctx context.Context, body []byte) ([]byte, error) {
	resp, err := o.send(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// send sends a request, again after a pause where the server says to try
// later, and returns the answer whose body is still to be read.
func (o OpenAI) send(ctx context.Context, body []byte) (*http.Response, error) {
	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	for attempt := 0; ; attempt++ {
		hr, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		hr.Header.Set("Content-Type", "application/json")
		if o.Key != "" {
			hr.Header.Set("Authorization", "Bearer "+o.Key)
		}
		resp, err := client.Do(hr)
		if err != nil {
			if ctx.Err() == nil && attempt < retries {
				if wait(ctx, backoff(attempt, "")) == nil {
					continue
				}
			}
			return nil, fmt.Errorf("model: %s: %w", o.BaseURL, err)
		}
		if resp.StatusCode/100 == 2 {
			return resp, nil
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if again(resp.StatusCode) && attempt < retries {
			if wait(ctx, backoff(attempt, resp.Header.Get("Retry-After"))) == nil {
				continue
			}
		}
		return nil, fmt.Errorf("model: %s answered %s: %s", o.BaseURL, resp.Status, strings.TrimSpace(string(raw)))
	}
}

// again says whether a status means: not now, later.
func again(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 529:
		return true
	}
	return false
}

// backoff is how long to wait before an attempt: what the server asked,
// or two, four, eight… seconds, never more than a minute.
func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s > 0 {
		return min(time.Duration(s)*time.Second, time.Minute)
	}
	return min(time.Duration(2<<attempt)*time.Second, time.Minute)
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// JSON extracts the first JSON object or array from a model's answer, which
// models wrap in prose and fences however they are asked not to.
func JSON(answer string, into any) error {
	s := answer
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[nl+1:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			s = rest[:j]
		}
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return fmt.Errorf("model: the answer holds no JSON: %.200q", answer)
	}
	dec := json.NewDecoder(strings.NewReader(s[start:]))
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("model: the answer's JSON does not read: %w", err)
	}
	return nil
}
