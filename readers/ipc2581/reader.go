// Package ipc2581 reads IPC-2581 (revision A/B/C) interchange XML into the neutral IR
// (agni.v1.ir). It is core, runtime-agnostic Go (CONSTRAINTS C1), so Read takes an io.Reader
// and records only provenance.
//
// IPC-2581 is PCB-fabrication-first, so this reader populates the physical tier (Footprint,
// Layer, Stackup, BomLine) as well as the netlist (Component, Net, Connection). Like a KiCad
// board, components are section-less (there is no logical unit concept). Copper and pad
// geometry go to the board-geometry sidecar (ReadBoardGeometry, C7/C8), not the netlist IR.
package ipc2581

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/num"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
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

// spanEl is a drill layer's <Span fromLayer= toLayer=>. It names the copper layers that a via on
// that drill layer bridges. A through via spans TOP..BOTTOM, and blind/buried vias narrow the pair.
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
	Name   string     `xml:"name,attr"`
	Type   string     `xml:"type,attr,omitempty"`
	Height string     `xml:"height,attr,omitempty"`
	Pins   []pkgPinEl `xml:"Pin,omitempty"`
}

// pkgPinEl is one <Pin> of a package. electricalType is ELECTRICAL, MECHANICAL or UNDEFINED, and a
// MECHANICAL pin, such as a fiducial's pad, is no pin of the part.
type pkgPinEl struct {
	Number         string `xml:"number,attr"`
	ElectricalType string `xml:"electricalType,attr,omitempty"`
}

type compEl struct {
	RefDes      string         `xml:"refDes,attr"`
	PackageRef  string         `xml:"packageRef,attr,omitempty"`
	Part        string         `xml:"part,attr,omitempty"`
	LayerRef    string         `xml:"layerRef,attr,omitempty"`
	MountType   string         `xml:"mountType,attr,omitempty"`
	NonstdAttrs []nonstdAttrEl `xml:"NonstandardAttribute"`
}

// nonstdAttrEl is an IPC-2581 <NonstandardAttribute name= value= type=>, a vendor slot. It holds a
// component's VALUE (e.g. "4.7UF") and other per-part properties. IPC has no first-class element
// for those properties.
type nonstdAttrEl struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type netEl struct {
	Name string `xml:"name,attr"`
	// NetClass is IPC-2581's LogicalNet/@netClass, a CLOSED enum (CLK/FIXED/GROUND/SIGNAL/POWER/
	// UNUSED) saying what the net IS. Despite the name it is NOT KiCad's net class, which is
	// user-named constraint-group membership and lives in ir.Net.net_classes (WS1-050). This one
	// feeds ir.Net.roles through the shared ingestion pass.
	NetClass string     `xml:"netClass,attr,omitempty"`
	Pins     []pinRefEl `xml:"PinRef"`
}

type pinRefEl struct {
	ComponentRef string `xml:"componentRef,attr"`
	Pin          string `xml:"pin,attr"`
}

// Read parses an IPC-2581 interchange file into an ir.Design.
//
// Fidelity: lossy-bounded. We extract the netlist (components, nets, connections) and the
// physical tier (footprints, layers, stackup, BOM). Copper/pad geometry, specs, and styles
// are dropped. sourceFile is provenance only.
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

// skipRefDes reports whether a component designator is absent or an unannotated placeholder. Such
// a designator names no part this reader should keep. Four sites call it: the netlist components,
// their connections, and the two geometry passes. The netlist and the board geometry join on
// ref_des. If one tier kept a component and the other dropped it, the result would be a placement
// with no component or a component with no placement.
func skipRefDes(ref string) bool { return ref == "" || refdes.IsPlaceholder(ref) }

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

	// Each package is also the part type of the components placed from it, with the package's
	// electrical pins as the part's pins. So a part whose package has none, such as a fiducial or a
	// label, reads as a part with no pins, not as a part whose pins are unknown (agni issue 1437).
	pkgPart := map[string]*ir.PartType{}
	for _, p := range f.Packages {
		fp := &ir.Footprint{Name: p.Name, Attributes: map[string]string{}, Prov: prov()}
		putAttr(fp.Attributes, "type", p.Type)
		putAttr(fp.Attributes, "height", p.Height)
		d.Footprints = append(d.Footprints, fp)
		if p.Name != "" && pkgPart[p.Name] == nil {
			pkgPart[p.Name] = packagePart(p, prov)
		}
	}
	lib := &ir.PartLibrary{Name: partLibrary, Prov: prov()}
	typed := map[string]bool{}

	for _, c := range f.Comps {
		// Skip a component with no designator or a placeholder one ("REF**", "C?1845"), as the
		// KiCad board reader does; see
		// docsite/content/architecture/ingestion-and-ir.md#placeholder-designators. Both tiers of
		// THIS reader have to agree, so the geometry passes skip through skipRefDes too.
		if skipRefDes(c.RefDes) {
			continue
		}
		comp := &ir.Component{RefDes: c.RefDes, FootprintRef: c.PackageRef, Attributes: map[string]string{}, Prov: prov()}
		putAttr(comp.Attributes, "part", c.Part)
		putAttr(comp.Attributes, "mount_type", c.MountType)
		putAttr(comp.Attributes, "layer_ref", c.LayerRef)
		// Map the conventional "VALUE" NonstandardAttribute onto the "Value" key, and keep any
		// others verbatim. Checks, BOM and diff read "Value", following the KiCad convention.
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
		// One placed component is one physical section (see kicad/pcb.go), and the "part" is the
		// package, so section-aware consumers behave the same across formats.
		sec := &ir.ComponentSection{
			Index:      0,
			PartRef:    c.PackageRef,
			Attributes: map[string]string{},
			Prov:       prov(),
		}
		if pt := pkgPart[c.PackageRef]; pt != nil {
			sec.LibraryRef = partLibrary
			if !typed[c.PackageRef] {
				typed[c.PackageRef] = true
				lib.Parts = append(lib.Parts, pt)
			}
		}
		comp.Sections = []*ir.ComponentSection{sec}
		d.Components = append(d.Components, comp)
	}
	if len(lib.Parts) > 0 {
		d.Libraries = []*ir.PartLibrary{lib}
	}
	// IPC-2581 has no gate or slot construct, so a repeated refDes is two placements claiming one
	// name. Declared even when empty so `duplicate-ref-des` knows this reader looked (agni issue
	// 309). A second diagnostic must MERGE into this struct, since assigning a fresh one drops this
	// one; see docsite/content/architecture/ingestion-and-ir.md#input-diagnostics.
	d.InputDiagnostics = refDesCollisions(f.Comps, src)

	for _, n := range f.Nets {
		if unusedNet(n.Name) {
			continue
		}
		net := &ir.Net{Name: n.Name, Attributes: map[string]string{}, Prov: prov()}
		// The enum is a lossy normalization, so keep the source term beside the mapped one, as
		// layer_function_raw does below. declared_role is the NEUTRAL key classify.StampNetRoles
		// reads. The format-specific translation happens here in the reader (C20's left-shift).
		putAttr(net.Attributes, "netclass_raw", n.NetClass)
		putAttr(net.Attributes, classify.AttrDeclaredRole, declaredRole(n.NetClass))
		for _, pr := range n.Pins {
			// Drop a connection to a component that the loop above skipped. The net table is
			// authored separately from the components, so the predicate runs twice. In KiCad a
			// skipped footprint takes its pads with it.
			if skipRefDes(pr.ComponentRef) {
				continue
			}
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
			bl := &ir.BomLine{Mpn: it.InternalPartNumber, Quantity: num.Int32(it.Quantity), Attributes: map[string]string{}, Prov: prov()}
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

// allegroUnused is the name Allegro's export gives the net of a pin that is on no net, such as
// `Unused_05CE8C28`. The board itself names no net there.
var allegroUnused = regexp.MustCompile(`^Unused_[0-9A-Fa-f]{8}$`)

// unusedNet reports whether name is a net the exporting tool invented for an unconnected pin. Such
// a net is read as no net, so each of those pins reads unconnected, as the board's own read has it
// (agni 1383).
func unusedNet(name string) bool { return allegroUnused.MatchString(name) }

// declaredRole translates IPC-2581's LogicalNet/@netClass enum into the engine's net-role
// vocabulary, or "" when the term states no role. Only GROUND and POWER state one. SIGNAL is the
// default, FIXED is a routing directive (do not re-route), and UNUSED is a lifecycle status. CLK is
// a role, but the engine has no clock role and a reader should not invent vocabulary. The source
// term is kept in netclass_raw either way.
func declaredRole(netClass string) string {
	switch netClass {
	case "GROUND":
		return classify.RoleToken(ir.Role_ROLE_GROUND)
	case "POWER":
		return classify.RoleToken(ir.Role_ROLE_RAIL)
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

// thicknessNm converts a thickness string in source units to nanometers. It reports ok=false
// when the unit is unknown or the value does not parse, and the caller then keeps the raw string.
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
// values fall through to UNSPECIFIED. The source term is always kept in attributes.
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

// refDesCollisions reports designators claimed by more than one placement, and declares that the
// question was asked. It applies the same skipRefDes filter as toDesign, so a placeholder is
// excluded as unmodeled rather than reported.
func refDesCollisions(comps []compEl, src string) *ir.InputDiagnostics {
	order := []string{}
	count := map[string]int{}
	for _, c := range comps {
		if skipRefDes(c.RefDes) {
			continue
		}
		if count[c.RefDes] == 0 {
			order = append(order, c.RefDes)
		}
		count[c.RefDes]++
	}
	diag := &ir.InputDiagnostics{Supplied: []string{"ref_des_collisions"}}
	for _, ref := range order {
		if count[ref] < 2 {
			continue
		}
		instances := make([]*ir.Provenance, 0, count[ref])
		for i := 0; i < count[ref]; i++ {
			instances = append(instances, &ir.Provenance{SourceFile: src})
		}
		diag.RefDesCollisions = append(diag.RefDesCollisions, &ir.RefDesCollision{RefDes: ref, Instances: instances})
	}
	return diag
}

// partLibrary names the library that holds a file's package part types.
const partLibrary = "ipc-2581"

// packagePart is the part type of a package, with one pin per pin number, leaving out a MECHANICAL
// pin.
func packagePart(p pkgEl, prov func() *ir.Provenance) *ir.PartType {
	pt := &ir.PartType{Name: p.Name, Prov: prov()}
	seen := map[string]bool{}
	for _, pin := range p.Pins {
		if pin.Number == "" || seen[pin.Number] || strings.EqualFold(pin.ElectricalType, "MECHANICAL") {
			continue
		}
		seen[pin.Number] = true
		pt.Pins = append(pt.Pins, &ir.Pin{Designator: pin.Number, Prov: prov()})
	}
	return pt
}
