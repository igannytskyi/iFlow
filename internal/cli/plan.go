package cli

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/estate"
)

// Plan says what the estate already answers about a specification: what its
// scope reaches, what could judge a change there, and what unfinished work
// holds the same ground. How the work divides, what each unit is called and
// which class each falls in are judgement, and are left to whoever plans.
func (app *App) Plan(_ context.Context, args []string) int {
	draft := slices.Contains(args, "--draft")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--draft" })
	if len(args) != 1 {
		fmt.Fprintln(app.Out, "usage: iflow plan <changes/slug> [--draft]")
		return Misused
	}
	c, err := artifact.Load(args[0])
	if err != nil {
		fmt.Fprintf(app.Out, "  %s is not a change folder\n", args[0])
		return Misused
	}
	if !c.Has(artifact.Specification) {
		fmt.Fprintf(app.Out, "  %s has no specification: there is nothing to plan from\n", args[0])
		return Misused
	}
	r := app.Estate.(*estate.Local).Repo(c.Root)
	var paths []string
	for _, p := range c.Doc(artifact.Specification).Column("Included") {
		if _, err := os.Stat(filepath.Join(c.Root, p)); err == nil {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	type crit struct{ id, text, plan string }
	var crits []crit
	for _, row := range c.Doc(artifact.Specification).Rows("Criterion", "Id") {
		crits = append(crits, crit{row.Cell("Id"), row.Cell("Criterion"), row.Cell("Plan")})
	}
	reach, err := r.Affects(paths)
	if err != nil {
		return app.fatal(err)
	}
	var firm []estate.Reached
	for _, x := range reach {
		if x.Confidence != "low" {
			firm = append(firm, x)
		}
	}
	if draft {
		return app.planDraft(r, c, paths, firm, len(reach), crits2ids(crits, func(c crit) string { return c.id }))
	}
	fmt.Fprintf(app.Out, "=== the specification states %d criterion(s) over %d path(s)\n", len(crits), len(paths))
	for _, x := range crits {
		need := ""
		if x.plan != "" {
			need = ", evidence plan " + x.plan
		}
		fmt.Fprintf(app.Out, "    %s  %s%s\n", x.id, clip(x.text, 90), need)
	}
	if len(paths) == 0 {
		fmt.Fprintln(app.Out, "    the scope names nothing that exists here, so nothing below can be derived: name the ground before planning over it")
		return Failed
	}
	fmt.Fprintln(app.Out, "=== what a change to this scope reaches")
	where := map[string]int{}
	for _, x := range firm {
		where[path.Dir(x.File)]++
	}
	regions := slices.SortedStableFunc(maps.Keys(where), func(a, b string) int { return cmp.Compare(where[b], where[a]) })
	for _, g := range regions[:min(12, len(regions))] {
		fmt.Fprintf(app.Out, "    %s  %d file(s)\n", g, where[g])
	}
	if len(regions) > 12 {
		fmt.Fprintf(app.Out, "    … and %d more region(s)\n", len(regions)-12)
	}
	fmt.Fprintf(app.Out, "    %d file(s) reached firmly, %d more on a name that matched and nothing else\n", len(firm), len(reach)-len(firm))
	fmt.Fprintln(app.Out, "=== what could judge a change there")
	for _, p := range paths {
		o, err := r.Observe(p)
		if err != nil {
			return app.fatal(err)
		}
		fmt.Fprintf(app.Out, "    %s  %s  (%d of %d symbol(s) named by a test)\n", p, o.Verdict, o.Named, o.Symbols)
		if o.Verdict == estate.Unclaimed {
			fmt.Fprintln(app.Out, "        nothing names it, so a criterion about behaviour here is not decidable as it stands: this needs a preparatory unit — a characterization of what it does now — before the work itself")
		}
	}
	// Who consumes this ground from beyond any change here is read from
	// traffic, where traffic is collected; without it, the contracts read
	// from code are all that can be said.
	fmt.Fprintln(app.Out, "=== who is beyond reach")
	fmt.Fprintln(app.Out, "    nothing this scope offers is consumed from outside what can be changed here — as far as the contracts this index can read go")
	fmt.Fprintln(app.Out, "=== what already holds this ground")
	held, err := r.Conflicts(filepath.Dir(c.Dir), paths)
	if err != nil {
		return app.fatal(err)
	}
	for _, h := range held {
		fmt.Fprintf(app.Out, "    %s/%s  holds %s  (%s)\n", h.Change, h.ID, strings.Join(h.On, ", "), h.How)
	}
	if len(held) == 0 {
		fmt.Fprintln(app.Out, "    nothing unfinished holds it")
	}
	fmt.Fprintln(app.Out, r.CoverageNote())
	fmt.Fprintln(app.Out, "  — everything above is derived and carries what it rests on. How the work divides, what each unit is called, and which class each falls in are not derivable from a repository and are not attempted here.")
	return OK
}

func crits2ids[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, x := range items {
		out[i] = id(x)
	}
	return out
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// planDraft prints the derivable half of a plan in the shape the gate reads.
func (app *App) planDraft(r *estate.Repo, c *artifact.Change, paths []string, firm []estate.Reached, total int, ids []string) int {
	n := "nnn"
	if len(ids) > 0 {
		if parts := strings.Split(ids[0], "-"); len(parts) > 1 {
			n = parts[1]
		}
	}
	fmt.Fprintln(app.Out, "## Work units\n\n| Id | Phase | Class | Scope | Area of effect | Criteria | Depends on | Preparatory for |\n|---|---|---|---|---|---|---|---|")
	unit := 1
	var prep []string
	for _, p := range paths {
		o, err := r.Observe(p)
		if err != nil {
			return app.fatal(err)
		}
		if o.Verdict != estate.Unclaimed {
			continue
		}
		id := fmt.Sprintf("WU-%s-%02d", n, unit)
		prep = append(prep, id)
		fmt.Fprintf(app.Out, "| %s | 1 | C2 | %s | a characterization of what this region does now; nothing else, derived, high | <the criterion this unit settles> | | WU-%s-%02d |\n", id, p, n, unit+1)
		unit++
	}
	regions := map[string]bool{}
	for _, x := range firm {
		regions[path.Dir(x.File)] = true
	}
	sorted := slices.Sorted(maps.Keys(regions))
	reach := strings.Join(sorted[:min(4, len(sorted))], ", ")
	if len(sorted) > 4 {
		reach += fmt.Sprintf(" and %d more", len(sorted)-4)
	}
	if reach == "" {
		reach = "nothing else here"
	}
	fmt.Fprintf(app.Out, "| WU-%s-%02d | 1 | <class> | %s | %s; %d file(s) reached firmly, derived, medium | %s | %s | |\n\n",
		n, unit, strings.Join(paths, " "), reach, len(firm), strings.Join(ids, ", "), strings.Join(prep, ", "))
	fmt.Fprintf(app.Out, "## Coverage\n\nThe units above cover %d criterion(s): %s. The estate query that produced the scope reached %d file(s) firmly and %d on a name alone; %s.\n\n",
		len(ids), strings.Join(ids, ", "), len(firm), total-len(firm), strings.TrimSpace(r.CoverageNote()))
	fmt.Fprintln(app.Out, "<!-- The class of each unit, how the work divides, and what each unit is called are judgement and are left blank. -->")
	return OK
}
