package ui

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
)

// graph is the path as what it is: six areas, each depending on the one
// before, each naming the objects it hands to the next. A node says how
// its area stands, what it cost and how long it took; an edge lights up
// once its objects have crossed. Choosing a node opens its area below.
func (u *UI) graph(p *project, s shown) W {
	return func(gtx C) D {
		// Every node is as tall as the tallest was last frame, so the path
		// reads as one row however its names wrap.
		p.nodeH, p.nextH = p.nextH, 0
		var children []layout.FlexChild
		for n := 1; n <= 6; n++ {
			if n > 1 {
				crossed := s.status[n-1] == flow.Done
				children = append(children, layout.Rigid(inset(0, 3, u.edge(crossed))))
			}
			children = append(children, layout.Flexed(1, u.node(p, s, n)))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// crossing is what the contracts say passes from one area to the next.
func (u *UI) crossing(from, to int) []string {
	var out []string
	for _, c := range u.model.Contracts {
		if slices.Contains(c.Producers, fmt.Sprint(from)) && slices.Contains(c.Consumers, to) {
			out = append(out, c.Objects...)
		}
	}
	return out
}

func (u *UI) node(p *project, s shown, n int) W {
	return func(gtx C) D {
		btn := u.clicks.of(fmt.Sprintf("graph/%s/%d", p.path, n))
		if u.clicked(gtx, btn) {
			p.area = n
		}
		a := u.model.Area(n)
		status := s.status[n]
		border, bg := u.p.Line, u.p.Surface
		switch status {
		case flow.Working:
			border, bg = u.p.Accent, u.p.AccentSoft
		case flow.Waiting:
			border, bg = u.p.Note, u.p.NoteSoft
		case flow.Failed:
			border, bg = u.p.Fail, u.p.FailSoft
		case flow.Done:
			border = u.p.OK
		}
		if p.area == n {
			border = u.p.Text
		}
		mine := inArea(s.costs, n)
		spent := 0.0
		for _, c := range mine {
			spent += c.CostUSD
		}
		took := ""
		if s.live {
			if first, last := areaTimes(s.v, n); !first.IsZero() {
				took = elapsed(last.Sub(first).Milliseconds())
			}
		} else if ms := artifact.Total(mine).ElapsedMS; ms > 0 {
			took = elapsed(ms)
		}
		objects := 0
		if s.c != nil {
			objects = len(s.c.InArea(n))
		}
		var facts []string
		if took != "" {
			facts = append(facts, took)
		}
		if spent > 0 {
			facts = append(facts, money(spent))
		}
		if objects > 0 {
			facts = append(facts, plural(objects, "object"))
		}
		hands := ""
		if n < 6 {
			hands = "→ " + strings.Join(u.crossing(n, n+1), ", ")
		}
		handColor := u.p.Faint
		if status == flow.Done {
			handColor = u.p.OK
		}
		return btn.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return box(bg, border, 8, 10, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				want := max(gtx.Dp(76), p.nodeH)
				gtx.Constraints.Min.Y = 0
				d := column(4,
					row(6, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", n)), u.oneLine(sizeSmall, u.stateText(status), statusWord(status))),
					u.strong(sizeBody, u.p.Text, a.Name),
					u.text(sizeSmall, u.p.Muted, strings.Join(facts, " · ")),
					u.text(sizeSmall, handColor, hands),
				)(gtx)
				p.nextH = max(p.nextH, d.Size.Y)
				d.Size.Y = max(d.Size.Y, want)
				return d
			})(gtx)
		})
	}
}

func (u *UI) stateText(s flow.Status) colorNRGBA {
	switch s {
	case flow.Working:
		return u.p.Accent
	case flow.Waiting:
		return u.p.Note
	case flow.Failed:
		return u.p.Fail
	case flow.Done:
		return u.p.OK
	}
	return u.p.Faint
}

// edge is an arrow between two areas, green once its objects have crossed.
func (u *UI) edge(crossed bool) W {
	return func(gtx C) D {
		c := u.p.Line
		if crossed {
			c = u.p.OK
		}
		aw, ah := gtx.Dp(22), gtx.Dp(12)
		mid := float32(ah) / 2
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32pt(0, mid))
		path.LineTo(f32pt(float32(aw-2), mid))
		path.MoveTo(f32pt(float32(aw-8), mid-4))
		path.LineTo(f32pt(float32(aw-2), mid))
		path.LineTo(f32pt(float32(aw-8), mid+4))
		paint.FillShape(gtx.Ops, c, clip.Stroke{Path: path.End(), Width: float32(gtx.Dp(1.4))}.Op())
		return D{Size: image.Pt(aw, ah)}
	}
}

// statusWord is how an area stands, said to a person.
func statusWord(s flow.Status) string {
	if s == flow.Idle {
		return "not yet"
	}
	return string(s)
}
