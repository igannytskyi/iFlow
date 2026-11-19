package flow

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/reader"
)

// The contract is the plan's provisions declared without behaviour, before
// the tests are written against them. They are data — a file, a name, a
// kind, the type a field or method belongs to, a declaration — and the
// framework writes them, each language by its own declarer: a model told
// to leave out behaviour may write it all the same, and the tests would
// then pass on what was to declare only.

// declarer is how a language declares provisions in a file without
// behaviour: path is where the file is, src what it holds (nothing for a
// file not yet written), ps what it is to declare. It answers the file as
// it is to be, or why the provisions cannot be declared as stated.
type declarer func(path string, src []byte, ps []meta.Provision) ([]byte, error)

// notYet is what a declaration without behaviour says when it is called.
func notYet(name string) string { return "not yet: " + name }

// canDeclare says whether the framework can declare every provision of u
// in its own language.
func canDeclare(u unit) bool {
	return !slices.ContainsFunc(u.provides, func(p meta.Provision) bool {
		return p.Kind == meta.Described || declarerOf(p.File) == nil
	})
}

// declarerOf is how provisions are declared in file, or nil where the
// framework cannot.
func declarerOf(file string) declarer {
	l, ok := languageOfFile(file)
	if !ok {
		return nil
	}
	return l.declare[filepath.Ext(file)]
}

// declareContract writes the contract unit's provisions into ws, file by
// file, puts them in the estate's form, and holds them to building and to
// declaring every name. What cannot be declared as the plan states it goes
// back to the plan.
func (r *Run) declareContract(ctx context.Context, ws string, u unit, prof profile, ok bool) (harness.Result, error) {
	back := func(why string) error {
		return &sentBack{area: 2, unit: u.id, why: "the contract cannot be declared as the plan states it: " + why}
	}
	var files []string
	for _, p := range u.provides {
		if !slices.Contains(files, p.File) {
			files = append(files, p.File)
		}
	}
	for _, f := range files {
		declare := declarerOf(f)
		path := filepath.Join(ws, filepath.FromSlash(f))
		src, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return harness.Result{}, fmt.Errorf("%s: %s could not be read: %v", u.id, f, err)
		}
		ps := slices.DeleteFunc(slices.Clone(u.provides), func(p meta.Provision) bool { return p.File != f })
		out, err := declare(path, src, ps)
		if err != nil {
			return harness.Result{}, back(f + ": " + err.Error())
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return harness.Result{}, err
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return harness.Result{}, err
		}
	}
	if ok && prof.format != nil {
		if cmd := prof.format(files); cmd != nil {
			_, _, _ = runIn(ctx, ws, resolve(ws, cmd))
		}
	}
	if ok && len(prof.build) > 0 {
		if passed, out, _ := runIn(ctx, ws, resolve(ws, prof.build)); !passed {
			return harness.Result{}, back("declared, the estate does not build: " + lastLines(out, 8))
		}
	}
	if missing := undeclared(ws, u.provides); len(missing) > 0 {
		return harness.Result{}, back("declared, the code does not declare " + strings.Join(missing, ", "))
	}
	r.say(4, u.id, "%s: the framework declared %s, without behaviour", u.id, strings.Join(u.names(), ", "))
	return harness.Result{Terminal: harness.Candidate, Summary: "declared " + strings.Join(u.names(), ", ") + ", without behaviour"}, nil
}

// bodyless is a declaration without the body or block an author may have
// begun: what stands before its first "{" or its last ":".
func bodyless(sig string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(sig), "{:"))
}

// goDeclare declares provisions in a Go file: functions and methods that
// panic as not yet done, types, variables and constants as stated, and
// fields inside the struct they belong to.
func goDeclare(path string, src []byte, ps []meta.Provision) ([]byte, error) {
	text := string(src)
	if strings.TrimSpace(text) == "" {
		pkg, err := goPackageOf(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		text = "package " + pkg + "\n"
	}
	var fields []meta.Provision
	for _, p := range ps {
		sig := bodyless(p.Signature)
		switch p.Kind {
		case meta.Function, meta.Method:
			text += fmt.Sprintf("\n%s {\n\tpanic(%q)\n}\n", sig, notYet(p.Name))
		case meta.TypeDecl:
			// A type's declaration is the whole of it, fields and all.
			decl := prefixed(strings.TrimSpace(p.Signature), "type ")
			if !strings.Contains(decl, "{") && (strings.HasSuffix(decl, " struct") || strings.HasSuffix(decl, " interface")) {
				decl += "{}"
			}
			text += "\n" + decl + "\n"
		case meta.Variable:
			text += "\n" + prefixed(sig, "var ") + "\n"
		case meta.Constant:
			text += "\n" + prefixed(sig, "const ") + "\n"
		case meta.Field:
			fields = append(fields, p)
		default:
			return nil, fmt.Errorf("%s is of kind %q, which Go does not declare", p.Name, p.Kind)
		}
	}
	if len(fields) > 0 {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, text, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("declared, it does not parse: %v", err)
		}
		// Inserted from the end, so that an offset already found stays where
		// it was.
		type insertion struct {
			at   int
			line string
		}
		var ins []insertion
		for _, p := range fields {
			closing := -1
			ast.Inspect(f, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == p.Of {
					if st, ok := ts.Type.(*ast.StructType); ok {
						closing = fset.Position(st.Fields.Closing).Offset
					}
				}
				return closing < 0
			})
			if closing < 0 {
				return nil, fmt.Errorf("the field %s belongs to %s, which is no struct of this file", p.Name, p.Of)
			}
			ins = append(ins, insertion{closing, "\n\t" + bodyless(p.Signature) + "\n"})
		}
		slices.SortStableFunc(ins, func(a, b insertion) int { return b.at - a.at })
		for _, in := range ins {
			text = text[:in.at] + in.line + text[in.at:]
		}
	}
	out, err := format.Source([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("declared, it is not Go: %v", err)
	}
	return out, nil
}

func prefixed(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}
	return prefix + s
}

// goPackageOf is the package the Go files in dir are of, or, with none
// there, the directory's own name.
func goPackageOf(dir string) (string, error) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.PackageClauseOnly)
		if err == nil {
			return f.Name.Name, nil
		}
	}
	name := strings.NewReplacer("-", "", ".", "").Replace(filepath.Base(dir))
	if !token.IsIdentifier(name) {
		return "", fmt.Errorf("no package can be named for %s", dir)
	}
	return name, nil
}

// classes is where each class of a file starts and ends, by the estate's
// reader, and the indentation of its body.
func classes(path string, src []byte) map[string]reader.Symbol {
	out := map[string]reader.Symbol{}
	for _, s := range reader.Read(filepath.Base(path), src).Symbols {
		if _, seen := out[s.Name]; !seen {
			out[s.Name] = s
		}
	}
	return out
}

// insertLines puts lines after line n (1-based) of text; 0 puts them first.
func insertLines(text string, n int, lines string) string {
	all := strings.SplitAfter(text, "\n")
	n = min(max(n, 0), len(all))
	return strings.Join(all[:n], "") + lines + strings.Join(all[n:], "")
}

// pyDeclare declares provisions in a Python file: functions and methods
// that raise NotImplementedError, classes with nothing in them, fields as
// annotations of their class, variables and constants as stated.
func pyDeclare(path string, src []byte, ps []meta.Provision) ([]byte, error) {
	text := string(src)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	var members []meta.Provision
	for _, p := range ps {
		sig := bodyless(p.Signature)
		switch p.Kind {
		case meta.Function:
			text += fmt.Sprintf("\n\n%s:\n    raise NotImplementedError(%q)\n", sig, notYet(p.Name))
		case meta.TypeDecl:
			text += fmt.Sprintf("\n\n%s:\n    pass\n", prefixed(sig, "class "))
		case meta.Variable, meta.Constant:
			text += "\n" + sig + "\n"
		case meta.Method, meta.Field:
			members = append(members, p)
		default:
			return nil, fmt.Errorf("%s is of kind %q, which Python does not declare", p.Name, p.Kind)
		}
	}
	// Each member is put at the end of its class, found again after every
	// one put before it.
	for _, p := range members {
		c, ok := classes(path, []byte(text))[p.Of]
		if !ok {
			return nil, fmt.Errorf("the %s %s belongs to %s, which is no class of this file", p.Kind, p.Name, p.Of)
		}
		indent := bodyIndent(text, c.Start, "    ")
		sig := bodyless(p.Signature)
		line := indent + sig + "\n"
		if p.Kind == meta.Method {
			line = fmt.Sprintf("\n%s%s:\n%s    raise NotImplementedError(%q)\n", indent, sig, indent, notYet(p.Name))
		}
		text = insertLines(text, c.End, line)
	}
	return []byte(text), nil
}

// bodyIndent is the indentation of the first line after line start (1-based)
// that holds anything, or fallback beyond that line's own.
func bodyIndent(text string, start int, fallback string) string {
	lines := strings.Split(text, "\n")
	head := ""
	if start-1 < len(lines) {
		head = lines[start-1][:len(lines[start-1])-len(strings.TrimLeft(lines[start-1], " \t"))]
	}
	for _, l := range lines[min(start, len(lines)):] {
		if strings.TrimSpace(l) != "" {
			if in := l[:len(l)-len(strings.TrimLeft(l, " \t"))]; len(in) > len(head) {
				return in
			}
			break
		}
	}
	return head + fallback
}

var (
	cjsExports = regexp.MustCompile(`module\.exports\s*=\s*\{([^}]*)\}`)
	esModule   = regexp.MustCompile(`(?m)^\s*(export|import)\s`)
)

// jsDeclare declares provisions in a JavaScript file: functions and methods
// that throw as not yet done, classes with nothing in them, fields of their
// class, variables and constants as stated — each exported the way the
// file exports: by export in a module, in module.exports in CommonJS.
func jsDeclare(path string, src []byte, ps []meta.Provision) ([]byte, error) {
	text := string(src)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	module := esModule.MatchString(text) || strings.HasSuffix(path, ".mjs")
	exports := cjsExports.FindStringSubmatchIndex(text)
	var decls string
	var exported []string
	var members []meta.Provision
	for _, p := range ps {
		sig := bodyless(p.Signature)
		export := ""
		if module && !strings.HasPrefix(sig, "export ") {
			export = "export "
		}
		switch p.Kind {
		case meta.Function:
			decls += fmt.Sprintf("\n%s%s {\n  throw new Error(%q);\n}\n", export, prefixed(sig, "function "), notYet(p.Name))
		case meta.TypeDecl:
			decls += fmt.Sprintf("\n%s%s {}\n", export, prefixed(sig, "class "))
		case meta.Variable, meta.Constant:
			decls += "\n" + export + sig + ";\n"
		case meta.Method, meta.Field:
			members = append(members, p)
			continue
		default:
			return nil, fmt.Errorf("%s is of kind %q, which JavaScript does not declare", p.Name, p.Kind)
		}
		exported = append(exported, p.Name)
	}
	// In CommonJS what is exported is declared before the exports name it:
	// a class or a constant is not there before its declaration runs.
	if !module && exports != nil {
		text = text[:exports[0]] + strings.TrimPrefix(decls, "\n") + "\n" + text[exports[0]:]
	} else {
		text += decls
	}
	for _, p := range members {
		c, ok := classes(path, []byte(text))[p.Of]
		if !ok {
			return nil, fmt.Errorf("the %s %s belongs to %s, which is no class of this file", p.Kind, p.Name, p.Of)
		}
		indent := bodyIndent(text, c.Start, "  ")
		line := indent + strings.TrimSuffix(bodyless(p.Signature), ";") + ";\n"
		if p.Kind == meta.Method {
			line = fmt.Sprintf("%s%s {\n%s  throw new Error(%q);\n%s}\n", indent, bodyless(p.Signature), indent, notYet(p.Name), indent)
		}
		// Before the line that closes the class.
		text = insertLines(text, c.End-1, line)
	}
	if !module && len(exported) > 0 {
		if m := cjsExports.FindStringSubmatchIndex(text); m != nil {
			// The exports, found again where the declarations moved them.
			inner := strings.TrimRight(strings.TrimSpace(text[m[2]:m[3]]), ",")
			names := strings.Join(exported, ", ")
			if inner != "" {
				names = inner + ", " + names
			}
			text = text[:m[2]] + " " + names + " " + text[m[3]:]
		} else {
			text += "\nmodule.exports = { " + strings.Join(exported, ", ") + " };\n"
		}
	}
	return []byte(text), nil
}
