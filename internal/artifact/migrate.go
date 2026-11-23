package artifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/meta"
)

// LegacyDir is where Migrate keeps a change's Markdown documents as they
// were before the data model, word for word.
const LegacyDir = "legacy"

// Migration is what migrating one change found and did.
type Migration struct {
	Dir string
	// Written are the documents written as JSON.
	Written []string
	// Problems are what could not be read, or breaks the model: a change
	// with any is not migrated, and says why.
	Problems []string
}

// Migrate reads every Markdown stage document of a change that has no JSON
// yet into its type, checks it against the model, and — unless dry — keeps
// the Markdown under legacy/, writes the JSON and renders the Markdown
// anew. A change any of whose documents cannot be read or breaks the model
// is left as it is, and the problems are returned.
func Migrate(dir string, dry bool) (Migration, error) {
	m := Migration{Dir: dir}
	name := filepath.Base(dir)
	type pending struct {
		stage Stage
		doc   Document
	}
	var todo []pending
	for _, s := range Stages {
		md := filepath.Join(dir, string(s)+".md")
		b, err := os.ReadFile(md)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return m, err
		}
		if _, err := os.Stat(JSONFile(dir, s)); err == nil {
			continue // already in the model
		}
		doc, problems := decode(s, name, string(b))
		m.Problems = append(m.Problems, problems...)
		for _, l := range lost(string(b), doc.Markdown()) {
			m.Problems = append(m.Problems, string(s)+": lost in migrating: "+l)
		}
		for _, p := range meta.Validate(doc) {
			m.Problems = append(m.Problems, string(s)+": "+p.String())
		}
		todo = append(todo, pending{s, doc})
	}
	var crossings []meta.Crossing
	recordMD := filepath.Join(dir, "record.md")
	_, hasJSONL := os.Stat(filepath.Join(dir, RecordFile))
	b, err := os.ReadFile(recordMD)
	switch {
	case err == nil && hasJSONL != nil:
		var problems []string
		crossings, problems = DecodeRecord(string(b))
		m.Problems = append(m.Problems, problems...)
		for i, c := range crossings {
			if c.N != i+1 {
				m.Problems = append(m.Problems, fmt.Sprintf("record: line %d has ordinal %d", i+1, c.N))
				break
			}
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return m, err
	}
	if len(m.Problems) > 0 || dry {
		for _, p := range todo {
			m.Written = append(m.Written, string(p.stage))
		}
		if crossings != nil {
			m.Written = append(m.Written, "record")
		}
		return m, upgradePlan(dir, name, true, &m) // said, not written
	}
	for _, p := range todo {
		if err := keep(dir, string(p.stage)+".md"); err != nil {
			return m, err
		}
		if err := Save(dir, p.stage, p.doc); err != nil {
			return m, err
		}
		m.Written = append(m.Written, string(p.stage))
	}
	if crossings != nil {
		if err := keep(dir, "record.md"); err != nil {
			return m, err
		}
		if err := writeRecord(dir, crossings); err != nil {
			return m, err
		}
		if err := fsx.WriteAtomic(recordMD, []byte(meta.RecordMarkdown(name, crossings)), 0o644); err != nil {
			return m, err
		}
		m.Written = append(m.Written, "record")
	}
	if len(m.Written) > 0 {
		// The record says the documents moved, and fixes the digest of the
		// specification as it now is: R1 follows the specification from its
		// admission, through legacy/, to its JSON.
		digest := ""
		if b, err := os.ReadFile(JSONFile(dir, Specification)); err == nil {
			digest = Digest(b)
		}
		if err := AppendRecord(dir, name, meta.Crossing{At: today(), Stage: "migrated", Object: "the data model", Digest: digest,
			Note: fmt.Sprintf("its documents written in the data model, schema %d; the Markdown they were read from is kept under %s/", meta.SchemaVersion, LegacyDir)}); err != nil {
			return m, err
		}
	}
	return m, upgradePlan(dir, name, dry, &m)
}

// upgradePlan writes a plan kept in an older version of its model in this
// one — read whole, nothing dropped — keeps the older JSON under legacy/,
// and says so in the record. The other documents keep their version, and
// with it the specification its digest.
func upgradePlan(dir, name string, dry bool, m *Migration) error {
	b, err := os.ReadFile(JSONFile(dir, Plan))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var head struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(b, &head); err != nil || head.Schema >= meta.PlanVersion {
		return err
	}
	var p meta.Plan
	if _, err := LoadJSON(dir, Plan, &p); err != nil {
		m.Problems = append(m.Problems, err.Error())
		return nil
	}
	if ps := meta.Validate(p); len(ps) > 0 {
		for _, pr := range ps {
			m.Problems = append(m.Problems, string(Plan)+": "+pr.String())
		}
		return nil
	}
	m.Written = append(m.Written, string(Plan))
	if dry {
		return nil
	}
	if err := keep(dir, filepath.Base(JSONFile(dir, Plan))); err != nil {
		return err
	}
	if err := Save(dir, Plan, p); err != nil {
		return err
	}
	return AppendRecord(dir, name, meta.Crossing{At: today(), Stage: "migrated", Object: "the plan",
		Note: fmt.Sprintf("its plan written in version %d of the model, from version %d, which is kept under %s/", meta.PlanVersion, head.Schema, LegacyDir)})
}

func today() string { return time.Now().Format("2006-01-02") }

// lost is what the Markdown a change kept says that the document read from
// it does not: a cell of a table, or a line of prose. Migrating loses
// nothing, or does not happen.
func lost(was, now string) []string {
	now = strings.ReplaceAll(now, "\\|", "|")
	var out []string
	for _, sec := range sectionsOf(was) {
		for _, p := range sec.prose {
			if !strings.Contains(now, p) {
				out = append(out, p)
			}
		}
		for _, t := range sec.tables {
			for _, r := range t.Rows {
				if r.Placeholder() {
					continue
				}
				for _, c := range r.Values() {
					if c != "" && !strings.Contains(now, c) {
						out = append(out, c)
					}
				}
			}
		}
	}
	return out
}

// keep copies a document under legacy/, once: a second migration does not
// overwrite what the first kept.
func keep(dir, file string) error {
	dst := filepath.Join(dir, LegacyDir, file)
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(dst, b, 0o644)
}

func decode(s Stage, name, text string) (Document, []string) {
	switch s {
	case Intent:
		return DecodeIntent(name, text)
	case Specification:
		return DecodeSpecification(name, text)
	case Plan:
		return DecodePlan(name, text)
	case Admission:
		return DecodeAdmission(name, text)
	case Execution:
		return DecodeExecution(name, text)
	case Assurance:
		return DecodeAssurance(name, text)
	case Landing:
		return DecodeLanding(name, text)
	}
	return nil, []string{"no decoder for " + string(s)}
}
