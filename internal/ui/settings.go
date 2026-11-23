package ui

import (
	"encoding/json"
	"os"
	"slices"

	"github.com/igannytskyi/iflow/internal/fsx"
	"github.com/igannytskyi/iflow/internal/home"
)

// settings is what the workbench remembers between sessions: the projects a
// person works on, which was open, and how wide they left each panel. Kept in
// the user's configuration directory, never in a project.
type settings struct {
	Projects []string           `json:"projects"`
	Open     string             `json:"open"`
	Widths   map[string]float32 `json:"widths"`
	Dark     bool               `json:"dark"`
	// ShowLanded lists landed tasks under a project, which are folded away
	// otherwise; HideCosts folds the panel of what tasks cost.
	ShowLanded bool `json:"show landed"`
	HideCosts  bool `json:"hide costs"`
	// CostsLeft puts the panel of costs on the task's left, not its right.
	CostsLeft bool `json:"costs left"`
	path      string
}

func loadSettings() *settings {
	s := &settings{Widths: map[string]float32{}, path: home.DeskFile()}
	if s.path == "" {
		return s
	}
	if b, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(b, s)
	}
	if s.Widths == nil {
		s.Widths = map[string]float32{}
	}
	// A project that is no longer on disk is forgotten rather than shown.
	s.Projects = slices.DeleteFunc(s.Projects, func(p string) bool {
		st, err := os.Stat(p)
		return err != nil || !st.IsDir()
	})
	return s
}

func (s *settings) add(path string) {
	if !slices.Contains(s.Projects, path) {
		s.Projects = append(s.Projects, path)
	}
}

func (s *settings) save() {
	if s.path == "" {
		return
	}
	if b, err := json.MarshalIndent(s, "", " "); err == nil {
		_ = fsx.WriteAtomic(s.path, b, 0o644)
	}
}
