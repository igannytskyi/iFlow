package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBodyFloorAppendsSuffixOnOpenRouter(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	o := OpenAI{BaseURL: "https://openrouter.ai/api/v1", Model: "z-ai/glm-5.3-flash", Floor: true}
	b, err := o.body(msgs, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "z-ai/glm-5.3-flash:floor" {
		t.Fatalf("model field with floor on = %q, want %q", got.Model, "z-ai/glm-5.3-flash:floor")
	}
	o.Floor = false
	b, err = o.body(msgs, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	got = struct {
		Model string `json:"model"`
	}{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "z-ai/glm-5.3-flash" {
		t.Fatalf("model field with floor off = %q, want bare %q", got.Model, "z-ai/glm-5.3-flash")
	}
}

func TestBodyFloorNeverOnOtherBase(t *testing.T) {
	o := OpenAI{BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat", Floor: true}
	b, err := o.body([]Message{{Role: "user", Content: "hi"}}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "deepseek-chat" {
		t.Fatalf("model field on non-OpenRouter base = %q, want bare %q", got.Model, "deepseek-chat")
	}
}

func TestBodyFloorSkippedWhenNameHasColon(t *testing.T) {
	o := OpenAI{BaseURL: "https://openrouter.ai/api/v1", Model: "a/b:c", Floor: true}
	b, err := o.body([]Message{{Role: "user", Content: "hi"}}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "a/b:c" {
		t.Fatalf("model field for name already holding a colon = %q, want %q as given", got.Model, "a/b:c")
	}
}

func TestUsageModelBareWithFloor(t *testing.T) {
	var seen struct{ Model string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen.Model = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"cost":0.001}}`))
	}))
	defer srv.Close()

	o := OpenAI{BaseURL: srv.URL + "/openrouter.ai/v1", Model: "z-ai/glm-5.3-flash",
		Floor: true, Client: &http.Client{Timeout: 5 * time.Second}}
	_, u, err := o.Ask(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if seen.Model != "z-ai/glm-5.3-flash:floor" {
		t.Fatalf("sent model = %q, want the floor-suffixed name", seen.Model)
	}
	if u.Model != "z-ai/glm-5.3-flash" {
		t.Fatalf("Usage.Model = %q, want the bare name %q", u.Model, "z-ai/glm-5.3-flash")
	}
	if strings.Contains(u.Model, ":floor") {
		t.Fatalf("Usage.Model carries the suffix: %q", u.Model)
	}
}
