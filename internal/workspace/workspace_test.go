package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/gate"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildFindsObjectsAndLinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "changes", "c")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "00-intent.md", "| Field | Value |\n|---|---|\n| Id | INT-001 |\n| Statement | it holds |\n")
	write(t, dir, "01-specification.md", "| Field | Value |\n|---|---|\n| Id | SPEC-001 |\n| Intent | INT-001 |\n\n"+
		"| Id | Criterion | When | Then | Plan | Procedure | Required evidence | Threshold |\n|---|---|---|---|---|---|---|---|\n"+
		"| CR-001-01 | it holds | always | it is so | not-required | machine | test-run | so |\n")
	write(t, dir, "05-assurance.md", "| Id | Unit | Criterion | Outcome | Evidence | State | Window | Baseline |\n|---|---|---|---|---|---|---|---|\n"+
		"| VE-001-01 | WU-001-01 | CR-001-01 | met | EV-001-01-01 | settled | | |\n")
	c, err := artifact.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wc := Build(c)
	for id, kind := range map[string]string{"INT-001": "Intent", "SPEC-001": "Specification", "CR-001-01": "Criterion", "VE-001-01": "Verdict"} {
		it := wc.Item(id)
		if it == nil || it.Kind != kind {
			t.Errorf("%s: got %+v, want a %s", id, it, kind)
		}
	}
	if got := wc.Item("CR-001-01").Area; got != 1 {
		t.Errorf("criterion area = %d, want 1", got)
	}
	linked := false
	for _, l := range wc.Links() {
		if l[0].ID == "VE-001-01" && l[1].ID == "CR-001-01" {
			linked = true
		}
	}
	if !linked {
		t.Error("the verdict is not linked to the criterion it judges")
	}
	wc.attach(gate.Report{Findings: []gate.Finding{
		{Rule: "R3", Message: "VE-001-01 is met on non-independent EV-001-01-01"},
		{Rule: "B4", Message: "no record.md — crossings are not recorded"},
	}})
	if n := len(wc.Item("VE-001-01").Findings); n != 1 {
		t.Errorf("verdict findings = %d, want 1", n)
	}
	if n := len(wc.General); n != 1 {
		t.Errorf("general findings = %d, want 1", n)
	}
}
