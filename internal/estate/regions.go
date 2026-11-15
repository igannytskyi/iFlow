package estate

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// What counts as a test. Looking for /tests/ is looking for one project's
// layout. The opposite error costs more: calling a region watched when it is
// not tells a change it has something to be judged against when it has
// nothing — so a directory that ships as an importable package, a project's
// own testing library, is not counted however it is named.
var (
	testDirs      = map[string]bool{"tests": true, "spec": true, "specs": true, "__tests__": true}
	ambiguousDirs = map[string]bool{"test": true, "testing": true}
)

// IsTest reports whether a file is test ground.
func (r *Repo) IsTest(rel string) bool {
	parts := strings.Split(rel, "/")
	for i, d := range parts[:len(parts)-1] {
		if testDirs[d] {
			return true
		}
		if ambiguousDirs[d] {
			pkg := filepath.Join(r.Root, filepath.FromSlash(strings.Join(parts[:i+1], "/")), "__init__.py")
			if _, err := os.Stat(pkg); err != nil {
				return true
			}
		}
	}
	name := parts[len(parts)-1]
	stem, _, _ := strings.Cut(name, ".")
	return strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.")
}

// TestGround reports whether a region is itself where tests are kept.
func (r *Repo) TestGround(region string) bool { return r.IsTest(path.Join(region, "x")) }

// illustration is ground that shows what code looks like rather than
// running it: a route in a getting-started snippet is written to be read,
// not deployed.
var illustrationDirs = map[string]bool{"docs": true, "doc": true, "examples": true, "example": true, "samples": true, "sample": true, "snippets": true, "gettingstarteddocs": true, "templates": true}

func illustration(rel string) bool {
	parts := strings.Split(rel, "/")
	return slices.ContainsFunc(parts[:len(parts)-1], func(p string) bool { return illustrationDirs[strings.ToLower(p)] })
}

// Verdicts on how well behaviour in a region is pinned down, by structure.
const (
	Claimed       = "claimed"
	PartlyClaimed = "partly claimed"
	Unclaimed     = "unclaimed"
)

func verdict(share float64, named int) string {
	switch {
	case share > 0.66:
		return Claimed
	case named > 0:
		return PartlyClaimed
	}
	return Unclaimed
}

// RegionRow is one region of the estate and whether anything judges it.
type RegionRow struct {
	Region     string  `json:"region"`
	Symbols    int     `json:"symbols"`
	Named      int     `json:"named by a test"`
	Share      float64 `json:"share"`
	TestGround bool    `json:"is test ground"`
	Verdict    string  `json:"verdict"`
}

// EveryRegion asks of the whole estate at once where nothing can judge a
// change, worst and largest first. A region is a directory directly holding
// code; nesting is not summed, because a verdict averaged over a subtree
// hides the part of it nothing watches.
func (r *Repo) EveryRegion() ([]RegionRow, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	where := map[string]map[string]bool{}
	var order []string
	for _, name := range ix.Names {
		for _, f := range ix.Defines[name] {
			region := path.Dir(f)
			if where[region] == nil {
				where[region] = map[string]bool{}
				order = append(order, region)
			}
			where[region][name] = true
		}
	}
	namedAnywhere := map[string]bool{}
	for _, ref := range ix.Refs {
		if r.IsTest(ref.From) {
			namedAnywhere[ref.Name] = true
		}
	}
	var rows []RegionRow
	for _, region := range order {
		syms := where[region]
		named := 0
		for s := range syms {
			if namedAnywhere[s] {
				named++
			}
		}
		share := float64(named) / float64(len(syms))
		rows = append(rows, RegionRow{Region: region, Symbols: len(syms), Named: named, Share: share,
			TestGround: r.TestGround(region), Verdict: verdict(share, named)})
	}
	order2 := map[string]int{Unclaimed: 0, PartlyClaimed: 1, Claimed: 2}
	slices.SortStableFunc(rows, func(a, b RegionRow) int {
		return cmp.Or(cmp.Compare(order2[a.Verdict], order2[b.Verdict]), cmp.Compare(b.Symbols, a.Symbols))
	})
	return rows, nil
}

// Observability is how well behaviour in one region can be pinned down, as
// far as structure can say. A test that names a symbol defined here has
// claimed the region, and a claim is not an observation.
type Observability struct {
	Region    string   `json:"region"`
	Verdict   string   `json:"verdict"`
	Symbols   int      `json:"symbols"`
	Named     int      `json:"named by a test"`
	WatchedBy []string `json:"watched by"`
	Why       string   `json:"why,omitempty"`
}

// Observe answers it for a path.
func (r *Repo) Observe(p string) (Observability, error) {
	ix, err := r.Index()
	if err != nil {
		return Observability{}, err
	}
	p = filepath.ToSlash(filepath.Clean(p))
	here := map[string]bool{}
	for name, where := range ix.Defines {
		if slices.ContainsFunc(where, func(w string) bool { return strings.HasPrefix(w, p) }) {
			here[name] = true
		}
	}
	if len(here) == 0 {
		why := "nothing is defined here that this index can see"
		if blind := r.Unseen(); len(blind) > 0 {
			why = "nothing here is in a language this index can read; that is not an empty region, it is an unread one"
		}
		return Observability{Region: p, Verdict: "unknown", Why: why}, nil
	}
	named, watchers := map[string]bool{}, map[string]bool{}
	for _, ref := range ix.Refs {
		if here[ref.Name] && r.IsTest(ref.From) {
			named[ref.Name] = true
			watchers[ref.From] = true
		}
	}
	share := float64(len(named)) / float64(len(here))
	return Observability{Region: p, Verdict: verdict(share, len(named)), Symbols: len(here), Named: len(named),
		WatchedBy: sortedSet(watchers)}, nil
}

// entryNames are files a program is started from, which nothing imports.
var entryNames = map[string]bool{"main": true, "__main__": true, "index": true, "app": true, "cli": true, "server": true, "program": true, "setup": true, "manage": true, "conftest": true}

// loadedByName are directories a framework loads from by where they sit.
var loadedByName = map[string]bool{"migrate": true, "migrations": true, "initializers": true, "models": true, "controllers": true, "jobs": true, "mailers": true, "channels": true, "helpers": true, "policies": true, "tasks": true, "middleware": true, "plugins": true, "hooks": true, "seeds": true, "steps": true}

// Coverage is the share of files defining anything that something else here
// is known to reach — a figure for comparing two indexes, never for judging
// this one: what nothing reaches may be reached by reflection, a container
// or a framework convention, and is reported as what it is.
type Coverage struct {
	Bearing, Covered, ByRunner, Entries, Loaded, AsText int
	Unreached                                           []string
	Unread                                              int
}

// Percent is the reached share.
func (c Coverage) Percent() float64 { return pct(c.Covered, c.Bearing) }

// WithConventions counts what a runner, an entry point, a loader or a name in
// text reaches as reached.
func (c Coverage) WithConventions() float64 {
	return pct(c.Covered+c.ByRunner+c.Entries+c.Loaded+c.AsText, c.Bearing)
}

func pct(n, of int) float64 {
	if of == 0 {
		return 0
	}
	return float64(int(1000.0*float64(n)/float64(of)+0.5)) / 10
}

// Coverage measures it.
func (r *Repo) Coverage() (Coverage, error) {
	ix, err := r.Index()
	if err != nil {
		return Coverage{}, err
	}
	all, _, err := r.Importers()
	if err != nil {
		return Coverage{}, err
	}
	defines := map[string]map[string]bool{}
	for name, files := range ix.Defines {
		for _, f := range files {
			if defines[f] == nil {
				defines[f] = map[string]bool{}
			}
			defines[f][name] = true
		}
	}
	reached := map[string]bool{}
	for _, ref := range ix.Refs {
		for _, target := range ix.Defines[ref.Name] {
			if target != ref.From {
				reached[target] = true
			}
		}
	}
	for target, who := range all {
		if len(who) > 0 {
			reached[target] = true
		}
	}
	var c Coverage
	var left []string
	for _, f := range sortedKeys(defines) {
		c.Bearing++
		parts := strings.Split(f, "/")
		switch {
		case reached[f]:
			c.Covered++
		case r.IsTest(f):
			c.ByRunner++
		case entryNames[stem(f)] || defines[f]["main"]:
			c.Entries++
		case slices.ContainsFunc(parts[:len(parts)-1], func(p string) bool { return loadedByName[p] }):
			c.Loaded++
		default:
			left = append(left, f)
		}
	}
	text := r.namedInText(left)
	for _, f := range left {
		if text[f] {
			c.AsText++
		} else {
			c.Unreached = append(c.Unreached, f)
		}
	}
	for _, n := range r.Unseen() {
		c.Unread += n
	}
	return c, nil
}

// namedInText is which of these files some other file names as text — a
// configuration naming a class a framework then loads.
func (r *Repo) namedInText(targets []string) map[string]bool {
	found := map[string]bool{}
	want := map[string]*regexp.Regexp{}
	for _, t := range targets {
		if s := stem(t); len(s) >= 5 {
			want[t] = regexp.MustCompile(`\b` + regexp.QuoteMeta(s) + `\b`)
		}
	}
	if len(want) == 0 {
		return found
	}
	for _, rel := range r.Sources() {
		b, err := os.ReadFile(r.abs(rel))
		if err != nil {
			continue
		}
		for t, re := range want {
			if t != rel && !found[t] && re.Match(b) {
				found[t] = true
			}
		}
	}
	return found
}

// Fresh is what one file was derived from, and whether that is current.
type Fresh struct {
	Region, DerivedFrom, LastTouched, When, State string
	OutsideHistory                                bool
}

// Freshness says, file by file, whether what the index holds is current.
// An index is never wholly fresh or wholly stale.
func (r *Repo) Freshness(under string) []Fresh {
	was := r.Cached()
	var out []Fresh
	for _, rel := range r.Sources() {
		if under != "" && !strings.HasPrefix(rel, filepath.ToSlash(under)) {
			continue
		}
		stamp, when := r.Stamp(rel)
		f := Fresh{Region: rel, DerivedFrom: "—", LastTouched: stamp, When: when, OutsideHistory: strings.HasPrefix(stamp, "contents:")}
		switch rec := was[rel]; {
		case rec == nil:
			f.State = "never derived"
		case rec.Commit == stamp:
			f.State, f.DerivedFrom = "fresh", rec.Commit
		default:
			f.State, f.DerivedFrom = "stale", rec.Commit
		}
		out = append(out, f)
	}
	return out
}

// Unknown is what the index could not resolve, by reason and how often.
func (r *Repo) Unknown() (map[string]int, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, u := range ix.Unresolved {
		out[u.Why]++
	}
	return out, nil
}

// CoverageNote is the line said beside every answer about how much of the
// estate the answer is about.
func (r *Repo) CoverageNote() string {
	read := len(r.Sources())
	tail := ""
	if made := r.MachineWritten(); made > 0 {
		tail = fmt.Sprintf("; %d more say they were generated and are not read — a change to one of those is a change to whatever produced it", made)
	}
	blind := r.Unseen()
	if len(blind) == 0 {
		return fmt.Sprintf("  this covers all %d file(s) here%s", read, tail)
	}
	total := 0
	exts := sortedKeys(blind)
	slices.SortStableFunc(exts, func(a, b string) int { return cmp.Compare(blind[b], blind[a]) })
	var kinds []string
	for _, e := range exts[:min(4, len(exts))] {
		kinds = append(kinds, fmt.Sprintf("%d %s", blind[e], e))
	}
	for _, n := range blind {
		total += n
	}
	return fmt.Sprintf("  this covers %d file(s); %d more are in languages this index cannot read (%s) and are not absent — they are unseen%s",
		read, total, strings.Join(kinds, ", "), tail)
}
