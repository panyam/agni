package odb

import (
	"fmt"
	"math"
	"strings"
)

// pt is a point in integer nanometres, Y-up.
type pt struct{ x, y int64 }

// featKind is a feature record's letter.
type featKind byte

const (
	featLine    featKind = 'L'
	featPad     featKind = 'P'
	featArc     featKind = 'A'
	featText    featKind = 'T'
	featBarcode featKind = 'B'
	featSurface featKind = 'S'
)

// feature is one record of a features file. Its index in the file is the id eda/data's FID
// records name it by, so every kind is kept, the ones nothing draws included.
type feature struct {
	kind     featKind
	a, b     pt // a line's or arc's ends; a pad's or text's position is a
	center   pt // an arc's centre
	cw       bool
	sym      int     // the symbol table index, -1 for none
	positive bool    // polarity P; a negative feature clears copper rather than adding it
	rot      float64 // clockwise degrees, as the file writes it
	mirror   bool
	text     string
	textH    int64       // a text record's glyph height
	islands  [][]contour // a surface's islands, each its outline then its holes
}

// contour is one closed path of a surface, its arcs already flattened.
type contour struct {
	hole bool
	pts  []pt
}

// symbolRef is one entry of a features file's symbol table: the symbol's name and the nanometres
// one of its dimensions is, which a `$n name M` entry can set apart from the file's own units.
type symbolRef struct {
	name string
	nm   float64
}

// featuresFile is a parsed features file: the symbols its records name and the records.
type featuresFile struct {
	symbols  []symbolRef
	features []feature
}

// arcSegments is how many straight pieces an arc becomes, the count the IPC-2581 and KiCad outline
// readers use, since the geom contract carries no arc primitive for copper (C6).
const arcSegments = 16

// parseFeatures reads a features file. A record this reader does not draw (a barcode) still takes
// its index, so the ids eda/data uses keep pointing at the right feature.
func parseFeatures(b []byte) (*featuresFile, error) {
	u := unitsOf(b)
	ff := &featuresFile{}
	var surf *feature
	var cont *contour
	var lastPt pt
	err := lines(b, func(n int, line string) error {
		w, _ := fields(line)
		if len(w) == 0 {
			return nil
		}
		head := w[0]
		if surf != nil {
			switch head {
			case "OB":
				if len(w) < 4 {
					return fmt.Errorf("line %d: OB needs x y and I|H", n)
				}
				p, err := coordPair(w[1], w[2], u)
				if err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
				hole := strings.EqualFold(w[3], "H")
				if !hole || len(surf.islands) == 0 {
					surf.islands = append(surf.islands, nil)
				}
				isl := &surf.islands[len(surf.islands)-1]
				*isl = append(*isl, contour{hole: hole, pts: []pt{p}})
				cont = &(*isl)[len(*isl)-1]
				lastPt = p
			case "OS":
				if cont == nil || len(w) < 3 {
					return fmt.Errorf("line %d: OS outside a contour", n)
				}
				p, err := coordPair(w[1], w[2], u)
				if err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
				cont.pts = append(cont.pts, p)
				lastPt = p
			case "OC":
				if cont == nil || len(w) < 6 {
					return fmt.Errorf("line %d: OC outside a contour", n)
				}
				e, err := coordPair(w[1], w[2], u)
				if err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
				c, err := coordPair(w[3], w[4], u)
				if err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
				arc := flattenArc(lastPt, e, c, strings.EqualFold(w[5], "Y"))
				cont.pts = append(cont.pts, arc[1:]...)
				lastPt = e
			case "OE":
				cont = nil
			case "SE":
				ff.features = append(ff.features, *surf)
				surf, cont = nil, nil
			default:
				return fmt.Errorf("line %d: %q inside a surface", n, head)
			}
			return nil
		}
		switch {
		case strings.HasPrefix(head, "$"):
			if len(w) >= 2 {
				su := u.symbol
				if len(w) >= 3 {
					switch strings.ToUpper(w[2]) {
					case "M":
						su = unitsMM.symbol
					case "I":
						su = unitsInch.symbol
					}
				}
				ff.symbols = append(ff.symbols, symbolRef{name: w[1], nm: su})
			}
			return nil
		case strings.HasPrefix(head, "@"), strings.HasPrefix(head, "&"), strings.Contains(head, "="):
			return nil
		case head == "F", head == "U":
			return nil
		}
		f, err := parseFeature(w, u)
		if err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
		if f.kind == featSurface {
			surf = &f
			return nil
		}
		ff.features = append(ff.features, f)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("odb: features: %w", err)
	}
	if surf != nil {
		return nil, fmt.Errorf("odb: features: a surface never ends (no SE)")
	}
	return ff, nil
}

// parseFeature reads one L, P, A, T, B or S record.
func parseFeature(w []string, u units) (feature, error) {
	f := feature{kind: featKind(w[0][0]), sym: -1}
	need := func(k int) error {
		if len(w) < k {
			return fmt.Errorf("%s record has %d fields, needs %d", w[0], len(w), k)
		}
		return nil
	}
	var err error
	switch w[0] {
	case "L":
		if err = need(7); err != nil {
			return f, err
		}
		if f.a, err = coordPair(w[1], w[2], u); err != nil {
			return f, err
		}
		if f.b, err = coordPair(w[3], w[4], u); err != nil {
			return f, err
		}
		if f.sym, err = atoi(w[5]); err != nil {
			return f, err
		}
		f.positive = !strings.EqualFold(w[6], "N")
	case "P":
		if err = need(5); err != nil {
			return f, err
		}
		if f.a, err = coordPair(w[1], w[2], u); err != nil {
			return f, err
		}
		i := 3
		if w[i] == "-1" { // a resized user symbol: -1 <sym> <resize factor>
			i++
			if err = need(i + 3); err != nil {
				return f, err
			}
			if f.sym, err = atoi(w[i]); err != nil {
				return f, err
			}
			i += 2
		} else {
			if f.sym, err = atoi(w[i]); err != nil {
				return f, err
			}
			i++
		}
		f.positive = !strings.EqualFold(w[i], "N")
		i += 2 // polarity, dcode
		f.rot, f.mirror = orient(w[i:])
	case "A":
		if err = need(11); err != nil {
			return f, err
		}
		if f.a, err = coordPair(w[1], w[2], u); err != nil {
			return f, err
		}
		if f.b, err = coordPair(w[3], w[4], u); err != nil {
			return f, err
		}
		if f.center, err = coordPair(w[5], w[6], u); err != nil {
			return f, err
		}
		if f.sym, err = atoi(w[7]); err != nil {
			return f, err
		}
		f.positive = !strings.EqualFold(w[8], "N")
		f.cw = strings.EqualFold(w[10], "Y")
	case "T":
		// T <x> <y> <font> <polarity> <orient_def> <xsize> <ysize> <width_factor> <text> [<version>]
		if err = need(9); err != nil {
			return f, err
		}
		if f.a, err = coordPair(w[1], w[2], u); err != nil {
			return f, err
		}
		f.positive = !strings.EqualFold(w[4], "N")
		i := 5
		f.rot, f.mirror = orient(w[i:])
		if w[i] == "8" || w[i] == "9" {
			i += 2
		} else {
			i++
		}
		if len(w) < i+4 {
			return f, fmt.Errorf("T record ends before its text")
		}
		if h, herr := atof(w[i+1]); herr == nil {
			f.textH = int64(math.Round(h * u.coord))
		}
		f.text = w[i+3]
	case "B":
		// A barcode keeps its index and nothing else.
	case "S":
		if err = need(2); err != nil {
			return f, err
		}
		f.positive = !strings.EqualFold(w[1], "N")
	default:
		return f, fmt.Errorf("unknown record %q", w[0])
	}
	return f, nil
}

// orient reads an orient_def: 8 or 9 followed by a clockwise angle (9 mirrored), or one of the
// legacy codes 0..7 (quarter turns, 4..7 mirrored).
func orient(w []string) (rot float64, mirror bool) {
	if len(w) == 0 {
		return 0, false
	}
	switch w[0] {
	case "8", "9":
		if len(w) > 1 {
			rot, _ = atof(w[1])
		}
		return rot, w[0] == "9"
	}
	code, err := atoi(w[0])
	if err != nil || code < 0 || code > 7 {
		return 0, false
	}
	return float64(code%4) * 90, code >= 4
}

func coordPair(xs, ys string, u units) (pt, error) {
	x, err := atof(xs)
	if err != nil {
		return pt{}, fmt.Errorf("coordinate %q: %w", xs, err)
	}
	y, err := atof(ys)
	if err != nil {
		return pt{}, fmt.Errorf("coordinate %q: %w", ys, err)
	}
	return pt{x: int64(math.Round(x * u.coord)), y: int64(math.Round(y * u.coord))}, nil
}

// flattenArc turns an arc from s to e about c into arcSegments+1 points, s first and e last. Equal
// ends are a full circle.
func flattenArc(s, e, c pt, cw bool) []pt {
	r := math.Hypot(float64(s.x-c.x), float64(s.y-c.y))
	a0 := math.Atan2(float64(s.y-c.y), float64(s.x-c.x))
	a1 := math.Atan2(float64(e.y-c.y), float64(e.x-c.x))
	sweep := a1 - a0
	if cw {
		for sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	} else {
		for sweep <= 0 {
			sweep += 2 * math.Pi
		}
	}
	out := make([]pt, 0, arcSegments+1)
	out = append(out, s)
	for i := 1; i < arcSegments; i++ {
		a := a0 + sweep*float64(i)/arcSegments
		out = append(out, pt{x: c.x + int64(math.Round(r*math.Cos(a))), y: c.y + int64(math.Round(r*math.Sin(a)))})
	}
	return append(out, e)
}
