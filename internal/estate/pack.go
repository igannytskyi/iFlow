package estate

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
)

// Tokens is about how many tokens a model reads text as: four bytes to a
// token, which is close for code and English and errs high for neither.
func Tokens(s string) int { return (len(s) + 3) / 4 }

// Section is one part of what an agent is handed, under a title saying what
// it is for.
type Section struct {
	Title string
	Body  string
}

// Pack is what one agent is handed for one piece of work: the least of the
// estate that work needs, and nothing it can find for itself is left for it
// to look for, since it cannot look.
type Pack struct {
	Sections []Section
	// Left is what did not fit the budget, by section title, so that a pack
	// cut short says so.
	Left []string
}

// String is the pack as it is handed over.
func (p Pack) String() string {
	var b strings.Builder
	for i, s := range p.Sections {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# %s\n\n%s\n", s.Title, strings.TrimRight(s.Body, "\n"))
	}
	if len(p.Left) > 0 {
		fmt.Fprintf(&b, "\n# Left out to keep this short\n\n%s\n", strings.Join(p.Left, "\n"))
	}
	return b.String()
}

// Held is the pack without the files the agent may change: what stays
// the same in every turn, the files it changes being sent as they stand.
func (p Pack) Held() Pack {
	var out Pack
	out.Left = p.Left
	for _, s := range p.Sections {
		if !strings.HasPrefix(s.Title, "May change: ") {
			out.Sections = append(out.Sections, s)
		}
	}
	return out
}

// Tokens is what the pack weighs.
func (p Pack) Tokens() int { return Tokens(p.String()) }

// Map is the estate as a list: every source file and the names it defines,
// with none of their bodies — enough to say where a change belongs, at a
// fraction of reading the code. Past budget tokens, names give way to paths.
func (r *Repo) Map(budget int) string {
	ix, err := r.Index()
	sources := r.Sources()
	line := func(rel string, names int) string {
		if err != nil || ix.Files[rel] == nil || names == 0 {
			return rel
		}
		var ns []string
		for _, s := range ix.Files[rel].Symbols {
			if !slices.Contains(ns, s.Name) {
				ns = append(ns, s.Name)
			}
		}
		if len(ns) > names {
			ns = append(ns[:names], fmt.Sprintf("… %d more", len(ns)-names))
		}
		if len(ns) == 0 {
			return rel
		}
		return rel + ": " + strings.Join(ns, ", ")
	}
	for _, names := range []int{24, 8, 0} {
		var b strings.Builder
		for _, s := range sources {
			b.WriteString(line(s, names) + "\n")
		}
		if Tokens(b.String()) <= budget || names == 0 {
			return b.String()
		}
	}
	return ""
}

// PackFor gathers what an agent needs to change the files in change, having
// read the files in read: those files whole, or their outline where whole
// would not fit; the signatures of what they use from elsewhere; who uses
// them; and what sits beside them. Within budget tokens, the parts further
// from the work are the first to go.
func (r *Repo) PackFor(change, read []string, budget int) Pack {
	var whole []Section
	for _, f := range change {
		whole = append(whole, r.fileSection("May change: "+f, f, "It does not exist yet; you create it."))
	}
	for _, f := range read {
		if !slices.Contains(change, f) {
			whole = append(whole, r.fileSection("Read: "+f, f, "It does not exist."))
		}
	}
	// What the work touches, whole, is what the pack is for: it stays, and
	// only a file too long for what is left is reduced to its outline.
	var p Pack
	left := budget
	for _, s := range whole {
		// A file to change is shown whole whenever it fits the whole budget
		// — an agent edits best what it sees — and only one longer than an
		// agent is handed at all is reduced to its outline, to be read by
		// parts; a file only read is reduced as soon as it would crowd out
		// what is left.
		bound := left * 2 / 3
		if strings.HasPrefix(s.Title, "May change: ") {
			bound = budget
		}
		if Tokens(s.Body) > bound {
			rel := strings.TrimPrefix(strings.TrimPrefix(s.Title, "May change: "), "Read: ")
			s.Body = r.outline(rel) + "\n(too long to show whole here: its outline, each part with the line it starts on)"
		}
		p.Sections = append(p.Sections, s)
		left -= Tokens(s.Body)
	}
	files := append(slices.Clone(change), read...)
	for _, s := range []Section{r.usesSection(files), r.usedBySection(change), r.besideSection(files)} {
		if s.Body == "" {
			continue
		}
		if Tokens(s.Body) > left {
			p.Left = append(p.Left, s.Title)
			continue
		}
		p.Sections = append(p.Sections, s)
		left -= Tokens(s.Body)
	}
	return p
}

func (r *Repo) fileSection(title, rel, absent string) Section {
	b, err := os.ReadFile(r.abs(rel))
	if err != nil {
		return Section{title, absent}
	}
	return Section{title, "```\n" + string(b) + "```"}
}

func (r *Repo) outline(rel string) string {
	var b strings.Builder
	for _, s := range r.Outline(rel, 0, 1<<30) {
		fmt.Fprintf(&b, "%d: %s\n", s.Line, s.Name)
	}
	return b.String()
}

// usesSection is the first line of every definition elsewhere that the files
// call: a signature to call it by, without the body.
func (r *Repo) usesSection(files []string) Section {
	seen := map[string]bool{}
	var lines []string
	for _, f := range files {
		callees, err := r.Callees(f, 0, 1<<30)
		if err != nil {
			continue
		}
		for _, c := range callees {
			if slices.Contains(files, c.File) || c.Confidence == "low" || seen[c.File+c.Name] {
				continue
			}
			seen[c.File+c.Name] = true
			places, _ := r.WhereDefined(c.Name)
			for _, pl := range places {
				if pl.File != c.File {
					continue
				}
				// A Go type is shown whole — a struct with its fields, an
				// interface with its methods: its first line says nothing of
				// what it holds, and an agent that guesses it guesses wrong.
				if strings.HasSuffix(pl.File, ".go") {
					if t := goType(r.abs(pl.File), c.Name); t != "" && strings.Count(t, "\n") < 80 {
						lines = append(lines, fmt.Sprintf("%s:%d\n%s", pl.File, pl.Start, t))
						continue
					}
				}
				if ex := r.Excerpt(pl.File, pl.Start, pl.Start, 1); len(ex) == 1 {
					lines = append(lines, fmt.Sprintf("%s:%d  %s", pl.File, pl.Start, strings.TrimSpace(ex[0].Text)))
				}
			}
		}
	}
	slices.Sort(lines)
	return Section{"Used from elsewhere — what to call it by", strings.Join(lines, "\n")}
}

// usedBySection is every file that reaches the files changed, firmest first:
// what a change to them could break.
func (r *Repo) usedBySection(change []string) Section {
	var have []string
	for _, f := range change {
		if exists(r.abs(f)) {
			have = append(have, f)
		}
	}
	if len(have) == 0 {
		return Section{}
	}
	reach, err := r.Affects(have)
	if err != nil {
		return Section{}
	}
	slices.SortStableFunc(reach, func(a, b Reached) int { return cmp.Compare(rank[a.Confidence], rank[b.Confidence]) })
	var lines []string
	for _, x := range reach {
		if !slices.Contains(change, x.File) && x.Confidence != "low" {
			lines = append(lines, x.File)
		}
		if len(lines) == 20 {
			lines = append(lines, fmt.Sprintf("… and %d more", len(reach)-20))
			break
		}
	}
	return Section{"Uses what you change — keep it working", strings.Join(lines, "\n")}
}

// besideSection is the other files of the same directories and what each
// defines: the package a new file joins and the names already taken there.
func (r *Repo) besideSection(files []string) Section {
	dirs := map[string]bool{}
	for _, f := range files {
		dirs[path.Dir(f)] = true
	}
	ix, err := r.Index()
	var lines []string
	for _, s := range r.Sources() {
		if !dirs[path.Dir(s)] || slices.Contains(files, s) {
			continue
		}
		// Each name by the first line of its definition: a signature an
		// agent can call, not a name it must guess the arguments of.
		lines = append(lines, s+":")
		if err == nil && ix.Files[s] != nil {
			seen := map[int]bool{}
			for _, sym := range ix.Files[s].Symbols {
				if seen[sym.Start] {
					continue
				}
				seen[sym.Start] = true
				if ex := r.Excerpt(s, sym.Start, sym.Start, 1); len(ex) == 1 {
					lines = append(lines, "  "+strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(ex[0].Text), "{")))
				}
			}
		}
	}
	return Section{"Beside it, in the same directory — every definition by its first line", strings.Join(lines, "\n")}
}

// Signatures is what exists where a change works — every file of the
// directories the paths name, its tests and their helpers among them, each
// definition by its first line — within budget tokens. A plan built on it
// relies on what is there, not on what its author assumes is.
func (r *Repo) Signatures(paths []string, budget int) string {
	dirs := map[string]bool{}
	for _, f := range paths {
		dirs[path.Dir(f)] = true
	}
	return r.declared(func(s string) bool { return dirs[path.Dir(s)] }, budget)
}

// Declarations is every definition in the files named, by its first line,
// within budget tokens: how the code is called, without its bodies.
func (r *Repo) Declarations(files []string, budget int) string {
	return r.declared(func(s string) bool { return slices.Contains(files, s) }, budget)
}

// declared is every definition of the sources that is chosen, by its first
// line, within budget tokens.
func (r *Repo) declared(is func(string) bool, budget int) string {
	ix, err := r.Index()
	var b strings.Builder
	for _, s := range r.Sources() {
		if !is(s) {
			continue
		}
		b.WriteString(s + ":\n")
		if err != nil || ix.Files[s] == nil {
			continue
		}
		seen := map[int]bool{}
		for _, sym := range ix.Files[s].Symbols {
			if seen[sym.Start] {
				continue
			}
			seen[sym.Start] = true
			if ex := r.Excerpt(s, sym.Start, sym.Start, 1); len(ex) == 1 {
				b.WriteString("  " + Signature(ex[0].Text) + "\n")
			}
		}
		if Tokens(b.String()) > budget {
			b.WriteString("[cut: more than the budget]\n")
			break
		}
	}
	return b.String()
}

// Signature is a definition's first line without its body: a one-line
// function shows how it is called, not what it does.
func Signature(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, " {"); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(strings.TrimSuffix(line, "{"))
}
