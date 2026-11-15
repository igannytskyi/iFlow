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

	"github.com/igannytskyi/iflow/internal/artifact"
)

// A contract is what one repository offers and another consumes: a route, an
// event. Finding both ends is the part nobody ships, and the reason a change
// in one repository breaks another with nothing in either saying so. The
// extraction is small and textual — a declaration that looks like a route,
// graded as such — and what is built here is the join.

type pattern struct {
	re   *regexp.Regexp
	kind string
}

func pats(kind string, exprs ...string) []pattern {
	out := make([]pattern, len(exprs))
	for i, e := range exprs {
		out[i] = pattern{regexp.MustCompile(e), kind}
	}
	return out
}

var (
	declaresPy = append(pats("http", `@\w+\.(?:route|get|post|put|patch|delete)\(\s*["']([^"']+)["']`),
		pats("event", `\bpublish(?:es)?\(\s*["']([^"']+)["']`)...)
	// A use is one only where what is passed looks like an address; get( on
	// its own is a dictionary or an environment lookup. Not preceded by @,
	// which is a declaration.
	usesPy = append(pats("http", `\b\w*\.?(?:get|post|put|patch|delete|request)\(\s*["']((?:[a-z]+:)?//[^"']+|/[^"']*)["']`),
		pats("event", `\bsubscribe(?:s)?\(\s*["']([^"']+)["']`)...)
	fragmentsPy = pats("http", `\b(?:re_path|path|url)\(\s*r?["']\^?([^"']+)["']`)
	usesText    = pats("http",
		`(?:GetFromJsonAsync|PostAsJsonAsync|PutAsJsonAsync|PatchAsJsonAsync|GetStringAsync|GetAsync|PostAsync|PutAsync|DeleteAsync)\s*(?:<[^>]*>)?\s*\(\s*\$?"((?:[a-z]+:)?//[^"]+|/[^"]*)"`,
		`\b\w*(?:BaseUrl|BaseAddress|baseUrl|Endpoint|endpoint)\w*\s*=\s*\$?"((?:[a-z]+:)?//[^"]+|/?[a-zA-Z][\w-]*(?:/[^"]*)+)"`,
		"\\bfetch\\(\\s*[`\"']((?:[a-z]+:)?//[^`\"']+|/[^`\"']*)",
		"\\baxios\\.(?:get|post|put|patch|delete)\\(\\s*[`\"']((?:[a-z]+:)?//[^`\"']+|/[^`\"']*)",
		`\bhttp\.(?:Get|Post|NewRequest)\([^)]*?"((?:[a-z]+:)?//[^"]+|/[^"]*)"`)
	basePattern  = 1 // the entry above that reads a base kept in a field
	declaresText = pats("http",
		`@(?:Request|Get|Post|Put|Patch|Delete)Mapping\(\s*(?:value\s*=\s*)?"([^"]+)"`,
		`@Path\(\s*"([^"]+)"\s*\)`,
		`\[(?:HttpGet|HttpPost|HttpPut|HttpDelete|Route)\(\s*"([^"]+)"`,
		`\.Map(?:Get|Post|Put|Patch|Delete|Forwarder)\(\s*"([^"]+)"`,
		`\.(?:HandleFunc|Handle|Path)\(\s*(?:[\w.()]+\s*\+\s*)?"(/[^"]*)"`,
		`\b(?:r|router|app|e|g)\.(?:GET|POST|PUT|PATCH|DELETE)\(\s*(?:[\w.()]+\s*\+\s*)?"(/[^"]*)"`,
		`\b(?:app|router)\.(?:get|post|put|patch|delete|all)\(\s*["'](/[^"']*)["']`)
	groups = []*regexp.Regexp{
		regexp.MustCompile(`\.MapGroup\(\s*"([^"]+)"`),
		regexp.MustCompile(`\bAPIRouter\(\s*prefix\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`\bBlueprint\([^)]*url_prefix\s*=\s*["']([^"']+)["']`),
	}
	forwarder  = regexp.MustCompile(`\.MapForwarder\(\s*"([^"]+)"\s*,\s*"([^"]+)"\s*,\s*"([^"]+)"`)
	degenerate = map[string]bool{"/": true, "": true, "/*": true, "/health": true, "/healthz": true, "/ping": true, "/metrics": true}
	urlRE      = regexp.MustCompile(`^(?:[a-z]+:)?//([^/]+)(/.*)$`)
	paramRE    = regexp.MustCompile(`\{[^/}]*\}|<[^/>]*>|\([^/)]*\)|:[A-Za-z_][A-Za-z0-9_]*|%[sd]|\$\{[^/}]*\}`)
	// Ways one service reaches another that this does not read. Naming them
	// matters more than the edges it finds: an estate bound by gRPC and
	// reported as unconnected is misdescribed.
	otherKinds = []struct {
		re   *regexp.Regexp
		name string
	}{
		{regexp.MustCompile(`\.proto\b`), "protocol buffers"},
		{regexp.MustCompile(`\b\w+ServiceStub\b|grpc\.`), "gRPC"},
		{regexp.MustCompile(`\b[A-Z_]+_SERVICE_ADDR\b`), "service addresses from the environment"},
		{regexp.MustCompile(`\bboto3\b|\bSQS\b|\bkafka\b|\bKafkaProducer\b`), "queues and brokers"},
	}
)

// findAll is every first group of a pattern in text, where a match is not
// preceded by notAfter (0 for none).
func findAll(re *regexp.Regexp, text string, notAfter byte) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if notAfter != 0 && m[0] > 0 && text[m[0]-1] == notAfter {
			continue
		}
		out = append(out, text[m[2]:m[3]])
	}
	return out
}

// normalise turns a key as written into the contract it names: a consumer
// writes a URL and a producer a path; parameters are spelled five ways; a
// router ignores case and a trailing slash.
func normalise(kind, key string) (string, string) {
	if kind != "http" {
		return key, ""
	}
	if !strings.Contains(key, "/") || strings.HasPrefix(strings.TrimLeft(key, " "), ".") {
		return "", "" // a dotted module path, not a route
	}
	host := ""
	if m := urlRE.FindStringSubmatch(key); m != nil {
		host, key = m[1], m[2]
	}
	key = paramRE.ReplaceAllString(key, "{}")
	key, _, _ = strings.Cut(key, "?")
	key = strings.ToLower(key)
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	if len(key) > 1 {
		key = strings.TrimRight(strings.TrimRight(key, "*"), "/")
		if key == "" {
			key = "/"
		}
	}
	return key, host
}

// Offer is a contract declared, and a Use one consumed.
type (
	Offer struct {
		Kind, Key, File string
	}
	Use struct {
		Kind, Key, File, Host string
		Base, InTest          bool
	}
)

// Facts is what one repository offers and consumes.
type Facts struct {
	Repo          string
	Declares      []Offer
	Uses          []Use
	StandsIn      []Offer // routes declared in its own tests: stand-ins for others
	Fragments     []Offer // route fragments joined at import time, unread
	Illustrations int
}

// Facts reads them.
func (r *Repo) Facts() Facts {
	f := Facts{Repo: filepath.Base(r.Root)}
	files := append(r.Sources(), r.unreadSources()...)
	slices.Sort(files)
	files = slices.Compact(files)
	for _, rel := range files {
		if illustration(rel) {
			f.Illustrations++
			continue
		}
		b, err := os.ReadFile(r.abs(rel))
		if err != nil {
			continue
		}
		text := string(b)
		inTest := r.IsTest(rel)
		if path.Ext(rel) == ".py" {
			decorated := map[string]bool{}
			for line := range strings.SplitSeq(text, "\n") {
				if strings.HasPrefix(strings.TrimLeft(line, " \t"), "@") {
					decorated[strings.TrimSpace(line)] = true
				}
			}
			for _, p := range fragmentsPy {
				for _, k := range findAll(p.re, text, 0) {
					if key, _ := normalise(p.kind, k); key != "" && !degenerate[key] {
						f.Fragments = append(f.Fragments, Offer{p.kind, key, rel})
					}
				}
			}
			for _, p := range declaresPy {
				for _, k := range findAll(p.re, text, 0) {
					if key, _ := normalise(p.kind, k); key != "" && !degenerate[key] {
						if inTest {
							f.StandsIn = append(f.StandsIn, Offer{p.kind, key, rel})
						} else {
							f.Declares = append(f.Declares, Offer{p.kind, key, rel})
						}
					}
				}
			}
			for line := range strings.SplitSeq(text, "\n") {
				if decorated[strings.TrimSpace(line)] {
					continue
				}
				for _, p := range usesPy {
					for _, k := range findAll(p.re, line, '@') {
						if key, host := normalise(p.kind, k); key != "" && !degenerate[key] {
							f.Uses = append(f.Uses, Use{p.kind, key, rel, host, strings.HasSuffix(strings.TrimRight(k, `"'`), "/"), inTest})
						}
					}
				}
			}
		}
		for n, p := range usesText {
			for line := range strings.SplitSeq(text, "\n") {
				for _, k := range findAll(p.re, line, 0) {
					if key, host := normalise(p.kind, k); key != "" && !degenerate[key] {
						f.Uses = append(f.Uses, Use{p.kind, key, rel, host, n == basePattern || strings.HasSuffix(k, "/"), inTest})
					}
				}
			}
		}
		for _, m := range forwarder.FindAllStringSubmatch(text, -1) {
			if key, _ := normalise("http", m[3]); key != "" && !degenerate[key] {
				host := m[2]
				if i := strings.LastIndex(host, "//"); i >= 0 {
					host = host[i+2:]
				}
				f.Uses = append(f.Uses, Use{"http", key, rel, host, false, false})
			}
		}
		prefixes := map[string]bool{}
		for _, g := range groups {
			for _, m := range g.FindAllStringSubmatch(text, -1) {
				prefixes[m[1]] = true
			}
		}
		for _, p := range declaresText {
			for _, k := range findAll(p.re, text, 0) {
				fulls := []string{k}
				if len(prefixes) > 0 && !strings.HasPrefix(k, "http") && !strings.HasPrefix(k, "//") {
					fulls = nil
					for pre := range prefixes {
						fulls = append(fulls, strings.TrimRight(pre, "/")+"/"+strings.TrimLeft(k, "/"))
					}
				}
				for _, full := range fulls {
					key, _ := normalise(p.kind, full)
					if key == "" || degenerate[key] {
						continue
					}
					if inTest {
						f.StandsIn = append(f.StandsIn, Offer{p.kind, key, rel})
					} else {
						f.Declares = append(f.Declares, Offer{p.kind, key, rel})
					}
				}
			}
		}
	}
	return f
}

// covers is whether an offered route covers a call the way a router would:
// the route's parameters match anything, the call's literals only
// themselves, and a call shorter than the route only where it is a base.
func covers(offered, consumed string, base bool) bool {
	split := func(s string) []string {
		return slices.DeleteFunc(strings.Split(strings.Trim(s, "/"), "/"), func(p string) bool { return p == "" })
	}
	o, c := split(offered), split(consumed)
	if !slices.ContainsFunc(o, func(s string) bool { return s != "{}" }) {
		return false // a route of nothing but parameters matches everything
	}
	if len(o) <= len(c) {
		for i := range o {
			if o[i] != c[i] && o[i] != "{}" {
				return false
			}
		}
		if len(o) >= 2 {
			return true
		}
		return !slices.ContainsFunc(c[len(o):], func(s string) bool { return s != "{}" })
	}
	if !base || len(c) < 2 {
		return false
	}
	for i := range c {
		if c[i] != o[i] {
			return false
		}
	}
	return true
}

// Edge is one contract joined across repositories.
type Edge struct {
	From, To, Kind, Key string
	Among               []string
	Provenance          string
	Confidence          string
	How                 string
}

// Contracts is what crosses between the repositories under one directory.
type Contracts struct {
	Edges       []Edge
	Unconsumed  []Offer // offered and taken by nobody here
	OfferedBy   map[Offer]string
	Unmatched   []Use // consumed and offered by nobody here
	Consumers   map[*Use]string
	Fakes       []Offer
	FakeOf      map[Offer]string
	FakeTo      map[Offer]string
	Internal    int
	Loose       []Offer
	Shown       int
	Other       map[string]int
	UnmatchedBy []string // consumer repository of each unmatched use
	UnseenCount int
}

// ContractsUnder joins every repository directly under dir.
func ContractsUnder(dir string) Contracts {
	entries, _ := os.ReadDir(dir)
	var facts []Facts
	for _, e := range entries {
		if e.IsDir() {
			facts = append(facts, Open(filepath.Join(dir, e.Name())).Facts())
		}
	}
	type key struct{ kind, key string }
	offered := map[key][]Offer{}
	offeredRepo := map[key][]string{}
	var order []key
	for _, f := range facts {
		for _, d := range f.Declares {
			k := key{d.Kind, d.Key}
			if _, ok := offered[k]; !ok {
				order = append(order, k)
			}
			offered[k] = append(offered[k], d)
			offeredRepo[k] = append(offeredRepo[k], f.Repo)
		}
	}
	repos := func(k key, not string) []string {
		set := map[string]bool{}
		for _, r := range offeredRepo[k] {
			if r != not {
				set[r] = true
			}
		}
		return sortedSet(set)
	}
	c := Contracts{OfferedBy: map[Offer]string{}, FakeOf: map[Offer]string{}, FakeTo: map[Offer]string{}}
	consumed := map[key]bool{}
	for _, f := range facts {
		for _, u := range f.Uses {
			k := key{u.Kind, u.Key}
			producers := repos(k, f.Repo)
			near, many := "", 0
			if len(producers) == 0 {
				for _, ok := range order {
					if ok.kind != u.Kind || !covers(ok.key, u.Key, u.Base) {
						continue
					}
					if reps := repos(ok, f.Repo); len(reps) > 0 {
						many++
						if near == "" || len(ok.key) < len(near) {
							near, producers = ok.key, reps
						}
					}
				}
			}
			if len(producers) == 0 {
				own := slices.Contains(offeredRepo[k], f.Repo)
				for _, ok := range order {
					if ok.kind == u.Kind && covers(ok.key, u.Key, u.Base) && slices.Contains(offeredRepo[ok], f.Repo) {
						own = true
					}
				}
				if own {
					c.Internal++ // a call to a route it offers itself crosses nothing
					continue
				}
				c.Unmatched = append(c.Unmatched, u)
				c.UnmatchedBy = append(c.UnmatchedBy, f.Repo)
				continue
			}
			if near == "" {
				consumed[k] = true
			} else {
				consumed[key{u.Kind, near}] = true
			}
			conf, how := "medium", "one repository offers this key"
			switch {
			case near != "" && many > 1:
				conf, how = "low", fmt.Sprintf("no repository offers this key; %s offers %d route(s) that cover it, %s among them", producers[0], many, near)
			case near != "":
				conf, how = "low", "no repository offers this key; the route "+near+" covers it"
			case len(producers) > 1:
				conf, how = "low", "this key is offered by "+strings.Join(producers, ", ")+", and which one it reaches is not derivable here"
			case u.Host != "" && u.Host == producers[0]:
				how = "the key matches and the address names the same repository"
			}
			if u.InTest {
				conf = "low"
				how += "; the call is made from the consumer's own test ground"
			}
			c.Edges = append(c.Edges, Edge{f.Repo, producers[0], u.Kind, u.Key, producers, "matched", conf, how})
		}
	}
	// A stand-in is evidence of the reverse: nobody writes a fake of a
	// service they do not call.
	seenFake := map[[3]string]bool{}
	for _, f := range facts {
		for _, d := range f.StandsIn {
			k := key{d.Kind, d.Key}
			if seenFake[[3]string{f.Repo, d.Kind, d.Key}] {
				continue
			}
			seenFake[[3]string{f.Repo, d.Kind, d.Key}] = true
			producers := repos(k, f.Repo)
			c.Fakes = append(c.Fakes, d)
			c.FakeOf[d] = f.Repo
			if len(producers) > 0 {
				c.FakeTo[d] = producers[0]
				consumed[k] = true
				c.Edges = append(c.Edges, Edge{f.Repo, producers[0], d.Kind, d.Key, producers, "matched", "low", "this repository keeps a stand-in for it in its own tests"})
			}
		}
	}
	for _, k := range order {
		if !consumed[k] {
			o := offered[k][0]
			c.Unconsumed = append(c.Unconsumed, o)
			c.OfferedBy[o] = offeredRepo[k][0]
		}
	}
	best := map[[4]string]Edge{}
	var edgeOrder [][4]string
	for _, e := range c.Edges {
		k := [4]string{e.From, e.To, e.Kind, e.Key}
		was, ok := best[k]
		if !ok {
			edgeOrder = append(edgeOrder, k)
		}
		if !ok || rank[e.Confidence] < rank[was.Confidence] {
			best[k] = e
		}
	}
	c.Edges = nil
	for _, k := range edgeOrder {
		c.Edges = append(c.Edges, best[k])
	}
	once := map[[3]string]bool{}
	var um []Use
	var umBy []string
	for i, u := range c.Unmatched {
		k := [3]string{c.UnmatchedBy[i], u.Kind, u.Key}
		if !once[k] {
			once[k] = true
			um, umBy = append(um, u), append(umBy, c.UnmatchedBy[i])
		}
	}
	c.Unmatched, c.UnmatchedBy = um, umBy
	for _, f := range facts {
		c.Loose = append(c.Loose, f.Fragments...)
		c.Shown += f.Illustrations
	}
	c.Other = otherKindsUnder(dir)
	for _, n := range Open(dir).Unseen() {
		c.UnseenCount += n
	}
	return c
}

func otherKindsUnder(dir string) map[string]int {
	found := map[string]int{}
	r := Open(dir)
	for _, f := range r.all() {
		switch path.Ext(f.rel) {
		case ".png", ".jpg", ".pdf", ".lock":
			continue
		}
		b, err := os.ReadFile(f.abs)
		if err != nil {
			continue
		}
		for _, k := range otherKinds {
			if k.re.Match(b) {
				found[k.name]++
			}
		}
	}
	return found
}

// OfferRow is one offer of a region, who takes it, and who is beyond reach.
type OfferRow struct {
	Repo, Kind, Key string
	TakenBy         []string
	AlsoOfferedBy   []string
}

// Reachability is who consumes what one repository under dir offers.
func Reachability(dir, region string) ([]OfferRow, Contracts) {
	c := ContractsUnder(dir)
	var out []OfferRow
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || !(e.Name() == region || strings.HasSuffix(filepath.Join(dir, e.Name()), region)) {
			continue
		}
		seen := map[[2]string]bool{}
		for _, d := range Open(filepath.Join(dir, e.Name())).Facts().Declares {
			k := [2]string{d.Kind, d.Key}
			if seen[k] {
				continue
			}
			seen[k] = true
			takers, shared := map[string]bool{}, map[string]bool{}
			for _, edge := range c.Edges {
				if edge.Kind != d.Kind || edge.Key != d.Key {
					continue
				}
				if slices.Contains(edge.Among, e.Name()) {
					takers[edge.From] = true
				}
				if len(edge.Among) > 1 {
					for _, o := range edge.Among {
						if o != e.Name() {
							shared[o] = true
						}
					}
				}
			}
			out = append(out, OfferRow{e.Name(), d.Kind, d.Key, sortedSet(takers), sortedSet(shared)})
		}
	}
	return out, c
}

// Unit is a unit planned and not entered, and the ground its plan says it
// holds — declared, not derived.
type Unit struct {
	Change, ID string
	Holds      []string
}

// Unfinished is every unit under a directory of changes that has been
// planned and has not entered.
func Unfinished(changes string) []Unit {
	entries, _ := os.ReadDir(changes)
	var out []Unit
	for _, e := range entries {
		dir := filepath.Join(changes, e.Name())
		if !e.IsDir() {
			continue
		}
		c, err := artifact.Load(dir)
		if err != nil || !c.Has(artifact.Plan) {
			continue
		}
		landed := map[string]bool{}
		for _, row := range c.Doc(artifact.Landing).Rows("Unit", "Entered at") {
			if row.Cell("Entered at") != "" {
				landed[row.Cell("Unit")] = true
			}
		}
		for _, row := range c.Doc(artifact.Plan).Rows("Scope", "Id") {
			if id := row.Cell("Id"); strings.HasPrefix(id, "WU-") && !landed[id] {
				var holds []string
				for p := range strings.FieldsFuncSeq(row.Cell("Scope"), func(r rune) bool { return r == ',' || r == ' ' }) {
					holds = append(holds, p)
				}
				out = append(out, Unit{e.Name(), id, holds})
			}
		}
	}
	return out
}

// Conflict is unfinished work holding ground a change would touch.
type Conflict struct {
	Unit
	On         []string
	Confidence string
	How        string
}

// Conflicts is who already holds these paths, and on what footing: the same
// region declared by both is a warning, what a change reaches running into
// what a unit holds is a fact.
func (r *Repo) Conflicts(changes string, paths []string) ([]Conflict, error) {
	wanted := wants(paths)
	reach, err := r.Affects(paths)
	if err != nil {
		return nil, err
	}
	named, err := r.NamedBy(paths)
	if err != nil {
		return nil, err
	}
	reached := map[string]bool{}
	for _, x := range slices.Concat(reach, named) {
		reached[x.File] = true
	}
	var out []Conflict
	for _, u := range Unfinished(changes) {
		direct, indirect := map[string]bool{}, map[string]bool{}
		for _, h := range u.Holds {
			for w := range wanted {
				if h == w || strings.HasPrefix(w, h) || strings.HasPrefix(h, w) {
					direct[h] = true
				}
			}
			for f := range reached {
				if f == h || strings.HasPrefix(f, h) {
					indirect[h] = true
				}
			}
		}
		switch {
		case len(direct) > 0:
			out = append(out, Conflict{u, sortedSet(direct), "low", "both declare the same region, and a declaration is what someone wrote down"})
		case len(indirect) > 0:
			out = append(out, Conflict{u, sortedSet(indirect), "medium", "what this change reaches runs into what that unit holds"})
		}
	}
	slices.SortStableFunc(out, func(a, b Conflict) int { return cmp.Compare(rank[a.Confidence], rank[b.Confidence]) })
	return out, nil
}
