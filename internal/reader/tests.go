package reader

import (
	"path"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// Tests is the tests a file declares, read from its syntax tree as its
// runner finds them — never by a pattern over its text — and whether this
// reader knows how its language declares a test at all:
//
//   - Python, as pytest collects them: a function named test…, at the top
//     of the module or as a method of a class;
//   - JavaScript and TypeScript, as node's test runner names them: a call
//     of test or it whose first argument is a string, by that string;
//   - Rust, as cargo test finds them: a function under an attribute whose
//     last part is test (#[test], #[tokio::test]), at the top of the crate
//     or in a module of it;
//   - Java and Kotlin, as JUnit runs them: a method under @Test, or under
//     one of JUnit's other test annotations, by its name.
//
// A file that does not parse declares no test it can be sure of.
func Tests(rel string, src []byte) ([]string, bool) {
	ext := path.Ext(rel)
	var g grammar
	var find func(t tree) []string
	switch ext {
	case ".py":
		g, find = pythonGrammar, pythonTests
	case ".js", ".cjs", ".mjs", ".jsx", ".ts", ".tsx":
		var ok bool
		if g, ok = grammars[ext]; !ok {
			return nil, false
		}
		find = callTests
	case ".rs":
		g, find = grammars[ext], rustTests
	case ".java":
		g, find = grammars[ext], junitTests("method_declaration")
	case ".kt":
		g, find = grammars[ext], junitTests("function_declaration")
	default:
		return nil, false
	}
	c := g.compile()
	p := ts.NewParser()
	defer p.Close()
	if err := p.SetLanguage(c.lang); err != nil {
		return nil, true
	}
	parsed := p.Parse(src, nil)
	if parsed == nil {
		return nil, true
	}
	defer parsed.Close()
	t := tree{src: src, root: parsed.RootNode()}
	if t.root.HasError() {
		return nil, true
	}
	return find(t), true
}

// pythonTests is every function named test… at the top of the module, and
// every method so named of a class there.
func pythonTests(t tree) []string {
	var out []string
	var visit func(n *ts.Node, inClass bool)
	visit = func(n *ts.Node, inClass bool) {
		for _, c := range children(n) {
			switch c.Kind() {
			case "decorated_definition":
				visit(c, inClass)
			case "function_definition":
				if name := c.ChildByFieldName("name"); name != nil && strings.HasPrefix(t.text(name), "test") {
					out = append(out, t.text(name))
				}
			case "class_definition":
				if inClass {
					continue
				}
				if body := c.ChildByFieldName("body"); body != nil {
					visit(body, true)
				}
			}
		}
	}
	visit(t.root, false)
	return out
}

// callTests is the name of every call of test or it whose first argument
// is a string.
func callTests(t tree) []string {
	var out []string
	walk(t.root, func(n *ts.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		fn := n.ChildByFieldName("function")
		if fn == nil || fn.Kind() != "identifier" || (t.text(fn) != "test" && t.text(fn) != "it") {
			return
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return
		}
		for _, a := range children(args) {
			if !a.IsNamed() {
				continue
			}
			if a.Kind() == "string" || a.Kind() == "template_string" {
				s := t.text(a)
				if len(s) >= 2 {
					out = append(out, s[1:len(s)-1])
				}
			}
			return // the first argument only
		}
	})
	return out
}

// rustTests is every function under an attribute whose path ends in test,
// at the top of the file and in the modules within it.
func rustTests(t tree) []string {
	var out []string
	var visit func(n *ts.Node)
	visit = func(n *ts.Node) {
		test := false // an attribute of the item that follows says it is a test
		for _, c := range children(n) {
			switch c.Kind() {
			case "attribute_item":
				if a := firstNamed(c); a != nil {
					if p := firstNamed(a); p != nil && lastPart(t, p) == "test" {
						test = true
					}
				}
				continue
			case "function_item":
				if name := c.ChildByFieldName("name"); test && name != nil {
					out = append(out, t.text(name))
				}
			case "mod_item":
				if body := c.ChildByFieldName("body"); body != nil {
					visit(body)
				}
			}
			if c.IsNamed() && c.Kind() != "line_comment" && c.Kind() != "block_comment" {
				test = false
			}
		}
	}
	visit(t.root)
	return out
}

// junitAnnotations are the annotations under which JUnit runs a method as
// a test, or as tests.
var junitAnnotations = map[string]bool{"Test": true, "ParameterizedTest": true, "RepeatedTest": true, "TestFactory": true, "TestTemplate": true}

// junitTests is, for a grammar whose methods are declared as kind, every
// method under one of JUnit's test annotations, by its name, wherever in
// the file — nested classes among it.
func junitTests(kind string) func(t tree) []string {
	return func(t tree) []string {
		var out []string
		walk(t.root, func(n *ts.Node) {
			if n.Kind() != kind {
				return
			}
			var mods *ts.Node
			for _, c := range children(n) {
				if c.Kind() == "modifiers" {
					mods = c
				}
			}
			name := n.ChildByFieldName("name")
			if mods == nil || name == nil {
				return
			}
			for _, a := range children(mods) {
				if (a.Kind() == "marker_annotation" || a.Kind() == "annotation") && junitAnnotations[annotationName(t, a)] {
					out = append(out, strings.Trim(t.text(name), "`")) // Kotlin's `a name with spaces`
					return
				}
			}
		})
		return out
	}
}

// annotationName is the simple name of an annotation, in Java or Kotlin.
func annotationName(t tree, a *ts.Node) string {
	if name := a.ChildByFieldName("name"); name != nil { // Java
		return lastPart(t, name)
	}
	var found string
	walk(a, func(n *ts.Node) { // Kotlin: the annotation's user_type, its last identifier
		if found == "" && n.Kind() == "user_type" {
			ids := children(n)
			for i := len(ids) - 1; i >= 0; i-- {
				if ids[i].IsNamed() {
					found = t.text(ids[i])
					break
				}
			}
		}
	})
	return found
}

// lastPart is the last segment of a name that may be qualified: a::b::test
// is test, org.junit.Test is Test.
func lastPart(t tree, n *ts.Node) string {
	if name := n.ChildByFieldName("name"); name != nil && n.Kind() == "scoped_identifier" {
		return t.text(name)
	}
	s := t.text(n)
	if i := strings.LastIndexAny(s, ".:"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func firstNamed(n *ts.Node) *ts.Node {
	for _, c := range children(n) {
		if c.IsNamed() {
			return c
		}
	}
	return nil
}
