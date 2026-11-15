// Package reader says what a file defines, what it refers to and what it
// imports, in whatever language it is written.
//
// The estate does not know how to parse anything. It asks this package, and
// grades every answer the same way whichever reader produced it. What is
// unread is derived from which extensions a reader here claims, so a language
// nothing reads is reported unseen, never absent.
//
//go:generate go run ./internal/gentags queries
package reader

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
)

// Symbol is one definition and the lines it spans.
type Symbol struct {
	Name  string
	Start int
	End   int
}

// Kind says how a reference was resolved, which is how far it is trusted.
type Kind string

// The kinds of reference, firmest first.
const (
	// Import is a module named in an import statement.
	Import Kind = "import"
	// Typed is a call on a receiver whose type the file itself states.
	Typed Kind = "typed"
	// Name is a bare name in the file's own scope.
	Name Kind = "name"
	// Attribute is a name behind a receiver whose type is unknown.
	Attribute Kind = "attribute"
)

// Call is one reference: what it names, how, and the lines it spans.
type Call struct {
	Name string
	Kind Kind
	Line int
	End  int
}

// File is what one file defines, refers to, and could not resolve.
type File struct {
	Symbols    []Symbol
	Calls      []Call
	Unresolved []string
}

// Reader reads one family of languages.
type Reader interface {
	// Name is how answers say who read them.
	Name() string
	// Extensions are the file extensions this reader claims.
	Extensions() []string
	// Read reads one file; rel names it and decides its language.
	Read(rel string, src []byte) File
}

// readers are asked in order, and the first to claim an extension keeps it.
var readers = []Reader{python{}, generic{}}

var byExtension = sync.OnceValue(func() map[string]Reader {
	claimed := map[string]Reader{}
	for _, r := range readers {
		for _, ext := range r.Extensions() {
			if _, taken := claimed[ext]; !taken {
				claimed[ext] = r
			}
		}
	}
	return claimed
})

// Extensions lists every extension some reader here claims, sorted.
func Extensions() []string {
	out := make([]string, 0, len(byExtension()))
	for ext := range byExtension() {
		out = append(out, ext)
	}
	slices.Sort(out)
	return out
}

// Claims reports whether a reader here claims the extension.
func Claims(ext string) bool { _, ok := byExtension()[ext]; return ok }

// shebangs maps what a first line names to the extension it implies.
var shebangs = map[string]string{
	"sh": ".sh", "bash": ".sh", "zsh": ".sh", "dash": ".sh", "ksh": ".sh",
	"python": ".py", "python3": ".py", "ruby": ".rb", "node": ".js", "pwsh": ".ps1",
}

// Shebang is the extension a file without one would have had, read off its
// first line, or "".
func Shebang(text []byte) string {
	head := string(text[:min(len(text), 200)])
	if strings.TrimSpace(head) == "" {
		return ""
	}
	first, _, _ := strings.Cut(strings.ReplaceAll(head, "\r\n", "\n"), "\n")
	if !strings.HasPrefix(first, "#!") {
		return ""
	}
	words := strings.Fields(strings.ReplaceAll(first[2:], "/", " "))
	for i := len(words) - 1; i >= 0; i-- {
		if ext, ok := shebangs[words[i]]; ok {
			return ext
		}
	}
	return ""
}

var peeked sync.Map // path → extension read off its first line

// Readable reports whether some reader here takes the file at p. The
// extension decides, except where there is none: a deploy script written as
// bin/deploy says what it is on its first line.
func Readable(p string) bool {
	ext := path.Ext(p)
	if ext != "" {
		return Claims(ext)
	}
	alias, ok := peeked.Load(p)
	if !ok {
		alias = ""
		if f, err := os.Open(p); err == nil {
			buf := make([]byte, 200)
			n, _ := f.Read(buf)
			f.Close()
			alias = Shebang(buf[:n])
		}
		peeked.Store(p, alias)
	}
	return alias != "" && Claims(alias.(string))
}

// Read says what a file holds, or nothing if no reader here can read it.
func Read(rel string, src []byte) File {
	ext := path.Ext(rel)
	if ext == "" {
		ext = Shebang(src)
		rel += ext
	}
	r, ok := byExtension()[ext]
	if !ok {
		return File{}
	}
	return r.Read(rel, src)
}

// Derivation is a short name for how files are being read now. An answer
// derived by one reader is not the answer another would give, so a cache
// keyed on the source alone would go on serving the old reading.
var Derivation = sync.OnceValue(func() string {
	h := sha256.New()
	h.Write([]byte(version))
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if strings.Contains(d.Path, "tree-sitter") {
				h.Write([]byte(d.Path + "@" + d.Version))
			}
		}
	}
	for _, ext := range Extensions() {
		h.Write([]byte(ext + ":" + byExtension()[ext].Name()))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
})

// version changes whenever what a reader here returns for the same file does.
const version = "go/1"

// Who says which reader reads which extensions.
func Who() map[string][]string {
	out := map[string][]string{}
	for _, ext := range Extensions() {
		n := byExtension()[ext].Name()
		out[n] = append(out[n], ext)
	}
	return out
}
