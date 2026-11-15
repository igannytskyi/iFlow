package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
