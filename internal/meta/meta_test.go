package meta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The JSON Schema in docs/method/schema is the model's, generated from its
// types: a type changed without its schema is caught here. Run with
// IFLOW_WRITE_SCHEMA=1 to write them anew.
func TestTheSchemaIsTheModels(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(here), "..", "..", "docs", "method", "schema")
	for _, d := range Documents {
		b, err := json.MarshalIndent(Schema(d.File, d.Doc), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, '\n')
		name := strings.TrimSuffix(strings.TrimSuffix(d.File, ".json"), ".jsonl") + ".schema.json"
		path := filepath.Join(dir, name)
		if os.Getenv("IFLOW_WRITE_SCHEMA") != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if was, err := os.ReadFile(path); err != nil || string(was) != string(b) {
			t.Errorf("%s is not the schema of %T; write it with IFLOW_WRITE_SCHEMA=1 go test ./internal/meta", name, d.Doc)
		}
	}
}

// A document that breaks the model says where and how.
func TestValidate(t *testing.T) {
	s := Specification{Schema: SchemaVersion, ID: "SPEC-001", Class: "C9",
		Criteria: []Criterion{{ID: "CR-1", Procedure: "robot", Evidence: TestRun}}}
	got := map[string]bool{}
	for _, p := range Validate(s) {
		got[p.Path] = true
	}
	for _, want := range []string{"default_class", "criteria[0].id", "criteria[0].procedure"} {
		if !got[want] {
			t.Errorf("%s was not found broken: %v", want, Validate(s))
		}
	}
	if ps := Validate(Intent{Schema: SchemaVersion, ID: "INT-001", Statement: "s", Status: Open}); len(ps) > 0 {
		t.Errorf("a whole intent breaks the model: %v", ps)
	}
	if ps := Validate(Intent{Schema: 2, ID: "INT-001", Status: Open}); len(ps) != 1 || ps[0].Path != "schema" {
		t.Errorf("a newer schema was not refused: %v", ps)
	}
}

// Every document renders, and says in Markdown what it holds.
func TestEveryDocumentRenders(t *testing.T) {
	a := Assurance{Name: "x", Evidence: []Evidence{{ID: "EV-001-01-01", Producer: "`go test ./...`"}},
		Verdicts: []Verdict{{ID: "VE-001-01", Evidence: []string{"EV-001-01-01"}}}}
	md := a.Markdown()
	if !strings.Contains(md, "`go test ./...`") || strings.Contains(md, "Repeatable") {
		t.Errorf("assurance renders wrong:\n%s", md)
	}
	for _, d := range Documents[:7] {
		if md := d.Doc.(interface{ Markdown() string }).Markdown(); !strings.HasPrefix(md, "# ") {
			t.Errorf("%s renders no heading", d.File)
		}
	}
}
