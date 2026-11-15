package gate

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	a "github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// entered is every unit the landing says entered.
func entered(c *a.Change) []string {
	var out []string
	for _, row := range c.Doc(a.Landing).Rows("Unit", "Entered at") {
		if row.Cell("Entered at") != "" {
			out = append(out, row.Cell("Unit"))
		}
	}
	return distinct(out)
}

// landingBacked — A6 and R10: nothing enters with no verdict, on a failed
// one, on an undecided criterion, or on evidence no longer valid.
func landingBacked(r *run) {
	if !r.has(a.Landing) {
		return
	}
	states := map[string]map[string]bool{}
	for _, row := range r.doc(a.Assurance).Rows("Unit", "State") {
		u := row.Cell("Unit")
		if states[u] == nil {
			states[u] = map[string]bool{}
		}
		switch o := row.Cell("Outcome"); o {
		case "failed", "undecided":
			states[u][o] = true
		default:
			states[u][row.Cell("State")] = true
		}
	}
	for _, row := range r.doc(a.Landing).Rows("Unit", "Entered at") {
		if row.Cell("Entered at") == "" {
			continue
		}
		u := row.Cell("Unit")
		st, ok := states[u]
		switch {
		case !ok:
			r.fail("A6", "%s entered with no verdict", orNone(row.Get("Unit")))
		case st["failed"]:
			r.fail("A6", "%s entered on a failed verdict", u)
		case st["undecided"]:
			r.fail("R10", "%s entered with an undecided criterion — acceptance was not established", u)
		case row.Cell("Evidence still valid") == "no":
			r.fail("A6", "%s entered on evidence no longer valid", u)
		}
	}
}

var scopeWords = regexp.MustCompile(`[,\s]+`)

// landingTaught — R21: work that had to touch ground the model never
// connected to its scope is evidence against the model, and is owed back to
// it. Being wrong is not detectable from the source; a landing is the outside
// that corrects it. Not an incident to be handled: a correction to be
// recorded.
func landingTaught(r *run) {
	if !r.has(a.Landing) || !r.has(a.Plan) {
		return
	}
	units := entered(r.c)
	if len(units) == 0 {
		return
	}
	scopeOf := map[string][]string{}
	for _, row := range r.doc(a.Plan).Rows("Scope", "Id") {
		var here []string
		for _, w := range scopeWords.Split(row.Cell("Scope"), -1) {
			p := strings.Trim(w, " ,")
			if p == "" {
				continue
			}
			if exists(filepath.Join(r.c.Root, p)) {
				here = append(here, p)
				continue
			}
			// Not a refusal — a scope may name ground this estate does not
			// hold — but what was not read is said.
			r.note("R21", "%s is scoped to %s, which names nothing here, so what it reaches is not asked", row.Cell("Id"), p)
		}
		scopeOf[row.Cell("Id")] = here
	}
	touched := map[string][]string{}
	for _, row := range r.doc(a.Execution).Rows("Stored at", "Unit") {
		where := filepath.Join(r.c.Dir, row.Cell("Stored at"))
		if !isDir(where) {
			continue
		}
		_ = filepath.WalkDir(where, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			// Only what the estate can read: a candidate folder also holds a
			// patch, a note, the output of a run, and none of that is ground
			// the model could reach or fail to reach.
			if ok, _ := r.gate.estate.Readable(r.ctx, p); ok {
				touched[row.Cell("Unit")] = append(touched[row.Cell("Unit")], d.Name())
			}
			return nil
		})
	}
	if len(touched) == 0 {
		return
	}
	corrections, err := r.gate.estate.Corrections(r.ctx, r.c.Root)
	if err != nil {
		return
	}
	if slices.ContainsFunc(corrections, func(k estate.Correction) bool { return k.By == r.c.Name() }) {
		return
	}
	for _, unit := range units {
		within, files := scopeOf[unit], touched[unit]
		if len(within) == 0 || len(files) == 0 {
			continue
		}
		reach, err := r.gate.estate.Affects(r.ctx, r.c.Root, within)
		if err != nil {
			continue
		}
		reached := map[string]bool{}
		for _, x := range reach {
			reached[base(x.File)] = true
		}
		for _, p := range within {
			reached[base(p)] = true
		}
		unforeseen := slices.DeleteFunc(distinct(files), func(f string) bool { return reached[f] })
		if len(unforeseen) > 0 {
			r.fail("R21", "%s landed having touched %s, which nothing reaches from %s — the model did not see that edge and cannot find it by reading again; record it with `iflow estate correct <scope> <what it reached> --by=%s --why=...`",
				unit, strings.Join(unforeseen, ", "), strings.Join(within, ", "), r.c.Name())
		}
	}
}

// verifiedBeforeEntry — R22: nothing enters on a run nobody made. A landing
// requires that the arbiters have been run against exactly this framework, and
// that every claim to repeat has been repeated against exactly this command —
// noted in a derived, disposable place that any edit voids.
func verifiedBeforeEntry(r *run) {
	if len(entered(r.c)) == 0 {
		return
	}
	if !exists(filepath.Join(r.c.Root, "internal", "gate", "gate.go")) {
		// A change checked away from the framework that judges it cannot be
		// asked whether that framework's arbiters were run.
		r.note("R22", "this change is checked outside the framework that judges it, so whether the arbiters were run against it cannot be asked here")
		return
	}
	held := Verified(r.gate.home)
	if _, ok := held[ArbitersKey(r.gate.home)]; !ok {
		r.fail("R22", "this entered before the arbiters were run against this framework, so nothing here knows whether what judges it still passes; run `iflow check --arbiters`")
	}
	ev := repeatable(r.c)
	for _, id := range ev.keys {
		if ev.vals[id] != "yes" {
			continue
		}
		cmd := ProducerCommand(r.c, id)
		if cmd == nil {
			continue // R13 says this one already
		}
		if _, ok := held[RepeatKey(cmd)]; !ok {
			r.fail("R22", "%s claims a run repeats and it has not been repeated against this command; run `iflow check --repeat %s`", id, r.c.Dir)
		}
	}
}
