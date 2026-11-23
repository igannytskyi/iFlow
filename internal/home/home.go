// Package home is what is kept of the person between sessions, beside
// their iflow configuration and never in a project: which projects they
// work on. Every screen reads it from here, so that the window and a
// terminal show the same projects.
package home

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// DeskFile is where the workbench keeps what it remembers: the projects,
// and how the window was left.
func DeskFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "iflow", "desk.json")
}

// Projects is the folders of the projects the person works on, in the
// order they were added; one no longer on disk is left out.
func Projects() []string {
	var kept struct {
		Projects []string `json:"projects"`
	}
	if b, err := os.ReadFile(DeskFile()); err == nil {
		_ = json.Unmarshal(b, &kept)
	}
	return slices.DeleteFunc(kept.Projects, func(p string) bool {
		st, err := os.Stat(p)
		return err != nil || !st.IsDir()
	})
}
