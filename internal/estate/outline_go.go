package estate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/reader"
)

// GoOutline is the Go file src as an agent deciding or changing code needs
// it, without reading it: each type with its
// fields or methods, each function with its signature and the functions and
// methods it calls — no bodies.
func GoOutline(src []byte) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	text := func(n ast.Node) string {
		t := string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
		return strings.Join(strings.Fields(t), " ")
	}
	var b strings.Builder
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			sig := string(src[fset.Position(d.Pos()).Offset:fset.Position(d.Type.End()).Offset])
			b.WriteString("  " + strings.Join(strings.Fields(sig), " "))
			var calls []string
			if d.Body != nil {
				ast.Inspect(d.Body, func(n ast.Node) bool {
					c, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					name := ""
					switch fn := c.Fun.(type) {
					case *ast.Ident:
						name = fn.Name
					case *ast.SelectorExpr:
						name = fn.Sel.Name
						if x, ok := fn.X.(*ast.Ident); ok {
							name = x.Name + "." + fn.Sel.Name
						}
					}
					if name != "" && !slices.Contains(calls, name) && len(calls) < 12 {
						calls = append(calls, name)
					}
					return true
				})
			}
			if len(calls) > 0 {
				b.WriteString("   — calls " + strings.Join(calls, ", "))
			}
			b.WriteString("\n")
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					var members *ast.FieldList
					kind := "type " + sp.Name.Name
					switch t := sp.Type.(type) {
					case *ast.StructType:
						members, kind = t.Fields, kind+" struct"
					case *ast.InterfaceType:
						members, kind = t.Methods, kind+" interface"
					default:
						kind += " " + text(sp.Type)
					}
					b.WriteString("  " + kind + "\n")
					if members != nil {
						for _, m := range members.List {
							b.WriteString("    " + text(m) + "\n")
						}
					}
				case *ast.ValueSpec:
					for _, id := range sp.Names {
						b.WriteString("  " + map[bool]string{true: "const ", false: "var "}[d.Tok == token.CONST] + id.Name + "\n")
					}
				}
			}
		}
	}
	return b.String()
}

// Outlined is the files of the folders paths name, as an agent needs them
// without reading them — every type with its fields or methods, every
// function with its signature and what it calls: Go by its own parser, any
// other language by its tree; the files named first, the rest of their
// folders after, within budget tokens.
func (r *Repo) Outlined(paths []string, budget int) string {
	dirs := map[string]bool{}
	for _, p := range paths {
		if strings.HasSuffix(p, "/") {
			dirs[strings.TrimSuffix(p, "/")] = true
		} else {
			dirs[path.Dir(p)] = true
		}
	}
	named := func(s string) bool { return slices.Contains(paths, s) }
	sources := slices.Clone(r.Sources())
	slices.SortStableFunc(sources, func(a, c string) int {
		switch {
		case named(a) && !named(c):
			return -1
		case named(c) && !named(a):
			return 1
		}
		return 0
	})
	var b strings.Builder
	for _, s := range sources {
		if !dirs[path.Dir(s)] || strings.HasSuffix(s, "_test.go") && !named(s) {
			continue
		}
		var outline string
		if filepath.Ext(s) == ".go" {
			src, err := os.ReadFile(r.abs(s))
			if err != nil {
				continue
			}
			outline = GoOutline(src)
		} else if f, ok := r.treeFile(s); ok {
			outline = TreeOutline(f)
		}
		b.WriteString(s + ":\n" + outline)
		if Tokens(b.String()) > budget {
			b.WriteString("[cut: more than the budget]\n")
			return b.String()
		}
	}
	return b.String()
}

// goType is the whole declaration of the type name in the Go file at abs —
// a struct with its fields, an interface with its methods — or "".
func goType(abs, name string) string {
	src, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, sp := range g.Specs {
			if ts, ok := sp.(*ast.TypeSpec); ok && ts.Name.Name == name {
				return "type " + string(src[fset.Position(ts.Pos()).Offset:fset.Position(ts.End()).Offset])
			}
		}
	}
	return ""
}

// TestsOutlined is the test files beside paths — in the folders they name,
// and not among them — outlined as Outlined outlines code: what tests
// written there can build on, their helpers by signature and their fakes
// by type, within budget tokens. A test file is a Go _test.go, or a file
// whose tree declares tests.
func (r *Repo) TestsOutlined(paths []string, budget int) string {
	dirs := map[string]bool{}
	for _, p := range paths {
		if strings.HasSuffix(p, "/") {
			dirs[strings.TrimSuffix(p, "/")] = true
		} else {
			dirs[path.Dir(p)] = true
		}
	}
	var b strings.Builder
	for _, s := range r.Sources() {
		if !dirs[path.Dir(s)] || slices.Contains(paths, s) {
			continue
		}
		src, err := os.ReadFile(r.abs(s))
		if err != nil {
			continue
		}
		var outline string
		switch {
		case strings.HasSuffix(s, "_test.go"):
			outline = GoOutline(src)
		case filepath.Ext(s) != ".go":
			if names, known := reader.Tests(s, src); known && len(names) > 0 {
				outline = TreeOutline(reader.Read(s, src))
			}
		}
		if outline == "" {
			continue
		}
		b.WriteString(s + ":\n" + outline)
		if Tokens(b.String()) > budget {
			b.WriteString("[cut: more than the budget]\n")
			break
		}
	}
	return b.String()
}
