// Package estate is what the gate asks of the system a change is carried
// against: what a change reaches, where nothing judges it, what landings
// taught. The gate does not read code itself; it asks, and where the estate
// cannot answer, the gate says so rather than passing on a question nobody
// asked.
package estate

import (
	"context"
	"errors"
)

// Reach is one answer to what a change reaches, and how far to trust it.
type Reach struct {
	File       string `json:"file"`
	Confidence string `json:"confidence"`
}

// Firm reports whether the answer is trusted enough to hold a plan to.
func (r Reach) Firm() bool { return r.Confidence == "high" || r.Confidence == "medium" }

// Region is a directory that directly holds code the index can read.
type Region struct {
	Path       string `json:"region"`
	Verdict    string `json:"verdict"`
	TestGround bool   `json:"is test ground"`
}

// Watched reports whether something already judges a change here.
func (r Region) Watched() bool { return r.Verdict != "unclaimed" || r.TestGround }

// Correction is an edge a landing showed, which no reading of the source
// finds.
type Correction struct {
	By string `json:"by"`
}

// Estate answers the gate's questions about one or more repositories.
// Implementations are safe for concurrent use.
type Estate interface {
	// Affects is what a change to these paths reaches.
	Affects(ctx context.Context, root string, paths []string) ([]Reach, error)
	// NamedBy is what names these paths as text rather than calling them.
	NamedBy(ctx context.Context, root string, paths []string) ([]Reach, error)
	// Regions is every region of the estate and whether it is judged.
	Regions(ctx context.Context, root string) ([]Region, error)
	// Corrections is what landings taught about this estate.
	Corrections(ctx context.Context, root string) ([]Correction, error)
	// Readable reports whether a reader here can read the file.
	Readable(ctx context.Context, path string) (bool, error)
}

// Error is a failure the estate reported, named by its kind, which is what
// the gate quotes when it says a question went unanswered.
type Error struct{ Kind string }

func (e *Error) Error() string { return "estate: " + e.Kind }

// Kind names the kind of an estate failure, or "Exception" for any other.
func Kind(err error) string {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Kind
	}
	return "Exception"
}

// ErrUnavailable means nothing can be asked at all.
var ErrUnavailable = &Error{Kind: "ModuleNotFoundError"}
