package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/igannytskyi/iflow/internal/model"
)

// fakeOpenRouter starts a server answering in OpenAI shape that records the
// model field of each request body. Its URL carries "openrouter.ai" as a
// path segment, so a model built from a provider at it is served through
// OpenRouter by openRouter()'s own test.
func fakeOpenRouter(t *testing.T) (base string, field func() string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"cost":0.001}}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/openrouter.ai/v1", func() string { return got }
}

func floorTestConfig(base string, floor bool) Config {
	return Config{
		Providers:    map[string]Provider{"openrouter": {Base: base, KeyEnv: "IFLOW_TEST_KEY"}},
		Roles:        Roles{Author: "openrouter:z-ai/glm-5.3-flash", Reviewer: "openrouter:z-ai/glm-5.3-flash", Planner: "openrouter:z-ai/glm-5.3-flash", Executor: "openrouter:z-ai/glm-5.3-flash"},
		FloorPricing: floor,
	}
}

func ask(t *testing.T, m model.Model) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := m.Ask(ctx, "sys", "hi"); err != nil {
		t.Fatal(err)
	}
}

func TestConfigFloorSwitchEndToEnd(t *testing.T) {
	base, field := fakeOpenRouter(t)
	SetFile(filepath.Join(t.TempDir(), "models.json"))
	defer SetFile("")
	t.Setenv("IFLOW_TEST_KEY", "k")

	on := floorTestConfig(base, true)
	md, err := on.Model(on.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}
	ask(t, md)
	if got := field(); got != "z-ai/glm-5.3-flash:floor" {
		t.Fatalf("request model with the switch on = %q, want %q", got, "z-ai/glm-5.3-flash:floor")
	}

	off := floorTestConfig(base, false)
	md, err = off.Model(off.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}
	ask(t, md)
	if got := field(); got != "z-ai/glm-5.3-flash" {
		t.Fatalf("request model with the switch off = %q, want bare %q", got, "z-ai/glm-5.3-flash")
	}
}

func TestMissingFloorKeyLoadsOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	if err := os.WriteFile(path, []byte(`{
  "providers": {"openrouter": {"base": "https://openrouter.ai/api/v1", "key_env": "IFLOW_TEST_KEY"}},
  "roles": {"author": "openrouter:z-ai/glm-5.3-flash", "reviewer": "openrouter:z-ai/glm-5.3-flash", "planner": "openrouter:z-ai/glm-5.3-flash", "executor": "openrouter:z-ai/glm-5.3-flash"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	SetFile(path)
	defer SetFile("")
	t.Setenv("IFLOW_TEST_KEY", "k")

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m.FloorPricing {
		t.Fatal("FloorPricing loaded true from a models.json without the key, want false")
	}

	base, field := fakeOpenRouter(t)
	loaded := floorTestConfig(base, m.FloorPricing)
	md, err := loaded.Model(loaded.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}
	ask(t, md)
	if got := field(); got != "z-ai/glm-5.3-flash" {
		t.Fatalf("request model from a config loaded without the key = %q, want bare %q", got, "z-ai/glm-5.3-flash")
	}
}

func TestBuiltModelKeepsItsFloorSnapshot(t *testing.T) {
	base, field := fakeOpenRouter(t)
	SetFile(filepath.Join(t.TempDir(), "models.json"))
	defer SetFile("")
	t.Setenv("IFLOW_TEST_KEY", "k")

	off := floorTestConfig(base, false)
	early, err := off.Model(off.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}

	on := floorTestConfig(base, true)
	later, err := on.Model(on.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}

	ask(t, early)
	if got := field(); got != "z-ai/glm-5.3-flash" {
		t.Fatalf("model built with the switch off sent %q after the config flipped on, want bare %q", got, "z-ai/glm-5.3-flash")
	}
	ask(t, later)
	if got := field(); got != "z-ai/glm-5.3-flash:floor" {
		t.Fatalf("model built with the switch on sent %q, want %q", got, "z-ai/glm-5.3-flash:floor")
	}
}

func TestModelCarriesFloorSwitchIntoBuiltModel(t *testing.T) {
	base, field := fakeOpenRouter(t)
	SetFile(filepath.Join(t.TempDir(), "models.json"))
	defer SetFile("")
	t.Setenv("IFLOW_TEST_KEY", "k")

	on := floorTestConfig(base, true)
	md, err := on.Model(on.Roles.Author, "low")
	if err != nil {
		t.Fatal(err)
	}
	o, ok := md.(model.OpenAI)
	if !ok {
		t.Fatalf("Config.Model built a %T, want a model.OpenAI", md)
	}
	if !o.Floor {
		t.Fatal("the model built with FloorPricing on does not carry the switch")
	}
	ask(t, o)
	if got := field(); got != "z-ai/glm-5.3-flash:floor" {
		t.Fatalf("request model = %q, want %q", got, "z-ai/glm-5.3-flash:floor")
	}
}
