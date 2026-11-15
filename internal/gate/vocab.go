package gate

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// code is one set of values a cell may hold.
type code []string

func (c code) allows(v string) bool { return slices.Contains(c, v) }

// Codes is every code a cell may hold, by what it codes for. Defined in
// docs/method/conventions.md and nowhere else; this is the gate's copy, and
// what finds the two apart is the check for codes nothing validates.
var Codes = map[string]code{
	"class":      {"C1P", "C1T", "C2", "C3", "C4", "C5"},
	"provenance": {"derived", "attested", "produced"},
	"confidence": {"high", "medium", "low"},
	"procedure":  {"machine", "human"},
	"subject":    {"candidate", "transformation"},
	"kind": {"test-run", "static-analysis", "runtime-observation",
		"human-affirmation", "transformation-proof"},
	"independence": {"independent-by-executor", "independent-by-precommitment",
		"not-independent"},
	"repeatable": {"yes", "no", "unknown"},
	"plan":       {"required", "not-required"},
	"refused_by": {"person", "area"},
	"reviewed":   {"yes", "no"},
	"outcome":    {"met", "failed", "undecided"},
	"state":      {"settled", "deferred"},
	"admission":  {"admitted", "held", "refused", "awaiting-authority"},
	"terminal": {"candidate", "terminated", "exhausted",
		"failed-substrate", "failed-task", "cancelled"},
}

// quote renders a cell inside a message: in single quotes, so that an empty
// value is visibly empty, with anything unprintable escaped.
func quote(s string) string {
	q := '\''
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		q = '"'
	}
	var b strings.Builder
	b.WriteRune(q)
	for _, r := range s {
		switch {
		case r == '\\' || r == q:
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case !unicode.IsPrint(r) && r != ' ':
			s := strconv.QuoteRuneToASCII(r)
			b.WriteString(s[1 : len(s)-1])
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune(q)
	return b.String()
}

func quoteList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = quote(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func intList(items []int) string {
	parts := make([]string, len(items))
	for i, n := range items {
		parts[i] = strconv.Itoa(n)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// orNone shows a cell a row may not have: a missing cell is not an empty one.
func orNone(v string, ok bool) string {
	if !ok {
		return "None"
	}
	return v
}
