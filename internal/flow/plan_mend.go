package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// tickets are a plan's units as the planner writes them: whole, as a list,
// or, mending a plan it was handed, as only the units it changes — an
// object keyed by each one's number in that plan, from 1, the unit written
// whole, null to take it out, the number after the last to add one. Units
// it does not name stand: a plan mended is not written again, and a unit
// that stood is not broken by being rewritten unasked.
type tickets []ticket

func (ts *tickets) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) == 0 || t[0] != '{' {
		var all []ticket
		if err := json.Unmarshal(b, &all); err != nil {
			return err
		}
		*ts = all
		return nil
	}
	var edits map[string]json.RawMessage
	if err := json.Unmarshal(b, &edits); err != nil {
		return err
	}
	numbers := make([]int, 0, len(edits))
	for k := range edits {
		n, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil {
			return fmt.Errorf("units are mended by their number, and %q is none", k)
		}
		numbers = append(numbers, n)
	}
	slices.Sort(numbers)
	units := slices.Clone(*ts)
	var out []int
	for _, n := range numbers {
		raw := edits[strconv.Itoa(n)]
		if raw == nil {
			for k, v := range edits { // the key as it was written: " 2", "02"
				if m, _ := strconv.Atoi(strings.TrimSpace(k)); m == n {
					raw = v
				}
			}
		}
		if n < 1 || n > len(units)+1 {
			return fmt.Errorf("there is no unit %d to mend: the plan has %d", n, len(units))
		}
		if string(bytes.TrimSpace(raw)) == "null" {
			if n <= len(units) {
				out = append(out, n)
			}
			continue
		}
		var u ticket
		if err := json.Unmarshal(raw, &u); err != nil {
			return fmt.Errorf("unit %d: %w", n, err)
		}
		if n == len(units)+1 {
			units = append(units, u)
		} else {
			units[n-1] = u
		}
	}
	for _, n := range slices.Backward(out) {
		units = slices.Delete(units, n-1, n)
	}
	*ts = units
	return nil
}

// clone is the plan with nothing shared, for a mend to be laid over.
func (p planDraft) clone() planDraft {
	var c planDraft
	_ = json.Unmarshal([]byte(p.json()), &c)
	return c
}

// json is the plan as the planner writes it, its units numbered after it.
func (p planDraft) json() string {
	b, _ := json.MarshalIndent(p, "", "  ")
	return string(b)
}

// numbered is each unit of the plan by its number and title, as a mend
// names it.
func (p planDraft) numbered() string {
	var b strings.Builder
	for i, u := range p.Units {
		fmt.Fprintf(&b, "%d. %s\n", i+1, u.Title)
	}
	return b.String()
}

// planMend and planMendWhole ask for a plan mended where it breaks: of a
// planner whose answer may leave keys out, only what changes; of one held
// to every key, the plan whole.
const (
	planMend      = "Answer with one JSON object that holds only what you change: \"approach\", whole, if you change it; and \"units\" as an object keyed by the number of each unit you change, the unit written whole — null takes a unit out, the number after the last adds one. Units you do not name stand as they are above."
	planMendWhole = "Answer with that plan, mended exactly where it breaks, and the rest as it was."
)
