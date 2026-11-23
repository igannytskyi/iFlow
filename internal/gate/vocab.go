package gate

import (
	"strconv"
	"strings"
	"unicode"
)

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
