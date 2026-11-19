package meta

import (
	"slices"
	"strings"
	"testing"
)

// goodPlan is a plan that holds to every invariant: a contract, a tests
// unit, two code units, against a specification of two criteria.
func goodPlan() (Plan, Specification) {
	s := Specification{
		Criteria: []Criterion{{ID: "CR-1", New: true}, {ID: "CR-2"}},
		Scope:    Scope{Included: []string{"shout.go", "loud.go"}},
		NewTests: []string{"shout_test.go"}, SuiteTests: []string{"greet_test.go"},
		Arbiters: []Arbiter{{Path: "shout_test.go"}, {Path: "greet_test.go"}, {Path: "test.sh"}},
	}
	shout := Provision{File: "shout.go", Name: "Shout", Kind: Function, Signature: "func Shout(n string) string"}
	loud := Provision{File: "loud.go", Name: "Loud", Kind: Function, Signature: "func Loud(n string) string"}
	p := Plan{Units: []WorkUnit{
		{ID: "WU-1", Kind: ContractUnit, Scope: []string{"shout.go", "loud.go"}, Criteria: []string{"CR-2"}, Brief: "declare", Provides: []Provision{shout, loud}},
		{ID: "WU-2", Kind: TestsUnit, Scope: []string{"shout_test.go"}, Criteria: []string{"CR-1"}, Brief: "test", Uses: []string{"Shout", "Loud"}},
		{ID: "WU-3", Kind: CodeUnit, Scope: []string{"shout.go"}, Criteria: []string{"CR-1"}, Brief: "shout", Provides: []Provision{shout}},
		{ID: "WU-4", Kind: CodeUnit, Scope: []string{"loud.go"}, Criteria: []string{"CR-1", "CR-2"}, Brief: "loud", Provides: []Provision{loud}, Uses: []string{"Shout"}},
	}}
	return p, s
}

func TestAPlanThatHoldsBreaksNothing(t *testing.T) {
	p, s := goodPlan()
	if b := PlanBreaches(p, s); len(b) > 0 {
		t.Errorf("a plan that holds breaks %v", b)
	}
	// One name in two modules of one folder is two names, outside Go.
	s.Scope.Included = append(s.Scope.Included, "a.py", "b.py")
	help := func(f string) WorkUnit {
		return WorkUnit{ID: "WU-" + f, Kind: CodeUnit, Scope: []string{f}, Criteria: []string{"CR-1"}, Brief: f,
			Provides: []Provision{{File: f, Name: "helper", Kind: Function, Signature: "def helper()"}}}
	}
	p.Units = append(p.Units, help("a.py"), help("b.py"))
	p.Units[0].Provides = append(p.Units[0].Provides, p.Units[4].Provides[0], p.Units[5].Provides[0])
	p.Units[0].Scope = append(p.Units[0].Scope, "a.py", "b.py")
	if b := PlanBreaches(p, s); len(b) > 0 {
		t.Errorf("one name in two Python modules breaks %v", b)
	}
}

// Each invariant, broken alone, is named by its number.
func TestEachInvariantIsNamedWhenBroken(t *testing.T) {
	for _, tc := range []struct {
		want   string
		break_ func(p *Plan, s *Specification)
	}{
		{"P1", func(p *Plan, _ *Specification) { p.Units[1], p.Units[2] = p.Units[2], p.Units[1] }},
		{"P2", func(p *Plan, _ *Specification) { p.Units[2].Brief = "" }},
		{"P3", func(p *Plan, _ *Specification) { p.Units[2].Scope = append(p.Units[2].Scope, "main.go") }},
		{"P4", func(p *Plan, _ *Specification) {
			p.Units = slices.Insert(p.Units, 2, WorkUnit{ID: "WU-9", Kind: TestsUnit, Scope: []string{"shout_test.go"}, Criteria: []string{"CR-1"}, Brief: "again"})
		}},
		{"P5", func(p *Plan, _ *Specification) { p.Units[3].Criteria = []string{"CR-1"} }},
		{"P6", func(p *Plan, _ *Specification) { p.Units[2].Provides[0].Signature = "" }},
		{"P8", func(p *Plan, _ *Specification) {
			p.Units[3].Provides = append(p.Units[3].Provides, Provision{File: "loud.go", Name: "Shout", Kind: Function, Signature: "func Shout()"})
		}},
		{"P9", func(p *Plan, _ *Specification) { p.Units[0].Provides = p.Units[0].Provides[:1] }},
		{"P10", func(p *Plan, _ *Specification) { p.Units[2].Uses = []string{"Loud"} }},
		{"P12", func(p *Plan, s *Specification) {
			s.Scope.Included = append(s.Scope.Included, "test.sh")
			p.Units[3].Scope = append(p.Units[3].Scope, "test.sh")
		}},
	} {
		t.Run(tc.want, func(t *testing.T) {
			p, s := goodPlan()
			tc.break_(&p, &s)
			got := PlanBreaches(p, s)
			if !slices.ContainsFunc(got, func(b Breach) bool { return b.Invariant == tc.want }) {
				t.Errorf("breaking %s alone was not named: %v", tc.want, got)
			}
			for _, b := range got {
				if !strings.HasPrefix(b.String(), b.Invariant+": ") {
					t.Errorf("a breach does not begin with its invariant: %s", b)
				}
			}
		})
	}
}
