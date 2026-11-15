package reader

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// generic reads every language but Python with the grammar its own community
// maintains.
//
// A grammar's tags query is not enough on its own — TypeScript's covers only
// what TypeScript adds, Kotlin ships none — so what is read is what grammars
// share: the names they give their nodes. A function declaration is called
// something containing "function" in every one of them, a call something
// containing "call" or "invocation", and what a node names sits in a field
// called "name". The tags query, where there is one, is asked as well.
type generic struct{}

func (generic) Name() string { return "tree-sitter" }

func (generic) Extensions() []string {
	out := make([]string, 0, len(grammars))
	for ext := range grammars {
		out = append(out, ext)
	}
	return out
}

var (
	definesKinds   = []string{"function", "method", "class", "struct", "interface", "trait", "enum", "module", "constructor", "record", "protocol", "object_declaration", "type_alias", "namespace_definition", "package_declaration"}
	notDefines     = []string{"call", "invocation", "expression", "type", "parameter", "argument"}
	callKinds      = []string{"call", "invocation", "object_creation", "new_expression", "command"}
	notCalls       = []string{"name", "argument", "element", "sep", "list", "chain", "clause", "body", "parameter", "declarator", "substitution"}
	inheritKinds   = []string{"superclass", "heritage", "extends", "implements", "base_list", "inherit", "super_interfaces", "supertypes", "derive", "impl_item"}
	importKinds    = []string{"import", "use_declaration", "namespace_use_clause", "using_directive", "preproc_include", "include_statement"}
	identSuffixes  = []string{"identifier", "name", "constant", "word"}
	importingCalls = set("require", "require_relative", "load", "include_once", "import", "source", ".", "import-module", "using", "dofile")
	keywords       = set("new", "self", "this", "super", "return", "await", "yield", "typeof", "delete")
	inheritWords   = set("extends", "implements", "impl", "for", "where", "class", "interface", "public", "private", "protected", "abstract", "sealed", "final", "static", "override", "open", "data")
)

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

func containsAny(kind string, parts []string) bool {
	return slices.ContainsFunc(parts, func(p string) bool { return strings.Contains(kind, p) })
}

func hasSuffixAny(kind string, suffixes []string) bool {
	return slices.ContainsFunc(suffixes, func(s string) bool { return strings.HasSuffix(kind, s) })
}

func line(p ts.Point) int { return int(p.Row) + 1 }

// tree is a parsed file and its source.
type tree struct {
	src  []byte
	root *ts.Node
}

func (t tree) text(n *ts.Node) string { return n.Utf8Text(t.src) }

func children(n *ts.Node) []*ts.Node {
	out := make([]*ts.Node, 0, n.ChildCount())
	for i := range n.ChildCount() {
		out = append(out, n.Child(i))
	}
	return out
}

// nameOf is what a node names: the field a grammar sets aside for it, or the
// last identifier under it when the grammar sets none.
func (t tree) nameOf(n *ts.Node) string {
	if got := n.ChildByFieldName("name"); got != nil {
		return t.text(got)
	}
	var last *ts.Node
	for _, c := range children(n) {
		if hasSuffixAny(c.Kind(), identSuffixes) {
			last = c
		}
	}
	if last == nil {
		return ""
	}
	return t.text(last)
}

var memberSep = regexp.MustCompile(`\.|::|->|\\`)

// callee is the name a call reaches, how firmly it is known, and the callee
// as written. A bare name is one thing in the file's own scope; a name behind
// a dot, an arrow or a pair of colons is a member of something this reader
// cannot type, graded as weakly as a Python attribute call.
func (t tree) callee(n *ts.Node) (string, Kind, string) {
	var target *ts.Node
	for _, f := range []string{"function", "method", "constructor", "type", "name"} {
		if target = n.ChildByFieldName(f); target != nil {
			break
		}
	}
	if target == nil {
		for _, c := range children(n) {
			if c.IsNamed() && c.Kind() != "comment" {
				target = c
				break
			}
		}
	}
	if target == nil {
		return "", Name, ""
	}
	raw := strings.TrimSpace(t.text(target))
	if raw == "." || raw == "&" || strings.HasSuffix(target.Kind(), "operator") {
		// A shell dot-sources a file with an operator where other languages
		// write a keyword. What it runs is the next thing along.
		for _, c := range children(n) {
			if c.IsNamed() && c.Id() != target.Id() {
				raw = strings.TrimSpace(t.text(c))
				break
			}
		}
		return strings.Trim(raw, "\"'`"), Name, "."
	}
	through := false
	for _, f := range []string{"object", "receiver", "operand", "instance"} {
		if n.ChildByFieldName(f) != nil {
			through = true
		}
	}
	qualified := through || strings.Contains(raw, ".") || strings.Contains(raw, "::") ||
		strings.Contains(raw, "->") || strings.Contains(raw, "\\")
	if strings.HasPrefix(target.Kind(), "command_name") || n.Kind() == "command" {
		qualified = false // a shell names what it runs; nothing is behind it
	}
	tail := raw
	if n.Kind() != "command" {
		parts := memberSep.Split(raw, -1)
		tail = parts[len(parts)-1]
	}
	tail = strings.Trim(strings.TrimSpace(tail), "\"'`")
	if tail == "" || !(unicode.IsLetter([]rune(tail)[0]) || tail[0] == '_') || keywords[tail] {
		return "", Name, raw
	}
	if qualified {
		return tail, Attribute, raw
	}
	return tail, Name, raw
}

// argument is what an import call was given.
func (t tree) argument(n *ts.Node, callee string) string {
	for _, f := range []string{"arguments", "argument", "value", "name"} {
		if got := n.ChildByFieldName(f); got != nil && strings.TrimSpace(t.text(got)) != callee {
			return t.text(got)
		}
	}
	var rest []*ts.Node
	for _, c := range children(n) {
		if c.IsNamed() && strings.TrimSpace(t.text(c)) != callee {
			rest = append(rest, c)
		}
	}
	if len(rest) == 0 {
		return ""
	}
	return t.text(rest[len(rest)-1])
}

type spelled struct {
	text string
	line int
}

// moduleTexts is every module an import statement names, each on its line:
// one statement can name many, as a grouped Go import does.
func (t tree) moduleTexts(n *ts.Node) []spelled {
	var out []spelled
	stack := []*ts.Node{n}
	for len(stack) > 0 {
		here := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if k := here.Kind(); strings.Contains(k, "string") && !strings.Contains(k, "content") {
			out = append(out, spelled{t.text(here), line(here.StartPosition())})
		} else {
			stack = append(stack, children(here)...)
		}
	}
	if len(out) == 0 {
		return []spelled{{t.text(n), line(n.StartPosition())}}
	}
	return out
}

var (
	importWords = []string{"import", "use", "using", "require_relative", "require", "from", "include", "#include", "source", "Import-Module"}
	pathSeps    = regexp.MustCompile(`[/\\]+`)
	dottedSeps  = regexp.MustCompile(`[.:]+`)
)

// modules is the part of an import that can be matched against a file. A
// path names its file and a dotted module names its last part; both offer
// the whole path too, because a namespace names a place.
func modules(text string) []string {
	text = strings.TrimSpace(strings.Trim(strings.TrimSpace(text), ";"))
	text = strings.TrimSpace(text)
	for _, w := range importWords {
		if strings.HasPrefix(strings.ToLower(text), strings.ToLower(w)+" ") {
			text = text[len(w)+1:]
		}
	}
	text = strings.Trim(strings.TrimSpace(text), "\"'`<>;,()")
	text, _, _ = strings.Cut(text, " as ")
	text, _, _ = strings.Cut(text, "{")
	text = strings.Trim(strings.TrimSpace(text), "\"'`")
	if text == "" {
		return nil
	}
	keep := func(items ...string) []string {
		return slices.DeleteFunc(items, func(s string) bool { return s == "" })
	}
	if strings.ContainsAny(text, "/\\") || strings.HasPrefix(text, ".") || strings.HasPrefix(text, "~") || strings.HasPrefix(text, "$") {
		parts := slices.DeleteFunc(pathSeps.Split(text, -1), func(p string) bool { return p == "" || p == "." || p == ".." })
		tail := ""
		if len(parts) > 0 {
			tail = parts[len(parts)-1]
		}
		if len(tail) > 1 && strings.Contains(tail[1:], ".") {
			tail = tail[:strings.LastIndex(tail, ".")]
		}
		var second, whole string
		if len(parts) > 1 {
			second = parts[len(parts)-2]
			whole = strings.Join(append(slices.Clone(parts[:len(parts)-1]), tail), "/")
		}
		return keep(tail, second, whole)
	}
	parts := slices.DeleteFunc(dottedSeps.Split(text, -1), func(p string) bool { return p == "" })
	if len(parts) == 0 {
		return nil
	}
	var second, whole string
	if len(parts) > 1 {
		second = parts[len(parts)-2]
		whole = strings.Join(parts, "/")
	}
	return keep(parts[len(parts)-1], second, whole)
}

var word = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func (generic) Read(rel string, src []byte) File {
	g, ok := grammars[path.Ext(rel)]
	if !ok {
		return File{}
	}
	return readTree(g.compile(), src)
}

func readTree(c *compiled, src []byte) File {
	p := ts.NewParser()
	defer p.Close()
	if err := p.SetLanguage(c.lang); err != nil {
		return File{Unresolved: []string{"file does not parse"}}
	}
	parsed := p.Parse(src, nil)
	if parsed == nil {
		return File{Unresolved: []string{"file does not parse"}}
	}
	defer parsed.Close()
	t := tree{src: src, root: parsed.RootNode()}
	var f File
	stack := []*ts.Node{t.root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		kind := n.Kind()
		if !n.IsNamed() || n.ChildCount() == 0 {
			continue // a keyword is a token, not a statement
		}
		if containsAny(kind, importKinds) {
			// Read whole, not descended into: its parts name symbols, and a
			// graph joined on symbols read as modules joins nothing.
			for _, m := range t.moduleTexts(n) {
				for _, mod := range modules(m.text) {
					f.Calls = append(f.Calls, Call{mod, Import, m.line, m.line})
				}
			}
			continue
		}
		stack = append(stack, children(n)...)
		start, end := line(n.StartPosition()), line(n.EndPosition())
		switch {
		case containsAny(kind, callKinds) && !containsAny(kind, notCalls):
			name, how, raw := t.callee(n)
			if importingCalls[strings.ToLower(name)] || importingCalls[raw] {
				for _, mod := range modules(t.argument(n, raw)) {
					f.Calls = append(f.Calls, Call{mod, Import, start, start})
				}
				continue
			}
			if name != "" {
				f.Calls = append(f.Calls, Call{name, how, start, end})
			} else {
				f.Unresolved = append(f.Unresolved, "a call through something with no name")
			}
		case containsAny(kind, inheritKinds):
			// Only the header: Rust writes a whole implementation inside the
			// node that names the trait.
			head := src[n.StartByte():n.EndByte()]
			body := -1
			for _, c := range children(n) {
				if containsAny(c.Kind(), []string{"body", "block", "declaration_list"}) {
					if b := int(c.StartByte()); body < 0 || b < body {
						body = b
					}
				}
			}
			if body >= 0 {
				head = src[n.StartByte():max(body, int(n.StartByte()))]
			}
			for _, w := range word.FindAllString(string(head), -1) {
				if !keywords[w] && !inheritWords[w] {
					f.Calls = append(f.Calls, Call{w, Name, start, start})
				}
			}
		case containsAny(kind, definesKinds) && !containsAny(kind, notDefines):
			if name := t.nameOf(n); name != "" {
				f.Symbols = append(f.Symbols, Symbol{name, start, end})
			}
		}
	}
	if c.query != nil {
		f = t.tags(c.query, f)
	}
	// One definition, one entry: the query and the general rule both find
	// the same class, and the second sighting adds nothing.
	type at struct {
		name  string
		start int
	}
	seen := map[at]bool{}
	f.Symbols = slices.DeleteFunc(f.Symbols, func(s Symbol) bool {
		k := at{s.Name, s.Start}
		if seen[k] {
			return true
		}
		seen[k] = true
		return false
	})
	return f
}

// tags asks the grammar's own query. It knows declarations peculiar to its
// language, and references a rule about node types cannot tell from a
// variable — Ruby's `helper` with no brackets — which are taken at the
// weakest grade and never for a name this file defines.
func (t tree) tags(q *ts.Query, f File) File {
	qc := ts.NewQueryCursor()
	defer qc.Close()
	names := q.CaptureNames()
	type ref struct {
		name string
		line int
	}
	var referenced []ref
	matches := qc.Matches(q, t.root, t.src)
	for m := matches.Next(); m != nil; m = matches.Next() {
		var named *ts.Node
		var marks []string
		for i := range m.Captures {
			c := &m.Captures[i]
			if names[c.Index] == "name" {
				if named == nil {
					node := c.Node
					named = &node
				}
			} else {
				marks = append(marks, names[c.Index])
			}
		}
		if named == nil {
			continue
		}
		name := strings.Trim(t.text(named), "\"'`")
		switch {
		case slices.ContainsFunc(marks, func(k string) bool { return strings.HasPrefix(k, "definition.") }):
			f.Symbols = append(f.Symbols, Symbol{name, line(named.StartPosition()), line(named.EndPosition())})
		case slices.ContainsFunc(marks, func(k string) bool { return strings.HasPrefix(k, "reference.") }):
			referenced = append(referenced, ref{name, line(named.StartPosition())})
		}
	}
	mine, seen := map[string]bool{}, map[string]bool{}
	for _, s := range f.Symbols {
		mine[s.Name] = true
	}
	for _, c := range f.Calls {
		seen[c.Name] = true
	}
	for _, r := range referenced {
		if !mine[r.name] && !seen[r.name] {
			f.Calls = append(f.Calls, Call{r.name, Attribute, r.line, r.line})
		}
	}
	return f
}
