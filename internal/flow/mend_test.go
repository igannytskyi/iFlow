package flow

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/igannytskyi/iflow/internal/estate"
	"github.com/igannytskyi/iflow/internal/meta"
	"github.com/igannytskyi/iflow/internal/model"
)

// A mend is laid over the last draft: the keys it names are taken whole,
// the rest stand as they were; the draft it is laid over is not changed by
// it, and a draft written whole again is the draft it says.
func TestAMendIsLaidOverTheLastDraft(t *testing.T) {
	t.Parallel()
	was := draft{Slug: "shout", Name: "Shout", Statement: "make it possible to shout", Class: "C2",
		Document: document{Summary: "It shouts.", Goals: []string{"a shout"}},
		Criteria: []draftCriterion{{Criterion: "Shouts", When: "a name", Then: "HELLO", Plan: "required", New: true, Procedure: "machine"}},
		Scope:    []string{"greet.go"}, NewTests: []string{"shout_test.go"}}
	was.Termination = append(was.Termination, struct {
		Condition string `json:"condition"`
		Action    string `json:"action"`
	}{"it cannot", "stop"})
	if c := was.clone(); !reflect.DeepEqual(c, was) {
		t.Fatalf("a draft does not come through whole:\n%+v\n%+v", c, was)
	}

	d := was.clone()
	if err := model.JSON(`{"scope": ["greet.go", "loud.go"], "document": {"summary": "It shouts loudly."}}`, &d); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Scope, []string{"greet.go", "loud.go"}) || d.Document.Summary != "It shouts loudly." {
		t.Errorf("the keys mended are not taken: %+v", d)
	}
	if d.Name != "Shout" || len(d.Criteria) != 1 || !reflect.DeepEqual(d.Document.Goals, []string{"a shout"}) || len(d.Termination) != 1 {
		t.Errorf("what the mend left out did not stand: %+v", d)
	}
	if !reflect.DeepEqual(was.Scope, []string{"greet.go"}) {
		t.Errorf("the draft mended over was changed: %v", was.Scope)
	}

	whole := was.clone()
	if err := model.JSON(was.json(), &whole); err != nil || !reflect.DeepEqual(whole, was) {
		t.Errorf("a draft written whole again is not that draft (%v)", err)
	}
}

// A plan is mended by its units' numbers: a unit named is taken whole, null
// takes one out, the number after the last adds one, and the units not
// named stand; a list is the units whole; a number with no unit is refused.
func TestAPlanIsMendedUnitByUnit(t *testing.T) {
	t.Parallel()
	was := planDraft{Units: tickets{{Title: "tests", Kind: "tests"}, {Title: "contract", Kind: "contract"}, {Title: "code", Kind: "code", Scope: []string{"a.go"}}}}
	was.Approach.Summary = "three units"
	titles := func(p planDraft) []string {
		var out []string
		for _, u := range p.Units {
			out = append(out, u.Title)
		}
		return out
	}
	p := was.clone()
	if err := model.JSON(`{"units": {"3": {"title": "code, mended", "kind": "code", "scope": ["a.go", "b.go"]}, "2": null, "4": {"title": "docs", "kind": "code"}}}`, &p); err != nil {
		t.Fatal(err)
	}
	if got := titles(p); !reflect.DeepEqual(got, []string{"tests", "code, mended", "docs"}) || p.Approach.Summary != "three units" {
		t.Errorf("the mend was not laid unit by unit: %v, %q", got, p.Approach.Summary)
	}
	if !reflect.DeepEqual(titles(was), []string{"tests", "contract", "code"}) {
		t.Errorf("the plan mended over was changed: %v", titles(was))
	}
	whole := was.clone()
	if err := model.JSON(`{"units": [{"title": "one", "kind": "code"}]}`, &whole); err != nil || !reflect.DeepEqual(titles(whole), []string{"one"}) {
		t.Errorf("a list is not the units whole: %v (%v)", titles(whole), err)
	}
	bad := was.clone()
	if err := model.JSON(`{"units": {"7": {"title": "far"}}}`, &bad); err == nil {
		t.Error("a unit with no number in the plan was mended")
	}
	if n := was.numbered(); n != "1. tests\n2. contract\n3. code\n" {
		t.Errorf("the units are numbered %q", n)
	}
}

// A partial mend is asked only of a model whose answer may leave keys out:
// one held to a strict schema writes every key, and the keys it leaves
// would come back emptied.
func TestAPartialMendIsAskedOnlyWhereKeysMayBeLeftOut(t *testing.T) {
	t.Parallel()
	if !freeForm(stubModel{}) {
		t.Error("a model asked with no schema is not free to leave keys out")
	}
	if freeForm(shapedStub{}) {
		t.Error("a model held to a schema is asked to leave keys out")
	}
}

type shapedStub struct{ stubModel }

func (shapedStub) AskShaped(context.Context, string, string, string, map[string]any) (string, model.Usage, error) {
	return "{}", model.Usage{}, nil
}

// A name two units give to two types is used by its type: Config.Floor
// waits for the unit that gives it, OpenAI.Floor for the other; used bare,
// it is sent back as ambiguous, not read as a circle; and a circle that is
// one says who waits for whom.
func TestAUseIsReadByItsType(t *testing.T) {
	t.Parallel()
	field := func(of string) meta.Provision {
		return meta.Provision{File: "x.go", Name: "Floor", Kind: meta.Field, Of: of, Signature: "Floor bool"}
	}
	config := unit{title: "config", provides: []meta.Provision{field("Config")}, uses: []string{"OpenAI.Floor"}}
	openai := unit{title: "openai", provides: []meta.Provision{field("OpenAI")}}
	ui := unit{title: "settings", uses: []string{"models.Config.Floor"}}
	if why := ambiguous([]unit{config, openai, ui}); why != "" {
		t.Errorf("uses by type were found ambiguous: %s", why)
	}
	order, why := orderByUse([]unit{ui, config, openai})
	if why != "" {
		t.Fatal(why)
	}
	var titles []string
	for _, u := range order {
		titles = append(titles, u.title)
	}
	if !slices.Equal(titles, []string{"openai", "config", "settings"}) {
		t.Errorf("ordered %v", titles)
	}
	config.uses = []string{"Floor"}
	settings := unit{title: "settings", uses: []string{"Floor"}}
	why = ambiguous([]unit{config, openai, settings})
	if !strings.Contains(why, `"config" uses Floor, and the plan gives Config.Floor and OpenAI.Floor`) || !strings.Contains(why, `"settings" uses Floor`) {
		t.Errorf("not every bare name two types share is sent back at once: %q", why)
	}
	a := unit{title: "a", provides: []meta.Provision{{Name: "A", Kind: meta.Function}}, uses: []string{"B"}}
	b := unit{title: "b", provides: []meta.Provision{{Name: "B", Kind: meta.Function}}, uses: []string{"A"}}
	if _, why := orderByUse([]unit{a, b}); !strings.Contains(why, `"a" uses B, which "b" provides`) {
		t.Errorf("a circle does not say who waits for whom: %q", why)
	}
}

// What a planner writes in another form than asked, where it can mean one
// thing only, is read as that thing: a field named with its type is the
// type's field, however its package is written before it; one string where
// a list of points is asked is a list of one.
func TestAPlanIsReadWhereItCanMeanOneThing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "models", "models.go"), []byte("package models\n\ntype Config struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Run{repo: dir, cfg: Config{Estate: estate.NewLocal()}}
	field := func(name, of string) unit {
		return unit{title: "config", provides: []meta.Provision{{File: "internal/models/models.go", Name: name, Kind: meta.Field, Of: of, Signature: "Floor bool"}}}
	}
	for _, c := range []struct{ name, of string }{{"Config.Floor", ""}, {"models.Config.Floor", ""}, {"Config.Floor", "Config"}} {
		code := []unit{field(c.name, c.of)}
		if why := r.canonical(nil, code); why != "" {
			t.Errorf("%s of %q: %s", c.name, c.of, why)
			continue
		}
		if p := code[0].provides[0]; p.Name != "Floor" || p.Of != "Config" {
			t.Errorf("%s of %q is read as %s of %s", c.name, c.of, p.Name, p.Of)
		}
	}
	if why := r.canonical(nil, []unit{field("Config.Floor", "OpenAI")}); !strings.Contains(why, "say one type") {
		t.Errorf("a field said to be of two types was taken: %q", why)
	}

	var tk ticket
	if err := model.JSON(`{"title": "t", "done_when": "the tests pass", "scope": ["a.go"]}`, &tk); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tk.DoneWhen, points{"the tests pass"}) || !slices.Equal(tk.Scope, points{"a.go"}) {
		t.Errorf("read as %+v", tk)
	}
}

// A new test file that no criterion is judged by is refused with the
// specification: a tests unit writing it would serve nothing, and no plan
// could hold it.
func TestANewTestFileJudgesSomething(t *testing.T) {
	t.Parallel()
	d := draft{NewTests: []string{"feed_test.go"}, Criteria: []draftCriterion{{Criterion: "It reads well", Procedure: "human", Plan: "required"}}}
	if !slices.ContainsFunc(d.problems(), func(p string) bool { return strings.Contains(p, "no criterion is judged_by_new_tests") }) {
		t.Errorf("a new test file no criterion is judged by was let stand: %v", d.problems())
	}
}
