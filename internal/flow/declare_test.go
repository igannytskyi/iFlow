package flow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/meta"
)

// declared writes src at name in a fresh directory, declares ps in it, and
// answers the directory and what the file came to.
func declared(t *testing.T, d declarer, name, src string, ps []meta.Provision, others map[string]string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for f, body := range others {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, name)
	var in []byte
	if src != "" {
		in = []byte(src)
	}
	out, err := d(path, in, ps)
	if err != nil {
		t.Fatalf("%s could not be declared: %v", name, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, string(out)
}

func run(t *testing.T, dir string, cmd ...string) string {
	t.Helper()
	if _, err := exec.LookPath(cmd[0]); err != nil {
		t.Skipf("no %s here", cmd[0])
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd, " "), err, out)
	}
	return string(out)
}

// Go: a function, a method, a type, a field of an existing struct and a
// constant, declared without behaviour, in a file that builds, beside one
// that is new to its package.
func TestGoIsDeclaredWithoutBehaviour(t *testing.T) {
	t.Parallel()
	src := "package shop\n\ntype Cart struct {\n\tItems []int\n}\n"
	ps := []meta.Provision{
		{File: "cart.go", Name: "Total", Kind: meta.Function, Signature: "func Total(c Cart) int"},
		{File: "cart.go", Name: "Add", Kind: meta.Method, Of: "Cart", Signature: "func (c *Cart) Add(n int) {"},
		{File: "cart.go", Name: "Owner", Kind: meta.Field, Of: "Cart", Signature: "Owner string"},
		{File: "cart.go", Name: "Receipt", Kind: meta.TypeDecl, Signature: "type Receipt struct{ Lines []string }"},
		{File: "cart.go", Name: "Limit", Kind: meta.Constant, Signature: "Limit = 10"},
	}
	dir, out := declared(t, goDeclare, "cart.go", src, ps, map[string]string{"go.mod": "module example.com/shop\n\ngo 1.22\n"})
	run(t, dir, "go", "build", "./...")
	if !strings.Contains(out, `panic("not yet: Total")`) || !strings.Contains(out, "Owner string") || strings.Contains(out, "return") {
		t.Errorf("not declared without behaviour:\n%s", out)
	}
	names, _ := goDeclares([]byte(out))
	for _, n := range []string{"Total", "Add", "Owner", "Receipt", "Limit", "Items"} {
		if !names[n] {
			t.Errorf("%s is not declared:\n%s", n, out)
		}
	}
	// A file new to the package takes the package of the files beside it.
	_, fresh := declared(t, goDeclare, "more.go", "", []meta.Provision{{File: "more.go", Name: "More", Kind: meta.Function, Signature: "func More() int"}},
		map[string]string{"cart.go": src})
	if !strings.HasPrefix(fresh, "package shop") {
		t.Errorf("a new file is not of its package:\n%s", fresh)
	}
}

// A field of a struct the file does not hold cannot be declared: that is
// said, not guessed around.
func TestAFieldWithoutItsTypeIsRefused(t *testing.T) {
	t.Parallel()
	_, err := goDeclare("x.go", []byte("package x\n"), []meta.Provision{{File: "x.go", Name: "N", Kind: meta.Field, Of: "Missing", Signature: "N int"}})
	if err == nil || !strings.Contains(err.Error(), "Missing") {
		t.Errorf("a field of no struct was declared: %v", err)
	}
}

// Python: a function, a class, a method and a field of an existing class,
// declared without behaviour, in a module that imports.
func TestPythonIsDeclaredWithoutBehaviour(t *testing.T) {
	t.Parallel()
	src := "\"\"\"Greets.\"\"\"\n\n\nclass Greeter:\n    \"\"\"Says hello.\"\"\"\n\n    def hello(self, name):\n        return \"Hello, \" + name\n\n\ndef greet(name):\n    return name\n"
	ps := []meta.Provision{
		{File: "greeting.py", Name: "greet_in", Kind: meta.Function, Signature: "def greet_in(lang, name=None) -> str:"},
		{File: "greeting.py", Name: "shout", Kind: meta.Method, Of: "Greeter", Signature: "def shout(self, name)"},
		{File: "greeting.py", Name: "language", Kind: meta.Field, Of: "Greeter", Signature: "language: str"},
		{File: "greeting.py", Name: "Farewell", Kind: meta.TypeDecl, Signature: "class Farewell"},
	}
	dir, out := declared(t, pyDeclare, "greeting.py", src, ps, nil)
	got := run(t, dir, "python3", "-c", `
import greeting
g = greeting.Greeter()
assert g.hello("a") == "Hello, a"
assert greeting.Greeter.__annotations__["language"] is str
for call in (lambda: greeting.greet_in("es"), lambda: g.shout("a")):
    try:
        call()
    except NotImplementedError as e:
        print(e)
    else:
        raise SystemExit("behaviour")
print(greeting.Farewell.__name__)
`)
	if !strings.Contains(got, "not yet: greet_in") || !strings.Contains(got, "not yet: shout") || !strings.Contains(got, "Farewell") {
		t.Errorf("not declared without behaviour:\n%s\n%s", got, out)
	}
}

// JavaScript: in CommonJS, a function, a class and a constant, exported in
// module.exports beside what it exported; a method and a field of an
// existing class.
func TestJavaScriptIsDeclaredWithoutBehaviour(t *testing.T) {
	t.Parallel()
	src := "// Says hello.\n\nclass Greeter {\n  hello(name) {\n    return \"Hello, \" + name;\n  }\n}\n\nfunction greet(name) {\n  return \"Hello, \" + name + \"!\";\n}\n\nmodule.exports = { greet, Greeter };\n"
	ps := []meta.Provision{
		{File: "greeting.js", Name: "shout", Kind: meta.Function, Signature: "function shout(name)"},
		{File: "greeting.js", Name: "Farewell", Kind: meta.TypeDecl, Signature: "class Farewell"},
		{File: "greeting.js", Name: "LIMIT", Kind: meta.Constant, Signature: "const LIMIT = 10"},
		{File: "greeting.js", Name: "wave", Kind: meta.Method, Of: "Greeter", Signature: "wave(name)"},
		{File: "greeting.js", Name: "language", Kind: meta.Field, Of: "Greeter", Signature: "language"},
	}
	dir, out := declared(t, jsDeclare, "greeting.js", src, ps, nil)
	got := run(t, dir, "node", "-e", `
const m = require("./greeting.js");
if (m.greet("a") !== "Hello, a!") throw new Error("greet moved");
if (m.LIMIT !== 10 || typeof m.Farewell !== "function") throw new Error("not exported");
const g = new m.Greeter();
if (!("language" in g)) throw new Error("no field");
for (const call of [() => m.shout("a"), () => g.wave("a")]) {
  try { call(); throw new Error("behaviour"); } catch (e) { if (!e.message.startsWith("not yet")) throw e; console.log(e.message); }
}
`)
	if !strings.Contains(got, "not yet: shout") || !strings.Contains(got, "not yet: wave") {
		t.Errorf("not declared without behaviour:\n%s\n%s", got, out)
	}
}

// What a Python module declares is read from its tree: a constant
// assigned at the top, a class's annotations, what a method assigns to
// self — and its fields are said read, so one missing is missing.
func TestPythonDeclaresAreReadFromTheTree(t *testing.T) {
	t.Parallel()
	src := "_OMITTED = object()\n\n\nclass Greeter:\n    language: str\n\n    def __init__(self):\n        self.mood = 1\n\n\ndef greet_in(lang, name=_OMITTED):\n    pass\n"
	names, fields := declaredIn("greet.py", []byte(src))
	if !fields {
		t.Error("the fields of a Python file are said not to be read")
	}
	for _, n := range []string{"_OMITTED", "Greeter", "language", "mood", "greet_in"} {
		if !names[n] {
			t.Errorf("%s is not seen as declared: %v", n, names)
		}
	}
	if names["lang"] || names["name"] {
		t.Errorf("a parameter is seen as declared: %v", names)
	}
}

// A declaration that names a type of another package of the module brings
// its import with it: the contract builds.
func TestGoDeclarationsImportWhatTheyName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for f, body := range map[string]string{
		"go.mod":         "module example.com/shop\n\ngo 1.22\n",
		"proto/proto.go": "package proto\n\n// Step is one step.\ntype Step struct{ Text string }\n",
		"ui/ui.go":       "package ui\n\n// UI is the window.\ntype UI struct{}\n",
	} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "ui", "feed.go")
	out, err := goDeclare(path, nil, []meta.Provision{{File: "ui/feed.go", Name: "feedLines", Kind: meta.Function, Signature: "func feedLines(steps []proto.Step, limit int) []string"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"example.com/shop/proto"`) {
		t.Fatalf("the import was not brought:\n%s", out)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "go", "build", "./...")
}

// What the units provide is declared at planning, in memory, on the estate
// as it stands: a name Go cannot declare goes back to the planner then,
// not to the contract after admission; what can be declared passes.
func TestWhatCannotBeDeclaredIsFoundAtPlanning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "view.go"), []byte("package x\n\ntype View struct{ Name string }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Run{repo: dir}
	good := []unit{{title: "view", provides: []meta.Provision{
		{File: "view.go", Name: "Floor", Kind: meta.Field, Of: "View", Signature: "Floor bool"},
		{File: "view.go", Name: "Toggle", Kind: meta.Function, Signature: "func Toggle(v *View)"},
	}}}
	if why := r.undeclarable(good); why != "" {
		t.Errorf("what can be declared was refused: %s", why)
	}
	bad := []unit{{title: "view", provides: []meta.Provision{{File: "view.go", Name: "floor/openrouter", Kind: meta.Variable, Signature: `choice key "floor/openrouter"`}}}}
	if why := r.undeclarable(bad); !strings.Contains(why, `"floor/openrouter" is no name Go declares`) {
		t.Errorf("a name Go cannot declare was not found at planning: %q", why)
	}
}
