package estate

import (
	"strings"
	"testing"
)

// A pack holds the file to change whole, the signature of what it calls,
// who uses it, and its neighbours — and none of the estate besides.
func TestAPackHoldsWhatTheWorkNeedsAndNothingElse(t *testing.T) {
	p := shop(t).PackFor([]string{"shop/checkout.py"}, nil, 4000).String()
	for _, want := range []string{
		"# May change: shop/checkout.py", "return total(items)", // the file, whole
		"shop/core.py:1  def total(items):", // what it calls, by its signature
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the pack lacks %q:\n%s", want, p)
		}
	}
	for _, never := range []string{"def alone", "def report", "return sum(items)"} {
		if strings.Contains(p, never) {
			t.Errorf("the pack holds %q, which the work does not need:\n%s", never, p)
		}
	}
}

// A file the work creates is said not to exist yet, and what is beside it
// is named so the new file joins it.
func TestAPackForANewFileNamesItsNeighbours(t *testing.T) {
	p := shop(t).PackFor([]string{"shop/refund.py"}, nil, 4000).String()
	if !strings.Contains(p, "does not exist yet") || !strings.Contains(p, "shop/core.py:\n  def total(items):") {
		t.Errorf("the pack for a new file says too little:\n%s", p)
	}
}

// Past its budget a pack drops what is furthest from the work first, and
// says what it dropped.
func TestAPackKeepsToItsBudget(t *testing.T) {
	p := shop(t).PackFor([]string{"shop/checkout.py"}, nil, 5)
	if len(p.Left) == 0 || !strings.Contains(p.String(), "Left out") {
		t.Errorf("a pack over its budget says nothing was left out:\n%s", p)
	}
}

// The map names every source file and what it defines, and no body.
func TestTheMapNamesWithoutQuoting(t *testing.T) {
	m := shop(t).Map(2000)
	if !strings.Contains(m, "shop/core.py: total") || strings.Contains(m, "return") {
		t.Errorf("the map is wrong:\n%s", m)
	}
}
