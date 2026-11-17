package kicad

import (
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/mesh"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/gltf"
	"github.com/panyam/agni/readers/step"
	"github.com/panyam/agni/readers/wrl"
)

// ModelOpener returns the bytes of a 3D model a footprint names, given the path exactly as the
// footprint writes it, such as `${KICAD6_3DMODEL_DIR}/Resistor_SMD.3dshapes/R_0402_1005Metric.step`.
// The caller resolves it, so the reader opens no file (C1). SplitModelPath says where each kind of
// path is anchored. The reader also asks for files beside the named one: a stand-in in another
// format, and a glTF's buffers. An error for a file that is not there wraps ErrModelNotFound, so the
// reader can try the next stand-in. Any other error says why the model could not be had, and the
// reader records it.
type ModelOpener func(path string) ([]byte, error)

// ErrModelNotFound is what a ModelOpener's error wraps when the file is not there.
var ErrModelNotFound = errors.New("not found")

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
	// AnchorVar is a path under any other variable, such as Antmicro's `${ANT3DMDL}` (agni 1687).
	// Only the configuration of the person who wrote the board defines it, so a reader maps it to
	// a folder through formats.Loader.ModelVars.
	AnchorVar
)

// ModelRef is a footprint's model path split at its anchor. Var names the variable for AnchorVar,
// and Rel is the slash-separated path below the anchor. For AnchorAbsolute, Rel is the whole path.
type ModelRef struct {
	Anchor ModelAnchor
	Var    string
	Rel    string
}

var modelVar = regexp.MustCompile(`^/?(?:\$\{([A-Za-z0-9_]+)\}|\$\(([A-Za-z0-9_]+)\))[/\\]?`)
var libraryVar = regexp.MustCompile(`^KICAD[0-9]*_3DMODEL_DIR$|^KISYS3DMOD$`)

// SplitModelPath splits a footprint's model path at its anchor. KiCad writes a variable as `${NAME}`
// or `$(NAME)`, and a board saved on Windows uses backslashes. KISYS3DMOD is KiCad 5's name for the
// library. A slash before a variable, as in Jetson's `/${ANT3DMDL}/x.step`, is ignored, since KiCad
// expands the variable to an absolute folder anyway.
func SplitModelPath(raw string) ModelRef {
	p := strings.ReplaceAll(raw, `\`, "/")
	if m := modelVar.FindStringSubmatch(p); m != nil {
		name := m[1] + m[2]
		rest := p[len(m[0]):]
		switch {
		case name == "KIPRJMOD":
			return ModelRef{Anchor: AnchorBoard, Rel: rest}
		case libraryVar.MatchString(name):
			return ModelRef{Anchor: AnchorLibrary, Rel: rest}
		default:
			return ModelRef{Anchor: AnchorVar, Var: name, Rel: rest}
		}
	}
	if strings.HasPrefix(p, "/") || len(p) > 2 && p[1] == ':' && p[2] == '/' {
		return ModelRef{Anchor: AnchorAbsolute, Rel: p}
	}
	return ModelRef{Anchor: AnchorBoard, Rel: p}
}

// modelCandidates lists the files a footprint's model path may stand for, in the order tried for a
// height. KiCad's library ships a .wrl beside each .step. A .wrl name tries the .step first, since
// kicad-cli's 3D export measures the STEP, and then the .wrl itself (agni 1652). Antmicro ships its
// library as glTF, so a .step name also tries a .gltf and a .glb of the same name (agni 1687).
func modelCandidates(raw string) []string {
	ext := path.Ext(raw)
	stem := strings.TrimSuffix(raw, ext)
	switch strings.ToLower(ext) {
	case ".wrl", ".wrz":
		return []string{stem + ".step", raw}
	case ".step", ".stp":
		return []string{raw, stem + ".gltf", stem + ".glb"}
	}
	return []string{raw}
}

// readModel reads one model file's extent points, in millimetres with Z up. named is the extension
// the footprint wrote. A STEP's points are in the model's own frame, and the footprint's model
// transform places them (footprintFrame false).
//
// A glTF's points are already in the footprint's frame (footprintFrame true), and no model
// transform applies. Antmicro authors its glTF library that way. On Jetson, 936 of its 938 glTF
// parts sit inside their courtyards untransformed, and 833 with the footprint's transform applied
// (agni 1687). A glTF that stands in for a named STEP is read in millimetres, as Antmicro's library
// writes it. A glTF the footprint names itself is read in metres, as the glTF specification says.
// glTF's Y is up, so its (x, y, z) is the footprint frame's (x, -z, y).
func readModel(open ModelOpener, file, named string) (pts [][3]float64, footprintFrame bool, err error) {
	data, err := open(file)
	if err != nil {
		return nil, false, err
	}
	switch strings.ToLower(path.Ext(file)) {
	case ".gltf", ".glb":
		dir := path.Dir(strings.ReplaceAll(file, `\`, "/"))
		m, err := gltf.Read(data, func(uri string) ([]byte, error) { return open(dir + "/" + uri) })
		if err != nil {
			return nil, false, err
		}
		scale := 1000.0
		if e := strings.ToLower(named); e == ".step" || e == ".stp" {
			scale = 1
		}
		src := m.Points()
		out := make([][3]float64, len(src))
		for i, q := range src {
			out[i] = [3]float64{q[0] * scale, -q[2] * scale, q[1] * scale}
		}
		return out, true, nil
	case ".wrl", ".wrz":
		m, err := wrl.Read(data)
		if err != nil {
			return nil, false, err
		}
		m.Scale(wrlMm)
		return m.Points(), false, nil
	default:
		pts, err := step.Points(data)
		return pts, false, err
	}
}

// wrlMm is the millimetres in one unit of KiCad's .wrl library, a tenth of an inch.
const wrlMm = 2.54

// ReadModelMesh reads a ModelInstance's mesh by its key (agni issue 1652). A .wrl comes back
// in millimetres. A glTF comes back in its own units with its Y-up axes turned to the footprint's
// Z-up ones, and the instance's transform scales it. open is ReadBoardGeometryWithModels' opener, and
// a missing file wraps ErrModelNotFound.
func ReadModelMesh(open ModelOpener, key string) (*mesh.Mesh, error) {
	data, err := open(key)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(path.Ext(key)) {
	case ".wrl", ".wrz":
		m, err := wrl.Read(data)
		if err != nil {
			return nil, err
		}
		m.Scale(wrlMm)
		return m, nil
	case ".gltf", ".glb":
		dir := path.Dir(strings.ReplaceAll(key, `\`, "/"))
		g, err := gltf.Read(data, func(uri string) ([]byte, error) { return open(dir + "/" + uri) })
		if err != nil {
			return nil, err
		}
		m := &mesh.Mesh{}
		for _, pr := range g.Primitives {
			pos := make([][3]float64, len(pr.Positions))
			for i, q := range pr.Positions {
				pos[i] = [3]float64{q[0], -q[2], q[1]}
			}
			m.Parts = append(m.Parts, mesh.Part{Positions: pos, Indices: pr.Indices, Color: pr.Color})
		}
		return m, nil
	}
	return nil, fmt.Errorf("%s: agni draws .wrl and glTF models, not %s", key, path.Ext(key))
}

// meshKey names the mesh file for a model whose height came from file. A glTF draws itself. A STEP
// draws as the .wrl of the same name. KiCad's library ships the .wrl beside the STEP, in the STEP's
// frame.
func meshKey(file string, footprintFrame bool) string {
	if footprintFrame {
		return file
	}
	return strings.TrimSuffix(file, path.Ext(file)) + ".wrl"
}

// matrix returns t as ModelInstance.transform's 12 numbers, a row-order 3x4 matrix.
// t is affine, so its columns are where it takes each unit axis, less where it takes the origin.
func (t modelTransform) matrix() []float64 {
	o := t.apply([3]float64{})
	var cols [3][3]float64
	for k := 0; k < 3; k++ {
		var e [3]float64
		e[k] = 1
		q := t.apply(e)
		cols[k] = [3]float64{q[0] - o[0], q[1] - o[1], q[2] - o[2]}
	}
	out := make([]float64, 12)
	for r := 0; r < 3; r++ {
		out[r*4], out[r*4+1], out[r*4+2], out[r*4+3] = cols[0][r], cols[1][r], cols[2][r], o[r]
	}
	return out
}

// halfTurn turns a ModelInstance transform half a turn about Z, in place, for a part on the back.
// KiCad flips a back part's model by a half turn about Y and then one about Z, which mirrors it top
// to bottom. The contract's mirror negates X instead, and the two differ by this half turn about Z.
func halfTurn(m []float64) {
	for i := 0; i < 8; i++ {
		m[i] = -m[i]
	}
}

// scaleMatrix is a uniform scale as a ModelInstance transform.
func scaleMatrix(s float64) []float64 { return []float64{s, 0, 0, 0, 0, s, 0, 0, 0, 0, s, 0} }

// modelHeights sets each placement's height from its footprint's models, and records the models it
// could not use. A file the board names many times is opened and read once.
func modelHeights(root *node, g *geom.BoardGeometry, open ModelOpener) {
	type read struct {
		pts            [][3]float64
		footprintFrame bool
		file           string // the candidate that gave the points
		reason         string
	}
	cache := map[string]read{}
	load := func(raw string) read {
		if r, ok := cache[raw]; ok {
			return r
		}
		// The reason a model went unread is the first file that was there and failed to read, or
		// else why the named file was not there.
		var r read
		var missing, broken error
		for _, file := range modelCandidates(raw) {
			pts, inFootprint, err := readModel(open, file, path.Ext(raw))
			if err == nil {
				r.pts, r.footprintFrame, r.file, missing, broken = pts, inFootprint, file, nil, nil
				break
			}
			switch {
			case !errors.Is(err, ErrModelNotFound) && broken == nil:
				broken = err
			case missing == nil:
				missing = err
			}
		}
		if broken != nil {
			r.reason = broken.Error()
		} else if missing != nil {
			r.reason = missing.Error()
		}
		cache[raw] = r
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
			raw := atomOf(m.Arg(1))
			r := load(raw)
			if r.reason != "" {
				unread[[2]string{raw, r.reason}]++
				continue
			}
			t := modelTransformOf(m)
			inst := &geom.ModelInstance{Mesh: meshKey(r.file, r.footprintFrame), Transform: t.matrix()}
			if r.footprintFrame {
				// A glTF stand-in is in millimetres, and one a footprint names directly in metres.
				inst.Transform = scaleMatrix(1)
				if e := strings.ToLower(path.Ext(raw)); e != ".step" && e != ".stp" {
					inst.Transform = scaleMatrix(1000)
				}
			}
			if p.GetMirror() {
				halfTurn(inst.Transform)
			}
			p.ModelInstances = append(p.ModelInstances, inst)
			for _, q := range r.pts {
				z := q[2]
				if !r.footprintFrame {
					z = t.z(q)
				}
				if !any || z > top {
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

// apply places a model point in the footprint's frame, in millimetres with Y up and Z above the
// pads' seating plane. KiCad scales, then turns about X, Y and Z by the negated angles, then offsets.
func (t modelTransform) apply(p [3]float64) [3]float64 {
	x, y, z := p[0]*t.scale[0], p[1]*t.scale[1], p[2]*t.scale[2]
	rx, ry, rz := -t.rotDeg[0]*math.Pi/180, -t.rotDeg[1]*math.Pi/180, -t.rotDeg[2]*math.Pi/180
	y, z = y*math.Cos(rx)-z*math.Sin(rx), y*math.Sin(rx)+z*math.Cos(rx)
	x, z = x*math.Cos(ry)+z*math.Sin(ry), -x*math.Sin(ry)+z*math.Cos(ry)
	x, y = x*math.Cos(rz)-y*math.Sin(rz), x*math.Sin(rz)+y*math.Cos(rz)
	return [3]float64{x + t.offsetMm[0], y + t.offsetMm[1], z + t.offsetMm[2]}
}

// z returns a model point's height above the seating plane.
func (t modelTransform) z(p [3]float64) float64 { return t.apply(p)[2] }
