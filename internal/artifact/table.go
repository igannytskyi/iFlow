// Package artifact reads the files a change is carried in. Every artefact is
// markdown whose tables are its data: the templates under docs/templates
// are the schema, and nothing here declares a column of its own.
package artifact

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Row is one row of a table. Cells keep the order of the header; a header
// named twice keeps its first place and its last value.
type Row struct {
	keys  []string
	cells map[string]string
}

// Get returns the cell under a header and whether the row has that header.
func (r Row) Get(header string) (string, bool) {
	v, ok := r.cells[header]
	return v, ok
}

// Cell returns the cell under a header, empty when the row has none.
func (r Row) Cell(header string) string { return r.cells[header] }

// Has reports whether the row has the header at all.
func (r Row) Has(header string) bool {
	_, ok := r.cells[header]
	return ok
}

// Values returns the cells in header order.
func (r Row) Values() []string {
	out := make([]string, len(r.keys))
	for i, k := range r.keys {
		out[i] = r.cells[k]
	}
	return out
}

// Joined returns every non-empty cell, separated by a space.
func (r Row) Joined() string {
	return strings.Join(slices.DeleteFunc(r.Values(), func(s string) bool { return s == "" }), " ")
}

// Table is a header and the rows under it, placeholders included.
type Table struct {
	Headers []string
	Rows    []Row
}

// Has reports whether every named header is present.
func (t Table) Has(headers ...string) bool {
	for _, h := range headers {
		if !slices.Contains(t.Headers, h) {
			return false
		}
	}
	return true
}

func trim(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

func separator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// cellsOf splits a table line into cells. A pipe written \| belongs to the
// cell — a command with an alternation in it is one cell, not two.
func cellsOf(line string) []string {
	inner := ""
	if len(line) >= 2 {
		inner = line[1 : len(line)-1]
	}
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == '\\' && i+1 < len(inner) && inner[i+1] == '|':
			cur.WriteByte('|')
			i++
		case inner[i] == '|':
			parts = append(parts, trim(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(inner[i])
		}
	}
	return append(parts, trim(cur.String()))
}

// ParseTables returns every markdown table in text, in order.
func ParseTables(text string) []Table {
	var (
		out []Table
		cur *Table
	)
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, line := range lines(text) {
		s := trim(line)
		if !strings.HasPrefix(s, "|") || !strings.HasSuffix(s, "|") {
			flush()
			continue
		}
		cells := cellsOf(s)
		switch {
		case cur == nil:
			cur = &Table{Headers: cells}
		case separator(cells):
		default:
			cur.Rows = append(cur.Rows, newRow(cur.Headers, cells))
		}
	}
	flush()
	return out
}

func newRow(headers, cells []string) Row {
	r := Row{cells: make(map[string]string, len(headers))}
	for i := range min(len(headers), len(cells)) {
		h := headers[i]
		if _, seen := r.cells[h]; !seen {
			r.keys = append(r.keys, h)
		}
		r.cells[h] = cells[i]
	}
	return r
}

func lines(text string) []string {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// slot is a cell a template left to be filled: one slot, or one word with a
// slot in it (CR-<nnn>-01). Prose that names a slot in passing — a message
// "iFlow — <project name>" an intent describes — is filled, not a slot.
var slot = regexp.MustCompile(`^<[^<>]*>$|^[^\s<>]*<[^<>\s]*>[^\s<>]*$`)

// unchosen reports whether a cell still offers the template's alternatives,
// as in "machine · human".
func unchosen(cell string) bool {
	if !strings.Contains(cell, "·") {
		return false
	}
	for part := range strings.SplitSeq(cell, "·") {
		if trim(part) == "" {
			return false
		}
	}
	return true
}

// Placeholder reports whether a row is the template's rather than data.
//
// A row is a placeholder when a cell carries a <...> slot, or when every cell
// it has is an unchosen list of alternatives. Punctuation inside prose a
// person wrote — a two-sided bound, an arrow — is not a placeholder, and
// dropping such a row would remove a criterion from the gate's view while the
// gate went on reporting that it passed.
func (r Row) Placeholder() bool {
	filled, choices := 0, 0
	for _, v := range r.Values() {
		c := trim(v)
		if c == "" {
			continue
		}
		filled++
		if slot.MatchString(c) {
			return true
		}
		if unchosen(c) {
			choices++
		}
	}
	return filled == 0 || choices == filled
}

var pathish = regexp.MustCompile(`[A-Za-z0-9_.\-/]+/[A-Za-z0-9_.\-/]*`)

// PathsIn returns every path-shaped word in a cell, sorted and distinct.
// Prose around it is prose; what can be checked is what names ground.
func PathsIn(text string) []string {
	var out []string
	for _, m := range pathish.FindAllString(text, -1) {
		out = append(out, strings.TrimRight(m, "/.,;:"))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

var id = regexp.MustCompile(`\b(?:INT|SPEC|CR|WU|GR|CA|EV|VE)-\d{3}(?:-\d{2}){0,2}\b`)

// IDs returns every identifier a cell cites, in order.
func IDs(text string) []string { return id.FindAllString(text, -1) }
