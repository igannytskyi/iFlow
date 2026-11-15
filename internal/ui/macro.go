package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
)

// recorded is a widget laid out once and drawn later, somewhere else.
type recorded struct {
	call op.CallOp
	dims D
}

func recordOp(gtx C, w W) recorded {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	return recorded{call: m.Stop(), dims: dims}
}

func (r recorded) at(gtx C, x, y int) {
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	r.call.Add(gtx.Ops)
}

func f32pt(x, y float32) f32.Point { return f32.Pt(x, y) }
