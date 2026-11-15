// Package iflow carries the method's own texts into the program, so that what
// the program shows about the method is read from where the method is
// written, and never from a second copy kept in code.
package iflow

import "embed"

// Texts holds the documents the method is defined in.
//
//go:embed docs/iflow.md docs/method/*.md docs/templates/*.md
var Texts embed.FS

// Paths of the documents inside Texts.
const (
	Research    = "docs/iflow.md"
	Rules       = "docs/method/rules.md"
	Conventions = "docs/method/conventions.md"
	Glossary    = "docs/method/glossary.md"
)
