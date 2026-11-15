package reader

import (
	"slices"
	"strings"
	"unicode"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// python reads Python, and does the one thing that separates a reported
// dependency from a guess.
//
// A call on a receiver — session.mount(...) — names a method and nothing
// else, and a method name belongs to every file that defines one. Where the
// receiver's type can be read off the file itself — it was assigned from a
// constructor, it is annotated, it is self inside a class — the call is
// reported as reaching that type, firmly, and the method name is kept beside
// it at the weakest grade rather than dropped.
type python struct{}

func (python) Name() string         { return "python" }
func (python) Extensions() []string { return []string{".py"} }

// walk visits every node breadth first, as a syntax tree walk does, so that
// where the file says a thing twice the outer saying is the one kept.
func walk(root *ts.Node, visit func(*ts.Node)) {
	queue := []*ts.Node{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		visit(n)
		for _, c := range children(n) {
			if c.IsNamed() {
				queue = append(queue, c)
			}
		}
	}
}

func (python) Read(rel string, src []byte) File {
	c := pythonGrammar.compile()
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
	if t.root.HasError() {
		// What does not parse is not read in part: half a file read as a
		// whole one reports dependencies that may not be there.
		return File{Unresolved: []string{"file does not parse"}}
	}
	return t.python()
}

func (t tree) python() File {
	var f File
	imported := map[string]int{}
	var order []string
	note := func(name string, line int) {
		if _, ok := imported[name]; !ok {
			imported[name] = line
			order = append(order, name)
		}
	}
	walk(t.root, func(n *ts.Node) {
		switch n.Kind() {
		case "import_statement":
			for _, name := range t.importNames(n) {
				note(name, line(n.StartPosition()))
			}
		case "import_from_statement", "future_import_statement":
			mod := t.fromModule(n)
			for _, name := range t.importNames(n) {
				note(name, line(n.StartPosition()))
				if mod != "" {
					note(mod[strings.LastIndex(mod, ".")+1:], line(n.StartPosition()))
				}
			}
		}
	})
	bound, inside := t.bindings(), t.enclosing()
	known := map[string]bool{}
	for name := range imported {
		known[name] = true
		known[name[strings.LastIndex(name, ".")+1:]] = true
	}
	walk(t.root, func(n *ts.Node) {
		start, end := line(n.StartPosition()), line(n.EndPosition())
		switch n.Kind() {
		case "function_definition", "class_definition":
			name := n.ChildByFieldName("name")
			if name == nil {
				return
			}
			f.Symbols = append(f.Symbols, Symbol{t.text(name), start, end})
			if bases := n.ChildByFieldName("superclasses"); bases != nil {
				// What a class inherits from is a dependency written down,
				// and in framework code often the only one there is.
				for _, b := range children(bases) {
					switch b.Kind() {
					case "identifier":
						l := line(b.StartPosition())
						f.Calls = append(f.Calls, Call{t.text(b), Name, l, l})
					case "attribute":
						l := line(b.StartPosition())
						f.Calls = append(f.Calls, Call{t.text(b.ChildByFieldName("attribute")), Name, l, l})
					}
				}
			}
		case "call":
			fn := n.ChildByFieldName("function")
			switch {
			case fn == nil:
				f.Unresolved = append(f.Unresolved, "a call through something with no name")
			case fn.Kind() == "identifier":
				f.Calls = append(f.Calls, Call{t.text(fn), Name, start, end})
			case fn.Kind() == "attribute":
				attr := t.text(fn.ChildByFieldName("attribute"))
				kind := ""
				if recv := fn.ChildByFieldName("object"); recv != nil && recv.Kind() == "identifier" {
					id := t.text(recv)
					switch {
					case id == "self":
						kind = inside[start]
					case bound[id] != "":
						kind = bound[id]
					case known[id] || (id != "" && unicode.IsUpper([]rune(id)[0])):
						kind = id
					}
				}
				if kind != "" {
					f.Calls = append(f.Calls, Call{kind, Typed, start, end})
				}
				// Kept whether the receiver was typed or not: a method
				// inherited from a parent in another file is reached by the
				// name and by nothing else.
				f.Calls = append(f.Calls, Call{attr, Attribute, start, end})
			default:
				f.Unresolved = append(f.Unresolved, "a call through something with no name")
			}
		}
	})
	slices.Sort(order)
	for _, name := range order {
		f.Calls = append(f.Calls, Call{name, Import, imported[name], imported[name]})
	}
	return f
}

// importNames is what an import statement brings in, as the names it binds
// are spelled: a.b for "import a.b", b for "from a import b as c", * for a
// wildcard.
func (t tree) importNames(n *ts.Node) []string {
	var out []string
	module := n.ChildByFieldName("module_name")
	for _, c := range children(n) {
		if module != nil && c.Id() == module.Id() {
			continue
		}
		switch c.Kind() {
		case "dotted_name":
			out = append(out, t.text(c))
		case "aliased_import":
			if name := c.ChildByFieldName("name"); name != nil {
				out = append(out, t.text(name))
			}
		case "wildcard_import":
			out = append(out, "*")
		}
	}
	return out
}

// fromModule is the module of "from m import x", without the dots a relative
// import starts with, or "" for "from . import x".
func (t tree) fromModule(n *ts.Node) string {
	if n.Kind() == "future_import_statement" {
		return "__future__"
	}
	m := n.ChildByFieldName("module_name")
	if m == nil {
		return ""
	}
	if m.Kind() == "relative_import" {
		for _, c := range children(m) {
			if c.Kind() == "dotted_name" {
				return t.text(c)
			}
		}
		return ""
	}
	return t.text(m)
}

// bindings are names in this file whose type the file itself states: an
// assignment from a constructor, an annotation, a parameter's declared type.
// Nothing here is inferred across files.
func (t tree) bindings() map[string]string {
	bound := map[string]string{}
	constructor := func(target, value *ts.Node) {
		if target == nil || value == nil || target.Kind() != "identifier" || value.Kind() != "call" {
			return
		}
		switch fn := value.ChildByFieldName("function"); {
		case fn == nil:
		case fn.Kind() == "identifier":
			bound[t.text(target)] = t.text(fn)
		case fn.Kind() == "attribute":
			bound[t.text(target)] = t.text(fn.ChildByFieldName("attribute"))
		}
	}
	walk(t.root, func(n *ts.Node) {
		switch n.Kind() {
		case "assignment":
			if annotation := n.ChildByFieldName("type"); annotation != nil {
				if left := n.ChildByFieldName("left"); left != nil && left.Kind() == "identifier" {
					t.annotate(bound, t.text(left), annotation)
				}
				return
			}
			// a = b = Thing() binds both, and is written as one assignment
			// inside another.
			var targets []*ts.Node
			value := n
			for value != nil && value.Kind() == "assignment" && value.ChildByFieldName("type") == nil {
				targets = append(targets, value.ChildByFieldName("left"))
				value = value.ChildByFieldName("right")
			}
			if n.Parent() != nil && n.Parent().Kind() == "assignment" {
				return // the outer assignment has already said this
			}
			for _, target := range targets {
				constructor(target, value)
			}
		case "function_definition":
			params := n.ChildByFieldName("parameters")
			if params == nil {
				return
			}
			for _, p := range children(params) {
				var name *ts.Node
				switch p.Kind() {
				case "typed_parameter":
					if first := p.NamedChild(0); first != nil && first.Kind() == "identifier" {
						name = first
					}
				case "typed_default_parameter":
					name = p.ChildByFieldName("name")
				}
				if name != nil {
					if annotation := p.ChildByFieldName("type"); annotation != nil {
						t.annotate(bound, t.text(name), annotation)
					}
				}
			}
		}
	})
	return bound
}

// annotate binds a name to the type an annotation states: Thing, mod.Thing,
// or the one thing inside Optional[Thing].
func (t tree) annotate(bound map[string]string, name string, n *ts.Node) {
	switch n.Kind() {
	case "type":
		if inner := n.NamedChild(0); inner != nil {
			t.annotate(bound, name, inner)
		}
	case "identifier":
		bound[name] = t.text(n)
	case "attribute":
		bound[name] = t.text(n.ChildByFieldName("attribute"))
	case "member_type":
		if last := n.NamedChild(n.NamedChildCount() - 1); last != nil && last.Kind() == "identifier" {
			bound[name] = t.text(last)
		}
	case "generic_type":
		for _, c := range children(n) {
			if c.Kind() == "type_parameter" && c.NamedChildCount() == 1 {
				t.annotate(bound, name, c.NamedChild(0))
			}
		}
	case "subscript":
		subs := n.ChildrenByFieldName("subscript", n.Walk())
		if len(subs) == 1 {
			t.annotate(bound, name, &subs[0])
		}
	}
}

// enclosing says which class each line sits in, so that self has a type. The
// outermost class wins, as it is the first a breadth-first walk meets.
func (t tree) enclosing() map[int]string {
	span := map[int]string{}
	walk(t.root, func(n *ts.Node) {
		if n.Kind() != "class_definition" {
			return
		}
		name := n.ChildByFieldName("name")
		if name == nil {
			return
		}
		for l := line(n.StartPosition()); l <= line(n.EndPosition()); l++ {
			if _, ok := span[l]; !ok {
				span[l] = t.text(name)
			}
		}
	})
	return span
}
