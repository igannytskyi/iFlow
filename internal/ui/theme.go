// Package ui is the desktop view of the method: its areas and the objects
// that cross between them, its rules, and every change carried through it,
// down to each cell the gate read.
//
// It decides nothing. Everything it shows is read from the method's
// documents, the change folders and the gate, and what it has not been told
// yet it says it is waiting for.
package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// palette is every colour the interface uses, by role.
type palette struct {
	Bg, Surface, Sunken, Line, Text, Muted, Faint color.NRGBA
	Accent, AccentSoft                            color.NRGBA
	Fail, FailSoft, Note, NoteSoft, OK, OKSoft    color.NRGBA
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

var lightPalette = palette{
	Bg: rgb(0xf7f7f5), Surface: rgb(0xffffff), Sunken: rgb(0xf0efec), Line: rgb(0xe4e2dd),
	Text: rgb(0x1c1b19), Muted: rgb(0x6f6b64), Faint: rgb(0xa29d94),
	Accent: rgb(0x2b59c3), AccentSoft: rgb(0xe6edfa),
	Fail: rgb(0xc2362b), FailSoft: rgb(0xfbe9e7), Note: rgb(0x9a6a10), NoteSoft: rgb(0xfaf1dc),
	OK: rgb(0x2f7d4a), OKSoft: rgb(0xe5f2e8),
}

var darkPalette = palette{
	Bg: rgb(0x131312), Surface: rgb(0x1c1c1a), Sunken: rgb(0x242422), Line: rgb(0x33322f),
	Text: rgb(0xedebe6), Muted: rgb(0xa39e95), Faint: rgb(0x6f6b64),
	Accent: rgb(0x7ea2f0), AccentSoft: rgb(0x22304d),
	Fail: rgb(0xf0877c), FailSoft: rgb(0x3d2422), Note: rgb(0xe2b75a), NoteSoft: rgb(0x3a3020),
	OK: rgb(0x7cc795), OKSoft: rgb(0x1f3326),
}

// Type sizes.
const (
	sizeSmall = unit.Sp(11.5)
	sizeBody  = unit.Sp(13)
	sizeTitle = unit.Sp(15)
	sizeHead  = unit.Sp(20)
)

// Spacing.
const (
	gap    = unit.Dp(8)
	pad    = unit.Dp(16)
	gutter = unit.Dp(24)
)

func newTheme(p palette) *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Palette = material.Palette{Bg: p.Bg, Fg: p.Text, ContrastBg: p.Accent, ContrastFg: p.Surface}
	th.TextSize = sizeBody
	return th
}
