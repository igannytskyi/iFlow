package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// oldShapeSpec is a 01-specification.json as the model wrote it before the
// change: every field the model keeps, and beside them a relies_on array of
// full provision objects.
const oldShapeSpec = `{
  "schema": 1,
  "id": "SPEC-064",
  "name": "cr-064",
  "intent": "INT-064",
  "default_class": "C1T",
  "prose": "The change described, as the old specification kept it.",
  "criteria": [
    {
      "id": "CR-064-01",
      "criterion": "an old-shape specification loads as it was written",
      "when": "a change folder holds it and is loaded",
      "then": "the loaded document reads every written field as written",
      "plan": "not-required",
      "procedure": "machine",
      "required_evidence": "test-run",
      "threshold": "every field equal",
      "judged_by_new_tests": true
    }
  ],
  "termination": [
    {"condition": "a second loading path is found", "action": "escalate"}
  ],
  "scope": {
    "included": ["internal/artifact/spec_load_test.go"],
    "excluded": ["internal/meta/meta.go"]
  },
  "arbiters": [
    {"path": "internal/artifact/spec_load_test.go", "arbitrates": "CR-064-01"}
  ],
  "arbiter_inputs": [
    {"input": "docs/method/data-model.md", "in_scope": "no"}
  ],
  "relies_on": [
    {
      "file": "internal/meta/meta.go",
      "name": "Specification",
      "kind": "type",
      "of": "",
      "signature": "type Specification struct"
    }
  ]
}
`

// writeSpecFolder writes a change folder holding only the specification
// document the fixture describes — the minimum the loader demands.
func writeSpecFolder(t *testing.T, doc string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cr-064")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "01-specification.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSpecificationWithReliesOnLoads judges that a specification written in
// the old shape — relies_on beside every other field — loads through the
// estate's loading entry point with every written field reading its written
// value.
func TestSpecificationWithReliesOnLoads(t *testing.T) {
	c, err := Load(writeSpecFolder(t, oldShapeSpec))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Meta.Specification == nil {
		t.Fatal("loaded change has no specification")
	}
	s := c.Meta.Specification
	if s.Schema != 1 {
		t.Errorf("schema = %d, want 1", s.Schema)
	}
	if s.ID != "SPEC-064" {
		t.Errorf("id = %q, want SPEC-064", s.ID)
	}
	if s.Name != "cr-064" {
		t.Errorf("name = %q, want cr-064", s.Name)
	}
	if s.Intent != "INT-064" {
		t.Errorf("intent = %q, want INT-064", s.Intent)
	}
	if s.Class != "C1T" {
		t.Errorf("default_class = %q, want C1T", s.Class)
	}
	if s.Prose != "The change described, as the old specification kept it." {
		t.Errorf("prose = %q", s.Prose)
	}
	if len(s.Criteria) != 1 {
		t.Fatalf("criteria: got %d, want 1", len(s.Criteria))
	}
	cr := s.Criteria[0]
	if cr.ID != "CR-064-01" {
		t.Errorf("criterion id = %q, want CR-064-01", cr.ID)
	}
	if cr.Criterion != "an old-shape specification loads as it was written" {
		t.Errorf("criterion = %q", cr.Criterion)
	}
	if cr.When != "a change folder holds it and is loaded" {
		t.Errorf("when = %q", cr.When)
	}
	if cr.Then != "the loaded document reads every written field as written" {
		t.Errorf("then = %q", cr.Then)
	}
	if cr.Plan != "not-required" {
		t.Errorf("plan = %q, want not-required", cr.Plan)
	}
	if cr.Procedure != "machine" {
		t.Errorf("procedure = %q, want machine", cr.Procedure)
	}
	if cr.Evidence != "test-run" {
		t.Errorf("required_evidence = %q, want test-run", cr.Evidence)
	}
	if cr.Threshold != "every field equal" {
		t.Errorf("threshold = %q", cr.Threshold)
	}
	if !cr.New {
		t.Error("judged_by_new_tests = false, want true")
	}
	if len(s.Termination) != 1 {
		t.Fatalf("termination: got %d, want 1", len(s.Termination))
	}
	if s.Termination[0].Condition != "a second loading path is found" {
		t.Errorf("termination condition = %q", s.Termination[0].Condition)
	}
	if s.Termination[0].Action != "escalate" {
		t.Errorf("termination action = %q", s.Termination[0].Action)
	}
	if len(s.Scope.Included) != 1 || s.Scope.Included[0] != "internal/artifact/spec_load_test.go" {
		t.Errorf("scope included = %v", s.Scope.Included)
	}
	if len(s.Scope.Excluded) != 1 || s.Scope.Excluded[0] != "internal/meta/meta.go" {
		t.Errorf("scope excluded = %v", s.Scope.Excluded)
	}
	if len(s.Arbiters) != 1 || s.Arbiters[0].Path != "internal/artifact/spec_load_test.go" {
		t.Errorf("arbiters = %v", s.Arbiters)
	}
	if s.Arbiters[0].Arbitrates != "CR-064-01" {
		t.Errorf("arbiter arbitrates = %q", s.Arbiters[0].Arbitrates)
	}
	if len(s.Inputs) != 1 || s.Inputs[0].Input != "docs/method/data-model.md" {
		t.Errorf("arbiter_inputs = %v", s.Inputs)
	}
	if s.Inputs[0].InScope != "no" {
		t.Errorf("arbiter input in_scope = %q", s.Inputs[0].InScope)
	}
}

// TestSpecificationRefusesOtherUnknownKeys judges that the strictness the
// model keeps — a key it does not have is refused — holds for every key
// other than relies_on.
func TestSpecificationRefusesOtherUnknownKeys(t *testing.T) {
	doc := `{
  "schema": 1,
  "id": "SPEC-064",
  "name": "cr-064",
  "intent": "INT-064",
  "default_class": "C1T",
  "criteria": [],
  "termination": [],
  "scope": {"included": []},
  "arbiters": [],
  "relies": []
}
`
	dir := writeSpecFolder(t, doc)
	// A document that does not load fails either with an error or, before
	// the specification's own unmarshalling exists, by aborting the load:
	// what is judged is that Load does not hand the document back loaded.
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("load panicked: %v", r)
			}
		}()
		_, err = Load(dir)
	}()
	if err == nil {
		t.Fatal("Load accepted a specification holding an unknown key relies; want an error")
	}
}
