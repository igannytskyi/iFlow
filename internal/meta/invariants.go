package meta

import (
	"fmt"
	"path"
	"slices"
)

// Breach is one invariant a plan breaks: which, of which unit, and why.
type Breach struct {
	Invariant string // P1 … P12, as docs/method/execution.md numbers them
	Unit      string // the unit's id, or empty for the plan as a whole
	Why       string
}

func (b Breach) String() string {
	if b.Unit == "" {
		return b.Invariant + ": " + b.Why
	}
	return b.Invariant + ": " + b.Unit + " " + b.Why
}

// PlanBreaches is every invariant the plan p breaks that it and its
// specification s decide alone: its order and kinds, what each unit may
// write, which criteria it serves, what it provides and uses, and what it
// reads. What only the estate can decide — whether a name exists already,
// whether a file is there, how much a unit weighs — is held at planning,
// against the estate as it stood then.
//
// A plan whose contract was described in prose (DeclKind described: carried
// before provisions were typed) is held to the invariants that do not
// depend on it.
func PlanBreaches(p Plan, s Specification) []Breach {
	var out []Breach
	add := func(inv, unit, format string, args ...any) {
		out = append(out, Breach{inv, unit, fmt.Sprintf(format, args...)})
	}
	byTests, all := map[string]bool{}, map[string]bool{}
	for _, c := range s.Criteria {
		all[c.ID] = true
		if c.New && len(s.NewTests) > 0 {
			byTests[c.ID] = true
		}
	}
	typed := !slices.ContainsFunc(p.Units, func(u WorkUnit) bool {
		return slices.ContainsFunc(u.Provides, func(pr Provision) bool { return pr.Kind == Described })
	})

	// P1: at most one contract, first; then the tests; then the code.
	rank := map[UnitKind]int{ContractUnit: 0, TestsUnit: 1, CodeUnit: 2}
	contracts, codes := 0, 0
	for i, u := range p.Units {
		if r, ok := rank[u.Kind]; ok && i > 0 {
			if prev, ok := rank[p.Units[i-1].Kind]; ok && r < prev {
				add("P1", u.ID, "is a %s unit after a %s unit: the contract comes first, then the tests, then the code", u.Kind, p.Units[i-1].Kind)
			}
		}
		switch u.Kind {
		case ContractUnit:
			contracts++
		case CodeUnit:
			codes++
		}
	}
	if contracts > 1 {
		add("P1", "", "has %d contract units; it has one at most", contracts)
	}
	if codes == 0 {
		add("P1", "", "has no code unit")
	}

	var provided []Provision // every name the code units provide, in order
	for _, u := range p.Units {
		// P2: every unit names its kind, a scope and what it is to do.
		if !Allowed("UnitKind", string(u.Kind)) {
			add("P2", u.ID, "is of kind %q, which is none of contract, tests, code", u.Kind)
		}
		if len(u.Scope) == 0 || u.Brief == "" {
			add("P2", u.ID, "has no scope or no brief")
		}
		// P3: a unit writes only what the specification lets its kind write.
		var allowed []string
		switch u.Kind {
		case TestsUnit:
			allowed = s.NewTests
		case CodeUnit:
			allowed = s.Scope.Included
		case ContractUnit:
			for _, pr := range u.Provides {
				allowed = append(allowed, pr.File)
			}
		}
		for _, f := range u.Scope {
			if !slices.Contains(allowed, f) {
				add("P3", u.ID, "would write %s, which a %s unit may not", f, u.Kind)
			}
		}
		// P5: every unit serves a criterion of the specification; a tests
		// unit, only those the new tests judge.
		if len(u.Criteria) == 0 {
			add("P5", u.ID, "serves no criterion")
		}
		for _, c := range u.Criteria {
			switch {
			case !all[c]:
				add("P5", u.ID, "names %s, which is not a criterion of the specification", c)
			case u.Kind == TestsUnit && !byTests[c]:
				add("P5", u.ID, "is a tests unit and names %s, which the new tests do not judge", c)
			}
		}
		if u.Kind != CodeUnit {
			continue
		}
		// P6: every provision names itself, a file the unit writes, a kind
		// and its declaration.
		for _, pr := range u.Provides {
			switch {
			case pr.Kind == Described:
			case pr.Name == "" || pr.File == "" || pr.Signature == "":
				add("P6", u.ID, "provides %q without its name, file and signature", pr.Name+pr.Signature)
			case !slices.Contains(u.Scope, pr.File):
				add("P6", u.ID, "provides %s in %s, which it does not write", pr.Name, pr.File)
			case (pr.Kind == Field || pr.Kind == Method) && pr.Of == "":
				add("P6", u.ID, "provides the %s %s without the type it belongs to", pr.Kind, pr.Name)
			}
			// P8: no name is provided twice in one package.
			if pr.Kind != Described && slices.ContainsFunc(provided, func(o Provision) bool {
				return o.Name == pr.Name && path.Dir(o.File) == path.Dir(pr.File)
			}) {
				add("P8", u.ID, "provides %s, which another unit provides in the same package", pr.Name)
			}
		}
		provided = append(provided, u.Provides...)
	}

	// P4: every new test file is written by exactly one tests unit.
	for _, t := range s.NewTests {
		n := 0
		for _, u := range p.Units {
			if u.Kind == TestsUnit && slices.Contains(u.Scope, t) {
				n++
			}
		}
		if n != 1 {
			add("P4", "", "%s is written by %d tests units; by exactly one", t, n)
		}
	}
	// P5: every criterion is made true by some code unit.
	for _, c := range s.Criteria {
		if !slices.ContainsFunc(p.Units, func(u WorkUnit) bool { return u.Kind == CodeUnit && slices.Contains(u.Criteria, c.ID) }) {
			add("P5", "", "no code unit serves %s", c.ID)
		}
	}
	// P9: where tests and code are written apart and the code provides,
	// a contract declares every provision first.
	hasTests := slices.ContainsFunc(p.Units, func(u WorkUnit) bool { return u.Kind == TestsUnit })
	if typed && hasTests && len(provided) > 0 {
		c := slices.IndexFunc(p.Units, func(u WorkUnit) bool { return u.Kind == ContractUnit })
		if c < 0 {
			add("P9", "", "has tests and code written apart, and no contract declaring what the code provides")
		} else {
			for _, pr := range provided {
				if !slices.ContainsFunc(p.Units[c].Provides, func(o Provision) bool { return o.Name == pr.Name && o.File == pr.File }) {
					add("P9", p.Units[c].ID, "does not declare %s, which a code unit provides in %s", pr.Name, pr.File)
				}
			}
		}
	}
	// P10: a unit uses no name a later code unit provides and no earlier one
	// does: the contract declares it, but its behaviour is not there yet.
	for i, u := range p.Units {
		for _, n := range u.Uses {
			earlier := slices.ContainsFunc(p.Units[:i], func(o WorkUnit) bool { return o.Kind == CodeUnit && provides(o, n) })
			later := slices.ContainsFunc(p.Units[i+1:], func(o WorkUnit) bool { return o.Kind == CodeUnit && provides(o, n) })
			if later && !earlier {
				add("P10", u.ID, "uses %s, which only a later unit provides", n)
			}
		}
	}
	// P11: a tests unit reads nothing a code unit writes: the tests are
	// written before any code but the contract's.
	for _, u := range p.Units {
		if u.Kind != TestsUnit {
			continue
		}
		for _, f := range u.Reads {
			if slices.ContainsFunc(p.Units, func(o WorkUnit) bool { return o.Kind == CodeUnit && slices.Contains(o.Scope, f) }) {
				add("P11", u.ID, "reads %s, which a code unit writes after it", f)
			}
		}
	}
	// P12: no unit writes what judges the change, but the tests units the
	// new tests they are for.
	for _, u := range p.Units {
		for _, f := range u.Scope {
			if u.Kind == TestsUnit && slices.Contains(s.NewTests, f) {
				continue
			}
			if slices.ContainsFunc(s.Arbiters, func(a Arbiter) bool { return a.Path == f }) || slices.Contains(s.SuiteTests, f) {
				add("P12", u.ID, "would write %s, which judges the change", f)
			}
		}
	}
	return out
}

func provides(u WorkUnit, name string) bool {
	return slices.ContainsFunc(u.Provides, func(p Provision) bool { return p.Name == name })
}
