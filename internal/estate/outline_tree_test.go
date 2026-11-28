package estate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/reader"
)

// A file of any language its tree is read in is outlined as a Go file is:
// a class with its fields and its methods beneath it, each definition as it
// is declared, each with what it calls — no bodies, no values.
func TestAFileIsOutlinedFromItsTree(t *testing.T) {
	for _, c := range []struct {
		file, src, want string
	}{
		{"shop.py", `LIMIT = 3

class Cart:
    owner: str = "nobody"

    def __init__(self, items):
        self.items = items

    def total(self):
        return sum(price(i) for i in self.items)

def price(item):
    return item.cost * LIMIT
`, `  LIMIT
  class Cart
    owner: str
    def __init__(self, items)
      self.items
    def total(self)   — calls sum, price
  def price(item)
`},
		{"shop.ts", `export const LIMIT = 3;
export class Cart {
  owner: string = "nobody";
  total(): number {
    return this.items.reduce(add, 0);
  }
}
function add(a: number, b: number): number { return a + b }
`, `  const LIMIT;
  class Cart
    owner: string
    total(): number   — calls reduce
  function add(a: number, b: number): number
`},
	} {
		got := TreeOutline(reader.Read(c.file, []byte(c.src)))
		if got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.file, got, c.want)
		}
		if strings.Contains(got, "nobody") || strings.Contains(got, "return") {
			t.Errorf("%s: a value or a body is in the outline:\n%s", c.file, got)
		}
	}
}

// A tests unit is handed the test files beside its own, outlined — their
// helpers and fakes — and not the code, nor the files it writes itself.
func TestTheTestsBesideAreOutlined(t *testing.T) {
	dir := t.TempDir()
	for f, src := range map[string]string{
		"go.mod":              "module example.com/x\n\ngo 1.22\n",
		"ui/view.go":          "package ui\n\nfunc Draw() {}\n",
		"ui/render_test.go":   "package ui\n\nimport \"testing\"\n\nfunc waitFor(t *testing.T, what string, done func() bool) {}\n\ntype cannedModel struct{ answer string }\n\nfunc TestDraw(t *testing.T) { Draw() }\n",
		"ui/feed_test.go":     "package ui\n\nimport \"testing\"\n\nfunc TestFeed(t *testing.T) {}\n",
		"other/other_test.go": "package other\n\nimport \"testing\"\n\nfunc helperElsewhere(t *testing.T) {}\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := NewLocal().Repo(dir).TestsOutlined([]string{"ui/feed_test.go"}, 4000)
	for _, want := range []string{"ui/render_test.go:", "func waitFor(t *testing.T, what string, done func() bool)", "type cannedModel struct"} {
		if !strings.Contains(got, want) {
			t.Errorf("the tests beside lack %q:\n%s", want, got)
		}
	}
	for _, never := range []string{"ui/view.go", "ui/feed_test.go", "helperElsewhere"} {
		if strings.Contains(got, never) {
			t.Errorf("the tests beside hold %q:\n%s", never, got)
		}
	}
}
