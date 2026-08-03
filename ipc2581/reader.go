// Package ipc2581 reads IPC-2581 (revision A/B/C) interchange XML into the neutral IR
// (agni.v1.ir). It is core, runtime-agnostic Go (CONSTRAINTS C1): Read takes an io.Reader
// and records only provenance.
//
// IPC-2581 is PCB-fabrication-first, so this is the first reader to populate the physical
// tier (Footprint, Layer, Stackup, BomLine) as well as the netlist (Component, Net,
// Connection). Like a KiCad board, components are section-less (there is no logical unit
// concept). Copper/pad geometry is dropped here (it belongs in a board-geometry sidecar,
// C7/C8, not the netlist IR).
package ipc2581

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ipcFile is the subset of the IPC-2581 tree we read. Paths mirror the real nesting
// (IPC-2581 > Bom; Ecad > CadData > {Layer, Stackup, Step > {Package, Component, LogicalNet}}).
// Element names are matched by local name, so the default XML namespace is ignored.
type ipcFile struct {
	XMLName  xml.Name   `xml:"IPC-2581"`
	Revision string     `xml:"revision,attr"`
	Units    []unitDict `xml:"Content>DictionaryStandard"`
	Boms     []bomEl    `xml:"Bom"`
	Layers   []layerEl  `xml:"Ecad>CadData>Layer"`
	Stackup  *stackupEl `xml:"Ecad>CadData>Stackup"`
	Packages []pkgEl    `xml:"Ecad>CadData>Step>Package"`
	Comps    []compEl   `xml:"Ecad>CadData>Step>Component"`
	Nets     []netEl    `xml:"Ecad>CadData>Step>LogicalNet"`
}

type unitDict struct {
	Units string `xml:"units,attr"`
}

type bomEl struct {
	Items []bomItemEl `xml:"BomItem"`
}

type bomItemEl struct {
	Quantity           int        `xml:"quantity,attr"`
	InternalPartNumber string     `xml:"internalPartNumber,attr,omitempty"`
	PackageRef         string     `xml:"packageRef,attr,omitempty"`
	Category           string     `xml:"category,attr,omitempty"`
	RefDes             []refDesEl `xml:"RefDes"`
}

type refDesEl struct {
	Name string `xml:"name,attr"`
}

type layerEl struct {
	Name     string  `xml:"name,attr"`
	Function string  `xml:"layerFunction,attr,omitempty"`
	Side     string  `xml:"side,attr,omitempty"`
	Polarity string  `xml:"polarity,attr,omitempty"`
	Span     *spanEl `xml:"Span"`
}

// spanEl is a drill layer's <Span fromLayer= toLayer=>: the copper layers a via on that layer
// bridges. A through via spans TOP..BOTTOM; blind/buried vias narrow the pair.
type spanEl struct {
	From string `xml:"fromLayer,attr"`
	To   string `xml:"toLayer,attr"`
}

type stackupEl struct {
	OverallThickness string           `xml:"overallThickness,attr,omitempty"`
	Layers           []stackupLayerEl `xml:"StackupGroup>StackupLayer"`
}

type stackupLayerEl struct {
	LayerRef  string `xml:"layerOrGroupRef,attr"`
	Material  string `xml:"materialType,attr,omitempty"`
	Thickness string `xml:"thickness,attr,omitempty"`
}

type pkgEl struct {
	Name   string `xml:"name,attr"`
	Type   string `xml:"type,attr,omitempty"`
	Height string `xml:"height,attr,omitempty"`
}

type compEl struct {
	RefDes      string         `xml:"refDes,attr"`
	PackageRef  string         `xml:"packageRef,attr,omitempty"`
	Part        string         `xml:"part,attr,omitempty"`
	LayerRef    string         `xml:"layerRef,attr,omitempty"`
	MountType   string         `xml:"mountType,attr,omitempty"`
	NonstdAttrs []nonstdAttrEl `xml:"NonstandardAttribute"`
}

// nonstdAttrEl is an IPC-2581 <NonstandardAttribute name= value= type=>: the vendor slot that
// carries a component's VALUE (e.g. "4.7UF") and other per-part properties IPC has no first-class
// element for.
type nonstdAttrEl struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type netEl struct {
	Name string     `xml:"name,attr"`
	Pins []pinRefEl `xml:"PinRef"`
}

type pinRefEl struct {
	ComponentRef string `xml:"componentRef,attr"`
	Pin          string `xml:"pin,attr"`
}

// Read parses an IPC-2581 interchange file into an ir.Design.
//
// Fidelity: lossy-bounded. We extract the netlist (components, nets, connections) and the
// physical tier (footprints, layers, stackup, BOM); copper/pad geometry, specs, and styles
// are dropped. sourceFile is recorded in provenance only (CONSTRAINTS C1).
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	var f ipcFile
	if err := xml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("ipc2581: %w", err)
	}
	if f.XMLName.Local != "IPC-2581" {
		return nil, fmt.Errorf("ipc2581: not an IPC-2581 file (root %q)", f.XMLName.Local)
	}
	return f.toDesign(sourceFile), nil
}

func (f *ipcFile) toDesign(src string) *ir.Design {
	prov := func() *ir.Provenance { return &ir.Provenance{SourceFile: src} }
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "ipc-2581",
		Attributes:   map[string]string{},
		Prov:         prov(),
	}
	putAttr(d.Attributes, "ipc2581_revision", f.Revision)
	nmPerUnit := unitToNm(f.units())

	for _, p := range f.Packages {
		fp := &ir.Footprint{Name: p.Name, Attributes: map[string]string{}, Prov: prov()}
		putAttr(fp.Attributes, "type", p.Type)
		putAttr(fp.Attributes, "height", p.Height)
		d.Footprints = append(d.Footprints, fp)
	}

	for _, c := range f.Comps {
		comp := &ir.Component{RefDes: c.RefDes, FootprintRef: c.PackageRef, Attributes: map[string]string{}, Prov: prov()}
		putAttr(comp.Attributes, "part", c.Part)
		putAttr(comp.Attributes, "mount_type", c.MountType)
		putAttr(comp.Attributes, "layer_ref", c.LayerRef)
		// NonstandardAttribute carries the component VALUE (and other per-part properties); map the
		// conventional "VALUE" onto the "Value" key checks/BOM/diff read (the KiCad convention), and
		// keep any others verbatim.
		for _, na := range c.NonstdAttrs {
			if na.Name == "" {
				continue
			}
			if na.Name == "VALUE" {
				putAttr(comp.Attributes, "Value", na.Value)
			} else {
				putAttr(comp.Attributes, na.Name, na.Value)
			}
		}
		// One placed component is one physical section (see kicad/pcb.go); the "part" is the
		// package, so section-aware consumers behave the same across formats.
		comp.Sections = []*ir.ComponentSection{{
			Index:      0,
			PartRef:    c.PackageRef,
			Attributes: map[string]string{},
			Prov:       prov(),
		}}
		d.Components = append(d.Components, comp)
	}

	for _, n := range f.Nets {
		net := &ir.Net{Name: n.Name, Prov: prov()}
		for _, pr := range n.Pins {
			net.Connections = append(net.Connections, &ir.Connection{
				ComponentRef: pr.ComponentRef, PinRef: pr.Pin, Prov: prov(),
			})
		}
		d.Nets = append(d.Nets, net)
	}

	for i, l := range f.Layers {
		layer := &ir.Layer{Name: l.Name, Index: int32(i), Function: mapLayerFunction(l.Function), Attributes: map[string]string{}, Prov: prov()}
		putAttr(layer.Attributes, "layer_function_raw", l.Function) // enum is a lossy normalization; keep the source term
		putAttr(layer.Attributes, "side", l.Side)
		putAttr(layer.Attributes, "polarity", l.Polarity)
		d.Layers = append(d.Layers, layer)
	}

	if f.Stackup != nil {
		st := &ir.Stackup{Attributes: map[string]string{}, Prov: prov()}
		putAttr(st.Attributes, "overall_thickness", f.Stackup.OverallThickness)
		for _, sl := range f.Stackup.Layers {
			esl := &ir.StackupLayer{LayerRef: sl.LayerRef, Material: sl.Material, Attributes: map[string]string{}}
			if nm, ok := thicknessNm(sl.Thickness, nmPerUnit); ok {
				esl.ThicknessNm = nm
			} else {
				putAttr(esl.Attributes, "thickness_raw", sl.Thickness)
			}
			st.Layers = append(st.Layers, esl)
		}
		d.Stackup = st
	}

	for _, b := range f.Boms {
		for _, it := range b.Items {
			bl := &ir.BomLine{Mpn: it.InternalPartNumber, Quantity: int32(it.Quantity), Attributes: map[string]string{}, Prov: prov()}
			for _, rd := range it.RefDes {
				bl.RefDes = append(bl.RefDes, rd.Name)
			}
			putAttr(bl.Attributes, "category", it.Category)
			putAttr(bl.Attributes, "package_ref", it.PackageRef)
			d.Bom = append(d.Bom, bl)
		}
	}
	return d
}

func (f *ipcFile) units() string {
	for _, u := range f.Units {
		if u.Units != "" {
			return u.Units
		}
	}
	return ""
}

// putAttr sets m[k]=v only when v is non-empty, keeping attribute maps free of empty noise.
func putAttr(m map[string]string, k, v string) {
	if v != "" {
		m[k] = v
	}
}

// unitToNm returns nanometers per IPC-2581 unit, or 0 if the unit is unknown.
func unitToNm(u string) float64 {
	switch strings.ToUpper(u) {
	case "MILLIMETER", "MM":
		return 1e6
	case "MICRON", "MICROMETER", "UM":
		return 1e3
	case "INCH", "IN":
		return 25.4e6
	case "NANOMETER", "NM":
		return 1
	default:
		return 0
	}
}

// thicknessNm converts a thickness string in source units to nanometers, reporting ok=false
// when the unit is unknown or the value does not parse (caller keeps the raw string instead).
func thicknessNm(s string, nmPerUnit float64) (int64, bool) {
	if s == "" || nmPerUnit == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int64(v*nmPerUnit + 0.5), true
}

// mapLayerFunction normalizes an IPC-2581 layerFunction onto ir.LayerFunction. Unmapped
// values fall through to UNSPECIFIED; the source term is always kept in attributes.
func mapLayerFunction(f string) ir.LayerFunction {
	switch strings.ToUpper(f) {
	case "CONDUCTOR", "SIGNAL":
		return ir.LayerFunction_LAYER_FUNCTION_SIGNAL
	case "PLANE", "POWER_GROUND", "MIXED":
		return ir.LayerFunction_LAYER_FUNCTION_PLANE
	case "DIELPREG", "DIELCORE", "DIELECTRIC":
		return ir.LayerFunction_LAYER_FUNCTION_DIELECTRIC
	case "SOLDERMASK", "SOLDER_MASK":
		return ir.LayerFunction_LAYER_FUNCTION_SOLDER_MASK
	case "SILKSCREEN", "LEGEND":
		return ir.LayerFunction_LAYER_FUNCTION_SILKSCREEN
	case "SOLDERPASTE", "PASTEMASK":
		return ir.LayerFunction_LAYER_FUNCTION_PASTE
	default:
		return ir.LayerFunction_LAYER_FUNCTION_UNSPECIFIED
	}
}
