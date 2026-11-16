package kicad

import (
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/step"
)

// ModelOpener returns the bytes of a 3D model a footprint names, given the path exactly as the
// footprint writes it, such as `${KICAD6_3DMODEL_DIR}/Resistor_SMD.3dshapes/R_0402_1005Metric.step`.
// The caller resolves it, so the reader opens no file (C1). SplitModelPath says where each kind of
// path is anchored. An error says why the model could not be had, and the reader records it.
type ModelOpener func(path string) ([]byte, error)

// ReadBoardGeometryWithModels is ReadBoardGeometry plus each part's height, taken from the STEP
// models its footprint names (agni 1652). A part whose models all fail to open or read keeps no
// height, and BoardGeometry.unread_models says which models and why. A nil open reads no model and
// records none, exactly as ReadBoardGeometry does.
func ReadBoardGeometryWithModels(r io.Reader, sourceFile string, open ModelOpener) (*geom.BoardGeometry, error) {
	root, err := parseBoard(r)
	if err != nil {
		return nil, err
	}
	g, err := boardFromRoot(root, sourceFile)
	if err != nil || open == nil {
		return g, err
	}
	modelHeights(root, g, open)
	return g, nil
}

// ModelAnchor says where a footprint's model path starts.
type ModelAnchor int

const (
	// AnchorBoard is the board's own folder: `${KIPRJMOD}/x.step`, or a plain relative path.
	AnchorBoard ModelAnchor = iota
	// AnchorLibrary is KiCad's 3D model library: `${KICAD9_3DMODEL_DIR}/x.step`, for any version.
	AnchorLibrary
	// AnchorAbsolute is a path rooted on the machine that wrote the board.
	AnchorAbsolute
	// AnchorUnknown is a path that starts with any other variable, which only that user's KiCad
	// configuration defines.
	AnchorUnknown
)

var modelVar = regexp.MustCompile(`^(?:\$\{([A-Za-z0-9_]+)\}|\$\(([A-Za-z0-9_]+)\))[/\\]?`)
var libraryVar = regexp.MustCompile(`^KICAD[0-9]*_3DMODEL_DIR$|^KISYS3DMOD$`)

// SplitModelPath splits a footprint's model path into its anchor and the slash-separated path
// below it. KiCad writes a variable as `${NAME}` or `$(NAME)`, and a board saved on Windows uses
// backslashes. KISYS3DMOD is KiCad 5's name for the library.
func SplitModelPath(raw string) (ModelAnchor, string) {
	p := strings.ReplaceAll(raw, `\`, "/")
	if m := modelVar.FindStringSubmatch(p); m != nil {
		name := m[1] + m[2]
		rest := p[len(m[0]):]
		switch {
		case name == "KIPRJMOD":
			return AnchorBoard, rest
		case libraryVar.MatchString(name):
			return AnchorLibrary, rest
		default:
			return AnchorUnknown, p
		}
	}
	if strings.HasPrefix(p, "/") || len(p) > 2 && p[1] == ':' && p[2] == '/' {
		return AnchorAbsolute, p
	}
	return AnchorBoard, p
}

// modelHeights sets each placement's height from its footprint's models, and records the models it
// could not use. A file the board names many times is opened and read once.
func modelHeights(root *node, g *geom.BoardGeometry, open ModelOpener) {
	type read struct {
		pts    [][3]float64
		reason string
	}
	cache := map[string]read{}
	load := func(path string) read {
		if r, ok := cache[path]; ok {
			return r
		}
		var r read
		data, err := open(path)
		if err == nil {
			r.pts, err = step.Points(data)
		}
		if err != nil {
			r.reason = err.Error()
		}
		cache[path] = r
		return r
	}
	byRef := map[string]*geom.ComponentPlacement{}
	for _, p := range g.GetPlacements() {
		byRef[p.GetRefDes()] = p
	}
	unread := map[[2]string]int32{}
	for _, fp := range root.Children("footprint") {
		p := byRef[partRef(fp)]
		if p == nil {
			continue
		}
		top, any := 0.0, false
		for _, m := range fp.Children("model") {
			if modelHidden(m) {
				continue
			}
			path := atomOf(m.Arg(1))
			r := load(path)
			if r.reason != "" {
				unread[[2]string{path, r.reason}]++
				continue
			}
			t := modelTransformOf(m)
			for _, q := range r.pts {
				if z := t.z(q); !any || z > top {
					top, any = z, true
				}
			}
		}
		if any && top > 0 {
			h := int64(math.Round(top * 1e6 / float64(g.GetUnitNm())))
			p.Height = &h
		}
	}
	for k, n := range unread {
		g.UnreadModels = append(g.UnreadModels, &geom.UnreadModel{Path: k[0], Reason: k[1], Parts: n})
	}
	sort.Slice(g.UnreadModels, func(i, j int) bool {
		a, b := g.UnreadModels[i], g.UnreadModels[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Reason < b.Reason
	})
}

// modelHidden reports a model KiCad does not draw: `(hide yes)` from KiCad 8 on, or a bare `hide`
// atom before that.
func modelHidden(m *node) bool {
	if h := m.Child("hide"); h != nil {
		return atomOf(h.Arg(1)) != "no"
	}
	for i := 2; m.Arg(i) != nil; i++ {
		if a := m.Arg(i); !a.IsList && a.Atom == "hide" {
			return true
		}
	}
	return false
}

// modelTransform is how a footprint places its model before the footprint itself is placed: scale,
// then turn about X, then Y, then Z, each by the negated angle, then offset, as KiCad's 3D viewer
// and its GLB export do. Its z is in millimetres above the pads' seating plane.
type modelTransform struct {
	scale, rotDeg [3]float64
	offsetMm      [3]float64
}

// modelTransformOf reads a model's (offset (xyz)) in millimetres, or KiCad 5's (at (xyz)) in
// inches, with (scale (xyz)) and (rotate (xyz)) in degrees.
func modelTransformOf(m *node) modelTransform {
	t := modelTransform{scale: [3]float64{1, 1, 1}}
	if v, ok := xyzOf(m.Child("offset")); ok {
		t.offsetMm = v
	} else if v, ok := xyzOf(m.Child("at")); ok {
		t.offsetMm = [3]float64{v[0] * 25.4, v[1] * 25.4, v[2] * 25.4}
	}
	if v, ok := xyzOf(m.Child("scale")); ok {
		t.scale = v
	}
	if v, ok := xyzOf(m.Child("rotate")); ok {
		t.rotDeg = v
	}
	return t
}

// xyzOf reads a child's (xyz x y z).
func xyzOf(n *node) ([3]float64, bool) {
	var v [3]float64
	x := n.Child("xyz")
	if x == nil {
		return v, false
	}
	for i := range v {
		f, err := strconv.ParseFloat(atomOf(x.Arg(i+1)), 64)
		if err != nil {
			return v, false
		}
		v[i] = f
	}
	return v, true
}

// z places a model point and returns its height above the seating plane. A turn about Z comes last
// and moves no point up or down, so z leaves it out.
func (t modelTransform) z(p [3]float64) float64 {
	x, y, z := p[0]*t.scale[0], p[1]*t.scale[1], p[2]*t.scale[2]
	rx, ry := -t.rotDeg[0]*math.Pi/180, -t.rotDeg[1]*math.Pi/180
	z = y*math.Sin(rx) + z*math.Cos(rx)
	z = -x*math.Sin(ry) + z*math.Cos(ry)
	return z + t.offsetMm[2]
}
