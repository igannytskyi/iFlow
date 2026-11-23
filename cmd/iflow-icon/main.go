// Command iflow-icon draws the workbench's icon: three stages of a change
// rising from left to right on an indigo square, the last one landed. It
// writes a 1024-pixel PNG, from which the macOS icon set is made:
//
//	go run ./cmd/iflow-icon icon.png
//	mkdir iFlow.iconset && for s in 16 32 128 256 512; do
//	  sips -z $s $s icon.png --out iFlow.iconset/icon_${s}x${s}.png
//	  sips -z $((s*2)) $((s*2)) icon.png --out iFlow.iconset/icon_${s}x${s}@2x.png
//	done && iconutil -c icns iFlow.iconset
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 1024

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: iflow-icon <out.png>")
		os.Exit(2)
	}
	f, err := os.Create(os.Args[1])
	if err == nil {
		err = png.Encode(f, draw())
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type point struct{ x, y float64 }

// The stages, left to right and rising, and how wide what joins them is.
var (
	stages = []point{{318, 650}, {512, 512}, {706, 374}}
	radius = 66.0
	line   = 30.0
)

// draw paints the icon pixel by pixel from the distance to each shape, so
// that every edge is smooth without drawing it larger first.
func draw() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			p := point{float64(x) + 0.5, float64(y) + 0.5}
			// The square, on the grid macOS icons share: 824 wide, centred.
			bg := cover(roundedRect(p, 100, 100, 924, 924, 186))
			if bg == 0 {
				continue
			}
			t := (p.x + p.y - 200) / (2 * 824)
			c := mix(rgb(0x4f46e5), rgb(0x0ea5e9), clamp(t))
			// A light from above, as the system's own icons have.
			c = mix(c, rgb(0xffffff), 0.10*clamp(1-(p.y-100)/400))
			glyph := 0.0
			for i := 0; i+1 < len(stages); i++ {
				// What joins the stages stops at the landed ring's edge.
				joint := math.Max(segment(p, stages[i], stages[i+1])-line/2, radius-dist(p, stages[len(stages)-1]))
				glyph = math.Max(glyph, 0.85*cover(joint))
			}
			for i, s := range stages {
				d := dist(p, s) - radius
				if i == len(stages)-1 {
					// The last stage is landed: a ring with its centre open.
					glyph = math.Max(glyph, cover(math.Abs(d+14)-14))
					glyph = math.Max(glyph, cover(dist(p, s)-28))
					continue
				}
				glyph = math.Max(glyph, cover(d))
			}
			c = mix(c, rgb(0xffffff), glyph)
			img.SetNRGBA(x, y, color.NRGBA{c[0], c[1], c[2], uint8(255 * bg)})
		}
	}
	return img
}

// cover is how much of a pixel a shape at signed distance d covers.
func cover(d float64) float64 { return clamp(0.5 - d) }

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func dist(a, b point) float64 { return math.Hypot(a.x-b.x, a.y-b.y) }

// segment is the distance from p to the segment ab.
func segment(p, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	t := clamp(((p.x-a.x)*dx + (p.y-a.y)*dy) / (dx*dx + dy*dy))
	return dist(p, point{a.x + t*dx, a.y + t*dy})
}

// roundedRect is the signed distance from p to a rectangle with round corners.
func roundedRect(p point, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hx, hy := (x1-x0)/2-r, (y1-y0)/2-r
	qx, qy := math.Abs(p.x-cx)-hx, math.Abs(p.y-cy)-hy
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return out + math.Min(math.Max(qx, qy), 0) - r
}

type rgbc [3]uint8

func rgb(v uint32) rgbc { return rgbc{uint8(v >> 16), uint8(v >> 8), uint8(v)} }

func mix(a, b rgbc, t float64) rgbc {
	var c rgbc
	for i := range c {
		c[i] = uint8(math.Round(float64(a[i])*(1-t) + float64(b[i])*t))
	}
	return c
}
