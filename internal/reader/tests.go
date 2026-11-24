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
//     of test or it whose first argument is a string, by that string.
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
