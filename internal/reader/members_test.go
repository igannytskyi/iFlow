package reader

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// The fields of a file's types and the values at its top are read from its
// tree in every language that says it reads them, each by its name and how
// it is declared without its value; what a function declares within it is
// its own, no field.
func TestFieldsAreReadFromTheTree(t *testing.T) {
	for _, c := range []struct {
		file, src string
		want      map[string]string // name → how it is declared
	}{
		{"greet.py", `
LIMIT = 3
a, b = 1, 2
class Greeter:
    name: str = "x"
    count = 0
    def __init__(self, n):
        self.n = n
        local = 1
    def bump(self):
        self.count += 1
`, map[string]string{"LIMIT": "LIMIT", "a": "a", "b": "b", "name": "name: str", "count": "count", "n": "self.n"}},
		{"greet.ts", "export const LIMIT = 3;\nclass G {\n  name: string;\n  private n = 1;\n  hello(n: string): string { const inner = 2; return n }\n}\ninterface S { width: number; area(): number }\n",
			map[string]string{"LIMIT": "const LIMIT;", "name": "name: string", "n": "private n", "width": "width: number"}},
		{"greet.js", "const A = 1;\nclass G { name = 'x'; #p = 2; hello() { let q = 3 } }\n",
			map[string]string{"A": "const A;", "name": "name", "p": "#p"}},
		{"G.java", "class G { private int count = 0, other; static final String N = \"x\"; void a() { int local = 1; } }\n",
			map[string]string{"count": "private int count, other;", "other": "private int count, other;", "N": "static final String N;"}},
		{"G.kt", "val TOP = 1\nclass G { var name: String = \"\"\n fun a() { val local = 2 } }\n",
			map[string]string{"TOP": "val TOP", "name": "var name: String"}},
		{"p.rs", "const MAX: u32 = 3;\nstatic S: &str = \"\";\nstruct P { pub x: i32, y: i32 }\nimpl P { fn a(&self) { let z = 1; } }\n",
			map[string]string{"MAX": "const MAX: u32;", "S": "static S: &str;", "x": "pub x: i32", "y": "y: i32"}},
		{"G.cs", "class G { private int count; public string Name { get; set; } void A() { int local = 1; } }\n",
			map[string]string{"count": "private int count;", "Name": "public string Name { get; set; }"}},
		{"g.php", "<?php\nclass G { private $count = 0; public int $n; function a() { $local = 1; } }\n",
			map[string]string{"count": "private $count;", "n": "public int $n;"}},
	} {
		if !FieldsRead(c.file) {
			t.Errorf("%s: its fields are said not to be read", c.file)
		}
		got := map[string]string{}
		for _, f := range Read(c.file, []byte(c.src)).Fields {
			if _, seen := got[f.Name]; !seen {
				got[f.Name] = f.Head // a name by where it is first declared
			}
		}
		if !maps.Equal(got, c.want) {
			t.Errorf("%s:\n got %v\nwant %v", c.file, got, c.want)
		}
	}
	if FieldsRead("deploy.sh") || FieldsRead("app.rb") {
		t.Error("a language whose fields are not read is said to have them read")
	}
}

// A definition carries how it is declared, without its body, on one line.
func TestADefinitionIsReadWithItsHead(t *testing.T) {
	for _, c := range []struct{ file, src, name, head string }{
		{"g.py", "def greet(\n    name: str,\n) -> str:\n    return name\n", "greet", "def greet( name: str, ) -> str"},
		{"g.ts", "export function greet(n: string): string {\n  return n\n}\n", "greet", "function greet(n: string): string"},
		{"G.java", "class G {\n  public String greet(String n) throws E {\n    return n;\n  }\n}\n", "greet", "public String greet(String n) throws E"},
		{"g.rs", "pub fn greet(n: &str) -> String {\n    n.into()\n}\n", "greet", "pub fn greet(n: &str) -> String"},
	} {
		f := Read(c.file, []byte(c.src))
		i := slices.IndexFunc(f.Symbols, func(s Symbol) bool { return s.Name == c.name })
		if i < 0 || strings.TrimSpace(f.Symbols[i].Head) != c.head {
			t.Errorf("%s: %+v, want the head %q", c.file, f.Symbols, c.head)
		}
	}
}
