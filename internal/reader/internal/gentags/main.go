// Command gentags copies each grammar's own tags query out of the module it
// ships in, so that the reader embeds the query the grammar's maintainers
// wrote rather than one written here.
//
// Run by go generate in the reader package; the copies are committed, and a
// grammar version change is followed by running it again.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// grammars maps the name a query is kept under to the module it comes from.
var grammars = map[string]string{
	"c":          "github.com/tree-sitter/tree-sitter-c",
	"cpp":        "github.com/tree-sitter/tree-sitter-cpp",
	"c-sharp":    "github.com/tree-sitter/tree-sitter-c-sharp",
	"go":         "github.com/tree-sitter/tree-sitter-go",
	"java":       "github.com/tree-sitter/tree-sitter-java",
	"javascript": "github.com/tree-sitter/tree-sitter-javascript",
	"php":        "github.com/tree-sitter/tree-sitter-php",
	"python":     "github.com/tree-sitter/tree-sitter-python",
	"ruby":       "github.com/tree-sitter/tree-sitter-ruby",
	"rust":       "github.com/tree-sitter/tree-sitter-rust",
	"scala":      "github.com/tree-sitter/tree-sitter-scala",
	"typescript": "github.com/tree-sitter/tree-sitter-typescript",
}

func main() {
	out := os.Args[1]
	for name, module := range grammars {
		dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module).Output()
		if err != nil {
			fail(fmt.Errorf("%s: %w", module, err))
		}
		src := filepath.Join(strings.TrimSpace(string(dir)), "queries", "tags.scm")
		b, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gentags: %s ships no tags query\n", module)
			continue
		}
		head := fmt.Appendf(nil, "; Copied from %s by go generate. Not edited here.\n\n", module)
		if err := os.WriteFile(filepath.Join(out, name+".scm"), bytes.Join([][]byte{head, b}, nil), 0o644); err != nil {
			fail(err)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gentags:", err)
	os.Exit(1)
}
