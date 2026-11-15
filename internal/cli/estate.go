package cli

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/reader"
)

const estateUsage = `usage: iflow estate affects <path>...        what a change here reaches
       iflow estate observability [<path>]   how well behaviour there is pinned
       iflow estate context <symbol|path>... the code to change, and what it touches
       iflow estate context --task "..."     where a task's words land, to choose from
       iflow estate coverage                 how much of this the index reaches
       iflow estate freshness [<path>]       what the answers were derived from
       iflow estate refresh                  derive again only what moved
       iflow estate unknown                  what it could not resolve
       iflow estate readers                  which languages are read here
       iflow estate corrections              what landings taught that reading cannot
       iflow estate correct <from> <to>... --by=<change> --why="..."
       iflow estate contracts <dir>          what crosses between repositories
       iflow estate reachability <dir> <repo> who consumes this, and who cannot be reached
       iflow estate inflight <changes>       what unfinished work already holds
       iflow estate conflicts <changes> <path>... who already holds these paths`

// Estate answers questions about the repository the program is run in.
func (app *App) EstateCommand(_ context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(app.Out, estateUsage)
		return Misused
	}
	wd, err := os.Getwd()
	if err != nil {
		return app.fatal(err)
	}
	r := app.Estate.(*estate.Local).Repo(wd)
	every := slices.Contains(args, "--all")
	rest := slices.DeleteFunc(slices.Clone(args[1:]), func(a string) bool { return a == "--all" })
	switch cmd := args[0]; {
	case cmd == "affects" && len(rest) > 0:
		return app.affects(r, rest, every)
	case cmd == "observability":
		return app.observability(r, rest, every)
	case cmd == "context" && len(rest) > 0:
		return app.context(r, rest, every)
	case cmd == "coverage":
		return app.coverage(r)
	case cmd == "freshness":
		return app.freshness(r, rest)
	case cmd == "refresh":
		moved, total, err := r.Refresh()
		if err != nil {
			return app.fatal(err)
		}
		for _, f := range moved {
			fmt.Fprintf(app.Out, "  re-derived  %s\n", f)
		}
		fmt.Fprintf(app.Out, "  %d of %d region(s) had moved; the rest were not looked at again\n%s\n", len(moved), total, r.CoverageNote())
	case cmd == "unknown":
		un, err := r.Unknown()
		if err != nil {
			return app.fatal(err)
		}
		total := 0
		for _, why := range slices.Sorted(maps.Keys(un)) {
			fmt.Fprintf(app.Out, "  %5d  %s\n", un[why], why)
			total += un[why]
		}
		fmt.Fprintf(app.Out, "  %d thing(s) this index cannot resolve inside what it can read\n%s\n", total, r.CoverageNote())
	case cmd == "readers":
		who := reader.Who()
		for _, name := range slices.Sorted(maps.Keys(who)) {
			fmt.Fprintf(app.Out, "  %12s  %s\n", name, strings.Join(who[name], " "))
		}
		var blind []string
		for ext := range estate.Unreadable {
			if !reader.Claims(ext) {
				blind = append(blind, ext)
			}
		}
		slices.Sort(blind)
		fmt.Fprintf(app.Out, "  %d extension(s) read here; %d known and unread (%s)\n", len(reader.Extensions()), len(blind), strings.Join(blind, " "))
		fmt.Fprintln(app.Out, "  a file with no extension is read by what its first line says it is")
		fmt.Fprintln(app.Out, "  a language nothing here reads is reported unseen, never absent")
	case cmd == "corrections":
		held := r.Corrections()
		for _, c := range held {
			fmt.Fprintf(app.Out, "  %s → %s  (%s, %s, with %s on %s)\n      %s\n", strings.Join(c.From, ", "), strings.Join(c.To, ", "), c.Provenance, c.Confidence, c.By, c.At, c.Why)
		}
		fmt.Fprintf(app.Out, "  %d edge(s) no reading of the source finds, and a landing did\n", len(held))
		fmt.Fprintln(app.Out, "  these are kept, not derived: the cache beside them is rebuilt on demand and these are what the estate learned from its own history")
	case cmd == "correct" && len(rest) >= 3:
		var scope []string
		var by, why string
		for _, a := range rest {
			switch {
			case strings.HasPrefix(a, "--by="):
				by = strings.TrimPrefix(a, "--by=")
			case strings.HasPrefix(a, "--why="):
				why = strings.TrimPrefix(a, "--why=")
			default:
				scope = append(scope, a)
			}
		}
		if by == "" || why == "" || len(scope) < 2 {
			fmt.Fprintln(app.Out, "  a correction says which change taught it and what the reading missed: --by=<change> --why=\"...\"")
			return Misused
		}
		held, err := r.Correct(scope[:1], scope[1:], by, why)
		if err != nil {
			return app.fatal(err)
		}
		fmt.Fprintf(app.Out, "  recorded: a change to %s reaches %s, observed with %s\n  %d correction(s) held\n", scope[0], strings.Join(scope[1:], ", "), by, len(held))
	case cmd == "contracts" && len(rest) > 0:
		return app.contracts(rest[0])
	case cmd == "reachability" && len(rest) >= 2:
		offers, c := estate.Reachability(rest[0], rest[1])
		for _, o := range offers {
			takers := strings.Join(o.TakenBy, ", ")
			if takers == "" {
				takers = "nobody in the estate"
			}
			also := ""
			if len(o.AlsoOfferedBy) > 0 {
				also = "; " + strings.Join(o.AlsoOfferedBy, ", ") + " offer(s) the same key, so a consumer of it may be reaching them instead"
			}
			fmt.Fprintf(app.Out, "  %s offers %s %s — taken by %s%s\n", o.Repo, o.Kind, o.Key, takers, also)
		}
		for i, u := range c.Unmatched {
			if c.UnmatchedBy[i] == rest[1] {
				fmt.Fprintf(app.Out, "    beyond reach  %s  this region consumes something the estate does not offer\n", u.Key)
			}
		}
		if len(offers) == 0 {
			fmt.Fprintln(app.Out, "  this region offers nothing across a boundary")
		} else {
			fmt.Fprintf(app.Out, "  %d offer(s)\n", len(offers))
		}
	case cmd == "inflight" && len(rest) > 0:
		units := estate.Unfinished(rest[0])
		for _, u := range units {
			holds := strings.Join(u.Holds, ", ")
			if holds == "" {
				holds = "—"
			}
			fmt.Fprintf(app.Out, "  %s/%s  holds %s\n", u.Change, u.ID, holds)
		}
		fmt.Fprintf(app.Out, "  %d unit(s) planned and not entered\n", len(units))
	case cmd == "conflicts" && len(rest) >= 2:
		rows, err := r.Conflicts(rest[0], rest[1:])
		if err != nil {
			return app.fatal(err)
		}
		for _, c := range rows {
			fmt.Fprintf(app.Out, "  %6s  %s/%s  on %s  (%s)\n", c.Confidence, c.Change, c.ID, strings.Join(c.On, ", "), c.How)
		}
		if len(rows) == 0 {
			fmt.Fprintln(app.Out, "  nothing unfinished holds this ground")
		} else {
			fmt.Fprintf(app.Out, "  %d unfinished unit(s) already hold this ground\n", len(rows))
		}
	default:
		fmt.Fprintln(app.Out, estateUsage)
		return Misused
	}
	return OK
}

func (app *App) affects(r *estate.Repo, paths []string, every bool) int {
	out, err := r.Affects(paths)
	if err != nil {
		return app.fatal(err)
	}
	var strong []estate.Reached
	for _, x := range out {
		if x.Confidence != "low" {
			strong = append(strong, x)
		}
	}
	shown := strong
	if every {
		shown = out
	} else if len(shown) > 20 {
		shown = shown[:20]
	}
	for _, x := range shown {
		names := strings.Join(x.Through[:min(3, len(x.Through))], ", ")
		if len(x.Through) > 3 {
			names += fmt.Sprintf(" and %d more", len(x.Through)-3)
		}
		note := ""
		if !x.ImportsIt {
			note = "  (no import reaches it)"
		}
		fmt.Fprintf(app.Out, "  %6s  %s  through %s%s\n", x.Confidence, x.File, names, note)
	}
	if !every && len(strong) > len(shown) {
		fmt.Fprintf(app.Out, "  … and %d more of the same strength (--all for every one)\n", len(strong)-len(shown))
	}
	if len(out) > 0 {
		grades, where := map[string]int{}, map[string]int{}
		stranger := 0
		for _, x := range out {
			grades[x.Confidence]++
			parts := strings.Split(x.File, "/")
			top := parts[0]
			if len(parts) > 2 {
				top += "/" + parts[1]
			}
			where[top]++
			if !x.ImportsIt {
				stranger++
			}
		}
		var gs, ws []string
		for _, g := range slices.Sorted(maps.Keys(grades)) {
			gs = append(gs, fmt.Sprintf("%d %s", grades[g], g))
		}
		tops := slices.SortedStableFunc(maps.Keys(where), func(a, b string) int { return cmp.Compare(where[b], where[a]) })
		for _, t := range tops[:min(4, len(tops))] {
			ws = append(ws, fmt.Sprintf("%s %d", t, where[t]))
		}
		fmt.Fprintf(app.Out, "  shape: %s; mostly in %s\n", strings.Join(gs, ", "), strings.Join(ws, ", "))
		if stranger > 0 {
			fmt.Fprintf(app.Out, "  %d of them share a name with it and import nothing that leads to it — a coincidence of vocabulary unless a framework loads them\n", stranger)
		}
	}
	named, err := r.NamedBy(paths)
	if err != nil {
		return app.fatal(err)
	}
	for _, x := range named {
		fmt.Fprintf(app.Out, "  %6s  %s  %s\n", x.Confidence, x.File, x.How)
	}
	all, _, err := r.Importers()
	if err != nil {
		return app.fatal(err)
	}
	direct := map[string]bool{}
	for _, x := range slices.Concat(out, named) {
		direct[x.File] = true
	}
	for _, p := range paths {
		direct[filepath.ToSlash(filepath.Clean(p))] = true
	}
	var seeds []string
	for f := range direct {
		seeds = append(seeds, f)
	}
	slices.Sort(seeds)
	far := estate.Transitive(seeds, all)
	ordered := slices.SortedFunc(maps.Keys(far), func(a, b string) int { return cmp.Or(cmp.Compare(far[a], far[b]), cmp.Compare(a, b)) })
	limit := len(ordered)
	if !every {
		limit = min(10, limit)
	}
	for _, f := range ordered[:limit] {
		fmt.Fprintf(app.Out, "     low  %s  through %d import(s)\n", f, far[f])
	}
	if !every && len(ordered) > 10 {
		fmt.Fprintf(app.Out, "  … and %d more through imports (--all for every one)\n", len(ordered)-10)
	}
	un, _ := r.Unknown()
	blind := 0
	for _, n := range un {
		blind += n
	}
	fmt.Fprintf(app.Out, "  %d reached through code, %d that only name it as text, %d further through imports.\n", len(out), len(named), len(far))
	fmt.Fprintf(app.Out, "  %d call(s) this index cannot resolve at all, so neither figure is a floor or a ceiling — it is what one parser could see.\n", blind)
	fmt.Fprintln(app.Out, r.CoverageNote())
	return OK
}

func (app *App) observability(r *estate.Repo, rest []string, every bool) int {
	if len(rest) > 0 {
		o, err := r.Observe(rest[0])
		if err != nil {
			return app.fatal(err)
		}
		fmt.Fprintf(app.Out, "  %s  %s", o.Region, o.Verdict)
		if o.Why != "" {
			fmt.Fprintf(app.Out, " — %s\n", o.Why)
			return OK
		}
		fmt.Fprintf(app.Out, "  %d/%d symbol(s) named by a test\n", o.Named, o.Symbols)
		for _, w := range o.WatchedBy[:min(12, len(o.WatchedBy))] {
			fmt.Fprintf(app.Out, "    watched by %s\n", w)
		}
		fmt.Fprintln(app.Out, "  a test naming a symbol is not a test exercising it")
		return OK
	}
	all, err := r.EveryRegion()
	if err != nil {
		return app.fatal(err)
	}
	var rows, ground, worst []estate.RegionRow
	for _, x := range all {
		if x.TestGround {
			ground = append(ground, x)
		} else {
			rows = append(rows, x)
			if x.Verdict != estate.Claimed {
				worst = append(worst, x)
			}
		}
	}
	shown := worst[:min(25, len(worst))]
	if every {
		shown = rows
	}
	for _, x := range shown {
		fmt.Fprintf(app.Out, "  %14s  %s  %d/%d symbol(s) named by a test\n", x.Verdict, x.Region, x.Named, x.Symbols)
	}
	if !every && len(worst) > 25 {
		fmt.Fprintf(app.Out, "  … and %d more that are not fully claimed (--all for every region)\n", len(worst)-25)
	}
	if len(all) == 0 {
		fmt.Fprintln(app.Out, "  nothing is defined here that this index can see")
		fmt.Fprintln(app.Out, r.CoverageNote())
		return OK
	}
	tally, syms, loose := map[string]int{}, 0, 0
	for _, x := range rows {
		tally[x.Verdict]++
		syms += x.Symbols
		if x.Verdict == estate.Unclaimed {
			loose += x.Symbols
		}
	}
	var parts []string
	for _, v := range []string{estate.Claimed, estate.PartlyClaimed, estate.Unclaimed} {
		if tally[v] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", tally[v], v))
		}
	}
	fmt.Fprintf(app.Out, "  %d region(s): %s\n", len(rows), strings.Join(parts, ", "))
	if len(ground) > 0 {
		fmt.Fprintf(app.Out, "  %d further region(s) are themselves where tests are kept and are not counted — a change there is judged by the test it is part of\n", len(ground))
	}
	fmt.Fprintf(app.Out, "  %d of %d symbol(s) sit in regions nothing names, so a change there has nothing to be judged against\n", loose, syms)
	fmt.Fprintln(app.Out, "  a test naming a symbol is not a test exercising it — this is what structure suggests, not what execution showed")
	fmt.Fprintln(app.Out, r.CoverageNote())
	return OK
}

func (app *App) coverage(r *estate.Repo) int {
	c, err := r.Coverage()
	if err != nil {
		return app.fatal(err)
	}
	for _, f := range c.Unreached[:min(15, len(c.Unreached))] {
		fmt.Fprintf(app.Out, "  unreached  %s\n", f)
	}
	if len(c.Unreached) > 15 {
		fmt.Fprintf(app.Out, "  … and %d more nothing here reaches\n", len(c.Unreached)-15)
	}
	fmt.Fprintf(app.Out, "  %d of %d file(s) that define anything are reached by something else here — %.1f%%\n", c.Covered, c.Bearing, c.Percent())
	for _, x := range []struct {
		how string
		n   int
	}{{"test ground a runner finds by name", c.ByRunner}, {"an entry point", c.Entries}, {"where a framework loads by name", c.Loaded}, {"named as text and nowhere else", c.AsText}} {
		if x.n > 0 {
			fmt.Fprintf(app.Out, "  %d more are %s\n", x.n, x.how)
		}
	}
	fmt.Fprintf(app.Out, "  counting those, %.1f%%\n", c.WithConventions())
	fmt.Fprintln(app.Out, "  a name matched is not a call resolved; this counts what one reader could see, and what nothing reaches may be reached by reflection, a container or a framework convention")
	fmt.Fprintln(app.Out, r.CoverageNote())
	return OK
}

func (app *App) freshness(r *estate.Repo, rest []string) int {
	under := ""
	if len(rest) > 0 {
		under = rest[0]
	}
	rows := r.Freshness(under)
	fresh, outside := 0, 0
	for _, x := range rows {
		if x.State == "fresh" {
			fresh++
		} else {
			fmt.Fprintf(app.Out, "  %13s  %s  last touched %s %s\n", x.State, x.Region, x.LastTouched, x.When)
		}
		if x.OutsideHistory {
			outside++
		}
	}
	fmt.Fprintf(app.Out, "  %d of %d region(s) current against what last touched them\n", fresh, len(rows))
	if outside > 0 {
		fmt.Fprintf(app.Out, "  %d of them sit outside any history this can read and are keyed on their contents instead: a change is still noticed, but nothing here can say which commit it came from\n", outside)
	}
	fmt.Fprintln(app.Out, r.CoverageNote())
	return OK
}

func (app *App) contracts(dir string) int {
	c := estate.ContractsUnder(dir)
	for _, e := range c.Edges {
		fmt.Fprintf(app.Out, "  %6s  %s → %s  %s %s  (%s)\n", e.Confidence, e.From, e.To, e.Kind, e.Key, e.How)
	}
	for i, u := range c.Unmatched {
		why := "nothing in the estate offers this, so it is outside it or missing from it"
		if c.UnseenCount > 0 {
			why = fmt.Sprintf("nothing the index can read offers this — it may be offered by one of the %d file(s) here it cannot read, or from outside the estate", c.UnseenCount)
		}
		fmt.Fprintf(app.Out, "     n/a  %s → ?  %s %s  %s\n", c.UnmatchedBy[i], u.Kind, u.Key, why)
	}
	for _, o := range c.Unconsumed {
		fmt.Fprintf(app.Out, "     n/a  %s offers %s %s and nothing in the estate consumes it\n", c.OfferedBy[o], o.Kind, o.Key)
	}
	fmt.Fprintf(app.Out, "  %d edge(s) joined, %d consumer(s) pointing outside, %d offer(s) nobody takes\n", len(c.Edges), len(c.Unmatched), len(c.Unconsumed))
	if c.Shown > 0 {
		fmt.Fprintf(app.Out, "  %d file(s) of documentation, examples and fixtures were not read: a route in a snippet is written to be read, not deployed\n", c.Shown)
	}
	if c.Internal > 0 {
		fmt.Fprintf(app.Out, "  %d call(s) go to routes the calling repository offers itself and cross no boundary\n", c.Internal)
	}
	if len(c.Loose) > 0 {
		fmt.Fprintf(app.Out, "  %d route(s) are declared as fragments assembled at import time; this index does not resolve them into paths, so what they offer is unread rather than absent\n", len(c.Loose))
	}
	for _, f := range c.Fakes {
		if c.FakeTo[f] == "" {
			fmt.Fprintf(app.Out, "     low  %s → ?  %s %s  a stand-in for it is kept in this repository's own tests, so it depends on it, and nothing the index can read offers it\n", c.FakeOf[f], f.Kind, f.Key)
		}
	}
	if len(c.Other) > 0 {
		var named []string
		for _, k := range slices.SortedFunc(maps.Keys(c.Other), func(a, b string) int { return cmp.Compare(c.Other[b], c.Other[a]) }) {
			named = append(named, fmt.Sprintf("%s (%d file(s))", k, c.Other[k]))
		}
		fmt.Fprintf(app.Out, "  and this estate is also bound together by %s, which this index does not read — those services are not unconnected, they are unexamined\n", strings.Join(named, ", "))
	}
	fmt.Fprintln(app.Out, estate.Open(dir).CoverageNote())
	return OK
}

func (app *App) context(r *estate.Repo, rest []string, every bool) int {
	budget, task := 120000, ""
	var targets []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case strings.HasPrefix(a, "--budget="):
			fmt.Sscan(strings.TrimPrefix(a, "--budget="), &budget)
		case strings.HasPrefix(a, "--task="):
			task = strings.TrimPrefix(a, "--task=")
		case a == "--task" && i+1 < len(rest):
			task, i = rest[i+1], i+1
		default:
			targets = append(targets, a)
		}
	}
	if task == "" && len(targets) > 0 && strings.Contains(targets[0], " ") {
		task, targets = targets[0], targets[1:]
	}
	if task != "" {
		found, missing, err := r.Survey(task, 40)
		if err != nil {
			return app.fatal(err)
		}
		fmt.Fprintln(app.Out, "=== where the words of this task land")
		for _, h := range found {
			fmt.Fprintf(app.Out, "    %s  %s:\n", h.Word, h.How)
			for _, p := range h.Places {
				mark := ""
				if r.IsTest(p.File) {
					mark = "  (test ground)"
				}
				fmt.Fprintf(app.Out, "        %s  line %d%s\n", p.File, p.Start, mark)
			}
			if h.More > 0 {
				fmt.Fprintf(app.Out, "        … and %d more place(s)\n", h.More)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(app.Out, "    placed nothing: %s — either this estate calls them something else, or the ground they name is not read here\n", strings.Join(missing, ", "))
		}
		fmt.Fprintln(app.Out, "  — these are places, not an answer. Which of them the task is about is not derivable from the words: ask again naming the ones you mean, as `context <file>:<symbol> …`, and each will be expanded in full.")
		fmt.Fprintln(app.Out, r.CoverageNote())
		return OK
	}
	share := budget / max(len(targets), 1)
	spent := 0
	for _, t := range targets {
		c, err := r.Context(t)
		if err != nil {
			return app.fatal(err)
		}
		spent += app.renderContext(r, c, share, every)
	}
	fmt.Fprintf(app.Out, "  — %d character(s) of a %d budget; what is firm is marked; the rest is a name that matched, and a name can belong to something else\n", spent, budget)
	fmt.Fprintln(app.Out, r.CoverageNote())
	return OK
}

// renderContext prints what was found, firmest first, and stops when the
// budget is gone: what a reader is handed first is what it acts on.
func (app *App) renderContext(r *estate.Repo, c *estate.Context, budget int, every bool) int {
	spent := 0
	say := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		spent += len(line) + 1
		fmt.Fprintln(app.Out, line)
	}
	if len(c.Subjects) > 1 {
		say("    (%d places define this name; all of them are below, because which one a caller means is not derivable from the name)", len(c.Subjects))
	}
	limit := max(120, budget/400)
	if every {
		limit = 1 << 30
	}
	for _, s := range c.Subjects {
		say("=== %s:%d-%d", s.File, s.Start, s.End)
		for _, l := range r.Excerpt(s.File, s.Start, s.End, limit) {
			say("%6d  %s", l.N, l.Text)
		}
		if s.End-s.Start+1 > limit {
			say("    … %d more line(s) not quoted; what is in them:", s.End-s.Start+1-limit)
			for _, p := range r.Outline(s.File, s.Start, s.End) {
				say("      %s:%d  %s", s.File, p.Line, p.Name)
			}
		}
	}
	perFile, crowded := map[string]int{}, map[string]int{}
	skipped := 0
	capPer := max(3, budget/20000)
	if every {
		capPer = 1 << 30
	}
	for _, who := range c.Callers {
		if (!every && float64(spent) >= float64(budget)*0.75) || perFile[who.File] >= capPer {
			skipped++
			crowded[who.File]++
			continue
		}
		perFile[who.File]++
		where := who.Confidence
		if who.Here {
			where = "in the same file"
		}
		if who.Reaches != "" {
			where += fmt.Sprintf(", and of the %d it can reach only %s", len(c.Subjects), who.Reaches)
		}
		say("--- called from %s:%d  (%s)", who.File, who.Line, where)
		last := max(who.Ends, who.Line)
		for _, l := range r.Excerpt(who.File, who.Line-2, last+2, 1<<30) {
			say("%6d  %s", l.N, l.Text)
		}
	}
	if skipped > 0 {
		files := slices.SortedFunc(maps.Keys(crowded), func(a, b string) int { return cmp.Compare(crowded[b], crowded[a]) })
		var worst []string
		for _, f := range files[:min(5, len(files))] {
			worst = append(worst, fmt.Sprintf("%s (%d more)", f, crowded[f]))
		}
		say("    … and %d call site(s) not quoted, in %d file(s): %s", skipped, len(crowded), strings.Join(worst, ", "))
	}
	var firm []estate.Callee
	for _, x := range c.CallsOut {
		if x.Confidence != "low" || every {
			firm = append(firm, x)
		}
	}
	if len(c.CallsOut) > 0 {
		say("--- what it calls that lives here")
		for _, x := range firm[:min(len(firm), max(40, budget/2000))] {
			say("    %6s  %s  %s (line %d)", x.Confidence, x.File, x.Name, x.Line)
		}
		if loose := len(c.CallsOut) - len(firm); loose > 0 {
			say("    … and %d more that share a name with something here and import nothing that leads to it (--all to see them)", loose)
		}
	}
	if len(c.Judges) > 0 || len(c.Taught) > 0 {
		say("--- what would judge a change here")
		for _, t := range slices.Sorted(maps.Keys(c.Judges))[:min(10, len(c.Judges))] {
			names := c.Judges[t]
			say("    %s  names %s", t, strings.Join(names[:min(4, len(names))], ", "))
		}
		for _, t := range slices.Sorted(maps.Keys(c.Taught)) {
			say("    %s  observed, not read: %s", t, c.Taught[t])
		}
	} else {
		say("--- nothing here names what this defines, and no landing has shown otherwise: a change to it is judged by nothing that can be found by reading")
	}
	return spent
}
