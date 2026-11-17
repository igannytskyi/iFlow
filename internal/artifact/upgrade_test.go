package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/igannytskyi/iflow/internal/meta"
)

// A plan of version 1 is read as version 2: what each unit writes from what
// it said, and its prose contract as named entries where the prose was a
// declaration with one file it could be in — kept as described where not.
func TestAVersionOnePlanIsReadAsVersionTwo(t *testing.T) {
	v1 := `{"schema":1,"name":"n","specification":"SPEC-001","phases":1,"intermediate_states_valid":"yes",
"units":[
 {"id":"WU-001-01","phase":1,"class":"C3","scope":["ui/a_test.go"],"criteria":["CR-001-01"],"title":"tests","writes_tests":true},
 {"id":"WU-001-02","phase":1,"class":"C3","scope":["ui/app.go"],"criteria":["CR-001-01"],"title":"the contract — every name",
  "provides":["func (u *UI) quiet() bool","field Notify notify.Sender on type Options","type Sender interface { Send() }","func New(x int) *T","(no new names)"]},
 {"id":"WU-001-03","phase":1,"class":"C3","scope":["a.go","b.go"],"criteria":["CR-001-01"],"title":"two files","provides":["func Two()"]}]}`
	b, err := upgrade(Plan, []byte(v1))
	if err != nil {
		t.Fatal(err)
	}
	var p meta.Plan
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if ps := meta.Validate(p); len(ps) > 0 {
		t.Fatalf("the plan read does not hold to the model: %v", ps)
	}
	if kinds := []meta.UnitKind{p.Units[0].Kind, p.Units[1].Kind, p.Units[2].Kind}; !slices.Equal(kinds, []meta.UnitKind{meta.TestsUnit, meta.ContractUnit, meta.CodeUnit}) {
		t.Errorf("kinds %v", kinds)
	}
	want := []meta.Provision{
		{File: "ui/app.go", Name: "quiet", Kind: meta.Method, Signature: "func (u *UI) quiet() bool"},
		{File: "ui/app.go", Name: "Notify", Kind: meta.Field, Signature: "field Notify notify.Sender on type Options"},
		{File: "ui/app.go", Name: "Sender", Kind: meta.TypeDecl, Signature: "type Sender interface { Send() }"},
		{File: "ui/app.go", Name: "New", Kind: meta.Function, Signature: "func New(x int) *T"},
		{Kind: meta.Described, Signature: "(no new names)"},
	}
	if !slices.Equal(p.Units[1].Provides, want) {
		t.Errorf("provides\n%+v\nwant\n%+v", p.Units[1].Provides, want)
	}
	if got := p.Units[2].Provides; len(got) != 1 || got[0].Kind != meta.Described || got[0].Signature != "func Two()" {
		t.Errorf("a provision whose file the plan did not say: %+v", got)
	}
}

// A plan of version 1 on disk is said, dry, and written, migrating: in
// version 2, the older JSON kept under legacy/, the record told.
func TestAVersionOnePlanIsMigrated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "c")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	v1 := `{"schema":1,"name":"c","specification":"SPEC-001","phases":1,"intermediate_states_valid":"yes","units":[{"id":"WU-001-01","phase":1,"class":"C3","scope":["a.go"],"criteria":["CR-001-01"],"provides":["func A()"]}]}`
	if err := os.WriteFile(JSONFile(dir, Plan), []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Migrate(dir, true)
	if err != nil || !slices.Contains(m.Written, string(Plan)) || len(m.Problems) > 0 {
		t.Fatalf("dry: %+v %v", m, err)
	}
	if b, _ := os.ReadFile(JSONFile(dir, Plan)); string(b) != v1 {
		t.Fatal("a dry migration wrote")
	}
	if _, err := Migrate(dir, false); err != nil {
		t.Fatal(err)
	}
	var p meta.Plan
	if _, err := LoadJSON(dir, Plan, &p); err != nil || p.Schema != meta.PlanVersion || p.Units[0].Provides[0].Name != "A" {
		t.Fatalf("migrated plan %+v %v", p, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, LegacyDir, "02-plan.json")); string(b) != v1 {
		t.Error("the older plan was not kept")
	}
	rec, _ := ReadRecord(dir)
	if len(rec) != 1 || rec[0].Stage != "migrated" {
		t.Errorf("the record was not told: %+v", rec)
	}
}
