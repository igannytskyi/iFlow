package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	// Docs holds each stage present.
	Docs map[Stage]*Doc
	// Record is record.md, or nil when there is none.
	Record *Doc
	// SpecDigest is the digest of the specification as it is now.
	SpecDigest string
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
	c := &Change{Dir: filepath.Clean(dir), Docs: map[Stage]*Doc{}}
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
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		c.Docs[s] = ParseDoc(string(b))
		if s == Specification {
			c.SpecDigest = Digest(b)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "record.md")); err == nil {
		c.Record = ParseDoc(string(b))
	}
	return c, nil
}

// Digest is the name the method gives content: the first sixteen hex digits
// of its SHA-256.
func Digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// Has reports whether a stage is present.
func (c *Change) Has(s Stage) bool { _, ok := c.Docs[s]; return ok }

// Doc returns a stage, or nil; every Doc method is safe on nil.
func (c *Change) Doc(s Stage) *Doc { return c.Docs[s] }

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
