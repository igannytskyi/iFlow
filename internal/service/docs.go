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
