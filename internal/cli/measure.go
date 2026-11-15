package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/measure"
)

// Baseline reports the present way of working on an estate, or, with
// --method, what work carried through the method cost beside what the same
// intent cost upstream.
func (app *App) Baseline(_ context.Context, args []string) int {
	asJSON := slices.Contains(args, "--json")
	method := slices.Contains(args, "--method")
	since := ""
	var paths []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--since" && i+1 < len(args):
			since, i = args[i+1], i+1
		case strings.HasPrefix(a, "--since="):
			since = strings.TrimPrefix(a, "--since=")
		case !strings.HasPrefix(a, "--"):
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(app.Out, "usage: iflow baseline <estate>... [--since YYYY-MM-DD] [--method] [--json]")
		return Misused
	}
	if method {
		return app.throughMethod(paths, asJSON)
	}
	b, err := measure.Measure(paths[0], since)
	if err != nil {
		return app.fatal(err)
	}
	if asJSON {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", " ")
		_ = enc.Encode(b)
		return OK
	}
	p := func(format string, a ...any) { fmt.Fprintf(app.Out, format+"\n", a...) }
	p("=== %s, %s to %s: %d commit(s) over %d intent(s)", b.Estate, b.Since, b.Until, b.Commits, b.Intents)
	p("    read at %s, which is what makes this figure one that can be taken again", b.ReadAt)
	p("=== class mix — every figure below is read per class, and the mix is why")
	for _, k := range b.MixOrder {
		p("    %-14s %6d intent(s)  %5.1f%%", k, b.Mix[k], 100*float64(b.Mix[k])/float64(max(b.Intents, 1)))
	}
	p("    the class is what the message called the work, which is a proxy: a class is defined by how acceptance is decided, and a subject line does not say that")
	p("=== human touchpoints per intent — the primary claim")
	for _, k := range sortedKeys(b.Touchpoints) {
		p("    %-14s %v", k, b.Touchpoints[k])
	}
	p("    intents: %s", b.ReadFrom)
	if strings.Contains(b.ReadFrom, "pull request") {
		p("    counted as stating it once, every review submitted on it, every comment, and every commit after the first — each an occasion on which a person had to act")
	} else {
		p("    counted from version history alone, which does not carry reviews or comments: this figure is a floor and not the number")
	}
	p("=== what was accepted and taken back")
	p("    %d revert(s), %.2f%% of intents", len(b.Reverts), 100*float64(len(b.Reverts))/float64(max(b.Intents, 1)))
	if len(b.Lag) > 0 {
		p("    lag to discovery: %d day(s) median, %d worst, over %d revert(s) that name what they undid", b.Lag[len(b.Lag)/2], b.Lag[len(b.Lag)-1], len(b.Lag))
		p("    this is a lagging, censored estimate: anything accepted within the lag has not had time to be found wrong")
	} else {
		p("    nothing here names what it undid, so no lag can be taken")
	}
	s := b.Strength
	share := 0.0
	if s.Symbols > 0 {
		share = float64(int(1000*float64(s.Named)/float64(s.Symbols)+0.5)) / 10
	}
	p("=== arbiter strength — what stands behind the criteria")
	p("    %d region(s): %d claimed, %d partly, %d unclaimed; %d of %d symbol(s) named by a test (%.1f%%)", s.Regions, s.Claimed, s.Partly, s.Unclaimed, s.Named, s.Symbols, share)
	p("    a test naming a symbol is not a test exercising it")
	p("=== what a repository cannot answer")
	p("    cost per unit of verified change — human time at a stated rate is not in version history")
	p("    share of undecided verdicts — there are no verdicts before the method; this instrument begins at zero when one starts")
	p("  the denominator is the intent, never the work unit: units split and any per-unit figure improves by splitting them")
	return OK
}

func (app *App) throughMethod(paths []string, asJSON bool) int {
	var rows []measure.Carried
	for _, where := range paths {
		for _, r := range measure.ThroughMethod(where) {
			number := r.PR
			if number == 0 && r.Issue != 0 {
				number = measure.AnsweredBy(where, r.Issue)
			}
			r.PR = number
			if number != 0 {
				if cost, _, ok := measure.Upstream(where, number); ok {
					r.Upstream = cost
				}
			}
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(app.Out, "  nothing here has been carried through the method")
		return Failed
	}
	if asJSON {
		enc := json.NewEncoder(app.Out)
		enc.SetIndent("", " ")
		_ = enc.Encode(rows)
		return OK
	}
	fmt.Fprintf(app.Out, "%-22s%7s%8s%10s   made of\n", "intent", "class", "method", "upstream")
	mine, theirs, carried := 0, 0, 0
	var unnamed []string
	for _, r := range rows {
		var made []string
		for _, x := range []struct {
			k string
			v int
		}{{"stating it", r.Stating}, {"reading the criteria", r.Reading}, {"escalations", r.Escalations}, {"left for a person", r.Left}} {
			if x.v > 0 {
				made = append(made, fmt.Sprintf("%s %d", x.k, x.v))
			}
		}
		up := "—"
		if r.Upstream >= 0 {
			up = fmt.Sprint(r.Upstream)
			mine, theirs, carried = mine+r.Touchpoints, theirs+r.Upstream, carried+1
		}
		if r.PR == 0 {
			unnamed = append(unnamed, r.Intent)
		}
		fmt.Fprintf(app.Out, "%-22s%7s%8d%10s   %s\n", r.Intent, r.Class, r.Touchpoints, up, strings.Join(made, ", "))
	}
	fmt.Fprintln(app.Out, "  a touchpoint is an occasion on which a person had to act, counted the same way as in the baseline: what an agent did in between counts on neither side")
	if carried > 0 {
		fmt.Fprintf(app.Out, "  over %d intent(s) carried and named: %d through the method, %d upstream. Neither is a rate, and the class average each belongs to is in the baseline for this estate\n", carried, mine, theirs)
	}
	if len(unnamed) > 0 {
		fmt.Fprintf(app.Out, "  no upstream intent is named by: %s — nothing is put beside a figure that names nothing\n", strings.Join(unnamed, ", "))
	}
	return OK
}

// Measure reports what the estate model is worth over each estate named.
func (app *App) Measure(_ context.Context, args []string) int {
	var paths []string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(app.Out, "usage: iflow measure <estate>...")
		return Misused
	}
	fmt.Fprintf(app.Out, "%-22s%8s%18s%12s\n", "estate", "reach", "with conventions", "quotation")
	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		r := estate.Open(abs)
		c, err := r.Coverage()
		if err != nil {
			return app.fatal(err)
		}
		q, err := measure.Quote(r, "")
		if err != nil {
			return app.fatal(err)
		}
		share := "—"
		if q.Share() >= 0 {
			share = fmt.Sprintf("%.1f%%", q.Share())
		}
		fmt.Fprintf(app.Out, "%-22s%7.1f%%%17.1f%%%12s\n", filepath.Base(abs), c.Percent(), c.WithConventions(), share)
		langs, _ := measure.Languages(r)
		for _, ext := range langs {
			if lq, err := measure.Quote(r, ext); err == nil && lq.Share() >= 0 {
				fmt.Fprintf(app.Out, "    %-18s%7s %17s%11.1f%% of %d site(s)\n", ext, "", "", lq.Share(), lq.Sites)
			}
		}
	}
	fmt.Fprintln(app.Out, "  reach is for comparing one index with another, never for judging this one: a representation of an estate is never complete and is measured by freshness")
	fmt.Fprintln(app.Out, "  quotation is what an answer handed to someone to act on rests on")
	return OK
}
