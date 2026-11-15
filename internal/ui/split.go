package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// split lays two panels side by side with a boundary a person can drag. The
// width of the left panel is kept in dp, so that it stays put when the window
// is resized, and is remembered between sessions.
type split struct {
	name     string
	left     unit.Dp // the left panel's width
	min, max unit.Dp
	drag     gesture.Drag
	from     float32 // pointer x where a drag began, in px
	start    unit.Dp // width where a drag began
	moved    func(name string, width unit.Dp)
}

const handle = unit.Dp(7)

// fixedRight lays a split whose right panel has the remembered width: for a
// panel that hangs off the right edge, like an inspector.
func (s *split) layout(gtx C, u *UI, left, right W, rightFixed bool) D {
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		switch e.Kind {
		case pointer.Press:
			s.from, s.start = e.Position.X, s.left
		case pointer.Drag:
			delta := unit.Dp((e.Position.X - s.from) / gtx.Metric.PxPerDp)
			if rightFixed {
				delta = -delta
			}
			s.left = min(max(s.start+delta, s.min), s.max)
		case pointer.Release, pointer.Cancel:
			if s.moved != nil {
				s.moved(s.name, s.left)
			}
		}
	}
	total := gtx.Constraints.Max.X
	h := gtx.Dp(handle)
	fixed := min(gtx.Dp(s.left), total-h-gtx.Dp(200))
	leftW := fixed
	if rightFixed {
		leftW = total - h - fixed
	}
	rightW := total - h - leftW

	lg := gtx
	lg.Constraints = layout.Exact(image.Pt(leftW, gtx.Constraints.Max.Y))
	left(lg)

	// The boundary: a hairline, wider to the pointer than to the eye.
	st := op.Offset(image.Pt(leftW, 0)).Push(gtx.Ops)
	bar := image.Rect(0, 0, h, gtx.Constraints.Max.Y)
	area := clip.Rect(bar).Push(gtx.Ops)
	pointer.CursorColResize.Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	area.Pop()
	line := u.p.Line
	if s.drag.Dragging() {
		line = u.p.Accent
	}
	paint.FillShape(gtx.Ops, line, clip.Rect(image.Rect(h/2, 0, h/2+max(gtx.Dp(1), 1), gtx.Constraints.Max.Y)).Op())
	st.Pop()

	st = op.Offset(image.Pt(leftW+h, 0)).Push(gtx.Ops)
	rg := gtx
	rg.Constraints = layout.Exact(image.Pt(rightW, gtx.Constraints.Max.Y))
	right(rg)
	st.Pop()
	return D{Size: gtx.Constraints.Max}
}

// splitOf hands out one split per name, sized as last left.
func (u *UI) splitOf(name string, initial, lo, hi unit.Dp) *split {
	if s, ok := u.splits[name]; ok {
		return s
	}
	w := initial
	if saved, ok := u.settings.Widths[name]; ok {
		w = unit.Dp(saved)
	}
	s := &split{name: name, left: w, min: lo, max: hi, moved: func(n string, w unit.Dp) {
		u.settings.Widths[n] = float32(w)
		u.settings.save()
	}}
	u.splits[name] = s
	return s
}
