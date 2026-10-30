package odb

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// symbolShape is what a pad or line symbol says about the copper it draws: its bounding size in
// nanometres and the pad-shape word the geom contract uses (the KiCad words, so the renderer and
// the board rules read every producer alike).
type symbolShape struct {
	w, h  int64
	shape string
}

var symbolPattern = regexp.MustCompile(`^([a-z_]+?)([0-9.]+)(?:x([0-9.]+))?(?:x([0-9.]+))?(?:x(r|c)([0-9.]+))?`)

// parseSymbol reads one of the spec's standard symbol names (`r250`, `rect600x700xr150`,
// `oval1000x500`, ...). A user-defined symbol, which a job defines under symbols/, has no size
// here and keeps its name as the shape word.
func parseSymbol(ref symbolRef) symbolShape {
	name := strings.ToLower(ref.name)
	m := symbolPattern.FindStringSubmatch(name)
	if m == nil {
		return symbolShape{shape: ref.name}
	}
	dim := func(s string) int64 {
		if s == "" {
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(math.Round(v * ref.nm))
	}
	a, b := dim(m[2]), dim(m[3])
	if b == 0 {
		b = a
	}
	switch m[1] {
	case "r":
		return symbolShape{w: a, h: a, shape: "circle"}
	case "s":
		return symbolShape{w: a, h: a, shape: "rect"}
	case "rect":
		switch {
		case m[5] == "r" || strings.Contains(name, "xr"):
			return symbolShape{w: a, h: b, shape: "roundrect"}
		case m[5] == "c" || strings.Contains(name, "xc"):
			return symbolShape{w: a, h: b, shape: "chamfered_rect"}
		}
		return symbolShape{w: a, h: b, shape: "rect"}
	case "oval", "oval_h", "el":
		return symbolShape{w: a, h: b, shape: "oval"}
	case "di":
		return symbolShape{w: a, h: b, shape: "diamond"}
	case "oct", "hex_l", "hex_s", "tri":
		return symbolShape{w: a, h: b, shape: "custom"}
	case "donut_r", "donut_s", "donut_rc", "donut_o", "thr", "ths", "s_ths", "s_tho", "sr_ths", "rc_ths", "rc_tho":
		return symbolShape{w: a, h: a, shape: "circle"}
	}
	return symbolShape{shape: ref.name}
}

// hole is a drill symbol's diameter, a slot's narrow width, the bit that cuts it.
func (s symbolShape) hole() int64 {
	if s.w > 0 && s.h > 0 && s.h < s.w {
		return s.h
	}
	return s.w
}

// width is a line symbol's stroke width: a round or square symbol's size.
func (s symbolShape) width() int64 {
	if s.w > s.h {
		return s.w
	}
	return s.h
}
