package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/igannytskyi/iflow/internal/model"
)

// The tools an agent is given, and nothing more: it reads anything in the
// workspace, writes only what the unit's scope names, and runs the check
// command and no other. The harness enforces this, not the agent.

type tool struct {
	name, about string
	params      string // JSON schema
	run         func(ws *workspace, args map[string]string) (string, error)
}

// workspace is where a run may act.
type workspace struct {
	dir   string
	scope []string
	check []string
	judge func(passed bool, output string) (ok bool, why string)
	ctx   context.Context

	pkgs map[string]bool // the repository's packages, read once
	// last is how the check went when it was last run.
	last struct {
		ran      bool
		ok       bool
		out, why string
	}
}

// local is the name of every package of the repository.
func (w *workspace) local() map[string]bool {
	if w.pkgs == nil {
		w.pkgs = Packages(w.dir)
	}
	return w.pkgs
}

// libraries is where the source of the libraries a Go repository uses is
// kept — the module cache and the standard library — which an agent may
// read, as a person reads a library's source rather than guess its API.
var libraries = sync.OnceValue(func() []string {
	out, err := exec.Command("go", "env", "GOMODCACHE", "GOROOT").Output()
	if err != nil {
		return nil
	}
	var dirs []string
	for _, l := range strings.Fields(string(out)) {
		dirs = append(dirs, filepath.Clean(l))
	}
	return dirs
})

// readable resolves a path the agent may read: one inside the workspace,
// or a library's source.
func (w *workspace) readable(p string) (string, error) {
	if clean := filepath.Clean(p); filepath.IsAbs(clean) {
		for _, lib := range libraries() {
			if clean == lib || strings.HasPrefix(clean, lib+string(filepath.Separator)) {
				return clean, nil
			}
		}
	}
	abs, _, err := w.within(p)
	return abs, err
}

// run runs the check in the workspace and judges it as the unit's task
// says: whether it passes, what it said, and why it does not pass where a
// judge says so.
func (w *workspace) run() (ok bool, out, why string) {
	ok, out = check(w.ctx, w.dir, w.check)
	if w.judge != nil {
		ok, why = w.judge(ok, out)
	}
	w.last.ran, w.last.ok, w.last.out, w.last.why = true, ok, out, why
	return ok, out, why
}

// errRefused marks a call the harness would not carry out.
type errRefused struct{ why string }

func (e errRefused) Error() string { return e.why }

// within resolves a path the agent names to one inside the workspace, or
// refuses it.
func (w *workspace) within(p string) (string, string, error) {
	clean := filepath.Clean(filepath.FromSlash(p))
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(w.dir, clean)
		if err != nil {
			return "", "", errRefused{"that path is outside the workspace"}
		}
		clean = rel
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errRefused{"that path is outside the workspace"}
	}
	return filepath.Join(w.dir, clean), filepath.ToSlash(clean), nil
}

// writable reports whether the unit's scope names a path.
func (w *workspace) writable(rel string) bool {
	return slices.ContainsFunc(w.scope, func(s string) bool {
		s = strings.TrimSuffix(filepath.ToSlash(s), "/")
		return rel == s || strings.HasPrefix(rel, s+"/")
	})
}

var tools = []tool{
	{"read_file", "Read a file in the repository, or of a library's source by its absolute path.", `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, err := w.readable(a["path"])
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			return numbered(string(b)), nil
		}},
	{"list_dir", "List a directory in the repository, or of a library's source by its absolute path.", `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, err := w.readable(a["path"])
			if err != nil {
				return "", err
			}
			entries, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			var out []string
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				out = append(out, name)
			}
			return strings.Join(out, "\n"), nil
		}},
	{"search", "Search the repository for a regular expression; answers file:line: text.", `{"type":"object","properties":{"pattern":{"type":"string"}},"required":["pattern"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			re, err := regexp.Compile(a["pattern"])
			if err != nil {
				return "", err
			}
			var out []string
			_ = filepath.WalkDir(w.dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || len(out) >= 200 {
					return nil
				}
				if d.IsDir() {
					if d.Name() == ".git" || d.Name() == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				f, err := os.Open(p)
				if err != nil {
					return nil
				}
				defer f.Close()
				sc := bufio.NewScanner(f)
				for n := 1; sc.Scan(); n++ {
					if re.Match(sc.Bytes()) {
						rel, _ := filepath.Rel(w.dir, p)
						out = append(out, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), n, clip(sc.Text(), 200)))
					}
				}
				return nil
			})
			return strings.Join(out, "\n"), nil
		}},
	{"write_file", "Write a whole file. Only files in the unit's scope may be written.", `{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, rel, err := w.within(a["path"])
			if err != nil {
				return "", err
			}
			if !w.writable(rel) {
				return "", errRefused{rel + " is not in the unit's scope"}
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			return "written", os.WriteFile(p, []byte(a["content"]), 0o644)
		}},
	{"edit_file", "Replace one exact occurrence of old with new in a file in the unit's scope.", `{"type":"object","properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}},"required":["path","old","new"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, rel, err := w.within(a["path"])
			if err != nil {
				return "", err
			}
			if !w.writable(rel) {
				return "", errRefused{rel + " is not in the unit's scope"}
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			out, how, err := replaceOnce(string(b), a["old"], a["new"])
			if err != nil {
				return "", fmt.Errorf("%s: %w", rel, err)
			}
			return "edited" + how, os.WriteFile(p, []byte(out), 0o644)
		}},
	{"run_check", "Run the unit's check — the repository's own tests or build — and see the result.", `{"type":"object","properties":{}}`,
		func(w *workspace, _ map[string]string) (string, error) {
			if len(w.check) == 0 {
				return "", errRefused{"there is no check command for this estate"}
			}
			ok, out, why := w.run()
			status := "passed"
			if !ok {
				status = "failed"
				if why != "" {
					status += ": " + why
				}
			}
			return status + "\n" + tail(out, 120), nil
		}},
}

// replaceOnce replaces the one place old occurs in s with new. Where old
// does not occur exactly, it is matched line by line with the indentation
// and trailing space ignored — models copy code with its spacing changed
// more often than with its words changed — and the second value says so.
func replaceOnce(s, old, new string) (string, string, error) {
	if strings.TrimSpace(old) == "" {
		return "", "", errors.New("old text is empty; to write a new file use write_file")
	}
	switch n := strings.Count(s, old); {
	case n == 1:
		return strings.Replace(s, old, new, 1), "", nil
	case n > 1:
		return "", "", fmt.Errorf("old text occurs %d times; give more of it", n)
	}
	lines := strings.Split(s, "\n")
	want := strings.Split(strings.Trim(old, "\n"), "\n")
	at, found := -1, 0
	for i := 0; i+len(want) <= len(lines); i++ {
		same := true
		for j, w := range want {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(w) {
				same = false
				break
			}
		}
		if same {
			at, found = i, found+1
		}
	}
	switch found {
	case 0:
		return "", "", errors.New("old text not found; read the file again and copy it exactly")
	case 1:
	default:
		return "", "", fmt.Errorf("old text occurs %d times once spacing is ignored; give more of it", found)
	}
	repl := strings.Split(strings.Trim(new, "\n"), "\n")
	if strings.TrimSpace(new) == "" {
		repl = nil
	}
	out := append(append(slices.Clone(lines[:at]), repl...), lines[at+len(want):]...)
	return strings.Join(out, "\n"), " (matched with spacing ignored)", nil
}

func numbered(s string) string {
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%5d  %s\n", i+1, l)
	}
	return b.String()
}

// specs is the tools as a model is told about them.
func specs() []model.Tool {
	out := make([]model.Tool, len(tools))
	for i, t := range tools {
		out[i].Type = "function"
		out[i].Function.Name = t.name
		out[i].Function.Description = t.about
		out[i].Function.Parameters = json.RawMessage(t.params)
	}
	return out
}

func lookup(name string) (tool, bool) {
	for _, t := range tools {
		if t.name == name {
			return t, true
		}
	}
	return tool{}, false
}
