package artifact

import (
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/meta"
)

// An escalation is settled by a decision once, by someone, and only one the
// change raised.
func TestSettlingAnEscalation(t *testing.T) {
	dir := t.TempDir() + "/c"
	if err := Save(dir, Assurance, meta.Assurance{Schema: meta.SchemaVersion, Name: "c",
		Raised: []meta.Escalation{{N: 1, Standing: "open", Ground: "it cannot be told", Against: "area 13", To: "a person"}}}); err != nil {
		t.Fatal(err)
	}
	if err := Settle(dir, "c#1", "a person", ""); err == nil {
		t.Error("settled with no decision")
	}
	if err := Settle(dir, "c#2", "a person", "accepted"); err == nil {
		t.Error("settled an escalation never raised")
	}
	if err := Settle(dir, "c#1", "a person", "accepted as a known limit"); err != nil {
		t.Fatal(err)
	}
	if err := Settle(dir, "c#1", "a person", "again"); err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("settled twice: %v", err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s := c.Meta.Assurance.Settled; len(s) != 1 || s[0].Note != "accepted as a known limit" {
		t.Errorf("settled %+v", s)
	}
	if r := c.Meta.Record; len(r) != 1 || r[0].Stage != "settled" || r[0].N != 1 {
		t.Errorf("recorded %+v", r)
	}
}
