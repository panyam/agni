// Package copperraster draws one layer of copper into a bitmap, from a board's geometry or from a
// Gerber film, so a reader's copper can be scored against the artwork the board's own tool plotted.
// It is test tooling for the board readers (agni issue 1376) and no shipped command uses it.
//
// A pixel is set when its centre lies inside a primitive. Filled regions (Gerber G36/G37) and zone
// pours are not drawn on either side, since the board geometry keeps a zone's outline and not its
// fill (agni issue 1357). What remains, tracks, pads and vias, is most of a board's copper.
package copperraster

import (
	"image"
	"image/color"
	"math"
)

// Bitmap is one layer's copper on a grid of square pixels, Res nanometres a side, whose pixel (0, 0)
// has its lower-left corner at (X0, Y0) in board nanometres, Y up.
type Bitmap struct {
	W, H   int
	X0, Y0 int64
	Res    int64
	bits   []uint64
}

// New makes an empty bitmap covering the box from (minX, minY) to (maxX, maxY) in nanometres.
func New(minX, minY, maxX, maxY, res int64) *Bitmap {
	w := int((maxX-minX)/res) + 1
	h := int((maxY-minY)/res) + 1
	return &Bitmap{W: w, H: h, X0: minX, Y0: minY, Res: res, bits: make([]uint64, (w*h+63)/64)}
}

func (b *Bitmap) set(x, y int) {
	i := y*b.W + x
	b.bits[i/64] |= 1 << (uint(i) % 64)
}

// Get reports whether pixel (x, y) is copper.
func (b *Bitmap) Get(x, y int) bool {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return false
	}
	i := y*b.W + x
	return b.bits[i/64]&(1<<(uint(i)%64)) != 0
}

// Count is the number of copper pixels.
func (b *Bitmap) Count() int {
	n := 0
	for _, w := range b.bits {
		for ; w != 0; w &= w - 1 {
			n++
		}
	}
	return n
}

// fill sets every pixel in the box (x0, y0)-(x1, y1), in nanometres, whose centre satisfies inside.
func (b *Bitmap) fill(x0, y0, x1, y1 float64, inside func(x, y float64) bool) {
	r := float64(b.Res)
	px0 := max(0, int(math.Floor((x0-float64(b.X0))/r)))
	py0 := max(0, int(math.Floor((y0-float64(b.Y0))/r)))
	px1 := min(b.W-1, int(math.Ceil((x1-float64(b.X0))/r)))
	py1 := min(b.H-1, int(math.Ceil((y1-float64(b.Y0))/r)))
	for py := py0; py <= py1; py++ {
		cy := float64(b.Y0) + (float64(py)+0.5)*r
		for px := px0; px <= px1; px++ {
			cx := float64(b.X0) + (float64(px)+0.5)*r
			if inside(cx, cy) {
				b.set(px, py)
			}
		}
	}
}

// Line draws a stroke of the given width with round ends, from (ax, ay) to (bx, by).
func (b *Bitmap) Line(ax, ay, bx, by, width float64) {
	h := width / 2
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	b.fill(min(ax, bx)-h, min(ay, by)-h, max(ax, bx)+h, max(ay, by)+h, func(x, y float64) bool {
		t := 0.0
		if l2 > 0 {
			t = math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/l2))
		}
		ex, ey := x-(ax+t*dx), y-(ay+t*dy)
		return ex*ex+ey*ey <= h*h
	})
}

// Disc draws a filled circle.
func (b *Bitmap) Disc(cx, cy, d float64) { b.Line(cx, cy, cx, cy, d) }

// Rect draws a w by h rectangle centred on (cx, cy), turned deg degrees counter-clockwise.
func (b *Bitmap) Rect(cx, cy, w, h, deg float64) {
	s, c := math.Sincos(deg * math.Pi / 180)
	r := math.Hypot(w, h) / 2
	b.fill(cx-r, cy-r, cx+r, cy+r, func(x, y float64) bool {
		u := (x-cx)*c + (y-cy)*s
		v := -(x-cx)*s + (y-cy)*c
		return math.Abs(u) <= w/2 && math.Abs(v) <= h/2
	})
}

// Oval draws a w by h stadium centred on (cx, cy), turned deg degrees counter-clockwise: a stroke
// along the longer side whose width is the shorter.
func (b *Bitmap) Oval(cx, cy, w, h, deg float64) {
	long, short, along := w, h, deg
	if h > w {
		long, short, along = h, w, deg+90
	}
	s, c := math.Sincos(along * math.Pi / 180)
	half := (long - short) / 2
	b.Line(cx-half*c, cy-half*s, cx+half*c, cy+half*s, short)
}

// Polygon fills a closed outline, its vertices given relative to (cx, cy), by the even-odd rule.
func (b *Bitmap) Polygon(cx, cy float64, pts [][2]float64) {
	if len(pts) < 3 {
		return
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		x0, y0, x1, y1 = math.Min(x0, p[0]), math.Min(y0, p[1]), math.Max(x1, p[0]), math.Max(y1, p[1])
	}
	b.fill(cx+x0, cy+y0, cx+x1, cy+y1, func(x, y float64) bool {
		x, y = x-cx, y-cy
		in := false
		for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
			pi, pj := pts[i], pts[j]
			if (pi[1] > y) != (pj[1] > y) && x < (pj[0]-pi[0])*(y-pi[1])/(pj[1]-pi[1])+pi[0] {
				in = !in
			}
		}
		return in
	})
}

// Overlap is the intersection over union of two bitmaps on the same grid: 1 when they agree pixel
// for pixel, 0 when they share no copper.
func Overlap(a, b *Bitmap) float64 {
	inter, union := 0, 0
	for i := range a.bits {
		x, y := a.bits[i], b.bits[i]
		inter += popcount(x & y)
		union += popcount(x | y)
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}

// Covered is the share of want's copper that got also has: 1 when got covers all of it.
func Covered(want, got *Bitmap) float64 {
	inter, total := 0, 0
	for i := range want.bits {
		inter += popcount(want.bits[i] & got.bits[i])
		total += popcount(want.bits[i])
	}
	if total == 0 {
		return 1
	}
	return float64(inter) / float64(total)
}

// Union is a new bitmap holding the copper of both, which must share a grid.
func Union(a, b *Bitmap) *Bitmap {
	u := &Bitmap{W: a.W, H: a.H, X0: a.X0, Y0: a.Y0, Res: a.Res, bits: make([]uint64, len(a.bits))}
	for i := range a.bits {
		u.bits[i] = a.bits[i] | b.bits[i]
	}
	return u
}

func popcount(w uint64) int {
	n := 0
	for ; w != 0; w &= w - 1 {
		n++
	}
	return n
}

// Diff draws two bitmaps on the same grid as one image: copper in both grey, only in want red, only
// in got blue. Y is flipped so the image reads as the board does.
func Diff(want, got *Bitmap) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, want.W, want.H))
	for y := 0; y < want.H; y++ {
		for x := 0; x < want.W; x++ {
			c := color.RGBA{255, 255, 255, 255}
			w, g := want.Get(x, y), got.Get(x, y)
			switch {
			case w && g:
				c = color.RGBA{150, 150, 150, 255}
			case w:
				c = color.RGBA{220, 30, 30, 255}
			case g:
				c = color.RGBA{30, 60, 220, 255}
			}
			img.Set(x, want.H-1-y, c)
		}
	}
	return img
}
