package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/igannytskyi/iflow/internal/fsx"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// Home is the directory holding the framework that judges. IFLOW_HOME names
// it when set; otherwise it is the nearest directory above this program,
// then above the working directory, then above this source file, that holds
// the framework. A home that cannot be found is an empty string, never a
// guess: a gate judging by a framework it did not find would answer every
// question about the estate with silence.
var Home = sync.OnceValue(func() string {
	if h := os.Getenv("IFLOW_HOME"); h != "" {
		return h
	}
	var starts []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		starts = append(starts, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if _, file, _, ok := runtime.Caller(0); ok && filepath.IsAbs(file) {
		starts = append(starts, filepath.Dir(file))
	}
	for _, s := range starts {
		if h, ok := frameworkAbove(s); ok {
			return h
		}
	}
	return ""
})

// frameworkAbove finds the nearest directory at or above dir holding the
// framework.
func frameworkAbove(dir string) (string, bool) {
	for {
		if isFile(filepath.Join(dir, "docs", "method", "conventions.md")) && isFile(filepath.Join(dir, "go.mod")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// FrameworkState is one name for the state of everything that judges a change
// here — every Go source of the framework and the module that pins its
// dependencies — and any edit to it is a different name.
func FrameworkState(home string) string {
	var files []string
	for _, dir := range []string{"cmd", "internal"} {
		_ = filepath.WalkDir(filepath.Join(home, dir), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".go") {
				files = append(files, p)
			}
			return nil
		})
	}
	files = append(files, filepath.Join(home, "iflow.go"), filepath.Join(home, "go.mod"), filepath.Join(home, "go.sum"))
	slices.SortFunc(files, func(x, y string) int {
		return slices.Compare(strings.Split(filepath.ToSlash(x), "/"), strings.Split(filepath.ToSlash(y), "/"))
	})
	h := sha256.New()
	for _, f := range files {
		rel, _ := filepath.Rel(home, f)
		h.Write([]byte(filepath.ToSlash(rel)))
		b, _ := os.ReadFile(f)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ArbitersKey names the note that the arbiters passed against this framework.
func ArbitersKey(home string) string { return "arbiters:" + FrameworkState(home) }

// RepeatKey names the note that a command was repeated: the command and the
// content of what it runs, as one.
func RepeatKey(cmd []string) string {
	body, _ := os.ReadFile(cmd[0])
	h := sha256.Sum256(append([]byte(strings.Join(cmd, " ")), body...))
	return "repeat:" + hex.EncodeToString(h[:])[:16]
}

func verifiedFile(home string) string { return filepath.Join(home, ".estate", "verified.json") }

// Verified is what has been run and agreed with itself, against which state.
// Derived and disposable: delete it and the runs must happen again, which is
// the point. Nothing here is a record of a past run to be believed.
func Verified(home string) map[string]json.RawMessage {
	held := map[string]json.RawMessage{}
	b, err := os.ReadFile(verifiedFile(home))
	if err != nil || json.Unmarshal(b, &held) != nil {
		return map[string]json.RawMessage{}
	}
	return held
}

var noting sync.Mutex

// NoteVerified records that a run has just been made against exactly this
// content.
func NoteVerified(home, key, what string) error {
	noting.Lock()
	defer noting.Unlock()
	held := Verified(home)
	v, err := json.Marshal(map[string]string{"what": what, "at": time.Now().Format("2006-01-02 15:04")})
	if err != nil {
		return err
	}
	held[key] = v
	b, err := json.MarshalIndent(held, "", " ")
	if err != nil {
		return err
	}
	return fsx.WriteAtomic(verifiedFile(home), b, 0o644)
}

// What is not a result. A run that takes 0.79 seconds and then 0.80 is the
// same run, and a temporary directory named afresh each time is not a
// difference in what happened. Everything else counts: this masks four things
// and no more, because the point of re-running is to find what did change.
var notAResult = []struct {
	pattern *regexp.Regexp
	instead string
}{
	{regexp.MustCompile(`\b\d+\.\d+\s*s\b`), "<duration>"},
	{regexp.MustCompile(`\b\d+(\.\d+)?\s*(ms|seconds?|minutes?)\b`), "<duration>"},
	{regexp.MustCompile(`/(?:tmp|var/folders)/[\w./-]+`), "<a temporary place>"},
	{regexp.MustCompile(`0x[0-9a-f]{6,}`), "<an address>"},
}

// SameResult masks what is not a result.
func SameResult(text string) string {
	for _, n := range notAResult {
		text = n.pattern.ReplaceAllString(text, n.instead)
	}
	return text
}
