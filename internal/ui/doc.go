package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/unit"
)

// docs is everything the window shows from disk, read once and kept until
// something changes it. A frame does no input or output and no heavy work:
// it lays out what is in memory, so that a click is answered in the time it
// takes to draw. The generation moves when anything may have changed — a
// project read again, a run's step, a check's answer — and only then is
// what the next frame asks for read again.
type docs struct {
	gen   atomic.Uint64
	files map[string]docEntry
	globs map[string]globEntry
	memo  map[string]memoEntry
}

type docEntry struct {
	gen   uint64
	lines []string // nil where the file is not there
}

type globEntry struct {
	gen     uint64
	matches []string
}

type memoEntry struct {
	gen   uint64
	value any
}

func newDocs() *docs {
	return &docs{files: map[string]docEntry{}, globs: map[string]globEntry{}, memo: map[string]memoEntry{}}
}

// bump says something on disk may have changed; safe from any goroutine.
func (d *docs) bump() { d.gen.Add(1) }

// read is a file's lines, nil where it is not there.
func (d *docs) read(path string) []string {
	g := d.gen.Load()
	if e, ok := d.files[path]; ok && e.gen == g {
		return e.lines
	}
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		lines = strings.Split(string(b), "\n")
		if lines == nil {
			lines = []string{}
		}
	}
	d.files[path] = docEntry{g, lines}
	return lines
}

// exists reports whether a file is there.
func (d *docs) exists(path string) bool { return d.read(path) != nil }

// glob is the paths matching a pattern, sorted.
func (d *docs) glob(pattern string) []string {
	g := d.gen.Load()
	if e, ok := d.globs[pattern]; ok && e.gen == g {
		return e.matches
	}
	m, _ := filepath.Glob(pattern)
	slices.Sort(m)
	d.globs[pattern] = globEntry{g, m}
	return m
}

// memo is what f computes, kept under key until the generation moves.
func memo[T any](d *docs, key string, f func() T) T {
	g := d.gen.Load()
	if e, ok := d.memo[key]; ok && e.gen == g {
		return e.value.(T)
	}
	v := f()
	d.memo[key] = memoEntry{g, v}
	return v
}

// prose is a document's words for a reader: its headings, paragraphs and
// points, without the tables the inspector shows as objects. from names the
// heading where reading starts, and every section before it is skipped.
func (u *UI) prose(path, from string) []W {
	lines := u.docs.read(path)
	if len(lines) == 0 {
		return nil
	}
	start := 0
	if from != "" {
		if i := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == from }); i >= 0 {
			start = i
		} else {
			return nil
		}
	}
	var out []W
	var para []string
	// A heading waits for something under it: one over nothing but a table
	// says nothing here.
	var heading W
	emit := func(w W) {
		if heading != nil {
			out = append(out, heading)
			heading = nil
		}
		out = append(out, w)
	}
	flush := func() {
		if len(para) > 0 {
			emit(u.text(sizeBody, u.p.Text, strings.Join(para, " ")))
			para = nil
		}
	}
	for _, raw := range lines[start:] {
		l := strings.TrimSpace(raw)
		switch {
		case l == "" || strings.HasPrefix(l, "|"):
			flush()
		case strings.HasPrefix(l, "# "):
			flush()
		case strings.HasPrefix(l, "## "):
			flush()
			heading = inset(6, 0, u.text(sizeSmall, u.p.Faint, strings.ToUpper(strings.TrimPrefix(l, "## "))))
		case strings.HasPrefix(l, "### "), strings.HasPrefix(l, "#### "):
			flush()
			emit(u.strong(sizeBody, u.p.Text, strings.TrimLeft(l, "# ")))
		case strings.HasPrefix(l, "- "):
			flush()
			emit(u.point("•", strings.TrimPrefix(l, "- ")))
		case len(l) > 2 && l[0] >= '0' && l[0] <= '9' && strings.Contains(l[:min(4, len(l))], ". "):
			flush()
			n, rest, _ := strings.Cut(l, ". ")
			emit(u.point(n+".", rest))
		case strings.HasSuffix(l, ":") && len(l) < 40:
			flush()
			emit(u.text(sizeSmall, u.p.Muted, l))
		default:
			para = append(para, l)
		}
	}
	flush()
	return out
}

func (u *UI) point(mark, text string) W {
	return func(gtx C) D {
		return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Gap: gtx.Dp(8)}.Layout(gtx,
				layout.Rigid(width(18, u.text(sizeBody, u.p.Faint, mark))),
				layout.Flexed(1, u.text(sizeBody, u.p.Text, text)))
		})
	}
}

// reviews is what the reviewing agents said about one kind of document,
// the latest round last.
func (u *UI) reviews(dir, kind string) []W {
	matches := u.docs.glob(filepath.Join(dir, "reviews", kind+"-*.md"))
	var out []W
	for _, m := range matches {
		lines := u.docs.read(m)
		title := strings.TrimPrefix(firstLine(lines), "# ")
		verdict := ""
		if len(lines) > 2 {
			verdict = strings.TrimSpace(lines[2])
		}
		body := append([]W{u.strong(sizeBody, u.p.Text, title), u.text(sizeSmall, u.p.Muted, verdict)}, u.findings(lines)...)
		out = append(out, box(u.p.Surface, u.p.Line, 8, 10, column(6, body...)))
	}
	return out
}

// findings are the rows of a review's table, and its questions.
func (u *UI) findings(lines []string) []W {
	var out []W
	questions := false
	for _, l := range lines {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "## Questions"):
			questions = true
		case questions && strings.HasPrefix(l, "- "):
			out = append(out, u.point("?", strings.TrimPrefix(l, "- ")))
		case strings.HasPrefix(l, "| ") && !strings.HasPrefix(l, "| Severity"):
			cells := strings.Split(strings.Trim(l, "| "), " | ")
			if len(cells) < 3 {
				continue
			}
			bg, fg := u.p.Sunken, u.p.Muted
			switch cells[0] {
			case "blocking":
				bg, fg = u.p.FailSoft, u.p.Fail
			case "major":
				bg, fg = u.p.NoteSoft, u.p.Note
			}
			text := cells[1] + ": " + cells[2]
			if len(cells) > 3 && cells[3] != "" {
				text += " — " + cells[3]
			}
			out = append(out, func(gtx C) D {
				return layout.Flex{Gap: gtx.Dp(8)}.Layout(gtx,
					layout.Rigid(width(70, u.chip(bg, fg, cells[0]))),
					layout.Flexed(1, u.text(sizeSmall, u.p.Text, text)))
			})
		}
	}
	return out
}

func firstLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// laterOf is what a specification says comes after it: the parts of a task
// too large for one change, each a task of its own.
func (u *UI) laterOf(dir string) []string {
	var out []string
	in := false
	for _, l := range u.docs.read(filepath.Join(dir, "01-specification.md")) {
		l = strings.TrimSpace(l)
		switch {
		case l == "### What comes after":
			in = true
		case strings.HasPrefix(l, "#"):
			in = false
		case in && strings.HasPrefix(l, "- "):
			out = append(out, strings.TrimPrefix(l, "- "))
		}
	}
	return out
}
