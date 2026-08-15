package ipc2581

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// ReadBoardGeometry parses an IPC-2581 interchange file into the board-geometry sidecar
// (geom.BoardGeometry), the second producer of that contract after KiCad (WS1-006/023). The
// caller owns file I/O (CONSTRAINTS C1); sourceFile is recorded in provenance only.
//
// Fidelity: lossy-bounded (C6). This producer carries the board outline (Profile), the layer
// stackup table, component placements with their pads (resolved through the padstack
// def/instance indirection), and routed copper: tracks (arcs approximated as polylines) and
// vias (drilled holes with the co-located pad as annular, with their layer span), silkscreen/fab
// body graphics (a package's Marking/Outline/AssemblyDrawing composed per placement), and copper
// plane/pour fills as zones (authored outline; WS1-031). Non-via copper lands are NOT carried yet;
// fill cutouts drop per the C6 zone bound. Emitted coordinates are
// integer nanometers (unit_nm=1), matching the KiCad producer so downstream consumers see one
// frame.
//
// Frame: IPC-2581 is Y-up like the geom contract, so coordinates map directly with no Y
// negation (unlike KiCad, which is Y-down). Sides normalize into the contract's KiCad-style
// vocabulary (TOP->F.Cu, BOTTOM->B.Cu) so the format-neutral renderer classifies them without
// change. Rotation is carried verbatim in degrees; the renderer applies its own Y-flip.
func ReadBoardGeometry(r io.Reader, sourceFile string) (*geom.BoardGeometry, error) {
	var f ipcGeomFile
	if err := xml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("ipc2581: %w", err)
	}
	if f.XMLName.Local != "IPC-2581" {
		return nil, fmt.Errorf("ipc2581: not an IPC-2581 file (root %q)", f.XMLName.Local)
	}
	return f.toBoardGeometry(sourceFile), nil
}

// ipcGeomFile is the geometry subset of the IPC-2581 tree, kept separate from the netlist
// reader's ipcFile (C7/C8: the netlist IR and the geometry sidecar decode independently).
type ipcGeomFile struct {
	XMLName   xml.Name         `xml:"IPC-2581"`
	Dicts     []dictStd        `xml:"Content>DictionaryStandard"`
	UserDicts []dictUser       `xml:"Content>DictionaryUser"`
	LineDescs []dictLineDesc   `xml:"Content>DictionaryLineDesc"`
	Layers    []layerEl        `xml:"Ecad>CadData>Layer"`
	Profile   profileEl        `xml:"Ecad>CadData>Step>Profile"`
	Pkgs      []gPkgEl         `xml:"Ecad>CadData>Step>Package"`
	Comps     []gCompEl        `xml:"Ecad>CadData>Step>Component"`
	Features  []layerFeatureEl `xml:"Ecad>CadData>Step>LayerFeature"`
}

// dictStd is a standard-primitive dictionary: a units tag plus the shape definitions that
// pads reference by id (the padstack def half of the def/instance split).
type dictStd struct {
	Units   string       `xml:"units,attr"`
	Entries []entryStdEl `xml:"EntryStandard"`
}

type entryStdEl struct {
	ID     string     `xml:"id,attr"`
	Circle *circleEl  `xml:"Circle"`
	Rect   *rectEl    `xml:"RectCenter"`
	Oval   *rectEl    `xml:"Oval"`
	Cont   *contourEl `xml:"Contour"`
}

// dictUser is a user-primitive dictionary: pad shapes IPC has no standard element for (custom
// contacts, stroke artwork), each a UserSpecial of one or more Contour polygons. Pads reference
// them by <UserPrimitiveRef>. It carries its own units, which may differ from the standard dict.
type dictUser struct {
	Units   string        `xml:"units,attr"`
	Entries []entryUserEl `xml:"EntryUser"`
}

type entryUserEl struct {
	ID       string     `xml:"id,attr"`
	Polygons []polyPath `xml:"UserSpecial>Contour>Polygon"`
}

type circleEl struct {
	Diameter string `xml:"diameter,attr"`
}

type rectEl struct {
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
}

// contourEl is an arbitrary polygon primitive; PR-1 reduces it to its bounding box.
type contourEl struct {
	Polygon polyPath `xml:"Polygon"`
}

type profileEl struct {
	Polygons []polyPath `xml:"Polygon"`
}

type ptEl struct {
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
}

type gPkgEl struct {
	Name string   `xml:"name,attr"`
	Pins []gPinEl `xml:"Pin"`
	// Silkscreen and fab artwork, package-local (composed to the board per placement). Outline is
	// the component body (fab), Marking is silkscreen, AssemblyDrawing is the assembly layer (fab).
	Outline  shapeHolderEl   `xml:"Outline"`
	Markings []shapeHolderEl `xml:"Marking"`
	Assembly struct {
		Outline  shapeHolderEl   `xml:"Outline"`
		Markings []shapeHolderEl `xml:"Marking"`
	} `xml:"AssemblyDrawing"`
}

// shapeHolderEl is an IPC element that wraps drawing geometry as either Polyline or Polygon (silk
// and outline elements use both interchangeably).
type shapeHolderEl struct {
	Polylines []polyPath `xml:"Polyline"`
	Polygons  []polyPath `xml:"Polygon"`
}

// pkgGraphic is one package-local drawing path plus its layer class (fab body/assembly vs
// silkscreen).
type pkgGraphic struct {
	path polyPath
	fab  bool
}

// graphics collects a package's silkscreen and fab drawing paths, each tagged with its layer class.
func (p gPkgEl) graphics() []pkgGraphic {
	var out []pkgGraphic
	add := func(h shapeHolderEl, fab bool) {
		for _, pp := range h.Polylines {
			out = append(out, pkgGraphic{pp, fab})
		}
		for _, pp := range h.Polygons {
			out = append(out, pkgGraphic{pp, fab})
		}
	}
	add(p.Outline, true) // component body outline -> fab
	for _, m := range p.Markings {
		add(m, false) // silkscreen
	}
	add(p.Assembly.Outline, true)
	for _, m := range p.Assembly.Markings {
		add(m, true) // assembly drawing -> fab
	}
	return out
}

type gPinEl struct {
	Number   string    `xml:"number,attr"`
	Location ptEl      `xml:"Location"`
	PrimRef  primRefEl `xml:"StandardPrimitiveRef"`
	UserRef  primRefEl `xml:"UserPrimitiveRef"`
}

// primID returns the pin's pad primitive id, preferring the standard-dictionary reference and
// falling back to a user-dictionary one.
func (p gPinEl) primID() string {
	if p.PrimRef.ID != "" {
		return p.PrimRef.ID
	}
	return p.UserRef.ID
}

type primRefEl struct {
	ID string `xml:"id,attr"`
}

type gCompEl struct {
	RefDes     string  `xml:"refDes,attr"`
	PackageRef string  `xml:"packageRef,attr"`
	LayerRef   string  `xml:"layerRef,attr"`
	Xform      xformEl `xml:"Xform"`
	Location   ptEl    `xml:"Location"`
}

type xformEl struct {
	Rotation string `xml:"rotation,attr"`
}

// primShape is a resolved standard primitive: a contract shape word and its size.
type primShape struct {
	shape string
	sizeX int64
	sizeY int64
}

func (f *ipcGeomFile) toBoardGeometry(src string) *geom.BoardGeometry {
	nmPerUnit := unitToNm(f.units())
	g := &geom.BoardGeometry{UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}}

	for i, l := range f.Layers {
		// kind keeps IPC-2581's layerFunction verbatim, same discipline as KiCad's kind word
		// (C9: BoardLayer carries no stackup material/thickness until a consumer earns it).
		g.Layers = append(g.Layers, &geom.BoardLayer{Number: int32(i), Name: l.Name, Kind: l.Function})
	}

	if o := f.outline(nmPerUnit); o != nil {
		g.Outline = o
	}

	prims := f.primitives(nmPerUnit)
	pkgPins := f.packagePins()
	for _, pl := range f.placements(src, nmPerUnit, prims, pkgPins) {
		g.Placements = append(g.Placements, pl)
	}
	g.Nets = f.copperNets(nmPerUnit, prims)
	g.Graphics = f.graphics(nmPerUnit)
	g.Zones = f.zones(nmPerUnit)
	return g
}

// graphics composes each placed component's package-local silk/fab artwork onto the board via the
// same placement transform pads use (geomath.ComposePlacement), so silk lands on its part. Every
// BoardGraphic is board-frame per the proto contract; the layer word normalizes into the KiCad
// F.SilkS/F.Fab vocabulary the renderer classifies on (N producers -> one vocabulary).
func (f *ipcGeomFile) graphics(nmPerUnit float64) []*geom.BoardGraphic {
	byPkg := map[string][]pkgGraphic{}
	for _, p := range f.Pkgs {
		byPkg[p.Name] = p.graphics()
	}
	var out []*geom.BoardGraphic
	for _, c := range f.Comps {
		if skipRefDes(c.RefDes) {
			continue
		}
		side := normalizeSide(c.LayerRef)
		at := ptNm(c.Location, nmPerUnit)
		rot := degrees(c.Xform.Rotation)
		back := side == "B.Cu"
		for _, g := range byPkg[c.PackageRef] {
			local := flattenPath(g.path, nmPerUnit)
			if len(local) < 2 {
				continue
			}
			pts := make([]*geom.Point, len(local))
			for i, lp := range local {
				pts[i] = geomath.ComposePlacement(at, rot, back, lp)
			}
			out = append(out, &geom.BoardGraphic{
				Shape:  &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts},
				Layer:  silkLayer(g.fab, back),
				RefDes: c.RefDes,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RefDes < out[j].RefDes })
	return out
}

// silkLayer normalizes a package graphic's class + side into the KiCad silk/fab layer vocabulary
// the renderer keys on.
func silkLayer(fab, back bool) string {
	switch {
	case fab && back:
		return "B.Fab"
	case fab:
		return "F.Fab"
	case back:
		return "B.SilkS"
	default:
		return "F.SilkS"
	}
}

// primitives resolves the standard-primitive dictionaries into a shape lookup keyed by id.
func (f *ipcGeomFile) primitives(nmPerUnit float64) map[string]primShape {
	m := map[string]primShape{}
	for _, d := range f.Dicts {
		for _, e := range d.Entries {
			switch {
			case e.Circle != nil:
				dia := lenNm(e.Circle.Diameter, nmPerUnit)
				m[e.ID] = primShape{shape: "circle", sizeX: dia, sizeY: dia}
			case e.Rect != nil:
				m[e.ID] = primShape{shape: "rect", sizeX: lenNm(e.Rect.Width, nmPerUnit), sizeY: lenNm(e.Rect.Height, nmPerUnit)}
			case e.Oval != nil:
				m[e.ID] = primShape{shape: "oval", sizeX: lenNm(e.Oval.Width, nmPerUnit), sizeY: lenNm(e.Oval.Height, nmPerUnit)}
			case e.Cont != nil:
				w, h := polyBBox(e.Cont.Polygon, nmPerUnit)
				m[e.ID] = primShape{shape: "polygon", sizeX: w, sizeY: h}
			}
		}
	}
	// User-primitive dictionaries: a pad referencing one of these via UserPrimitiveRef would
	// otherwise resolve to a zero-size shape. Each entry's extent is the bounding box over all its
	// UserSpecial contours, measured in the DICTIONARY's own units (which may differ from the file's).
	for _, d := range f.UserDicts {
		uNm := unitToNm(d.Units)
		for _, e := range d.Entries {
			w, h := polygonsBBox(e.Polygons, uNm)
			m[e.ID] = primShape{shape: "polygon", sizeX: w, sizeY: h}
		}
	}
	return m
}

// polygonsBBox returns the width and height of the bounding box over ALL the given polygons'
// flattened points (arc steps expanded), in nanometers.
func polygonsBBox(polys []polyPath, nmPerUnit float64) (w, h int64) {
	var minX, minY, maxX, maxY int64
	set := false
	for _, p := range polys {
		for _, pt := range flattenPath(p, nmPerUnit) {
			if !set {
				minX, minY, maxX, maxY = pt.X, pt.Y, pt.X, pt.Y
				set = true
				continue
			}
			minX, maxX = minI64(minX, pt.X), maxI64(maxX, pt.X)
			minY, maxY = minI64(minY, pt.Y), maxI64(maxY, pt.Y)
		}
	}
	if !set {
		return 0, 0
	}
	return maxX - minX, maxY - minY
}

// packagePins indexes each package's pins (number -> local location + primitive id) so a
// placement resolves its pads once per component.
func (f *ipcGeomFile) packagePins() map[string][]gPinEl {
	m := map[string][]gPinEl{}
	for _, p := range f.Pkgs {
		m[p.Name] = p.Pins
	}
	return m
}

// placements emits one ComponentPlacement per placed component, resolving each pin through the
// package's pin list and the primitive dictionary into a flattened Pad (the def/instance split
// collapsed to the contract's inline pad form). Sorted by ref_des for deterministic output.
func (f *ipcGeomFile) placements(src string, nmPerUnit float64, prims map[string]primShape, pkgPins map[string][]gPinEl) []*geom.ComponentPlacement {
	var out []*geom.ComponentPlacement
	for _, c := range f.Comps {
		if skipRefDes(c.RefDes) {
			continue
		}
		side := normalizeSide(c.LayerRef)
		// IPC-2581 is Y-up like the canonical geom frame, so the source Xform rotation is
		// carried verbatim; the back side is flagged for the renderer to mirror (WS1-030).
		// Pads stay footprint-local and unmodified — the renderer composes rotation and mirror.
		pl := &geom.ComponentPlacement{
			RefDes:      c.RefDes,
			At:          ptNm(c.Location, nmPerUnit),
			RotationDeg: degrees(c.Xform.Rotation),
			Layer:       side,
			Mirror:      side == "B.Cu",
			Prov:        &geom.Provenance{SourceFile: src},
		}
		for _, pin := range pkgPins[c.PackageRef] {
			p := prims[pin.primID()]
			pl.Pads = append(pl.Pads, &geom.Pad{
				Number: pin.Number,
				At:     ptNm(pin.Location, nmPerUnit),
				Size:   &geom.Point{X: p.sizeX, Y: p.sizeY},
				Shape:  p.shape,
				Layers: []string{side},
			})
		}
		out = append(out, pl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RefDes < out[j].RefDes })
	return out
}

// outline emits the board Profile polygons as BoardOutline paths (one polyline per polygon).
func (f *ipcGeomFile) outline(nmPerUnit float64) *geom.BoardOutline {
	var paths []*geom.Polyline
	for _, poly := range f.Profile.Polygons {
		if pl := polyline(poly, nmPerUnit); pl != nil {
			paths = append(paths, pl)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return &geom.BoardOutline{Paths: paths}
}

func (f *ipcGeomFile) units() string {
	for _, d := range f.Dicts {
		if d.Units != "" {
			return d.Units
		}
	}
	return ""
}

// polyline converts a closed IPC polygon into a Polyline, expanding arc steps (PolyStepCurve)
// into chord approximations in document order via the shared flattenPath (WS1-028). A polygon
// that interleaves straight and arc steps keeps its edge; returns nil for a degenerate polygon.
func polyline(p polyPath, nmPerUnit float64) *geom.Polyline {
	pts := flattenPath(p, nmPerUnit)
	if len(pts) < 2 {
		return nil
	}
	return &geom.Polyline{Points: pts}
}

// polyBBox returns the width and height of a polygon's bounding box in nanometers, the C6
// reduction PR-1 uses for arbitrary Contour pad primitives. Arc steps are flattened first, so
// the box spans the arc bulge, not just its endpoints (WS1-028).
func polyBBox(p polyPath, nmPerUnit float64) (w, h int64) {
	pts := flattenPath(p, nmPerUnit)
	if len(pts) == 0 {
		return 0, 0
	}
	minX, maxX := pts[0].X, pts[0].X
	minY, maxY := pts[0].Y, pts[0].Y
	for _, pt := range pts[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	return maxX - minX, maxY - minY
}

// normalizeSide maps an IPC-2581 layer/side reference onto the contract's KiCad-style side
// vocabulary so the format-neutral renderer's front/back classifier works unchanged. An
// unrecognized ref is kept verbatim.
func normalizeSide(ref string) string {
	switch strings.ToUpper(ref) {
	case "TOP", "F.CU", "FRONT":
		return "F.Cu"
	case "BOTTOM", "B.CU", "BACK":
		return "B.Cu"
	default:
		return ref
	}
}

// ptNm converts an IPC coordinate (x,y strings in source units, Y-up) into an integer-nm Point.
func ptNm(p ptEl, nmPerUnit float64) *geom.Point {
	return &geom.Point{
		X: int64(math.Round(parseFloat(p.X) * nmPerUnit)),
		Y: int64(math.Round(parseFloat(p.Y) * nmPerUnit)),
	}
}

// lenNm converts a scalar length string in source units into integer nanometers.
func lenNm(s string, nmPerUnit float64) int64 {
	return int64(math.Round(parseFloat(s) * nmPerUnit))
}

func degrees(s string) float64 {
	return parseFloat(s)
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
