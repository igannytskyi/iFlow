// Package fsx holds the one file operation every derived note needs: writing
// it so that a reader never sees half of it, however many writers race.
package fsx

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to name by writing a temporary file beside it and
// renaming it into place. Concurrent writers each use their own temporary
// file, so the last rename wins whole.
func WriteAtomic(name string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
