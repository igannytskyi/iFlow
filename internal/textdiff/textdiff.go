// Package textdiff says how one text became another, line by line, as a
// unified diff: what a unit changed, for a person to read.
package textdiff

import (
	"fmt"
	"strings"
)

// maxCells bounds the table the longest common subsequence is found in:
// past it, the lines between what the two texts share at their ends are
// said to be replaced whole.
const maxCells = 4 << 20

// op is one line of the diff: kept, removed or added.
type op struct {
	kind byte // ' ', '-' or '+'
	line string
}

// Lines is the diff of a and b as lines: those they share at their start
// and end kept, and between them the longest common subsequence kept and
// the rest removed or added.
func Lines(a, b string) (ops []op, added, removed int) {
	x, y := split(a), split(b)
	pre := 0
	for pre < len(x) && pre < len(y) && x[pre] == y[pre] {
		pre++
	}
	suf := 0
	for suf < len(x)-pre && suf < len(y)-pre && x[len(x)-1-suf] == y[len(y)-1-suf] {
		suf++
	}
	for _, l := range x[:pre] {
		ops = append(ops, op{' ', l})
	}
	mx, my := x[pre:len(x)-suf], y[pre:len(y)-suf]
	if len(mx)*len(my) > maxCells {
		for _, l := range mx {
			ops = append(ops, op{'-', l})
		}
		for _, l := range my {
			ops = append(ops, op{'+', l})
		}
	} else {
		ops = append(ops, lcs(mx, my)...)
	}
	for _, l := range x[len(x)-suf:] {
		ops = append(ops, op{' ', l})
	}
	for _, o := range ops {
		switch o.kind {
		case '+':
			added++
		case '-':
			removed++
		}
	}
	return ops, added, removed
}

// lcs is x turned into y by their longest common subsequence.
func lcs(x, y []string) []op {
	n, m := len(x), len(y)
	t := make([][]int32, n+1)
	for i := range t {
		t[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if x[i] == y[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case x[i] == y[j]:
			ops = append(ops, op{' ', x[i]})
			i, j = i+1, j+1
		case t[i+1][j] >= t[i][j+1]:
			ops = append(ops, op{'-', x[i]})
			i++
		default:
			ops = append(ops, op{'+', y[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{'-', x[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{'+', y[j]})
	}
	return ops
}

func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// Unified is the diff of a and b in unified form, with context lines of
// what is kept around each change, and how many lines were added and
// removed.
func Unified(a, b string, context int) (text string, added, removed int) {
	ops, added, removed := Lines(a, b)
	var out strings.Builder
	aLine, bLine := 1, 1
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i, aLine, bLine = i+1, aLine+1, bLine+1
			continue
		}
		// A hunk: from context lines before the change to context lines
		// after the last change no further than 2*context from the next.
		start := max(0, i-context)
		end := i
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			k := end
			for k < len(ops) && ops[k].kind == ' ' {
				k++
			}
			if k == len(ops) || k-end > 2*context {
				end = min(len(ops), end+context)
				break
			}
			end = k
		}
		as, bs := aLine-(i-start), bLine-(i-start)
		var na, nb int
		var body strings.Builder
		for _, o := range ops[start:end] {
			body.WriteString(string(o.kind) + o.line + "\n")
			if o.kind != '+' {
				na++
			}
			if o.kind != '-' {
				nb++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n%s", as, na, bs, nb, body.String())
		for _, o := range ops[i:end] {
			if o.kind != '+' {
				aLine++
			}
			if o.kind != '-' {
				bLine++
			}
		}
		i = end
	}
	return out.String(), added, removed
}
