package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// An answer says what it cost where the server says so, and what it read
// from cache; where the server says nothing, the price given counts it.
func TestAnOpenAIAnswerSaysWhatItCost(t *testing.T) {
	var asked map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &asked)
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Error("the key was not sent")
		}
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],
"usage":{"prompt_tokens":1000,"completion_tokens":100,"cost":0.0123,"prompt_tokens_details":{"cached_tokens":400}}}`)
	}))
	defer srv.Close()
	o := OpenAI{BaseURL: srv.URL + "/openrouter.ai/v1", Key: "k", Model: "z-ai/glm-5.3-flash", Provider: "openrouter", Effort: "low"}
	got, u, err := o.Ask(context.Background(), "Answer with one JSON object.", "q")
	if err != nil || got != "ok" {
		t.Fatalf("%q %v", got, err)
	}
	if u.Input != 600 || u.CacheRead != 400 || u.Output != 100 || u.CostUSD != 0.0123 || u.Model != "openrouter/z-ai/glm-5.3-flash" {
		t.Errorf("usage %+v", u)
	}
	if asked["usage"] == nil || asked["reasoning"] == nil || asked["tools"] != nil || asked["response_format"] == nil {
		t.Errorf("the request was %v", asked)
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":2000000,"completion_tokens":1000000,"prompt_cache_hit_tokens":1000000}}`)
	}))
	defer srv2.Close()
	d := OpenAI{BaseURL: srv2.URL, Model: "deepseek-chat", Price: Price{In: 0.30, Cached: 0.03, Out: 1.20}}
	_, u, _ = d.Ask(context.Background(), "s", "q")
	if want := 0.30 + 0.03 + 1.20; strings.TrimSpace(fmtf(u.CostUSD)) != fmtf(want) {
		t.Errorf("priced at %v, want %v", u.CostUSD, want)
	}
}

func fmtf(f float64) string { b, _ := json.Marshal(f); return string(b) }

// A schema says every field, of its type, required, and nothing else.
func TestASchemaHoldsEveryField(t *testing.T) {
	type inner struct {
		When string `json:"when"`
	}
	type answer struct {
		Scope    []string `json:"scope"`
		Criteria []inner  `json:"criteria"`
		New      bool     `json:"new"`
		hidden   int
	}
	s := Schema(reflect.TypeOf(answer{}))
	b, _ := json.Marshal(s)
	for _, want := range []string{`"scope":{"items":{"type":"string"},"type":"array"}`, `"additionalProperties":false`, `"new":{"type":"boolean"}`, `"when":{"type":"string"}`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("schema lacks %s: %s", want, b)
		}
	}
	if strings.Contains(string(b), "hidden") {
		t.Error("an unexported field is in the schema")
	}
}

// A server that says "not now" is asked again after the pause it names.
func TestAServerThatSaysLaterIsAskedAgain(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"yes"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer srv.Close()
	got, _, err := OpenAI{BaseURL: srv.URL, Model: "m"}.Ask(context.Background(), "s", "q")
	if err != nil || got != "yes" || calls != 2 {
		t.Fatalf("got %q, %v after %d call(s)", got, err, calls)
	}
}

func TestUsageNamesEachModelOnce(t *testing.T) {
	u := Usage{Model: "glm-5.3-flash"}.Add(Usage{Model: "glm-5.3"}).Add(Usage{Model: "glm-5.3-flash"}).Add(Usage{})
	if u.Model != "glm-5.3-flash, glm-5.3" {
		t.Errorf("named %q", u.Model)
	}
}

// An answer streamed in pieces is handed over piece by piece and comes
// back whole, its tool calls put together and its cost counted.
func TestAStreamedAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["stream"] != true {
			t.Error("the request did not ask for a stream")
		}
		for _, l := range []string{
			`: OPENROUTER PROCESSING`,
			`data: {"choices":[{"delta":{"content":"Hel"}}]}`,
			`data: {"choices":[{"delta":{"content":"lo"}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"read_file","arguments":"{\"pa"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"a\"}"}}]}}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"cost":0.001}}`,
			`data: [DONE]`,
		} {
			fmt.Fprintln(w, l)
			fmt.Fprintln(w)
		}
	}))
	defer srv.Close()
	var pieces []string
	m, u, err := OpenAI{BaseURL: srv.URL, Model: "m"}.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, func(s string) { pieces = append(pieces, s) })
	if err != nil {
		t.Fatal(err)
	}
	if m.Content != "Hello" || len(pieces) != 2 || u.CostUSD != 0.001 || u.Output != 3 {
		t.Errorf("got %+v, %v, %+v", m, pieces, u)
	}
	if len(m.ToolCalls) != 1 || m.ToolCalls[0].Function.Arguments != `{"path":"a"}` || m.ToolCalls[0].ID != "c1" {
		t.Errorf("tool calls %+v", m.ToolCalls)
	}
}

// The web's search is offered as a tool the server runs to OpenRouter, and
// to no other server, which would not run it; what it found comes back as
// annotations, nested or flat, each page once.
func TestTheWebIsSearchedByTheServer(t *testing.T) {
	var asked map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		asked = nil // each request read afresh
		_ = json.Unmarshal(b, &asked)
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Go 1.27 [1]","annotations":[
 {"type":"url_citation","url_citation":{"url":"https://go.dev/doc/go1.27","title":"Go 1.27 Release Notes"}},
 {"type":"url_citation","url":"https://go.dev/blog","title":"The Go Blog"},
 {"type":"url_citation","url_citation":{"url":"https://go.dev/doc/go1.27","title":"again"}},
 {"type":"file","url":"x"}]}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer srv.Close()
	o := OpenAI{BaseURL: srv.URL + "/openrouter.ai/v1", Model: "z-ai/glm-5.3-flash"}
	if !o.Searches() {
		t.Fatal("OpenRouter does not search")
	}
	m, _, err := o.Chat(context.Background(), []Message{{Role: "user", Content: "latest Go?"}}, []Tool{WebSearch()})
	if err != nil {
		t.Fatal(err)
	}
	tools, _ := asked["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["type"] != "openrouter:web_search" || tools[0].(map[string]any)["function"] != nil {
		t.Errorf("the search was not offered as the server's tool: %v", asked["tools"])
	}
	want := []Citation{{URL: "https://go.dev/doc/go1.27", Title: "Go 1.27 Release Notes"}, {URL: "https://go.dev/blog", Title: "The Go Blog"}}
	if got := Sources(m.Annotations); !reflect.DeepEqual(got, want) {
		t.Errorf("the pages cited are %v, want %v", got, want)
	}

	other := OpenAI{BaseURL: srv.URL + "/api.deepseek.com", Model: "deepseek-chat"}
	if other.Searches() {
		t.Error("a server that does not search says it does")
	}
	if _, _, err := other.Chat(context.Background(), []Message{{Role: "user", Content: "q"}}, []Tool{WebSearch()}); err != nil {
		t.Fatal(err)
	}
	if asked["tools"] != nil {
		t.Errorf("a server that would not run it was offered the search: %v", asked["tools"])
	}
}

// What an asker knows the only answers to be is held to them in the shape:
// a list of strings within a list of objects, by its path.
func TestAnAnswersStringsAreHeldToTheValuesGiven(t *testing.T) {
	type unit struct {
		Title    string   `json:"title"`
		Criteria []string `json:"criteria"`
	}
	type plan struct {
		Units []unit `json:"units"`
	}
	schema := Schema(reflect.TypeOf(plan{}))
	restrict(schema, []string{"units", "criteria"}, []string{"CR-001-01", "CR-001-02"})
	units := schema["properties"].(map[string]any)["units"].(map[string]any)["items"].(map[string]any)
	crit := units["properties"].(map[string]any)["criteria"].(map[string]any)["items"].(map[string]any)
	if got, _ := crit["enum"].([]string); len(got) != 2 || got[0] != "CR-001-01" {
		t.Errorf("the criteria are not held to the values: %v", crit)
	}
	if _, held := units["properties"].(map[string]any)["title"].(map[string]any)["enum"]; held {
		t.Error("a string not named was held too")
	}
}
