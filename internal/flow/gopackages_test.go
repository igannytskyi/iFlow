package flow

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A run of named Go tests is a run of the packages that declare them, as
// go test ./... finds them; a name found nowhere, or a test file that does
// not parse, leaves the run over ./... .
func TestNamedGoTestsRunInTheirPackagesAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module x\n")
	write("root_test.go", "package x\nimport \"testing\"\nfunc TestRoot(t *testing.T) {}\n")
	write("a/a_test.go", "package a\nimport \"testing\"\nfunc TestA(t *testing.T) {}\nfunc FuzzA(f *testing.F) {}\n")
	write("b/c/c_test.go", "package c_test\nimport \"testing\"\nfunc TestC(t *testing.T) {}\nfunc TestHelper(x int) {}\n")
	write("a/testdata/d_test.go", "package d\nimport \"testing\"\nfunc TestD(t *testing.T) {}\n")
	write("_skip/e_test.go", "package e\nimport \"testing\"\nfunc TestE(t *testing.T) {}\n")
	write("nested/go.mod", "module y\n")
	write("nested/f_test.go", "package f\nimport \"testing\"\nfunc TestF(t *testing.T) {}\n")

	for _, c := range []struct {
		names []string
		want  []string
		all   bool
	}{
		{[]string{"TestA", "FuzzA"}, []string{"./a"}, true},
		{[]string{"TestRoot", "TestC"}, []string{".", "./b/c"}, true},
		{[]string{"TestA", "TestMissing"}, []string{"./..."}, false},
		{[]string{"TestHelper"}, []string{"./..."}, false}, // not a test: its argument is no *testing.T
		{[]string{"TestD"}, []string{"./..."}, false},      // testdata is no package of ./...
		{[]string{"TestE"}, []string{"./..."}, false},
		{[]string{"TestF"}, []string{"./..."}, false}, // another module
	} {
		got, all := goPackagesOf(dir, c.names)
		if !slices.Equal(got, c.want) || all != c.all {
			t.Errorf("%v: %v %v, want %v %v", c.names, got, all, c.want, c.all)
		}
	}

	write("b/broken_test.go", "package b\nfunc (\n")
	if got, all := goPackagesOf(dir, []string{"TestA"}); all || !slices.Equal(got, []string{"./..."}) {
		t.Errorf("a test file that does not parse left the run at %v", got)
	}
}
