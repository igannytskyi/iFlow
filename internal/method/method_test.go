package method

import (
	"testing"

	"github.com/igannytskyi/iflow"
)

func TestModelReadsTheMethod(t *testing.T) {
	m, err := Load(iflow.Texts)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(m.Areas); got != 13 {
		t.Errorf("areas = %d, want 13", got)
	}
	for _, a := range m.Areas {
		if a.Object == "" || a.Aim == "" || a.Criterion == "" {
			t.Errorf("area %d %q lacks object, aim or criterion", a.N, a.Name)
		}
		if len(a.Params) < 9 {
			t.Errorf("area %d %q has %d parameters", a.N, a.Name, len(a.Params))
		}
		if want := a.N > 6; (a.Part == Foundation) != want {
			t.Errorf("area %d is in the wrong part", a.N)
		}
	}
	for _, name := range []string{"Intent", "Specification", "Candidate", "Evidence", "Verdict", "Escalation"} {
		if m.Object(name) == nil {
			t.Errorf("object %s is not read", name)
		}
	}
	if len(m.Contracts) < 10 {
		t.Errorf("contracts = %d", len(m.Contracts))
	}
	if len(m.Invariants) != 9 {
		t.Errorf("invariants = %d, want 9", len(m.Invariants))
	}
	for _, id := range []string{"R1", "R2", "R7b", "R13", "R22", "B1", "B4"} {
		if m.Rule(id) == nil {
			t.Errorf("rule %s is not read", id)
		}
	}
	if r := m.Rule("B2"); r == nil || !r.Boundary {
		t.Error("B2 is not read as a rule of the boundary")
	}
	if len(m.Classes) != 6 {
		t.Errorf("classes = %d, want 6", len(m.Classes))
	}
}
