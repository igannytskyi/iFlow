package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
	W = layout.Widget
)

// text draws a line or paragraph in a size and colour.
func (u *UI) text(size unit.Sp, c color.NRGBA, s string) W {
	return func(gtx C) D {
		l := material.Label(u.th, size, s)
		l.Color = c
		return l.Layout(gtx)
	}
}

// strong is text in the heavier weight.
func (u *UI) strong(size unit.Sp, c color.NRGBA, s string) W {
	return func(gtx C) D {
		l := material.Label(u.th, size, s)
		l.Color = c
		l.Font.Weight = font.SemiBold
		return l.Layout(gtx)
	}
}

// mono is text in the fixed-width face, for identifiers and commands.
func (u *UI) mono(size unit.Sp, c color.NRGBA, s string) W {
	return func(gtx C) D {
		l := material.Label(u.th, size, s)
		l.Color = c
		l.Font.Typeface = "Go Mono"
		return l.Layout(gtx)
	}
}

// oneLine is text cut to a single line.
func (u *UI) oneLine(size unit.Sp, c color.NRGBA, s string) W {
	return func(gtx C) D {
		l := material.Label(u.th, size, s)
		l.Color = c
		l.MaxLines = 1
		return l.Layout(gtx)
	}
}

// box draws a rounded rectangle behind a widget, with an optional border.
func box(bg, border color.NRGBA, radius unit.Dp, inset unit.Dp, w W) W {
	return func(gtx C) D {
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(inset).Layout(gtx, w)
		call := macro.Stop()
		r := gtx.Dp(radius)
		rect := image.Rectangle{Max: dims.Size}
		if border.A > 0 {
			paint.FillShape(gtx.Ops, border, clip.UniformRRect(rect, r).Op(gtx.Ops))
			inner := rect.Inset(1)
			paint.FillShape(gtx.Ops, bg, clip.UniformRRect(inner, max(r-1, 0)).Op(gtx.Ops))
		} else if bg.A > 0 {
			paint.FillShape(gtx.Ops, bg, clip.UniformRRect(rect, r).Op(gtx.Ops))
		}
		call.Add(gtx.Ops)
		return dims
	}
}

// fill paints the whole of the constraints in a colour.
func fill(gtx C, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

// rule is a hairline across the available width.
func (u *UI) hairline(gtx C) D {
	h := max(gtx.Dp(1), 1)
	size := image.Pt(gtx.Constraints.Max.X, h)
	paint.FillShape(gtx.Ops, u.p.Line, clip.Rect{Max: size}.Op())
	return D{Size: size}
}

// dot is a small filled circle.
func dot(c color.NRGBA, d unit.Dp) W {
	return func(gtx C) D {
		s := gtx.Dp(d)
		paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(s, s)}.Op(gtx.Ops))
		return D{Size: image.Pt(s, s)}
	}
}

// chip is a short label on a soft background.
func (u *UI) chip(bg, fg color.NRGBA, s string) W {
	return box(bg, color.NRGBA{}, 4, 0, func(gtx C) D {
		return layout.Inset{Left: 6, Right: 6, Top: 2, Bottom: 2}.Layout(gtx, u.mono(sizeSmall, fg, s))
	})
}

// pressable makes a widget clickable with a hover tint and a hand cursor.
func (u *UI) pressable(gtx C, btn *widget.Clickable, selected bool, radius unit.Dp, w W) D {
	return btn.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		dims := w(gtx)
		call := macro.Stop()
		var bg color.NRGBA
		switch {
		case selected:
			bg = u.p.AccentSoft
		case btn.Hovered():
			bg = u.p.Sunken
		}
		if bg.A > 0 {
			paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(radius)).Op(gtx.Ops))
		}
		call.Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return dims
	})
}

// clicks hands out one Clickable per name, so that a widget drawn anew each
// frame keeps its state.
type clicks map[string]*widget.Clickable

func (c clicks) of(name string) *widget.Clickable {
	b, ok := c[name]
	if !ok {
		b = new(widget.Clickable)
		c[name] = b
	}
	return b
}

// lists hands out one List per name.
type lists map[string]*widget.List

func (l lists) of(name string) *widget.List {
	v, ok := l[name]
	if !ok {
		v = &widget.List{List: layout.List{Axis: layout.Vertical}}
		l[name] = v
	}
	return v
}

// column lays widgets out top to bottom with a gap between them.
func column(space unit.Dp, ws ...W) W {
	return func(gtx C) D {
		children := make([]layout.FlexChild, 0, len(ws))
		for _, w := range ws {
			children = append(children, layout.Rigid(w))
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(space)}.Layout(gtx, children...)
	}
}

// row lays widgets out left to right, centred, with a gap between them.
func row(space unit.Dp, ws ...W) W {
	return func(gtx C) D {
		children := make([]layout.FlexChild, 0, len(ws))
		for _, w := range ws {
			children = append(children, layout.Rigid(w))
		}
		return layout.Flex{Alignment: layout.Middle, Gap: gtx.Dp(space)}.Layout(gtx, children...)
	}
}

// width fixes a widget's width.
func width(w unit.Dp, wd W) W {
	return func(gtx C) D {
		px := gtx.Dp(w)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		return wd(gtx)
	}
}

// flexWidth gives a widget all the width it is offered.
func stretch(wd W) W {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return wd(gtx)
	}
}

func inset(top, side unit.Dp, w W) W {
	return func(gtx C) D {
		return layout.Inset{Top: top, Bottom: top, Left: side, Right: side}.Layout(gtx, w)
	}
}

func nothing(gtx C) D { return D{} }

// wrap lays widgets out left to right, starting a new line when the next one
// would not fit.
func (u *UI) wrap(space unit.Dp, ws []W) W {
	return func(gtx C) D {
		maxW := gtx.Constraints.Max.X
		s := gtx.Dp(space)
		x, y, lineH, width := 0, 0, 0, 0
		for _, w := range ws {
			cgtx := gtx
			cgtx.Constraints.Min = image.Point{}
			macro := recordOp(cgtx, w)
			if x > 0 && x+macro.dims.Size.X > maxW {
				x, y, lineH = 0, y+lineH+s, 0
			}
			macro.at(gtx, x, y)
			x += macro.dims.Size.X + s
			lineH = max(lineH, macro.dims.Size.Y)
			width = max(width, x-s)
		}
		return D{Size: image.Pt(width, y+lineH)}
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	if word == "criterion" {
		return fmt.Sprintf("%d criteria", n)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
