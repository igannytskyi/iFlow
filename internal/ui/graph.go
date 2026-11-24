package ui

import (
	"cmp"
	"fmt"
	"image"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/igannytskyi/iflow/internal/artifact"
	"github.com/igannytskyi/iflow/internal/flow"
	"github.com/igannytskyi/iflow/internal/harness"
)

// graph is the path as what it is: six areas, each depending on the one
// before. A node says how its area stands, how long it took and what it
// cost, how many objects it made, and while it works what it is doing now;
// an edge lights up once its objects have crossed. Choosing a node opens
// its area below.
func (u *UI) graph(p *project, s shown) W {
	return func(gtx C) D {
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
		var cost []string
		if took != "" {
			cost = append(cost, took)
		}
		if spent > 0 {
			cost = append(cost, money(spent))
		}
		count := ""
		if objects > 0 {
			count = plural(objects, "object")
		}
		// While the area works, what is being done in it now: the latest
		// thing its run said.
		now := ""
		if s.live && status == flow.Working {
			for _, st := range s.v.Steps {
				if st.Area == n {
					now = st.Text
				}
			}
			// In the execution, the agents at work, each by its unit and
			// what it did last: what the run said is only that it began.
			if n == 4 {
				if at := atWork(s.v.Agent); len(at) > 0 {
					now = strings.Join(at, "\n")
				}
			}
		}
		return btn.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return box(bg, border, 8, 10, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				// Every node has the same rows at the same heights, so that
				// what one node says stands level with the same in the next.
				small := gtx.Sp(sizeSmall) * 3 / 2
				body := gtx.Sp(sizeBody) * 3 / 2
				rows := []W{
					slot(small, row(6, u.mono(sizeSmall, u.p.Accent, fmt.Sprintf("%02d", n)), u.oneLine(sizeSmall, u.stateText(status), statusWord(status)))),
					slot(body*2, u.clipped(sizeBody, u.p.Text, a.Name, 2, true)),
					slot(small, u.oneLine(sizeSmall, u.p.Muted, dash(strings.Join(cost, " · ")))),
					slot(small, u.oneLine(sizeSmall, u.p.Faint, count)),
				}
				if s.live && !s.v.Finished {
					rows = append(rows, slot(small*2, u.clipped(sizeSmall, u.p.Accent, now, 2, false)))
				}
				return column(2, rows...)(gtx)
			})(gtx)
		})
	}
}

// slot lays w in a row exactly h high, so that rows line up across nodes.
func slot(h int, w W) W {
	return func(gtx C) D {
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = 0, h
		d := w(gtx)
		d.Size.Y = h
		return d
	}
}

// clipped is text cut to at most n lines.
func (u *UI) clipped(size unit.Sp, c colorNRGBA, s string, n int, strong bool) W {
	return func(gtx C) D {
		l := material.Label(u.th, size, s)
		l.Color, l.MaxLines = c, n
		if strong {
			l.Font.Weight = font.SemiBold
		}
		return l.Layout(gtx)
	}
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
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
	switch s {
	case flow.Idle:
		return "not yet"
	case flow.Waiting:
		return "waiting" // for you: the node is too narrow to say so
	}
	return string(s)
}

// atWork is each agent still at work, by its unit, and what it did last —
// a call, a saying, a refusal: a line each, the units in order.
func atWork(agent map[string][]harness.Event) []string {
	var out []string
	for _, unit := range sortedStrings(agent) {
		events := agent[unit]
		if len(events) == 0 || events[len(events)-1].Kind == harness.Ended {
			continue
		}
		// What it did, not what it was answered or what it cost so far.
		i := len(events) - 1
		for i > 0 && (events[i].Kind == harness.Spent || events[i].Kind == harness.Got) {
			i--
		}
		last := events[i]
		what := cmp.Or(last.Tool, string(last.Kind))
		if d := strings.Join(strings.Fields(last.Detail), " "); d != "" {
			what += " " + d
		}
		out = append(out, unit+" · "+what)
	}
	return out
}
