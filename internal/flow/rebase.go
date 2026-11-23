package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/igannytskyi/iflow/internal/harness"
)

// rebased is what a landing enters where the estate moved under the change
// after admission — another change landed on the same files — as a merge
// queue does it: each moved file is the change's edits merged with what
// moved it, three ways from the ground the change was admitted on, and the
// whole suite is run on the estate as it is now with the change laid on
// it. Merged holds the files to enter in place of the candidates' own; why
// is not empty when the change cannot land: an edit that overlaps the
// other change's, a file removed or created under it, or a suite that
// fails on the merged ground. Moved names what moved, for the record.
func (r *Run) rebased(ctx context.Context) (merged map[string][]byte, moved []string, why string) {
	merged = map[string][]byte{}
	var conflicts []string
	for _, u := range r.units {
		if u.isTests() {
			continue // the change's own files; no baseline holds them
		}
		dir := r.cands[u.id]
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == "CHANGES" {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			rel = filepath.ToSlash(rel)
			if !slices.Contains(r.spec.Scope, rel) {
				return nil
			}
			base := filepath.Join(r.dir, "baseline", path.Base(rel))
			now, nowErr := os.ReadFile(filepath.Join(r.repo, filepath.FromSlash(rel)))
			switch {
			case fileExists(base + ".before"):
				was, _ := os.ReadFile(base + ".before")
				if nowErr != nil {
					moved = append(moved, rel+" was removed")
					conflicts = append(conflicts, rel+" was removed under it")
					return nil
				}
				if string(was) == string(now) {
					return nil
				}
				moved = append(moved, rel+" was changed")
				ours, _ := os.ReadFile(p)
				m, clean, err := merge3(was, ours, now)
				switch {
				case err != nil:
					conflicts = append(conflicts, rel+" could not be merged: "+err.Error())
				case !clean:
					conflicts = append(conflicts, rel+": its edits overlap the other change's")
				default:
					merged[rel] = m
				}
			case fileExists(base + ".absent"):
				if nowErr == nil {
					moved = append(moved, rel+" was created by something else")
					conflicts = append(conflicts, rel+" was created by something else")
				}
			}
			return nil
		})
	}
	switch {
	case len(conflicts) > 0:
		return nil, moved, strings.Join(conflicts, "; ")
	case len(merged) == 0:
		return nil, moved, ""
	}
	prof, ok := detect(r.repo)
	if !ok || len(prof.suite) == 0 {
		return nil, moved, "the estate moved under it and no test runner can say whether the merged change holds"
	}
	if failed := r.holds(ctx, prof, merged); failed != "" {
		return nil, moved, "merged with what moved it, the change fails the suite: " + failed
	}
	return merged, moved, ""
}

// holds runs the whole suite on the estate as it is now with every
// candidate laid on it and the merged files over them, and says what failed.
func (r *Run) holds(ctx context.Context, prof profile, merged map[string][]byte) string {
	var overlays []string
	for _, u := range r.units {
		overlays = append(overlays, r.cands[u.id])
	}
	ws, err := harness.Snapshot(r.repo, overlays...)
	if err != nil {
		return "a workspace could not be made: " + err.Error()
	}
	defer os.RemoveAll(ws)
	for rel, b := range merged {
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), b, 0o644); err != nil {
			return err.Error()
		}
	}
	passed, out, _ := runIn(ctx, ws, resolve(ws, prof.suite))
	if passed {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.Join(lines[max(0, len(lines)-12):], "\n")
}

// merge3 merges the change's edits of a file (ours) with another's (theirs)
// from the ground both started on (base), the way git does; clean is false
// where the two edit the same lines.
func merge3(base, ours, theirs []byte) (merged []byte, clean bool, err error) {
	dir, err := os.MkdirTemp("", "iflow-merge-")
	if err != nil {
		return nil, false, err
	}
	defer os.RemoveAll(dir)
	names := []string{"ours", "base", "theirs"}
	for i, b := range [][]byte{ours, base, theirs} {
		if err := os.WriteFile(filepath.Join(dir, names[i]), b, 0o644); err != nil {
			return nil, false, err
		}
	}
	cmd := exec.Command("git", "merge-file", "-p", "ours", "base", "theirs")
	cmd.Dir = dir
	out, err := cmd.Output()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out, true, nil
	case errors.As(err, &exit) && exit.ExitCode() > 0 && exit.ExitCode() < 128:
		return out, false, nil // the number of conflicts
	}
	return nil, false, fmt.Errorf("git merge-file: %w", err)
}
