package artifact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/meta"
)

// upgrade is a stage document as the model now has it: one written in an
// older version of its model, carried into this one, the rest as they are.
// What it reads of an older document is all of it — nothing is dropped —
// and what the older model left unsaid is said as unknown, never guessed.
func upgrade(s Stage, b []byte) ([]byte, error) {
	var head struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	if s != Plan || head.Schema != 1 {
		return b, nil
	}
	var old planV1
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&old); err != nil {
		return nil, fmt.Errorf("a plan of version 1: %w", err)
	}
	return json.Marshal(old.upgrade())
}

// planV1 is a plan as version 1 of the model kept it: a unit said whether it
// wrote the tests, and its contract was what it provides in prose.
type planV1 struct {
	meta.Plan
	Units []workUnitV1 `json:"units"`
}

type workUnitV1 struct {
	ID             string     `json:"id"`
	Phase          int        `json:"phase"`
	Class          meta.Class `json:"class"`
	Scope          []string   `json:"scope"`
	AreaOfEffect   string     `json:"area_of_effect,omitempty"`
	Criteria       []string   `json:"criteria"`
	DependsOn      []string   `json:"depends_on,omitempty"`
	PreparatoryFor []string   `json:"preparatory_for,omitempty"`
	Title          string     `json:"title,omitempty"`
	Brief          string     `json:"brief,omitempty"`
	Provides       []string   `json:"provides,omitempty"`
	Tests          bool       `json:"writes_tests,omitempty"`
}

func (p planV1) upgrade() meta.Plan {
	out := p.Plan
	out.Schema = meta.PlanVersion
	out.Units = nil
	for _, u := range p.Units {
		kind := meta.CodeUnit
		switch {
		case u.Tests:
			kind = meta.TestsUnit
		case strings.HasPrefix(u.Title, "the contract —"):
			// How version 1 knew the unit that declares the contract.
			kind = meta.ContractUnit
		}
		w := meta.WorkUnit{ID: u.ID, Phase: u.Phase, Class: u.Class, Scope: u.Scope, AreaOfEffect: u.AreaOfEffect,
			Criteria: u.Criteria, DependsOn: u.DependsOn, PreparatoryFor: u.PreparatoryFor, Kind: kind, Title: u.Title, Brief: u.Brief}
		for _, text := range u.Provides {
			w.Provides = append(w.Provides, provisionOf(text, u.Scope))
		}
		out.Units = append(out.Units, w)
	}
	return out
}

// provisionOf is a provision version 1 wrote in prose, as a named entry
// where the prose is a declaration and the unit has one file it could be
// in, and as described — its words kept, naming nothing — where not.
func provisionOf(text string, scope []string) meta.Provision {
	described := meta.Provision{Kind: meta.Described, Signature: text}
	m := declaration.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return described
	}
	var files []string
	for _, f := range scope {
		if base := path.Base(f); !strings.Contains(base, "_test.") && !strings.HasPrefix(base, "test_") {
			files = append(files, f)
		}
	}
	if len(files) != 1 {
		return described // which file it is in, the plan did not say
	}
	kind := map[string]meta.DeclKind{"type": meta.TypeDecl, "var": meta.Variable, "const": meta.Constant, "field": meta.Field}[m[1]]
	name := m[5]
	switch {
	case m[1] == "func" && m[2] != "":
		kind, name = meta.Method, m[3]
	case m[1] == "func":
		kind, name = meta.Function, m[4]
	}
	if name == "" || !slices.Contains(meta.Codes["DeclKind"], string(kind)) {
		return described
	}
	return meta.Provision{File: files[0], Name: name, Kind: kind, Signature: text}
}

// declaration is how version 1's provisions began when they were Go
// declarations: "func (u *UI) quiet() bool", "func New(", "type Sender …",
// "var lookPath …", "field Notify notify.Sender …".
var declaration = regexp.MustCompile(`^(func|type|var|const|field)\s+(?:(\([^)]*\))\s*(\w+)\s*\(|(\w+)\s*\(|(\w+))`)

// kindOf is what a unit read from a plan written before the data model
// writes: the tests, where it prepares criteria; its code, otherwise.
func kindOf(preparatoryFor []string) meta.UnitKind {
	if len(preparatoryFor) > 0 {
		return meta.TestsUnit
	}
	return meta.CodeUnit
}
