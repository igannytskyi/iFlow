package estate

import (
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
