package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/meta"
)

// Stage is one file of a change, named as it is on disk.
type Stage string

// The stages, in the order they are written.
const (
	Intent        Stage = "00-intent"
	Specification Stage = "01-specification"
	Plan          Stage = "02-plan"
	Admission     Stage = "03-admission"
	Execution     Stage = "04-execution"
	Assurance     Stage = "05-assurance"
	Landing       Stage = "06-landing"
)

// Stages lists every stage in order.
var Stages = []Stage{Intent, Specification, Plan, Admission, Execution, Assurance, Landing}

// Index is the stage's position in the path.
func (s Stage) Index() int { return slices.Index(Stages, s) }

// Name is the stage as the record names it: "specification", not
// "01-specification".
func (s Stage) Name() string {
	_, name, _ := strings.Cut(string(s), "-")
	return name
}

// Doc is one artefact, read once.
type Doc struct {
	Tables []Table
}

// ParseDoc reads an artefact's tables.
func ParseDoc(text string) *Doc { return &Doc{Tables: ParseTables(text)} }

// Rows returns the data rows of every table that has all the named headers.
// A nil Doc has none.
func (d *Doc) Rows(headers ...string) []Row {
	if d == nil {
		return nil
	}
	var out []Row
	for _, t := range d.Tables {
		if !t.Has(headers...) {
			continue
		}
		for _, r := range t.Rows {
			if !r.Placeholder() {
				out = append(out, r)
			}
		}
	}
	return out
}

// HasTable reports whether any table has all the named headers.
func (d *Doc) HasTable(headers ...string) bool {
	if d == nil {
		return false
	}
	return slices.ContainsFunc(d.Tables, func(t Table) bool { return t.Has(headers...) })
}

// Headers returns every header any table declares.
func (d *Doc) Headers() map[string]bool {
	out := map[string]bool{}
	if d == nil {
		return out
	}
	for _, t := range d.Tables {
		for _, h := range t.Headers {
			out[h] = true
		}
	}
	return out
}

// Column returns every non-empty cell under a header, across the tables that
// have all the named headers (the column itself included).
func (d *Doc) Column(header string, alongside ...string) []string {
	var out []string
	for _, r := range d.Rows(append([]string{header}, alongside...)...) {
		if v := r.Cell(header); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Change is one change folder, read once.
type Change struct {
	// Dir is the folder as it was named.
	Dir string
	// Root is the estate the change is carried against: the repository it
	// lives in, which is not the folder it lives in.
	Root string
	// SpecDigest is the digest of the specification as it is now: of its
	// JSON, where it has been written in the data model.
	SpecDigest string
	// LegacySpecDigest is the digest of the specification's Markdown as it
	// was before the change was migrated into the data model, kept under
	// legacy/; empty for a change never migrated.
	LegacySpecDigest string
	// Meta is the change in the data model: read from its JSON, or, for a
	// change not yet migrated, from its Markdown.
	Meta Meta
}

// Meta is every document of a change in the data model; a stage absent is nil.
type Meta struct {
	Intent        *meta.Intent
	Specification *meta.Specification
	Plan          *meta.Plan
	Admission     *meta.Admission
	Execution     *meta.Execution
	Assurance     *meta.Assurance
	Landing       *meta.Landing
	Record        []meta.Crossing
	// Problems are what could not be read into the model.
	Problems []string
}

// Load reads a change folder. A folder that does not exist is an error; a
// folder holding none of the stages is a change with nothing in it, which the
// gate says.
func Load(dir string) (*Change, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, &fs.PathError{Op: "load", Path: dir, Err: errors.New("not a directory")}
	}
	c := &Change{Dir: filepath.Clean(dir)}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	c.Root = filepath.Dir(filepath.Dir(abs))
	for _, s := range Stages {
		b, err := os.ReadFile(filepath.Join(dir, string(s)+".md"))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if _, err := os.Stat(JSONFile(dir, s)); err != nil {
				continue // the stage is not there, in either form
			}
		case err != nil:
			return nil, err
		case s == Specification:
			c.SpecDigest = Digest(b) // of the Markdown, until the change is in the model
		}
		if err := c.loadMeta(s, b); err != nil {
			return nil, err
		}
	}
	if b, err := os.ReadFile(JSONFile(dir, Specification)); err == nil {
		c.SpecDigest = Digest(b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, LegacyDir, string(Specification)+".md")); err == nil {
		c.LegacySpecDigest = Digest(b)
	}
	if _, err := os.Stat(filepath.Join(dir, RecordFile)); err == nil {
		rec, err := ReadRecord(dir)
		if err != nil {
			return nil, err
		}
		c.Meta.Record = rec
	} else if b, err := os.ReadFile(filepath.Join(dir, "record.md")); err == nil {
		var problems []string
		c.Meta.Record, problems = DecodeRecord(string(b))
		c.Meta.Problems = append(c.Meta.Problems, problems...)
	}
	return c, nil
}

// loadMeta reads one stage into the model: its JSON where it has one, its
// Markdown where it has not been migrated.
func (c *Change) loadMeta(s Stage, md []byte) error {
	name := c.Name()
	read := func(d any, decoded func() (any, []string)) error {
		ok, err := LoadJSON(c.Dir, s, d)
		if err != nil || ok {
			return err
		}
		v, problems := decoded()
		c.Meta.Problems = append(c.Meta.Problems, problems...)
		reflectSet(d, v)
		return nil
	}
	switch s {
	case Intent:
		c.Meta.Intent = &meta.Intent{}
		return read(c.Meta.Intent, func() (any, []string) { return DecodeIntent(name, string(md)) })
	case Specification:
		c.Meta.Specification = &meta.Specification{}
		return read(c.Meta.Specification, func() (any, []string) { return DecodeSpecification(name, string(md)) })
	case Plan:
		c.Meta.Plan = &meta.Plan{}
		return read(c.Meta.Plan, func() (any, []string) { return DecodePlan(name, string(md)) })
	case Admission:
		c.Meta.Admission = &meta.Admission{}
		return read(c.Meta.Admission, func() (any, []string) { return DecodeAdmission(name, string(md)) })
	case Execution:
		c.Meta.Execution = &meta.Execution{}
		return read(c.Meta.Execution, func() (any, []string) { return DecodeExecution(name, string(md)) })
	case Assurance:
		c.Meta.Assurance = &meta.Assurance{}
		return read(c.Meta.Assurance, func() (any, []string) { return DecodeAssurance(name, string(md)) })
	case Landing:
		c.Meta.Landing = &meta.Landing{}
		return read(c.Meta.Landing, func() (any, []string) { return DecodeLanding(name, string(md)) })
	}
	return nil
}

// reflectSet stores v, a document value, where d, a pointer to one of its
// type, points.
func reflectSet(d, v any) { reflect.ValueOf(d).Elem().Set(reflect.ValueOf(v)) }

// Digest is the name the method gives content: the first sixteen hex digits
// of its SHA-256.
func Digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// Has reports whether a stage is present.
func (c *Change) Has(s Stage) bool {
	m := c.Meta
	switch s {
	case Intent:
		return m.Intent != nil
	case Specification:
		return m.Specification != nil
	case Plan:
		return m.Plan != nil
	case Admission:
		return m.Admission != nil
	case Execution:
		return m.Execution != nil
	case Assurance:
		return m.Assurance != nil
	case Landing:
		return m.Landing != nil
	}
	return false
}

// Present lists the stages present, in order.
func (c *Change) Present() []Stage {
	var out []Stage
	for _, s := range Stages {
		if c.Has(s) {
			out = append(out, s)
		}
	}
	return out
}

// Name is the change's slug.
func (c *Change) Name() string { return filepath.Base(c.Dir) }

// All returns every table, placeholders included. A nil Doc has none.
func (d *Doc) All() []Table {
	if d == nil {
		return nil
	}
	return d.Tables
}

// Written returns every row of every table that has all the named headers,
// placeholders included: for a document that is not a template, a cell
// reading <slug> is prose about a slot, not an unfilled one.
func (d *Doc) Written(headers ...string) []Row {
	var out []Row
	for _, t := range d.All() {
		if t.Has(headers...) {
			out = append(out, t.Rows...)
		}
	}
	return out
}
