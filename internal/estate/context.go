package estate

import (
	"cmp"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/reader"
)

// Every other answer here is shaped for a decision. This one is shaped for
// work: an agent asked to change something needs the code, not a list of
// paths to go and read, and it needs to be told which part of what it was
// handed is firm. The source is verbatim with its line numbers, the firm
// ground comes first, and everything is cut to a budget that is stated.

// Place is where something is defined: a file and the lines it spans.
type Place struct {
	File       string
	Start, End int
}

// WhereDefined is every place a name is defined. Places, not sightings: what
// sits inside something already listed is part of it.
func (r *Repo) WhereDefined(name string) ([]Place, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	var out []Place
	for rel, rec := range ix.Files {
		for _, s := range rec.Symbols {
			if s.Name == name {
				out = append(out, Place{rel, s.Start, s.End})
			}
		}
	}
	slices.SortFunc(out, func(a, b Place) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start), cmp.Compare(b.End, a.End))
	})
	var kept []Place
	for _, p := range out {
		if slices.ContainsFunc(kept, func(k Place) bool { return k.File == p.File && k.Start <= p.Start && p.End <= k.End }) {
			continue
		}
		kept = append(kept, p)
	}
	return kept, nil
}

// Line is one numbered line of a file.
type Line struct {
	N    int
	Text string
}

// Excerpt is the lines themselves, numbered as they are in the file, at most
// limit of them. Split on newlines and nothing else: a parser counts lines
// that way, and quoting by any other count puts text on the wrong line.
func (r *Repo) Excerpt(rel string, start, end, limit int) []Line {
	b, err := os.ReadFile(r.abs(rel))
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	start = max(1, start)
	end = min(len(lines), end)
	if end-start+1 > limit {
		end = start + limit - 1
	}
	var out []Line
	for n := start; n <= end; n++ {
		out = append(out, Line{n, lines[n-1]})
	}
	return out
}

// Site is one place a name is called, and how firmly that is known.
type Site struct {
	File       string
	Line, Ends int
	Kind       reader.Kind
	Confidence string
	Here       bool   // in the file that defines it
	Reaches    string // which of several definitions the caller can reach
}

// CallSites is who calls a name, on which line, firmest first; one line is
// one place to look, however often the name appears on it.
func (r *Repo) CallSites(name string) ([]Site, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	where := ix.Defines[name]
	type key struct {
		file string
		line int
	}
	best := map[key]Site{}
	for rel, rec := range ix.Files {
		for _, c := range rec.Calls {
			if c.Name != name {
				continue
			}
			var s Site
			if slices.Contains(where, rel) {
				// A call in the defining file resolves to nothing else — but
				// a typed one is a use of the class, and quoting it as a
				// call to a method misleads.
				if c.Kind == reader.Typed {
					continue
				}
				s = Site{rel, c.Line, c.End, c.Kind, "high", true, ""}
			} else {
				conf := "low"
				switch {
				case c.Kind == reader.Typed && len(where) == 1:
					conf = "high"
				case c.Kind == reader.Import || (c.Kind == reader.Name && len(where) == 1):
					conf = "medium"
				}
				s = Site{rel, c.Line, c.End, c.Kind, conf, false, ""}
			}
			k := key{rel, c.Line}
			if was, ok := best[k]; !ok || rank[s.Confidence] < rank[was.Confidence] {
				best[k] = s
			}
		}
	}
	out := make([]Site, 0, len(best))
	for _, s := range best {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Site) int {
		return cmp.Or(cmp.Compare(rank[a.Confidence], rank[b.Confidence]), cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
	return out, nil
}

// Callee is something a file calls that lives elsewhere here.
type Callee struct {
	File, Name string
	Line       int
	Confidence string
	Reachable  bool
}

// Callees is what a file calls that is defined somewhere else here, within
// the lines asked about. A name matched in a file nothing imports is a
// coincidence of vocabulary, and is graded as one.
func (r *Repo) Callees(rel string, from, to int) ([]Callee, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	all, _, err := r.Importers()
	if err != nil {
		return nil, err
	}
	// Which files this one reaches: the importers graph read the other way.
	out := edgeSet{}
	for target, who := range all {
		for w := range who {
			add(out, w, target)
		}
	}
	seen := map[string]bool{rel: true}
	for f := range Transitive([]string{rel}, out) {
		seen[f] = true
	}
	best := map[[2]string]Callee{}
	rec := ix.Files[rel]
	if rec == nil {
		return nil, nil
	}
	for _, c := range rec.Calls {
		if from > 0 && (c.Line < from || c.Line > to) {
			continue
		}
		defs := ix.Defines[c.Name]
		for _, target := range defs {
			if target == rel {
				continue
			}
			conf := "low"
			switch {
			case c.Kind == reader.Typed && len(defs) == 1:
				conf = "high"
			case c.Kind == reader.Import || (c.Kind == reader.Name && len(defs) == 1):
				conf = "medium"
			}
			if !seen[target] {
				conf = "low"
			}
			k := [2]string{target, c.Name}
			if was, ok := best[k]; !ok || rank[conf] < rank[was.Confidence] {
				best[k] = Callee{target, c.Name, c.Line, conf, seen[target]}
			}
		}
	}
	res := make([]Callee, 0, len(best))
	for _, c := range best {
		res = append(res, c)
	}
	slices.SortFunc(res, func(a, b Callee) int {
		return cmp.Or(cmp.Compare(rank[a.Confidence], rank[b.Confidence]), cmp.Compare(a.File, b.File), cmp.Compare(a.Name, b.Name))
	})
	return res, nil
}

// JudgedBy is what would judge a change to a file: the tests that name what
// it defines, and the test ground a landing showed reaches it — which no
// reading finds, and which outranks what reading found.
func (r *Repo) JudgedBy(rel, name string) (read map[string][]string, taught map[string]string, err error) {
	ix, err := r.Index()
	if err != nil {
		return nil, nil, err
	}
	mine := map[string]bool{}
	if rec := ix.Files[rel]; rec != nil {
		for _, s := range rec.Symbols {
			if name == "" || s.Name == name {
				mine[s.Name] = true
			}
		}
	}
	found := map[string]map[string]bool{}
	for other, rec := range ix.Files {
		if other == rel || !r.IsTest(other) {
			continue
		}
		for _, c := range rec.Calls {
			if mine[c.Name] {
				if found[other] == nil {
					found[other] = map[string]bool{}
				}
				found[other][c.Name] = true
			}
		}
	}
	read = map[string][]string{}
	for f, names := range found {
		read[f] = sortedSet(names)
	}
	taught = map[string]string{}
	learnt, _ := r.Learned([]string{rel})
	for _, t := range learnt {
		if t.File != rel && r.IsTest(t.File) {
			taught[t.File] = t.Why
		}
	}
	return read, taught, nil
}

// Context is what an agent needs to change one thing, in one answer.
type Context struct {
	Target   string
	Subjects []Place
	Callers  []Site
	CallsOut []Callee
	Judges   map[string][]string
	Taught   map[string]string
}

// Context gathers it for a symbol, a file, or a file and a symbol in it
// written file:symbol.
func (r *Repo) Context(target string) (*Context, error) {
	ix, err := r.Index()
	if err != nil {
		return nil, err
	}
	c := &Context{Target: target}
	var name, rel string
	switch {
	case exists(r.abs(target)):
		rel = filepath.ToSlash(target)
	case strings.Contains(target, ":"):
		i := strings.LastIndex(target, ":")
		rel, name = target[:i], target[i+1:]
		if !exists(r.abs(rel)) {
			rel = ""
		}
	default:
		name = target
	}
	if name != "" {
		places, err := r.WhereDefined(name)
		if err != nil {
			return nil, err
		}
		for _, p := range places {
			if rel == "" || p.File == rel {
				c.Subjects = append(c.Subjects, p)
			}
		}
		if c.Callers, err = r.CallSites(name); err != nil {
			return nil, err
		}
	} else {
		start, end := 0, 0
		if rec := ix.Files[rel]; rec != nil {
			for _, s := range rec.Symbols {
				if start == 0 || s.Start < start {
					start = s.Start
				}
				end = max(end, s.End)
			}
		}
		if start == 0 {
			start, end = 1, 1<<30
		}
		c.Subjects = []Place{{rel, start, end}}
		reach, err := r.Affects([]string{rel})
		if err != nil {
			return nil, err
		}
		for _, x := range reach {
			c.Callers = append(c.Callers, Site{File: x.File, Kind: reader.Name, Confidence: x.Confidence})
		}
	}
	// Where a name is defined in several places, the import graph usually
	// says which a caller means: a file that can reach exactly one means it.
	if len(c.Subjects) > 1 {
		_, firm, err := r.Importers()
		if err != nil {
			return nil, err
		}
		reach := map[string]map[string]int{}
		for _, s := range c.Subjects {
			reach[s.File] = Transitive([]string{s.File}, firm)
		}
		for i := range c.Callers {
			var candidates []string
			for f, files := range reach {
				if _, ok := files[c.Callers[i].File]; ok {
					candidates = append(candidates, f)
				}
			}
			if len(candidates) == 1 {
				c.Callers[i].Reaches = candidates[0]
				if c.Callers[i].Confidence == "low" {
					c.Callers[i].Confidence = "medium"
				}
			}
		}
		slices.SortStableFunc(c.Callers, func(a, b Site) int {
			return cmp.Or(cmp.Compare(rank[a.Confidence], rank[b.Confidence]), cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
		})
	}
	if len(c.Subjects) > 0 {
		s := c.Subjects[0]
		if c.CallsOut, err = r.Callees(s.File, s.Start, s.End); err != nil {
			return nil, err
		}
		if c.Judges, c.Taught, err = r.JudgedBy(s.File, name); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// Outline is what is inside something too long to quote: its parts and the
// line each starts on.
func (r *Repo) Outline(rel string, start, end int) []Symbol {
	ix, err := r.Index()
	if err != nil || ix.Files[rel] == nil {
		return nil
	}
	var out []Symbol
	for _, s := range ix.Files[rel].Symbols {
		if start < s.Start && s.Start <= end {
			out = append(out, Symbol{s.Name, s.Start})
		}
	}
	slices.SortFunc(out, func(a, b Symbol) int { return cmp.Compare(a.Line, b.Line) })
	return out
}

// Symbol is a name and the line it starts on.
type Symbol struct {
	Name string
	Line int
}

// ------------------------------------------------- placing a task's words
//
// A task is not a query: what a person says about a bug names the
// behaviour, and the code may call none of it that. So this is deliberately
// shallow and deliberately loud — the words are matched against what the
// estate defines, the matches offered as places to start, and every word
// that placed nothing is named. Choosing among the places means
// understanding the sentence, which is whoever asked's work, not this.

var stopwords = map[string]bool{}

func init() {
	for w := range strings.FieldsSeq(`the a an and or not but if then when where this that these those it its is are was were be been being to of in on for with from by as at into about after before should must can will would we i you they he she our my fix bug issue problem add make change update support feature need needs want please instead does do did doing have has had get gets set new old use used using so now also only some all any every there here out up down over`) {
		stopwords[w] = true
	}
}

var (
	quoted    = regexp.MustCompile("\"([^\"]+)\"|`([^`]+)`")
	pathShape = regexp.MustCompile(`[A-Za-z_][\w./-]*[./][\w./-]+`)
	plainWord = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// wordsOf is the words of a task that could name something: anything
// quoted or path-shaped, then every word that is not grammar, longest first.
func wordsOf(statement string) []string {
	strong := map[string]bool{}
	for _, m := range quoted.FindAllStringSubmatch(statement, -1) {
		strong[m[1]+m[2]] = true
	}
	for _, m := range pathShape.FindAllString(statement, -1) {
		strong[m] = true
	}
	plain := map[string]bool{}
	for _, w := range plainWord.FindAllString(statement, -1) {
		if len(w) >= 3 && !stopwords[strings.ToLower(w)] && !strong[w] {
			plain[w] = true
		}
	}
	byLength := func(m map[string]bool) []string {
		out := sortedSet(m)
		slices.SortStableFunc(out, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
		return out
	}
	return append(byLength(strong), byLength(plain)...)
}

func stemOf(word string) string {
	low := strings.ToLower(word)
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(low, suffix) && len(low)-len(suffix) >= 3 {
			return low[:len(low)-len(suffix)]
		}
	}
	return low
}

// splitName is a symbol as the words it is made of — CamelCase, snake_case,
// kebab-case — where an acronym before a word is a word of its own:
// HTTPServer is HTTP and Server.
func splitName(name string) []string {
	upper := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	lower := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }
	var parts []string
	for i := 0; i < len(name); {
		j := i
		switch {
		case upper(name[i]):
			for j < len(name) && upper(name[j]) {
				j++
			}
			if j < len(name) && lower(name[j]) {
				if j-i > 1 {
					j-- // the last capital starts the next word
				} else {
					for j < len(name) && lower(name[j]) {
						j++
					}
				}
			}
		case lower(name[i]):
			for j < len(name) && lower(name[j]) {
				j++
			}
		default:
			i++
			continue
		}
		parts = append(parts, name[i:j])
		i = j
	}
	var out []string
	for _, p := range parts {
		if len(p) >= 3 {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// Landing is where one word of a task lands, and how.
type Landing struct {
	Word   string
	How    string
	Places []Place
	Names  []string
	More   int
}

// Survey is everywhere the words of a task land, and the words that land
// nowhere — which mean either that the code calls them something else or
// that the ground they name is not read here.
func (r *Repo) Survey(statement string, limit int) (found []Landing, missing []string, err error) {
	ix, err := r.Index()
	if err != nil {
		return nil, nil, err
	}
	byWord := map[string]map[string]bool{}
	for _, name := range ix.Names {
		for _, part := range splitName(name) {
			if byWord[part] == nil {
				byWord[part] = map[string]bool{}
			}
			byWord[part][name] = true
		}
	}
	files := sortedKeys(ix.Files)
	for _, w := range wordsOf(statement) {
		low := strings.ToLower(w)
		var exact []string
		for _, n := range ix.Names {
			if strings.ToLower(n) == low {
				exact = append(exact, n)
			}
		}
		var hit Landing
		switch {
		case len(exact) > 0:
			hit = Landing{Word: w, How: "is the name of", Names: exact}
		case len(byWord[low]) > 0:
			hit = Landing{Word: w, How: "is part of the name of", Names: sortedSet(byWord[low])}
		default:
			var inPath []string
			for _, f := range files {
				if strings.Contains(strings.ToLower(f), low) {
					inPath = append(inPath, f)
				}
			}
			if len(inPath) > 0 {
				hit = Landing{Word: w, How: "is part of the path of"}
				for _, f := range inPath {
					hit.Places = append(hit.Places, Place{f, 1, 1})
				}
			} else if s := stemOf(low); s != low && len(byWord[s]) > 0 {
				hit = Landing{Word: w, How: "shares a stem with the name of", Names: sortedSet(byWord[s])}
			} else {
				missing = append(missing, w)
				continue
			}
		}
		for _, n := range hit.Names {
			places, _ := r.WhereDefined(n)
			hit.Places = append(hit.Places, places...)
		}
		if len(hit.Places) > limit {
			hit.More, hit.Places = len(hit.Places)-limit, hit.Places[:limit]
		}
		found = append(found, hit)
	}
	return found, missing, nil
}

func region(rel string) string { return path.Dir(rel) }
