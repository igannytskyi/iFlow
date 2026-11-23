package estate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/reader"
)

// Record is what one file was found to hold, and what it was derived from.
// It is the cache's unit: a file whose stamp and reader have not moved is not
// read again.
type Record struct {
	Commit string `json:"commit"`
	// Digest is the file's contents, as derived: what the record stands for.
	Digest     string          `json:"digest"`
	When       string          `json:"when"`
	ReadBy     string          `json:"read by"`
	Symbols    []reader.Symbol `json:"symbols"`
	Calls      []reader.Call   `json:"calls"`
	Unresolved []string        `json:"unresolved"`
}

// Ref is one reference: which file makes it, to what name, and how.
type Ref struct {
	From string
	Name string
	Kind reader.Kind
}

// Unresolved is one thing a reader could not resolve, and where.
type Unresolved struct {
	File string
	Why  string
}

// Index is every symbol the estate defines, every reference between files,
// and what could not be resolved.
type Index struct {
	// Defines maps a name to every file defining it, once per definition: a
	// name defined twice in one file is two candidates for a caller.
	Defines map[string][]string
	// Names are the defined names in the order they were first met.
	Names      []string
	Refs       []Ref
	Unresolved []Unresolved
	// Files holds each source's record, for the lines things sit on.
	Files map[string]*Record
}

func (r *Repo) cacheFile() string { return filepath.Join(r.Home(), "index.json") }

// Cached is the index as last derived, or nothing.
func (r *Repo) Cached() map[string]*Record {
	out := map[string]*Record{}
	b, err := os.ReadFile(r.cacheFile())
	if err != nil {
		return out
	}
	if json.Unmarshal(b, &out) != nil {
		return map[string]*Record{}
	}
	return out
}

// Index derives what moved since the cache was written and reads the rest
// from it. Files are read concurrently, one parser per worker; the answer is
// assembled in path order, so it does not depend on which finished first.
func (r *Repo) Index() (*Index, error) {
	r.indexOnce.Do(func() { r.index, r.indexErr = r.derive() })
	return r.index, r.indexErr
}

// derive reads every source whose contents moved since what it was last
// derived from — the cache, or, for a repository over another, the other's
// index — and keeps the rest as it was.
func (r *Repo) derive() (*Index, error) {
	how := reader.Derivation()
	was := r.Cached()
	if r.base != nil {
		bix, err := r.base.Index()
		if err != nil {
			return nil, err
		}
		was = bix.Files
	}
	sources := r.Sources()
	recs := make([]*Record, len(sources))
	problems := make([]string, len(sources))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range jobs {
				rel := sources[i]
				src, err := os.ReadFile(r.abs(rel))
				if err != nil {
					problems[i] = "file could not be read"
					continue
				}
				digest := digestOf(src)
				if keep := was[rel]; keep != nil && keep.Digest == digest && keep.ReadBy == how && keep.Calls != nil {
					recs[i] = keep
					continue
				}
				commit, when := r.stamp(rel, digest)
				got := reader.Read(rel, src)
				recs[i] = &Record{Commit: commit, Digest: digest, When: when, ReadBy: how,
					Symbols: nonNil(got.Symbols), Calls: nonNil(got.Calls), Unresolved: nonNil(got.Unresolved)}
			}
		})
	}
	for i := range sources {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	ix := &Index{Defines: map[string][]string{}, Files: map[string]*Record{}}
	for i, rel := range sources {
		if problems[i] != "" {
			ix.Unresolved = append(ix.Unresolved, Unresolved{rel, problems[i]})
			continue
		}
		rec := recs[i]
		for _, s := range rec.Symbols {
			if _, ok := ix.Defines[s.Name]; !ok {
				ix.Names = append(ix.Names, s.Name)
			}
			ix.Defines[s.Name] = append(ix.Defines[s.Name], rel)
		}
		for _, c := range rec.Calls {
			ix.Refs = append(ix.Refs, Ref{rel, c.Name, c.Kind})
		}
		for _, w := range rec.Unresolved {
			ix.Unresolved = append(ix.Unresolved, Unresolved{rel, w})
		}
		ix.Files[rel] = rec
	}
	if r.base == nil && len(ix.Files) > 0 {
		// The cache is disposable: an answer derived and not kept is still
		// the answer, and the next query derives it again. A repository over
		// another is for one piece of work, and keeps none.
		_ = r.writeCache(ix.Files)
	}
	return ix, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (r *Repo) writeCache(files map[string]*Record) error {
	b, err := json.MarshalIndent(files, "", " ")
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(r.cacheFile(), b, 0o644)
}

// Refresh derives again what moved and nothing else, and says what that was.
func (r *Repo) Refresh() (rederived []string, total int, err error) {
	was, how := r.Cached(), reader.Derivation()
	for _, rel := range r.Sources() {
		src, err := os.ReadFile(r.abs(rel))
		if keep := was[rel]; err == nil && keep != nil && keep.Digest == digestOf(src) && keep.ReadBy == how {
			continue
		}
		rederived = append(rederived, rel)
	}
	_, err = r.Index()
	return rederived, len(r.Sources()), err
}
