// Package step reads where a STEP model's shape reaches, without tessellating it (agni 1652).
//
// Say a footprint names a capacitor's STEP model, and the stack view wants the part's height. STEP
// (ISO 10303-21) stores exact surfaces, and drawing them takes a CAD kernel. A height needs only the
// model's extent, and the file states that through three kinds of point.
//
//   - Each VERTEX_POINT is a corner where edges meet. A box-shaped part reaches exactly its vertices.
//   - Each B-spline's control points bound the curve or surface they shape.
//   - Each CIRCLE reaches its centre plus its radius along each axis, so a cylinder lying on its side
//     reaches above the vertices at its seam.
//
// Other CARTESIAN_POINTs are not on the part. A LINE's origin and an AXIS2_PLACEMENT_3D's location can
// sit anywhere on an unbounded line or plane. Counting them made KiCad's 2x22 pin header 0.067 mm too
// tall.
//
// Fidelity: lossy-bounded. Points returns the extent points above in millimetres, in the model's own
// frame. It drops every surface, colour and name. A circle gives its six axis extremes whether or not
// its edge spans them. That is exact for an axis-aligned full circle, and over-reaches by up to the
// radius for a partial arc. An assembly whose sub-parts are moved by a transform other than the
// identity is refused (ErrAssembly), since its points would sit in their sub-parts' frames.
package step

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
)

// ErrAssembly reports a model built from sub-parts placed by a transform other than the identity.
// Points refuses one rather than return points in the wrong frame.
var ErrAssembly = errors.New("step: an assembly with placed sub-parts")

// ErrNoShape reports a file with no vertex, B-spline or circle to take an extent from.
var ErrNoShape = errors.New("step: no shape points")

// Points returns the points that bound a STEP model's shape, in millimetres in the model's frame. The
// package comment says which points count. A file that is not STEP, has no DATA section, uses a length
// unit other than a metre multiple or an inch, or is an assembly with placed sub-parts returns an
// error.
func Points(data []byte) ([][3]float64, error) {
	ents, err := parse(data)
	if err != nil {
		return nil, err
	}
	mm, err := ents.lengthUnitMm()
	if err != nil {
		return nil, err
	}
	if err := ents.checkTransforms(); err != nil {
		return nil, err
	}
	var out [][3]float64
	seen := map[int]bool{}
	add := func(id int) {
		if seen[id] {
			return
		}
		if p, ok := ents.point(id); ok {
			seen[id] = true
			out = append(out, [3]float64{p[0] * mm, p[1] * mm, p[2] * mm})
		}
	}
	for _, id := range ents.ids() {
		e := ents[id]
		switch {
		case e.typ == "VERTEX_POINT":
			if ref, ok := refAt(e.args, 1); ok {
				add(ref)
			}
		case isBSpline(e):
			for _, ref := range refsIn(e.args) {
				if ents[ref].typ == "CARTESIAN_POINT" {
					add(ref)
				}
			}
		case e.typ == "CIRCLE":
			for _, p := range ents.circleExtremes(e) {
				out = append(out, [3]float64{p[0] * mm, p[1] * mm, p[2] * mm})
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrNoShape
	}
	return out, nil
}

// entity is one instance in the DATA section. typ is the entity's keyword, or "" for a complex
// instance written as a list of partial entities, such as a unit. args is the text inside the
// outermost parentheses.
type entity struct {
	typ  string
	args []byte
}

type entities map[int]entity

// ids returns the instance names in ascending order, so Points returns its points in file order.
func (ents entities) ids() []int {
	out := make([]int, 0, len(ents))
	for id := range ents {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func isBSpline(e entity) bool {
	if e.typ == "" {
		return bytes.Contains(e.args, []byte("B_SPLINE_CURVE")) || bytes.Contains(e.args, []byte("B_SPLINE_SURFACE"))
	}
	return bytes.HasPrefix([]byte(e.typ), []byte("B_SPLINE_")) || bytes.HasPrefix([]byte(e.typ), []byte("RATIONAL_B_SPLINE_"))
}

// parse splits the DATA section into its entity instances. It skips comments and keeps a quoted
// string's semicolons and parentheses out of the structure.
func parse(data []byte) (entities, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("ISO-10303-21")) {
		return nil, errors.New("step: not an ISO-10303-21 file")
	}
	start := bytes.Index(data, []byte("DATA;"))
	if start < 0 {
		return nil, errors.New("step: no DATA section")
	}
	ents := entities{}
	i := start + len("DATA;")
	for {
		i = skipSpace(data, i)
		if i >= len(data) || bytes.HasPrefix(data[i:], []byte("ENDSEC")) {
			return ents, nil
		}
		if data[i] != '#' {
			return nil, fmt.Errorf("step: byte %d: want an instance, got %q", i, data[i])
		}
		j := i + 1
		for j < len(data) && data[j] >= '0' && data[j] <= '9' {
			j++
		}
		id, err := strconv.Atoi(string(data[i+1 : j]))
		if err != nil {
			return nil, fmt.Errorf("step: byte %d: bad instance name", i)
		}
		j = skipSpace(data, j)
		if j >= len(data) || data[j] != '=' {
			return nil, fmt.Errorf("step: #%d: want '='", id)
		}
		j = skipSpace(data, j+1)
		k := j
		for k < len(data) && (data[k] == '_' || data[k] >= 'A' && data[k] <= 'Z' || data[k] >= 'a' && data[k] <= 'z' || data[k] >= '0' && data[k] <= '9') {
			k++
		}
		typ := string(bytes.ToUpper(data[j:k]))
		k = skipSpace(data, k)
		if k >= len(data) || data[k] != '(' {
			return nil, fmt.Errorf("step: #%d: want '('", id)
		}
		end, err := closing(data, k)
		if err != nil {
			return nil, fmt.Errorf("step: #%d: %w", id, err)
		}
		ents[id] = entity{typ: typ, args: data[k+1 : end]}
		i = skipSpace(data, end+1)
		if i >= len(data) || data[i] != ';' {
			return nil, fmt.Errorf("step: #%d: want ';'", id)
		}
		i++
	}
}

// skipSpace skips white space and /* */ comments.
func skipSpace(data []byte, i int) int {
	for i < len(data) {
		switch {
		case data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n':
			i++
		case bytes.HasPrefix(data[i:], []byte("/*")):
			e := bytes.Index(data[i+2:], []byte("*/"))
			if e < 0 {
				return len(data)
			}
			i += e + 4
		default:
			return i
		}
	}
	return i
}

// closing returns the index of the parenthesis that closes the one at open. A quoted string is
// skipped whole, and two quotes inside one stand for a quote.
func closing(data []byte, open int) (int, error) {
	depth := 0
	for i := open; i < len(data); i++ {
		switch data[i] {
		case '\'':
			for i++; i < len(data); i++ {
				if data[i] == '\'' {
					if i+1 < len(data) && data[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("unclosed parenthesis")
}

// topArgs splits a parameter list at its top-level commas.
func topArgs(args []byte) [][]byte {
	var out [][]byte
	depth, from := 0, 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '\'':
			for i++; i < len(args); i++ {
				if args[i] == '\'' {
					if i+1 < len(args) && args[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, bytes.TrimSpace(args[from:i]))
				from = i + 1
			}
		}
	}
	return append(out, bytes.TrimSpace(args[from:]))
}

// refAt reads the n-th top-level parameter as an instance reference.
func refAt(args []byte, n int) (int, bool) {
	a := topArgs(args)
	if n >= len(a) || len(a[n]) < 2 || a[n][0] != '#' {
		return 0, false
	}
	id, err := strconv.Atoi(string(a[n][1:]))
	return id, err == nil
}

// refsIn returns every instance reference in a parameter list, at any depth, outside quoted strings.
func refsIn(args []byte) []int {
	var out []int
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '\'':
			for i++; i < len(args); i++ {
				if args[i] == '\'' {
					if i+1 < len(args) && args[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		case '#':
			j := i + 1
			for j < len(args) && args[j] >= '0' && args[j] <= '9' {
				j++
			}
			if id, err := strconv.Atoi(string(args[i+1 : j])); err == nil {
				out = append(out, id)
			}
			i = j - 1
		}
	}
	return out
}

// triple reads a parenthesised list of three numbers, such as a point's coordinates.
func triple(b []byte) ([3]float64, bool) {
	var v [3]float64
	b = bytes.TrimSpace(b)
	if len(b) < 2 || b[0] != '(' || b[len(b)-1] != ')' {
		return v, false
	}
	a := topArgs(b[1 : len(b)-1])
	if len(a) != 3 {
		return v, false
	}
	for i, s := range a {
		f, err := strconv.ParseFloat(string(s), 64)
		if err != nil {
			return v, false
		}
		v[i] = f
	}
	return v, true
}

// point reads a CARTESIAN_POINT('name', (x, y, z)).
func (ents entities) point(id int) ([3]float64, bool) {
	e, ok := ents[id]
	if !ok || e.typ != "CARTESIAN_POINT" {
		return [3]float64{}, false
	}
	a := topArgs(e.args)
	if len(a) != 2 {
		return [3]float64{}, false
	}
	return triple(a[1])
}

// direction reads a DIRECTION('name', (x, y, z)) as a unit vector. A missing one, written $, is def.
func (ents entities) direction(arg []byte, def [3]float64) [3]float64 {
	if len(arg) < 2 || arg[0] != '#' {
		return def
	}
	id, err := strconv.Atoi(string(arg[1:]))
	if err != nil {
		return def
	}
	e, ok := ents[id]
	if !ok || e.typ != "DIRECTION" {
		return def
	}
	a := topArgs(e.args)
	if len(a) != 2 {
		return def
	}
	v, ok := triple(a[1])
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if !ok || n == 0 {
		return def
	}
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

// placement is an AXIS2_PLACEMENT_3D: an origin, its z axis and its x axis.
type placement struct{ origin, axis, ref [3]float64 }

func (ents entities) placement(id int) (placement, bool) {
	e, ok := ents[id]
	if !ok || e.typ != "AXIS2_PLACEMENT_3D" {
		return placement{}, false
	}
	a := topArgs(e.args)
	if len(a) != 4 {
		return placement{}, false
	}
	loc, ok := refAt(e.args, 1)
	if !ok {
		return placement{}, false
	}
	o, ok := ents.point(loc)
	if !ok {
		return placement{}, false
	}
	return placement{
		origin: o,
		axis:   ents.direction(a[2], [3]float64{0, 0, 1}),
		ref:    ents.direction(a[3], [3]float64{1, 0, 0}),
	}, true
}

// circleExtremes returns where a CIRCLE('name', #placement, radius) reaches along each axis. A circle
// of radius r about unit normal n reaches r*sqrt(1-n_k^2) either side of its centre along axis k.
func (ents entities) circleExtremes(e entity) [][3]float64 {
	a := topArgs(e.args)
	if len(a) != 3 {
		return nil
	}
	pid, ok := refAt(e.args, 1)
	if !ok {
		return nil
	}
	pl, ok := ents.placement(pid)
	if !ok {
		return nil
	}
	r, err := strconv.ParseFloat(string(a[2]), 64)
	if err != nil {
		return nil
	}
	var out [][3]float64
	for k := 0; k < 3; k++ {
		reach := r * math.Sqrt(math.Max(0, 1-pl.axis[k]*pl.axis[k]))
		lo, hi := pl.origin, pl.origin
		lo[k] -= reach
		hi[k] += reach
		out = append(out, lo, hi)
	}
	return out
}

// lengthUnitMm returns how many millimetres one of the file's length units is. A file stating no
// length unit is taken as millimetres, which is what every KiCad library model states. A file
// stating two different ones is refused.
func (ents entities) lengthUnitMm() (float64, error) {
	unit := 0.0
	for _, e := range ents {
		if e.typ != "" || !bytes.Contains(e.args, []byte("LENGTH_UNIT")) {
			continue
		}
		var mm float64
		switch {
		case bytes.Contains(e.args, []byte("CONVERSION_BASED_UNIT")):
			switch {
			case bytes.Contains(bytes.ToUpper(e.args), []byte("'INCH'")):
				mm = 25.4
			case bytes.Contains(bytes.ToUpper(e.args), []byte("'FOOT'")):
				mm = 304.8
			default:
				return 0, fmt.Errorf("step: unknown conversion-based length unit %q", e.args)
			}
		case bytes.Contains(e.args, []byte("SI_UNIT")):
			i := bytes.Index(e.args, []byte("SI_UNIT"))
			open := bytes.IndexByte(e.args[i:], '(')
			if open < 0 {
				return 0, errors.New("step: malformed SI_UNIT")
			}
			end, err := closing(e.args, i+open)
			if err != nil {
				return 0, err
			}
			a := topArgs(e.args[i+open+1 : end])
			if len(a) != 2 || string(a[1]) != ".METRE." {
				return 0, fmt.Errorf("step: length unit %q is not a metre multiple", e.args[i:end+1])
			}
			p, ok := siPrefixMm[string(a[0])]
			if !ok {
				return 0, fmt.Errorf("step: unknown SI prefix %q", a[0])
			}
			mm = p
		default:
			continue
		}
		if unit != 0 && math.Abs(unit-mm) > 1e-12 {
			return 0, fmt.Errorf("step: two length units, %g mm and %g mm", unit, mm)
		}
		unit = mm
	}
	if unit == 0 {
		return 1, nil
	}
	return unit, nil
}

var siPrefixMm = map[string]float64{
	"$": 1000, ".MILLI.": 1, ".CENTI.": 10, ".DECI.": 100, ".MICRO.": 1e-3, ".NANO.": 1e-6,
}

// checkTransforms refuses an assembly that places a sub-part anywhere but where it was drawn. Each
// ITEM_DEFINED_TRANSFORMATION('name', 'description', #from, #to) maps one placement onto another,
// and it moves nothing when the two are the same. A MAPPED_ITEM places a copy of a shape, so a file
// with one is refused as well.
func (ents entities) checkTransforms() error {
	for _, e := range ents {
		switch e.typ {
		case "MAPPED_ITEM":
			return ErrAssembly
		case "ITEM_DEFINED_TRANSFORMATION":
			from, ok1 := refAt(e.args, 2)
			to, ok2 := refAt(e.args, 3)
			if !ok1 || !ok2 {
				return ErrAssembly
			}
			a, ok1 := ents.placement(from)
			b, ok2 := ents.placement(to)
			if !ok1 || !ok2 || !samePlacement(a, b) {
				return ErrAssembly
			}
		}
	}
	return nil
}

func samePlacement(a, b placement) bool {
	const eps = 1e-9
	for k := 0; k < 3; k++ {
		if math.Abs(a.origin[k]-b.origin[k]) > eps || math.Abs(a.axis[k]-b.axis[k]) > eps || math.Abs(a.ref[k]-b.ref[k]) > eps {
			return false
		}
	}
	return true
}
