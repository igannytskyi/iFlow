package artifact

import (
	"slices"
	"testing"
)

func TestParseTables(t *testing.T) {
	text := "# Title\n\n| Id | Criterion |\n|---|---|\n| CR-001-01 | holds |\n| CR-001-02 |\n\nprose\n| A | A |\n|:-:|---|\n| 1 | 2 |\n"
	got := ParseTables(text)
	if len(got) != 2 {
		t.Fatalf("got %d tables, want 2", len(got))
	}
	if want := []string{"Id", "Criterion"}; !slices.Equal(got[0].Headers, want) {
		t.Errorf("headers = %v, want %v", got[0].Headers, want)
	}
	if n := len(got[0].Rows); n != 2 {
		t.Fatalf("got %d rows, want 2", n)
	}
	if got[0].Rows[1].Has("Criterion") {
		t.Error("a short row has a cell it never had")
	}
	// A header named twice keeps its first place and its last value.
	dup := got[1].Rows[0]
	if v := dup.Cell("A"); v != "2" || len(dup.Values()) != 1 {
		t.Errorf("duplicate header: value %q, %d cells", v, len(dup.Values()))
	}
}

func TestPlaceholder(t *testing.T) {
	headers := []string{"Id", "Procedure", "Note"}
	for _, tc := range []struct {
		name  string
		cells []string
		want  bool
	}{
		{"filled", []string{"CR-001-01", "machine", "holds"}, false},
		{"slot", []string{"CR-<nnn>-01", "machine", ""}, true},
		{"unchosen", []string{"", "machine · human", ""}, true},
		{"one chosen", []string{"CR-001-01", "machine · human", ""}, false},
		{"empty", []string{"", "", ""}, true},
		{"prose with a bound", []string{"CR-001-01", "machine", "0 < x < 10"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := newRow(headers, tc.cells).Placeholder(); got != tc.want {
				t.Errorf("Placeholder() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPathsIn(t *testing.T) {
	got := PathsIn("the region src/flask/ and src/flask, also tests/test_basic.py.")
	want := []string{"src/flask", "tests/test_basic.py"}
	if !slices.Equal(got, want) {
		t.Errorf("PathsIn = %v, want %v", got, want)
	}
}

func TestEscapedPipeStaysInItsCell(t *testing.T) {
	got := ParseTables("| A | B |\n|---|---|\n| `go test -run ^(X\\|Y)$` | yes |\n")
	if v := got[0].Rows[0].Cell("A"); v != "`go test -run ^(X|Y)$`" {
		t.Errorf("cell A = %q", v)
	}
	if v := got[0].Rows[0].Cell("B"); v != "yes" {
		t.Errorf("cell B = %q", v)
	}
}
