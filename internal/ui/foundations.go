package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/workspace"
)

// estateFacts is what the estate model says about a project as a whole —
// area 7 and what area 10 has learned into it — read once in the
// background, since reading an estate takes time a frame does not have.
type estateFacts struct {
	files, names, calls int
	regions, unwatched  int
	indexedAt           time.Time
	corrections         int
	err                 string
}

func readEstate(l *estate.Local, root string) *estateFacts {
	f := &estateFacts{}
	if l == nil {
		f.err = "no estate model is configured"
		return f
	}
	repo := l.Repo(root)
	ix, err := repo.Index()
	if err != nil {
		f.err = err.Error()
		return f
	}
	f.files, f.names, f.calls = len(ix.Files), len(ix.Names), len(ix.Refs)
	if st, err := os.Stat(filepath.Join(repo.Home(), "index.json")); err == nil {
		f.indexedAt = st.ModTime()
	}
	if regions, err := repo.EveryRegion(); err == nil {
		f.regions = len(regions)
		for _, g := range regions {
			if g.Verdict == "unclaimed" && !g.TestGround {
				f.unwatched++
			}
		}
	}
	f.corrections = len(repo.Corrections())
	return f
}

// foundation is one of areas 7 to 13 as it stands for a project: a few
// facts, and whether it has anything to show yet.
type foundation struct {
	n     int
	facts [][2]string // value, what it is
	empty string      // said when there is nothing yet
}

// foundationsOf gathers, for areas 7 to 13, what the project's changes and
// its estate say.
func foundationsOf(p *project, e *estateFacts) []foundation {
	var crossings, refusals, reviewsDone, reviewFiles, reviewed, unreviewed int
	var met, failed, undecided, passing, checked, repeatable, evidence int
	var handed, fresh, cached, out int
	var spent float64
	people := map[string]bool{}
	for _, c := range p.ws.Changes {
		if c.State == workspace.Checked {
			checked++
			if c.Report.Passed() {
				passing++
			}
		}
		files, _ := filepath.Glob(filepath.Join(c.Change.Dir, "reviews", "*.md"))
		reviewFiles += len(files)
		for _, k := range c.Costs {
			handed += k.Handed
			fresh += k.Input + k.CacheWrite
			cached += k.CacheRead
			out += k.Output
			spent += k.CostUSD
		}
		for _, it := range c.Items {
			switch it.Kind {
			case "Crossing":
				crossings++
				note := strings.ToLower(it.Field("Note"))
				if strings.Contains(note, "refused") {
					refusals++
				}
				if it.Field("Stage") == "review" {
					reviewsDone++
				}
			case "Review":
				if it.Field("Reviewed before admission") == "yes" {
					reviewed++
				} else {
					unreviewed++
				}
				if by := it.Field("By"); by != "" {
					people[by] = true
				}
			case "Verdict":
				switch it.Field("Outcome") {
				case "met":
					met++
				case "failed":
					failed++
				default:
					undecided++
				}
			case "Evidence":
				evidence++
				if it.Field("Repeatable") == "yes" {
					repeatable++
				}
			}
		}
	}
	owed, settled := 0, 0
	if p.led != nil {
		owed, settled = len(p.led.Owed), len(p.led.Settled)
	}
	var out7 foundation
	switch {
	case e == nil:
		out7 = foundation{n: 7, empty: "reading the estate…"}
	case e.err != "":
		out7 = foundation{n: 7, empty: e.err}
	default:
		when := "never"
		if !e.indexedAt.IsZero() {
			when = e.indexedAt.Format("2 Jan 15:04")
		}
		out7 = foundation{n: 7, facts: [][2]string{{itoa(e.files), "files read"}, {itoa(e.names), "names defined"},
			{itoa(e.calls), "references between them"}, {itoa(e.regions), "regions"}, {itoa(e.unwatched), "regions no test watches"}, {when, "last read"}}}
	}
	return []foundation{
		out7,
		{n: 8, facts: [][2]string{{itoa(crossings), "crossings recorded"}, {itoa(len(p.ws.Changes)), "changes with a record"}},
			empty: "nothing recorded yet"},
		{n: 9, facts: [][2]string{{money(spent), "spent"}, {tokens(handed), "tokens handed by the framework"},
			{tokens(fresh), "tokens read fresh"}, {tokens(cached), "read from cache"}, {tokens(out), "tokens written"}}, empty: "nothing spent yet"},
		{n: 10, facts: [][2]string{{itoa(e.correctionsOr()), "edges learned from landings"}},
			empty: "nothing learned yet: a landing that touches ground the estate did not see teaches it here"},
		{n: 11, facts: [][2]string{{itoa(owed), "escalations open"}, {itoa(settled), "settled"}, {itoa(reviewed), "specifications read before admission"},
			{itoa(unreviewed), "admitted unread"}, {itoa(refusals), "refusals"}, {itoa(reviewFiles), "agent reviews kept"}}},
		{n: 12, facts: [][2]string{{strings.Join(keys(people), ", "), "who accepted criteria"}, {itoa(reviewsDone), "reviews recorded"}}},
		{n: 13, facts: [][2]string{{itoa(passing) + " / " + itoa(checked), "changes pass the gate"}, {itoa(met), "verdicts met"},
			{itoa(failed), "failed"}, {itoa(undecided), "undecided"}, {itoa(repeatable) + " / " + itoa(evidence), "evidence repeatable"}}},
	}
}

func (e *estateFacts) correctionsOr() int {
	if e == nil {
		return 0
	}
	return e.corrections
}

func itoa(v int) string { return strconv.Itoa(v) }

func keys(m map[string]bool) []string { return sortedStrings(m) }
