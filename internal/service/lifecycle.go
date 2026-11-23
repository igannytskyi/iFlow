package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/igannytskyi/iflow/internal/protocol"
)

// A service is replaced, never patched: a screen built later than the
// service it finds asks it to end, keeping the runs it carries, and starts
// the one it was built with, which carries them on. The runs are kept by
// their folders, from which any run can be carried on, so that nothing a
// person started is lost to a new build.

var (
	buildOnce sync.Once
	buildID   string
	builtAt   time.Time
)

// Build is the program this process runs: the hash of its executable, and
// when the executable was written.
func Build() (string, time.Time) {
	buildOnce.Do(func() {
		self, err := os.Executable()
		if err != nil {
			return
		}
		f, err := os.Open(self)
		if err != nil {
			return
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return
		}
		buildID = hex.EncodeToString(h.Sum(nil))[:12]
		if st, err := f.Stat(); err == nil {
			builtAt = st.ModTime()
		}
	})
	return buildID, builtAt
}

// Older says whether the service that answered res runs an older program
// than this one: another version of the protocol, or another build written
// before this one. A service built later is left as it is, whichever screen
// finds it: two screens of two builds never take turns replacing it.
func Older(res protocol.InitializeResult) bool {
	if res.Version != protocol.Version {
		return true
	}
	id, at := Build()
	return id != "" && res.Build != "" && res.Build != id && at.After(res.BuiltAt)
}

// identity is what the service says of itself.
func (s *Service) identity() protocol.InitializeResult {
	id, at := Build()
	s.mu.Lock()
	running := 0
	for _, e := range s.runs {
		e.mu.Lock()
		if !e.finished {
			running++
		}
		e.mu.Unlock()
	}
	s.mu.Unlock()
	return protocol.InitializeResult{Version: protocol.Version, Server: "iflow", Build: id, BuiltAt: at, PID: os.Getpid(), Since: s.since, Running: running}
}

// shutdown ends the service: the runs it carries are stopped — kept first,
// where keep, in the file the next service reads — and, once they have
// ended or a minute has passed, the service stops listening.
func (s *Service) shutdown(keep bool) protocol.ShutdownResult {
	var res protocol.ShutdownResult
	var live []*entry
	s.mu.Lock()
	for _, id := range s.order {
		e := s.runs[id]
		e.mu.Lock()
		done := e.finished
		e.mu.Unlock()
		if done {
			continue
		}
		live = append(live, e)
		v := e.run.View()
		k := protocol.Kept{Dir: v.Dir}
		if v.Dir == "" {
			k = protocol.Kept{Project: e.project, Statement: v.Statement}
		}
		res.Kept = append(res.Kept, k)
	}
	s.mu.Unlock()
	if keep && s.keepFile != "" && len(res.Kept) > 0 {
		if b, err := json.MarshalIndent(res.Kept, "", "  "); err == nil {
			_ = os.WriteFile(s.keepFile, b, 0o600)
		}
	}
	if !keep {
		res.Kept = nil
	}
	go func() {
		for _, e := range live {
			e.mu.Lock()
			e.stopped = true
			e.mu.Unlock()
			e.cancel()
		}
		deadline := time.Now().Add(time.Minute)
		for time.Now().Before(deadline) && slices.ContainsFunc(live, func(e *entry) bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			return !e.finished
		}) {
			time.Sleep(100 * time.Millisecond)
		}
		if s.quit != nil {
			s.quit()
		}
	}()
	return res
}

// carryOnKept carries on the runs the service before this one kept, and
// forgets them: each once.
func (s *Service) carryOnKept() {
	if s.keepFile == "" {
		return
	}
	b, err := os.ReadFile(s.keepFile)
	if err != nil {
		return
	}
	_ = os.Remove(s.keepFile)
	var kept []protocol.Kept
	if json.Unmarshal(b, &kept) != nil {
		return
	}
	for _, k := range kept {
		switch {
		case k.Dir != "":
			_, _ = s.start(projectOf(k.Dir), func(changed func()) (Run, error) { return s.maker.Resume(k.Dir, nil, changed) })
		case k.Project != "" && k.Statement != "":
			_, _ = s.start(k.Project, func(changed func()) (Run, error) { return s.maker.New(k.Project, k.Statement, nil, changed) })
		}
	}
}

// serveUntil is ctx, ended also when the service is asked to shut down.
func (s *Service) serveUntil(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	s.quit = cancel
	return ctx
}
