package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

type colorNRGBA = color.NRGBA

// bar is one proportion: a title, a stacked bar, and its legend.
func (u *UI) bar(title string, segs []segment) W {
	return func(gtx C) D {
		total := 0
		for _, s := range segs {
			total += s.n
		}
		w := gtx.Dp(260)
		h := gtx.Dp(8)
		track := func(gtx C) D {
			rect := image.Rect(0, 0, w, h)
			paint.FillShape(gtx.Ops, u.p.Sunken, clip.UniformRRect(rect, h/2).Op(gtx.Ops))
			if total > 0 {
				defer clip.UniformRRect(rect, h/2).Push(gtx.Ops).Pop()
				x := 0
				for _, s := range segs {
					sw := w * s.n / total
					if s.n > 0 && sw == 0 {
						sw = 1
					}
					st := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
					paint.FillShape(gtx.Ops, s.color, clip.Rect{Max: image.Pt(sw, h)}.Op())
					st.Pop()
					x += sw
				}
			}
			return D{Size: image.Pt(w, h)}
		}
		var legend []W
		for _, s := range segs {
			legend = append(legend, row(4, dot(s.color, 7), u.text(sizeSmall, u.p.Muted, fmt.Sprintf("%s %d", s.label, s.n))))
		}
		return column(6,
			u.text(sizeSmall, u.p.Faint, title),
			track,
			row(10, legend...),
		)(gtx)
	}
}

type segment struct {
	label string
	n     int
	color colorNRGBA
}
