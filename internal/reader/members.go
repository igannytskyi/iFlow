package reader

import (
	"slices"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// What a file declares besides its code — the fields of its types, the
// values at its top — read, as the rest of the tree is, from the names the
// grammars share: a member is a node whose kind names a field or a
// property and says it is declared, defined or signed; its names are in its
// name field, or in the declarators under it.

// fieldsRead are the languages whose fields this reader reads, each one
// tested: in the others a field is neither found nor said missing.
var fieldsRead = set(".py", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".java", ".kt", ".rs", ".cs", ".php")

// FieldsRead says whether the fields of a file named rel are read: where
// they are, a field not among a file's Fields is not there.
func FieldsRead(rel string) bool {
	i := strings.LastIndex(rel, ".")
	return i >= 0 && fieldsRead[rel[i:]]
}

// member says a node of kind declares a field or a property.
func member(kind string) bool {
	return (strings.Contains(kind, "field") || strings.Contains(kind, "property")) &&
		containsAny(kind, []string{"declaration", "definition", "signature"}) && !containsAny(kind, []string{"list", "identifier"})
}

// inCode says n stands inside a function, a method or a lambda: what it
// declares is local, no field — Kotlin writes a local val as it writes a
// property.
func inCode(n *ts.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if containsAny(p.Kind(), []string{"function", "method", "constructor", "lambda", "accessor"}) {
			return true
		}
	}
	return false
}

// valueKinds are the declarations of a value at a file's top, as grammars
// name them.
var valueKinds = set("lexical_declaration", "variable_declaration", "const_declaration", "var_declaration", "const_item", "static_item")

// topValue says n declares a value at the top of its file, exported or not.
func (t tree) topValue(n *ts.Node) bool {
	if !valueKinds[n.Kind()] {
		return false
	}
	p := n.Parent()
	if p != nil && p.Kind() == "export_statement" {
		p = p.Parent()
	}
	return p != nil && p.Id() == t.root.Id()
}

// memberNames is every name n declares: its own, or each of its
// declarators' — `int a = 1, b;` declares two.
func (t tree) memberNames(n *ts.Node) []string {
	for _, f := range []string{"name", "property"} {
		if got := n.ChildByFieldName(f); got != nil {
			return []string{bare(t.text(got))}
		}
	}
	var out []string
	for _, c := range children(n) {
		if c.IsNamed() && containsAny(c.Kind(), []string{"declarator", "declaration", "element", "spec"}) {
			out = append(out, t.memberNames(c)...)
		}
	}
	if len(out) == 0 && strings.Contains(n.Kind(), "declaration") {
		for _, c := range children(n) {
			if strings.HasSuffix(c.Kind(), "identifier") {
				return []string{bare(t.text(c))} // Kotlin's: the variable, with no field to say so
			}
		}
	}
	return out
}

// bare is a name as code calls it: PHP's $count is count, a private #p is p.
func bare(name string) string {
	return strings.TrimLeft(name, "$#")
}

// head is a definition as it is declared: its text up to its body, on one
// line.
func (t tree) head(n *ts.Node) string {
	end := n.EndByte()
	if b := n.ChildByFieldName("body"); b != nil {
		end = b.StartByte()
	} else {
		for _, c := range children(n) {
			if containsAny(c.Kind(), []string{"body", "block", "declaration_list", "compound_statement"}) {
				end = c.StartByte()
				break
			}
		}
	}
	return oneLine(string(t.src[n.StartByte():end]))
}

// declaration is a member as it is declared, without the value it is given:
// `private int count = 0;` is `private int count;`.
func (t tree) declaration(n *ts.Node) string {
	type span struct{ from, to uint }
	var cut []span
	walk(n, func(c *ts.Node) { // what follows an =, in whatever node it stands
		kids := children(c)
		for i, k := range kids {
			if k.Kind() == "=" && i+1 < len(kids) {
				cut = append(cut, span{k.StartByte(), kids[i+1].EndByte()})
			}
		}
	})
	slices.SortFunc(cut, func(a, b span) int { return int(a.from) - int(b.from) })
	var b strings.Builder
	at := n.StartByte()
	for _, c := range cut {
		if c.from < at {
			continue // within a value already cut
		}
		b.Write(t.src[at:c.from])
		at = c.to
	}
	b.Write(t.src[at:n.EndByte()])
	return oneLine(b.String())
}

// oneLine is text on one line, its runs of space one space.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, " ;", ";")
	return strings.ReplaceAll(s, " ,", ",")
}
