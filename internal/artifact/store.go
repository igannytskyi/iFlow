package artifact

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/meta"
)

// A stage document is kept as <stage>.json, in the shape of its type in
// package meta, and rendered as <stage>.md beside it. The JSON is the
// record; the Markdown is for a person and is never read back once the
// JSON exists.

// Document is a stage document of the model.
type Document interface {
	Markdown() string
}

// Invalid is a document refused because it breaks the model.
type Invalid struct {
	Stage    Stage
	Problems []meta.Problem
}

func (e *Invalid) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s breaks the data model:", e.Stage)
	for _, p := range e.Problems {
		b.WriteString("\n  " + p.String())
	}
	return b.String()
}

// JSONFile is where a stage's record is kept.
func JSONFile(dir string, s Stage) string { return filepath.Join(dir, string(s)+".json") }

// Save writes a stage document: checked against the model first, and
// refused whole if it breaks it; then its JSON and its Markdown, each
// replaced at once so that no reader sees half of either.
func Save(dir string, s Stage, d Document) error {
	if ps := meta.Validate(d); len(ps) > 0 {
		return &Invalid{s, ps}
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if err := fsx.WriteAtomic(JSONFile(dir, s), append(b, '\n'), 0o644); err != nil {
		return err
	}
	return fsx.WriteAtomic(filepath.Join(dir, string(s)+".md"), []byte(d.Markdown()), 0o644)
}

// LoadJSON reads a stage document into d. It reports false when the stage
// has no JSON, which is a change written before the model, to be migrated.
func LoadJSON(dir string, s Stage, d any) (bool, error) {
	b, err := os.ReadFile(JSONFile(dir, s))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if b, err = upgrade(s, b); err != nil {
		return true, fmt.Errorf("%s: %w", JSONFile(dir, s), err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // a field the model does not have is an error, not a variation
	if err := dec.Decode(d); err != nil {
		return true, fmt.Errorf("%s: %w", JSONFile(dir, s), err)
	}
	return true, nil
}

// RecordFile is where the record's crossings are kept, one a line.
const RecordFile = "record.jsonl"

// records serialises appends to the records of this process; the ordinal of
// a crossing is the number of crossings before it plus one.
var records sync.Mutex

// ReadRecord is every crossing of a change's record, in order.
func ReadRecord(dir string) ([]meta.Crossing, error) {
	f, err := os.Open(filepath.Join(dir, RecordFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []meta.Crossing
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var c meta.Crossing
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return out, fmt.Errorf("%s line %d: %w", RecordFile, n, err)
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// AppendRecord adds crossings to a change's record, numbering each after the
// last, and renders record.md anew from the whole record.
func AppendRecord(dir, name string, cs ...meta.Crossing) error {
	records.Lock()
	defer records.Unlock()
	all, err := ReadRecord(dir)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, RecordFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, c := range cs {
		c.N = len(all) + 1
		b, err := json.Marshal(c)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			f.Close()
			return err
		}
		all = append(all, c)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return fsx.WriteAtomic(filepath.Join(dir, "record.md"), []byte(meta.RecordMarkdown(name, all)), 0o644)
}

// writeRecord writes crossings as record.jsonl, whole.
func writeRecord(dir string, cs []meta.Crossing) error {
	var lines []byte
	for _, c := range cs {
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		lines = append(append(lines, b...), '\n')
	}
	return fsx.WriteAtomic(filepath.Join(dir, RecordFile), lines, 0o644)
}
