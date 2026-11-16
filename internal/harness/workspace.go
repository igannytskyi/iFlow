package harness

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// skipped is never copied into a workspace: version control, installed
// dependencies, derived material, and the changes carried against the
// estate — which hold what will judge the work, and an executor that could
// read its own judge could shape itself to it.
var skipped = map[string]bool{".git": true, "node_modules": true, ".estate": true, ".venv": true, "venv": true, "__pycache__": true}

// skip reports whether a folder is left out of a copy of the estate rooted
// at root: the dependencies and derived material wherever they are, and the
// estate's own changes folder — only at its root, since a changes folder
// deeper down (an example's) is part of the estate its tests read.
func skipFolder(root, p string, d os.DirEntry) bool {
	if p == root || !d.IsDir() {
		return false
	}
	return skipped[d.Name()] || (d.Name() == "changes" && filepath.Dir(p) == root)
}

// Snapshot copies an estate into a fresh directory for one unit, and lays
// the given candidates over it. It returns the directory, which the caller
// removes.
func Snapshot(estate string, overlays ...string) (string, error) {
	dir, err := os.MkdirTemp("", "iflow-ws-")
	if err != nil {
		return "", err
	}
	// The path the agent is told is the path the system reports, so that a
	// permission written for one matches a command naming the other.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if err := copyTree(estate, dir, true); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	for _, o := range overlays {
		if err := copyTree(o, dir, false); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
}

func copyTree(src, dst string, skip bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			if skip && skipFolder(src, p, d) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Changed lists, relative to the estate, every file the workspace holds that
// differs from the estate or is new. Deletions are listed too, with Gone.
type Change struct {
	Path string
	Gone bool
}

// Diff compares a workspace with the estate it was copied from.
func Diff(estate, workspace string) ([]Change, error) {
	digest := func(p string) ([32]byte, bool) {
		b, err := os.ReadFile(p)
		if err != nil {
			return [32]byte{}, false
		}
		return sha256.Sum256(b), true
	}
	seen := map[string]bool{}
	var out []Change
	err := filepath.WalkDir(workspace, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(workspace, p)
		if d.IsDir() {
			if skipFolder(workspace, p, d) {
				return filepath.SkipDir
			}
			return nil
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		now, _ := digest(p)
		was, ok := digest(filepath.Join(estate, filepath.FromSlash(rel)))
		if !ok || was != now {
			out = append(out, Change{Path: rel})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(estate, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(estate, p)
		if d.IsDir() {
			if skipFolder(estate, p, d) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel = filepath.ToSlash(rel); !seen[rel] && d.Type().IsRegular() {
			out = append(out, Change{Path: rel, Gone: true})
		}
		return nil
	})
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	return out, err
}

// Keep copies the changed files of a workspace into a candidate's folder,
// and writes a unified-style listing of each change beside them.
func Keep(workspace string, changes []Change, into string) error {
	var listing bytes.Buffer
	for _, c := range changes {
		if c.Gone {
			listing.WriteString("deleted " + c.Path + "\n")
			continue
		}
		if err := copyFile(filepath.Join(workspace, filepath.FromSlash(c.Path)), filepath.Join(into, filepath.FromSlash(c.Path))); err != nil {
			return err
		}
		listing.WriteString("changed " + c.Path + "\n")
	}
	return os.WriteFile(filepath.Join(into, "CHANGES"), listing.Bytes(), 0o644)
}

// Manifest is what every file in a directory holds, by path, so that what an
// agent changed can be told apart from what it was given.
func Manifest(dir string) (map[string][32]byte, error) {
	out := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipFolder(dir, p, d) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = sha256.Sum256(b)
		return nil
	})
	return out, err
}

// Since lists what changed in a directory since its manifest was taken.
func Since(dir string, before map[string][32]byte) ([]Change, error) {
	now, err := Manifest(dir)
	if err != nil {
		return nil, err
	}
	var out []Change
	for rel, sum := range now {
		if was, ok := before[rel]; !ok || was != sum {
			out = append(out, Change{Path: rel})
		}
	}
	for rel := range before {
		if _, ok := now[rel]; !ok {
			out = append(out, Change{Path: rel, Gone: true})
		}
	}
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}
