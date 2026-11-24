package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/harness"
	"github.com/igannytskyi/iflow/internal/protocol"
	"github.com/igannytskyi/iflow/internal/textdiff"
)

// document is a change as its folder holds it, for a screen to show.
func document(dir string) (protocol.ChangeDoc, error) {
	sum, ok := changeOf(dir)
	if !ok {
		return protocol.ChangeDoc{}, &protocol.Error{Code: protocol.Refused, Message: dir + " is no change that can be read"}
	}
	c, err := artifact.Load(dir)
	if err != nil {
		return protocol.ChangeDoc{}, err
	}
	doc := protocol.ChangeDoc{Change: sum, Specification: c.Meta.Specification, Plan: c.Meta.Plan,
		Admission: c.Meta.Admission, Execution: c.Meta.Execution, Assurance: c.Meta.Assurance, Landing: c.Meta.Landing}
	// The change described; a change carried before it had one is described
	// by its specification's own document.
	for _, f := range []string{artifact.ChangeFile, string(artifact.Specification) + ".md"} {
		if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
			doc.Document = string(b)
			break
		}
	}
	if doc.Record, err = record(dir); err != nil {
		return doc, err
	}
	costs, err := artifact.Costs(dir)
	if err != nil {
		return doc, err
	}
	for _, k := range costs {
		doc.Costs = append(doc.Costs, protocol.Cost{Area: k.Area, Unit: k.Unit, What: k.What, Actor: k.Actor, Model: k.Model,
			Handed: k.Handed, Input: k.Input, CacheRead: k.CacheRead, CacheWrite: k.CacheWrite, Output: k.Output,
			CostUSD: k.CostUSD, ElapsedMS: k.ElapsedMS})
	}
	return doc, nil
}

// trace is what the agent of a candidate did, as the candidate keeps it.
func trace(p protocol.TraceParams) ([]protocol.Event, error) {
	if filepath.Base(p.Candidate) != p.Candidate || p.Candidate == "" {
		return nil, &protocol.Error{Code: protocol.InvalidParams, Message: "a candidate is named by its id alone"}
	}
	f, err := os.Open(filepath.Join(p.Dir, "candidates", p.Candidate, "trace.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []protocol.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var e harness.Event
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, protocol.Event{At: e.At, Kind: string(e.Kind), Tool: e.Tool, Detail: e.Detail})
		}
	}
	return out, sc.Err()
}

// diff is what a candidate changed, file by file, against the estate as it
// stood before the change: the scope's files as the admission kept them,
// and, for a file the change made, nothing.
func diff(p protocol.TraceParams) ([]protocol.FileDiff, error) {
	if filepath.Base(p.Candidate) != p.Candidate || p.Candidate == "" {
		return nil, &protocol.Error{Code: protocol.InvalidParams, Message: "a candidate is named by its id alone"}
	}
	files := filepath.Join(p.Dir, "candidates", p.Candidate, "files")
	var out []protocol.FileDiff
	err := filepath.WalkDir(files, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "CHANGES" {
			return err
		}
		rel, _ := filepath.Rel(files, path)
		rel = filepath.ToSlash(rel)
		after, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		before, existed, known := beforeChange(p.Dir, rel)
		if !known {
			out = append(out, protocol.FileDiff{Path: rel, Unrecorded: true})
			return nil
		}
		text, added, removed := textdiff.Unified(string(before), string(after), 3)
		out = append(out, protocol.FileDiff{Path: rel, Unified: text, Added: added, Removed: removed, New: !existed})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return out, err
}

// beforeChange is rel as it stood before the change in dir, whether it
// existed, and whether that is known: as the admission kept it, or, where
// it kept nothing of it, as the estate holds it while the change has not
// landed. Once it has, the estate holds what the change made, and how the
// file stood before is not known.
func beforeChange(dir, rel string) (before []byte, existed, known bool) {
	base := filepath.Join(dir, "baseline", filepath.Base(rel))
	if b, err := os.ReadFile(base + ".before"); err == nil {
		return b, true, true
	}
	if _, err := os.Stat(base + ".absent"); err == nil {
		return nil, false, true
	}
	if _, err := os.Stat(filepath.Join(dir, string(artifact.Landing)+".json")); err == nil {
		return nil, false, false
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(dir)), filepath.FromSlash(rel)))
	return b, err == nil, true
}
