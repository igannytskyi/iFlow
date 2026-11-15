package adopt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAScenarioBecomesACriterionAndTheRestIsNamed(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "specs", "cart", "spec.md")
	if err := os.MkdirAll(filepath.Dir(spec), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Change: Add a cart\n\n## Why\nPeople want to buy.\n\n## Impact\n- `src/cart.go`\n"), 0o644)
	os.WriteFile(spec, []byte(`## ADDED Requirements
### Requirement: Adding an item
#### Scenario: an item is added
- **WHEN** a person adds an item
- **AND** the cart was empty
- **THEN** the cart holds one item
- **AND** the total is its price
### Requirement: A wish with no scenario
## REMOVED Requirements
### Requirement: The old basket
`), 0o644)
	tr, err := OpenSpec(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Criteria) != 1 {
		t.Fatalf("criteria = %+v", tr.Criteria)
	}
	c := tr.Criteria[0]
	if c.When != "a person adds an item; the cart was empty" || c.Then != "the cart holds one item; the total is its price" {
		t.Errorf("an AND did not follow what came before it: %+v", c)
	}
	if len(tr.Untranslated) != 2 || !strings.Contains(tr.Untranslated[1].Why, "removed") {
		t.Errorf("what did not translate is not named: %+v", tr.Untranslated)
	}
	if tr.Title != "Add a cart" || len(tr.Scope) != 1 || tr.Scope[0] != "src/cart.go" {
		t.Errorf("title %q, scope %v", tr.Title, tr.Scope)
	}
}
