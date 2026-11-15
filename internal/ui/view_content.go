package ui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/igannytskyi/iflow/internal/artifact"
)

// content is what an area hands on, as the thing itself rather than its
// record: the plan and its tickets, the prior state captured, what each
// unit changed, the output the verdicts rest on, what entered. The record —
// the method's metadata about it — is drawn under it, apart.
func (u *UI) content(p *project, s shown, n int) []W {
	if s.c == nil {
		return nil
	}
	dir := s.c.Change.Dir
	switch n {
	case 2:
		return u.planContent(p, dir)
	case 3:
		return u.admissionContent(p, dir)
	case 4:
		return u.executionContent(p, dir)
	case 5:
		return u.assuranceContent(p, dir)
	case 6:
		return u.landingContent(p, dir)
	}
	return nil
}

func (u *UI) planContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "THE PLAN — how the change is built, and every unit as its agent is told it")}
	doc := u.prose(filepath.Join(dir, artifact.PlanFile), "")
	if len(doc) == 0 {
		doc = u.prose(filepath.Join(dir, "02-plan.md"), "") // written before the plan had a document of its own
	}
	if len(doc) > 0 {
		out = append(out, box(u.p.Surface, u.p.Line, 8, 14, column(8, doc...)))
	}
	tickets := u.docs.glob(filepath.Join(dir, artifact.UnitsDir, "*.md"))
	for _, t := range tickets {
		lines := u.docs.read(t)
		out = append(out, u.fold(p, "ticket/"+t, strings.TrimPrefix(firstLine(lines), "# "), "the ticket, word for word",
			func() []W { return u.prose(t, "") }))
	}
	return out
}

func (u *UI) admissionContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "PRIOR STATE — every file the change may touch, as it stood before, to compare against")}
	files := u.docs.glob(filepath.Join(dir, "baseline", "*"))
	if len(files) == 0 {
		out = append(out, u.text(sizeSmall, u.p.Faint, "nothing captured"))
	}
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasSuffix(name, ".absent") {
			out = append(out, u.text(sizeSmall, u.p.Muted, strings.TrimSuffix(name, ".absent")+" — did not exist before this change"))
			continue
		}
		out = append(out, u.fold(p, "baseline/"+f, strings.TrimSuffix(name, ".before"), "as it stood", func() []W { return u.code(f, 400) }))
	}
	return out
}

func (u *UI) executionContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "WHAT EACH UNIT CHANGED — against the prior state, line by line")}
	cands := u.docs.glob(filepath.Join(dir, "candidates", "CA-*"))
	for _, ca := range cands {
		id := filepath.Base(ca)
		summary := strings.TrimSpace(strings.Join(u.docs.read(filepath.Join(ca, "summary.txt")), " "))
		head := []W{row(10, u.mono(sizeSmall, u.p.Accent, id), u.text(sizeSmall, u.p.Text, summary))}
		if u.docs.exists(filepath.Join(ca, "pack.md")) {
			rel := "candidates/" + id + "/pack.md"
			btn := u.clicks.of("pack/" + ca)
			head = append(head, func(gtx C) D {
				if u.clicked(gtx, btn) {
					p.handedView, p.item = rel, nil
				}
				return u.pressable(gtx, btn, p.handedView == rel, 4, inset(2, 6, u.text(sizeSmall, u.p.Accent, "what its agent was handed ›")))
			})
		}
		out = append(out, column(4, head...))
		for _, f := range u.candidateFiles(ca) {
			rel, err := filepath.Rel(filepath.Join(ca, "files"), f)
			if err != nil || strings.HasPrefix(rel, "..") {
				rel = filepath.Base(f)
			}
			before := filepath.Join(dir, "baseline", path.Base(filepath.ToSlash(rel))+".before")
			d := memo(u.docs, "diff|"+before+"|"+f, func() diffResult {
				a, r, l := diff(u.lines(before), u.lines(f))
				return diffResult{a, r, l}
			})
			out = append(out, u.fold(p, "diff/"+f, filepath.ToSlash(rel), fmt.Sprintf("+%d −%d", d.added, d.removed), func() []W { return u.diffLines(d.lines) }))
		}
	}
	if len(cands) == 0 {
		out = append(out, u.text(sizeSmall, u.p.Faint, "no candidate yet"))
	}
	return out
}

func (u *UI) assuranceContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "WHAT THE VERDICTS REST ON — the output of every run, as it was")}
	runs := u.docs.glob(filepath.Join(dir, "evidence", "*.txt"))
	before := u.docs.glob(filepath.Join(dir, "candidates", "*", "failing-before.txt"))
	for _, f := range append(before, runs...) {
		title := strings.TrimSuffix(filepath.Base(f), ".txt")
		if filepath.Base(f) == "failing-before.txt" {
			title = filepath.Base(filepath.Dir(f)) + " — its tests before the change existed"
		}
		lines := u.lines(f)
		verdict := "passed"
		for _, l := range lines {
			if strings.HasPrefix(l, "FAIL") || strings.HasPrefix(l, "--- FAIL") {
				verdict = "failed"
			}
		}
		out = append(out, u.fold(p, "run/"+f, title, verdict, func() []W { return u.code(f, 300) }))
	}
	if len(runs)+len(before) == 0 {
		out = append(out, u.text(sizeSmall, u.p.Faint, "no run yet"))
	}
	return out
}

func (u *UI) landingContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "WHAT ENTERED THE PROJECT")}
	var files []string
	for _, ca := range u.docs.glob(filepath.Join(dir, "candidates", "CA-*")) {
		for _, f := range u.candidateFiles(ca) {
			rel, err := filepath.Rel(filepath.Join(ca, "files"), f)
			if err != nil || strings.HasPrefix(rel, "..") {
				rel = filepath.Base(f)
			}
			files = append(files, filepath.ToSlash(rel))
		}
	}
	if !u.docs.exists(filepath.Join(dir, "06-landing.md")) {
		return append(out, u.text(sizeSmall, u.p.Faint, "nothing yet: the change has not landed"))
	}
	slices.Sort(files)
	for _, f := range slices.Compact(files) {
		out = append(out, u.mono(sizeSmall, u.p.Text, f))
	}
	return out
}

// fold is a titled block opened with a click.
func (u *UI) fold(p *project, key, title, note string, body func() []W) W {
	return func(gtx C) D {
		btn := u.clicks.of("fold/" + key)
		if u.clicked(gtx, btn) {
			p.open[key] = !p.open[key]
		}
		mark := "▸"
		if p.open[key] {
			mark = "▾"
		}
		head := func(gtx C) D {
			return u.pressable(gtx, btn, false, 4, stretch(inset(4, 6, func(gtx C) D {
				return layout.Flex{Gap: gtx.Dp(8), Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(u.text(sizeSmall, u.p.Muted, mark)),
					layout.Flexed(1, u.oneLine(sizeBody, u.p.Text, title)),
					layout.Rigid(u.oneLine(sizeSmall, u.p.Muted, note)))
			})))
		}
		if !p.open[key] {
			return head(gtx)
		}
		return column(4, head, box(u.p.Surface, u.p.Line, 6, 10, column(2, body()...)))(gtx)
	}
}

// code is a file's lines as they are, at most limit of them.
func (u *UI) code(file string, limit int) []W {
	lines := u.lines(file)
	var out []W
	for i, l := range lines {
		if i == limit {
			out = append(out, u.text(sizeSmall, u.p.Faint, fmt.Sprintf("… %d more lines", len(lines)-limit)))
			break
		}
		out = append(out, u.mono(sizeSmall, u.p.Text, l))
	}
	return out
}

// lines is a file's lines without the newline that ends the last.
func (u *UI) lines(file string) []string {
	l := u.docs.read(file)
	for len(l) > 0 && l[len(l)-1] == "" {
		l = l[:len(l)-1]
	}
	return l
}

// candidateFiles is every file a candidate holds, kept until something
// changes.
func (u *UI) candidateFiles(ca string) []string {
	return memo(u.docs, "files|"+ca, func() []string {
		var out []string
		_ = filepath.WalkDir(filepath.Join(ca, "files"), func(f string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() != "CHANGES" {
				out = append(out, f)
			}
			return err
		})
		if len(out) == 0 {
			// Candidates kept before files/ held them lie beside what was
			// said about them.
			meta := map[string]bool{"summary.txt": true, "failing-before.txt": true, "passing-after.txt": true, "pack.md": true, "trace.jsonl": true, "trace.tsv": true}
			entries, _ := os.ReadDir(ca)
			for _, e := range entries {
				if !e.IsDir() && !meta[e.Name()] && !strings.HasSuffix(e.Name(), ".patch") {
					out = append(out, filepath.Join(ca, e.Name()))
				}
			}
		}
		return out
	})
}

type diffResult struct {
	added, removed int
	lines          []diffLine
}

// diffLine is one line of a difference: kept, added or removed.
type diffLine struct {
	op   byte // ' ', '+', '-', or '~' for lines folded away
	text string
}

// diff is what changed from a to b, as lines with three of context around
// each change, and how many lines each side gained and lost.
func diff(a, b []string) (added, removed int, out []diffLine) {
	n, m := len(a), len(b)
	if n*m > 4_000_000 {
		for _, l := range b {
			out = append(out, diffLine{'+', l})
		}
		return len(b), len(a), out
	}
	// The longest common subsequence, from the end, then walked forward.
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var all []diffLine
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			all = append(all, diffLine{' ', a[i]})
			i, j = i+1, j+1
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			all = append(all, diffLine{'+', b[j]})
			added++
			j++
		default:
			all = append(all, diffLine{'-', a[i]})
			removed++
			i++
		}
	}
	// Unchanged stretches far from any change are folded to one line.
	near := make([]bool, len(all))
	for k, l := range all {
		if l.op != ' ' {
			for d := max(0, k-3); d <= min(len(all)-1, k+3); d++ {
				near[d] = true
			}
		}
	}
	for k := 0; k < len(all); k++ {
		if near[k] {
			out = append(out, all[k])
			continue
		}
		start := k
		for k < len(all) && !near[k] {
			k++
		}
		out = append(out, diffLine{'~', fmt.Sprintf("… %d unchanged lines", k-start)})
		k--
	}
	return added, removed, out
}

func (u *UI) diffLines(lines []diffLine) []W {
	var out []W
	for _, l := range lines {
		switch l.op {
		case '+':
			out = append(out, box(u.p.OKSoft, colorNRGBA{}, 0, 0, u.mono(sizeSmall, u.p.OK, "+ "+l.text)))
		case '-':
			out = append(out, box(u.p.FailSoft, colorNRGBA{}, 0, 0, u.mono(sizeSmall, u.p.Fail, "− "+l.text)))
		case '~':
			out = append(out, u.text(sizeSmall, u.p.Faint, l.text))
		default:
			out = append(out, u.mono(sizeSmall, u.p.Muted, "  "+l.text))
		}
	}
	return out
}
