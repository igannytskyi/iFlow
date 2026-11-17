package gate

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/meta"
)

// ordered is a mapping that remembers the order keys first arrived in.
type ordered[V any] struct {
	keys []string
	vals map[string]V
}

func newOrdered[V any]() *ordered[V] { return &ordered[V]{vals: map[string]V{}} }

func (o *ordered[V]) put(k string, v V) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *ordered[V]) get(k string) (V, bool) { v, ok := o.vals[k]; return v, ok }
func (o *ordered[V]) len() int               { return len(o.keys) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func distinct(items []string) []string {
	out := slices.Clone(items)
	slices.Sort(out)
	return slices.Compact(out)
}

func nonEmpty(items []string) []string {
	return slices.DeleteFunc(slices.Clone(items), func(s string) bool { return s == "" })
}

// scope is what the specification says the change may touch.
func scope(c *artifact.Change) []string {
	if c.Meta.Specification == nil {
		return nil
	}
	return distinct(nonEmpty(c.Meta.Specification.Scope.Included))
}

// arbiter is what the specification says will judge the change.
func arbiter(c *artifact.Change) []string {
	if c.Meta.Specification == nil {
		return nil
	}
	var out []string
	for _, a := range c.Meta.Specification.Arbiters {
		out = append(out, a.Path)
	}
	return distinct(nonEmpty(out))
}

// criteria is every criterion the specification fixes.
func criteria(c *artifact.Change) []string {
	if c.Meta.Specification == nil {
		return nil
	}
	var out []string
	for _, cr := range c.Meta.Specification.Criteria {
		out = append(out, cr.ID)
	}
	return distinct(nonEmpty(out))
}

// candidates is every candidate execution produced.
func candidates(c *artifact.Change) []string {
	if c.Meta.Execution == nil {
		return nil
	}
	var out []string
	for _, ca := range c.Meta.Execution.Candidates {
		out = append(out, ca.ID)
	}
	return distinct(nonEmpty(out))
}

// stated reports whether the change's evidence says, for any run, whether
// it repeats: evidence written before the method asked says it of none.
func stated(c *artifact.Change) bool {
	return c.Meta.Assurance != nil && slices.ContainsFunc(c.Meta.Assurance.Evidence, func(e meta.Evidence) bool { return e.Repeatable != "" })
}

// repeatable is every piece of evidence and what it says about repeating;
// nothing, for evidence written before the method asked.
func repeatable(c *artifact.Change) *ordered[string] {
	out := newOrdered[string]()
	if !stated(c) {
		return out
	}
	for _, e := range c.Meta.Assurance.Evidence {
		out.put(e.ID, string(e.Repeatable))
	}
	return out
}

// Producers is evidence that claims to repeat, and what each says produced
// it.
func Producers(c *artifact.Change) (ids []string, said map[string]string) {
	o := newOrdered[string]()
	if c.Meta.Assurance != nil {
		for _, e := range c.Meta.Assurance.Evidence {
			if e.Repeatable == meta.Yes {
				o.put(e.ID, e.Producer)
			}
		}
	}
	return o.keys, o.vals
}

var backticked = regexp.MustCompile("`([^`]+)`")

// ProducerCommand is the command an evidence row names, as something that can
// be run, or nil. Named in backticks, because a sentence is not a command and
// guessing which of its words were arguments got it wrong both ways.
func ProducerCommand(c *artifact.Change, id string) []string {
	_, said := Producers(c)
	for _, m := range backticked.FindAllStringSubmatch(said[id], -1) {
		parts := slices.DeleteFunc(strings.Fields(m[1]), func(w string) bool {
			return w == "python3" || w == "python"
		})
		if len(parts) == 0 {
			continue
		}
		// The first word is what runs, and it need not be a Python file: an
		// estate is judged by whatever it is judged by.
		for _, base := range []string{".", c.Dir, c.Root} {
			if p := filepath.Join(base, parts[0]); isFile(p) {
				// Absolute, so that the command is the same command — and its
				// repeat the same note — wherever the gate is run from.
				if abs, err := filepath.Abs(p); err == nil {
					p = abs
				}
				return append([]string{p}, parts[1:]...)
			}
		}
		// A tool the estate is judged by is often installed rather than
		// kept in it — the language's own test runner.
		if p, err := exec.LookPath(parts[0]); err == nil && filepath.Base(parts[0]) == parts[0] {
			return append([]string{p}, parts[1:]...)
		}
	}
	return nil
}

// producerNamed reports whether an evidence names, in backticks, something
// to re-run — whether or not it can be found now.
func producerNamed(c *artifact.Change, id string) bool {
	_, said := Producers(c)
	return backticked.MatchString(said[id])
}

// migratedAs reports whether the record says the change was migrated into
// the data model with its specification at this digest.
func migratedAs(c *artifact.Change, digest string) bool {
	for _, r := range c.Meta.Record {
		if r.Stage == "migrated" && r.Digest == digest {
			return true
		}
	}
	return false
}

// readmissionRecorded reports whether the record shows the specification
// written again after an admission.
func readmissionRecorded(c *artifact.Change) bool {
	admitted := false
	for _, r := range c.Meta.Record {
		switch r.Stage {
		case "admission":
			admitted = true
		case "specification":
			if admitted {
				return true
			}
		}
	}
	return false
}

// pure cleans a path as a pure path does: separators collapsed, "." parts
// and a trailing "/" dropped, ".." kept.
func pure(p string) string {
	abs := strings.HasPrefix(p, "/")
	parts := slices.DeleteFunc(strings.Split(p, "/"), func(s string) bool { return s == "" || s == "." })
	out := strings.Join(parts, "/")
	switch {
	case abs:
		return "/" + out
	case out == "":
		return "."
	}
	return out
}

func pureParent(p string) string {
	p = pure(p)
	switch i := strings.LastIndex(p, "/"); {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	default:
		return p[:i]
	}
}

// regionOf is the region a path names: itself when it is a directory, its
// parent when it is a file.
func regionOf(root, p string) string {
	q := pure(strings.TrimRight(p, "/"))
	if isDir(filepath.Join(root, q)) {
		return q
	}
	return pureParent(q)
}

func base(p string) string { return path.Base(pure(p)) }

// or is an identifier as a message names it: None for one left empty.
func or(id string) string {
	if id == "" {
		return "None"
	}
	return id
}
