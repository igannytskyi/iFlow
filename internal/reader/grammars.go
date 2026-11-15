package reader

import (
	"embed"
	"fmt"
	"sync"
	"unsafe"

	tskotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
	ts "github.com/tree-sitter/go-tree-sitter"
	tsbash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	tscsharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tsc "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tscpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tsscala "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

//go:embed queries/*.scm
var queries embed.FS

// grammar is one language: the parser's language and the name its tags query
// is kept under, "" where the grammar ships none.
type grammar struct {
	language func() unsafe.Pointer
	tags     string
}

// grammars maps an extension to the grammar that reads it. A language is a
// line here and nothing else: what is read from a tree is read from the names
// grammars give their nodes, which they share.
var grammars = map[string]grammar{
	".go":    {tsgo.Language, "go"},
	".java":  {tsjava.Language, "java"},
	".cs":    {tscsharp.Language, "c-sharp"},
	".rs":    {tsrust.Language, "rust"},
	".js":    {tsjs.Language, "javascript"},
	".jsx":   {tsjs.Language, "javascript"},
	".mjs":   {tsjs.Language, "javascript"},
	".cjs":   {tsjs.Language, "javascript"},
	".ts":    {tsts.LanguageTypescript, "typescript"},
	".tsx":   {tsts.LanguageTSX, "typescript"},
	".rb":    {tsruby.Language, "ruby"},
	".php":   {tsphp.LanguagePHP, "php"},
	".c":     {tsc.Language, "c"},
	".h":     {tsc.Language, "c"},
	".cpp":   {tscpp.Language, "cpp"},
	".cc":    {tscpp.Language, "cpp"},
	".cxx":   {tscpp.Language, "cpp"},
	".hpp":   {tscpp.Language, "cpp"},
	".hh":    {tscpp.Language, "cpp"},
	".kt":    {tskotlin.Language, ""},
	".scala": {tsscala.Language, "scala"},
	".sh":    {tsbash.Language, ""},
	".bash":  {tsbash.Language, ""},
	".zsh":   {tsbash.Language, ""},
}

var pythonGrammar = grammar{tspython.Language, "python"}

// compiled is a grammar made ready: its language and its compiled query.
type compiled struct {
	lang  *ts.Language
	query *ts.Query // nil where there is none, or this version refuses it
}

var (
	compiledMu sync.Mutex
	compiledBy = map[string]*compiled{}
)

// compile readies a grammar once. Languages and queries are immutable and
// shared; parsers and cursors are not, and are made per use.
func (g grammar) compile() *compiled {
	key := fmt.Sprintf("%s|%p", g.tags, g.language())
	compiledMu.Lock()
	defer compiledMu.Unlock()
	if c, ok := compiledBy[key]; ok {
		return c
	}
	c := &compiled{lang: ts.NewLanguage(g.language())}
	if g.tags != "" {
		if src, err := queries.ReadFile("queries/" + g.tags + ".scm"); err == nil {
			if q, qerr := ts.NewQuery(c.lang, string(src)); qerr == nil {
				c.query = q
			}
		}
	}
	compiledBy[key] = c
	return c
}
