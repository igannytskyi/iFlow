package gate

import (
	"slices"
	"strconv"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
)

// requiredHeaders are the columns the checks bind to. A check whose column is
// absent cannot run, and a check that cannot run must say so rather than pass
// by looking at nothing.
var requiredHeaders = []struct {
	stage  a.Stage
	groups [][]string
}{
	{a.Specification, [][]string{{"Criterion", "Procedure"}, {"Included"}, {"Path", "What it arbitrates"}}},
	{a.Plan, [][]string{{"Class"}, {"Preparatory for"}}},
	{a.Admission, [][]string{{"Outcome"}, {"Field", "Value"}}},
	{a.Execution, [][]string{{"Terminal state"}, {"Stored at", "Artefacts"}}},
	{a.Assurance, [][]string{{"Independence"}, {"Outcome", "State"}}},
	{a.Landing, [][]string{{"Entered at"}}},
}

// stageOrder — B1: the stages are written in order, from the intent.
func stageOrder(r *run) {
	present := r.c.Present()
	if len(present) == 0 {
		r.fail("B1", "no stage files found")
		return
	}
	if present[0] != a.Intent {
		r.fail("B1", "%s present without 00-intent", present[0])
	}
	for i := 1; i < len(present); i++ {
		if present[i].Index() != present[i-1].Index()+1 {
			r.fail("B1", "%s present but %s is missing", present[i], a.Stages[present[i].Index()-1])
		}
	}
}

// headersPresent — B1: every column a check binds to is there, or the gate
// says which is not.
func headersPresent(r *run) {
	for _, rh := range requiredHeaders {
		if !r.has(rh.stage) {
			continue
		}
		seen := r.doc(rh.stage).Headers()
		for _, group := range rh.groups {
			missing := slices.DeleteFunc(slices.Clone(group), func(h string) bool { return seen[h] })
			if len(missing) > 0 {
				slices.Sort(missing)
				r.fail("B1", "%s: column %s is absent, so the check binding to it would not run",
					rh.stage, strings.Join(missing, ", "))
			}
		}
	}
}

// vocabulary — every coded cell holds a defined code.
func vocabulary(r *run) {
	for _, v := range []struct {
		stage  a.Stage
		header string
		code   string
	}{
		{a.Specification, "Procedure", "procedure"},
		{a.Plan, "Class", "class"},
		{a.Admission, "Outcome", "admission"},
		{a.Admission, "Confidence of the edges relied on", "confidence"},
		{a.Execution, "Terminal state", "terminal"},
		{a.Assurance, "Subject", "subject"},
		{a.Assurance, "Kind", "kind"},
		{a.Assurance, "Independence", "independence"},
		{a.Assurance, "Outcome", "outcome"},
		{a.Assurance, "State", "state"},
	} {
		for _, cell := range r.doc(v.stage).Column(v.header) {
			if !Codes[v.code].allows(cell) {
				r.fail("CONVENTIONS", "%s: %s = %s is not a defined value", v.stage, v.header, quote(cell))
			}
		}
	}
}

// recordComplete — B4: every stage present has its crossing recorded.
func recordComplete(r *run) {
	if r.c.Record == nil {
		r.fail("B4", "no record.md — crossings are not recorded")
		return
	}
	logged := map[string]bool{}
	for _, row := range r.c.Record.Rows("Stage") {
		if s := row.Cell("Stage"); s != "" {
			logged[s] = true
		}
	}
	for _, s := range r.c.Present() {
		if !logged[s.Name()] {
			r.fail("B4", "stage %s present but not recorded", s.Name())
		}
	}
}

func digits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

// recordAppendOnly — R4: ordinals form 1..N, in order, with no gap and no
// repeat. A line removed from the tail is not detected: that needs a digest
// chain the record does not yet carry, and rules.md says so.
func recordAppendOnly(r *run) {
	var seen []int
	for _, row := range r.c.Record.Rows("#", "Stage") {
		if n := row.Cell("#"); digits(n) {
			v, _ := strconv.Atoi(n)
			seen = append(seen, v)
		}
	}
	if len(seen) == 0 {
		return
	}
	for i, n := range seen {
		if n != i+1 {
			r.fail("R4", "the record is not append-only: ordinals %s are not 1..%d in order", intList(seen), len(seen))
			return
		}
	}
}
