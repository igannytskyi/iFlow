package flow

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// goPackagesOf is the packages under dir whose test files declare the
// tests named, as `go test ./...` finds them — a folder of the module, not
// testdata, not hidden, not one beginning with an underscore, not another
// module — each as ./path; and whether every name was found in one. A run
// of named tests is the run of those packages alone: what `./...` would
// build, vet and link besides runs none of them, and costs the most of a
// run in a large estate. Where a name is found nowhere, the run is of
// ./..., so that go test, not this reading, says it is not there; and so
// where a test file does not parse.
func goPackagesOf(dir string, names []string) ([]string, bool) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	found := map[string]bool{}
	pkgs := map[string]bool{}
	broken := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != dir && (base == "testdata" || base == "vendor" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") || fileExists(filepath.Join(path, "go.mod"))) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		tests, err := goTests(path, src)
		if err != nil {
			broken = true // its package does not build; go test, over all, says why
			return filepath.SkipAll
		}
		for _, t := range tests {
			if want[t] {
				found[t] = true
				rel, _ := filepath.Rel(dir, filepath.Dir(path))
				pkgs["./"+filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})
	if broken || len(found) < len(want) || len(pkgs) == 0 {
		return []string{"./..."}, false
	}
	out := make([]string, 0, len(pkgs))
	for p := range pkgs {
		out = append(out, strings.TrimSuffix(p, "/."))
	}
	slices.Sort(out)
	return out, true
}
