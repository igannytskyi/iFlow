package harness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

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
	ctx   context.Context
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
	{"read_file", "Read a file in the repository.", `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, _, err := w.within(a["path"])
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			return numbered(string(b)), nil
		}},
	{"list_dir", "List a directory in the repository.", `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
		func(w *workspace, a map[string]string) (string, error) {
			p, _, err := w.within(a["path"])
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
			switch n := bytes.Count(b, []byte(a["old"])); {
			case a["old"] == "" || n == 0:
				return "", fmt.Errorf("old text not found in %s", rel)
			case n > 1:
				return "", fmt.Errorf("old text occurs %d times in %s; give more of it", n, rel)
			}
			return "edited", os.WriteFile(p, bytes.Replace(b, []byte(a["old"]), []byte(a["new"]), 1), 0o644)
		}},
	{"run_check", "Run the repository's own tests and see the result.", `{"type":"object","properties":{}}`,
		func(w *workspace, _ map[string]string) (string, error) {
			if len(w.check) == 0 {
				return "", errRefused{"there is no check command for this estate"}
			}
			ctx, cancel := context.WithTimeout(w.ctx, 5*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, w.check[0], w.check[1:]...)
			cmd.Dir = w.dir
			out, err := cmd.CombinedOutput()
			status := "passed"
			if err != nil {
				status = "failed: " + err.Error()
			}
			return status + "\n" + clip(string(out), 6000), nil
		}},
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
