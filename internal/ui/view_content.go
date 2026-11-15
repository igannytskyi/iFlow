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
	tickets, _ := filepath.Glob(filepath.Join(dir, artifact.UnitsDir, "*.md"))
	slices.Sort(tickets)
	for _, t := range tickets {
		lines := u.docs.read(t)
		out = append(out, u.fold(p, "ticket/"+t, strings.TrimPrefix(firstLine(lines), "# "), "the ticket, word for word",
			func() []W { return u.prose(t, "") }))
	}
	return out
}

func (u *UI) admissionContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "PRIOR STATE — every file the change may touch, as it stood before, to compare against")}
	files, _ := filepath.Glob(filepath.Join(dir, "baseline", "*"))
	slices.Sort(files)
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
	cands, _ := filepath.Glob(filepath.Join(dir, "candidates", "CA-*"))
	slices.Sort(cands)
	for _, ca := range cands {
		id := filepath.Base(ca)
		summary := strings.TrimSpace(strings.Join(u.docs.read(filepath.Join(ca, "summary.txt")), " "))
		head := []W{row(10, u.mono(sizeSmall, u.p.Accent, id), u.text(sizeSmall, u.p.Text, summary))}
		if _, err := os.Stat(filepath.Join(ca, "pack.md")); err == nil {
			rel := "candidates/" + id + "/pack.md"
			btn := u.clicks.of("pack/" + ca)
			head = append(head, func(gtx C) D {
				if btn.Clicked(gtx) {
					p.handedView, p.item = rel, nil
				}
				return u.pressable(gtx, btn, p.handedView == rel, 4, inset(2, 6, u.text(sizeSmall, u.p.Accent, "what its agent was handed ›")))
			})
		}
		out = append(out, column(4, head...))
		_ = filepath.WalkDir(filepath.Join(ca, "files"), func(f string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() == "CHANGES" {
				return err
			}
			rel, _ := filepath.Rel(filepath.Join(ca, "files"), f)
			before := filepath.Join(dir, "baseline", path.Base(filepath.ToSlash(rel))+".before")
			added, removed, lines := diff(readLines(before), readLines(f))
			out = append(out, u.fold(p, "diff/"+f, filepath.ToSlash(rel), fmt.Sprintf("+%d −%d", added, removed), func() []W { return u.diffLines(lines) }))
			return nil
		})
	}
	if len(cands) == 0 {
		out = append(out, u.text(sizeSmall, u.p.Faint, "no candidate yet"))
	}
	return out
}

func (u *UI) assuranceContent(p *project, dir string) []W {
	out := []W{u.text(sizeSmall, u.p.Faint, "WHAT THE VERDICTS REST ON — the output of every run, as it was")}
	runs, _ := filepath.Glob(filepath.Join(dir, "evidence", "*.txt"))
	before, _ := filepath.Glob(filepath.Join(dir, "candidates", "*", "failing-before.txt"))
	for _, f := range append(before, runs...) {
		title := strings.TrimSuffix(filepath.Base(f), ".txt")
		if filepath.Base(f) == "failing-before.txt" {
			title = filepath.Base(filepath.Dir(f)) + " — its tests before the change existed"
		}
		lines := readLines(f)
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
	cands, _ := filepath.Glob(filepath.Join(dir, "candidates", "CA-*", "files"))
	for _, c := range cands {
		_ = filepath.WalkDir(c, func(f string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() != "CHANGES" {
				rel, _ := filepath.Rel(c, f)
				files = append(files, filepath.ToSlash(rel))
			}
			return err
		})
	}
	if _, err := os.Stat(filepath.Join(dir, "06-landing.md")); err != nil {
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
		if btn.Clicked(gtx) {
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
	lines := readLines(file)
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

func readLines(file string) []string {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
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
