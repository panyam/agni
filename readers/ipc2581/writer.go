package ipc2581

import (
	"encoding/xml"
	"io"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// emitFile mirrors the reader's nesting for marshaling. It reuses the reader's leaf element
// structs (layerEl, compEl, ...), so read and write are symmetric. Only the wrapper elements
// are new here.
type emitFile struct {
	XMLName  xml.Name    `xml:"IPC-2581"`
	Revision string      `xml:"revision,attr,omitempty"`
	Content  emitContent `xml:"Content"`
	Bom      *bomEl      `xml:"Bom,omitempty"`
	Ecad     emitEcad    `xml:"Ecad"`
}

type emitContent struct {
	Dict unitDict `xml:"DictionaryStandard"`
}

type emitEcad struct {
	CadHeader *cadHeaderEl `xml:"CadHeader,omitempty"`
	CadData   emitCadData  `xml:"CadData"`
}

// cadHeaderEl holds the Specs that give a stack row its dielectric constant and loss tangent.
type cadHeaderEl struct {
	Units string       `xml:"units,attr"`
	Specs []emitSpecEl `xml:"Spec"`
}

type emitSpecEl struct {
	Name       string           `xml:"name,attr"`
	Dielectric []emitSpecPropEl `xml:"Dielectric"`
}

type emitSpecPropEl struct {
	Type     string `xml:"type,attr"`
	Property struct {
		Value string `xml:"value,attr"`
	} `xml:"Property"`
}

type stackupEl struct {
	OverallThickness string           `xml:"overallThickness,attr,omitempty"`
	Layers           []stackupLayerEl `xml:"StackupGroup>StackupLayer"`
}

type stackupLayerEl struct {
	LayerRef  string `xml:"layerOrGroupRef,attr"`
	Material  string `xml:"materialType,attr,omitempty"`
	Thickness string `xml:"thickness,attr,omitempty"`
	Sequence  int    `xml:"sequence,attr"`
	SpecRef   *struct {
		ID string `xml:"id,attr"`
	} `xml:"SpecRef"`
}

type emitCadData struct {
	Layers  []layerEl  `xml:"Layer"`
	Stackup *stackupEl `xml:"Stackup,omitempty"`
	Step    emitStep   `xml:"Step"`
}

type emitStep struct {
	Packages []pkgEl  `xml:"Package"`
	Comps    []compEl `xml:"Component"`
	Nets     []netEl  `xml:"LogicalNet"`
}

// Write emits an IPC-2581 document from an ir.Design (the inverse of Read), with the board's stack
// when stack is non-nil.
//
// Fidelity: lossy-bounded, matching the reader. It writes the netlist and physical tier
// (components, nets, footprints, layers, BOM), and the stack it is given. It does not write the
// source's byte layout or XML namespace. The reader matches by local name, so the output round-trips at the IR
// level. Copper and pad geometry live in the board sidecar (WS1-006), and Write does not take
// it. Byte/spec-lossless output waits on the FidelityFragment layer. Units are emitted as
// NANOMETER with raw nanometre values, so a read-back recovers the same IR exactly.
//
// The stack comes from the board, BoardGeometry.stackup, since the design carries none (agni 1660).
// Every row is written, dielectrics included, each naming a Layer of its kind, so a read-back of the
// board gives the same rows. The total is written only when the stack states one.
func Write(w io.Writer, d *ir.Design, stack *geom.Stackup) error {
	f := emitFile{
		Revision: d.Attributes["ipc2581_revision"],
		Content:  emitContent{Dict: unitDict{Units: "NANOMETER"}},
	}

	for _, fp := range d.Footprints {
		f.Ecad.CadData.Step.Packages = append(f.Ecad.CadData.Step.Packages, pkgEl{
			Name: fp.Name, Type: fp.Attributes["type"], Height: fp.Attributes["height"],
		})
	}
	for _, c := range d.Components {
		f.Ecad.CadData.Step.Comps = append(f.Ecad.CadData.Step.Comps, compEl{
			RefDes: c.RefDes, PackageRef: c.FootprintRef,
			Part: c.Attributes["part"], MountType: c.Attributes["mount_type"], LayerRef: c.Attributes["layer_ref"],
		})
	}
	for _, n := range d.Nets {
		// The reader keeps the source term verbatim in netclass_raw. Emitting it back makes the
		// round trip lossless on @netClass, the same way layer_function_raw does for @layerFunction.
		ne := netEl{Name: n.Name, NetClass: n.Attributes["netclass_raw"]}
		for _, cn := range n.Connections {
			ne.Pins = append(ne.Pins, pinRefEl{ComponentRef: cn.ComponentRef, Pin: cn.PinRef})
		}
		f.Ecad.CadData.Step.Nets = append(f.Ecad.CadData.Step.Nets, ne)
	}
	for _, l := range d.Layers {
		fn := l.Attributes["layer_function_raw"]
		if fn == "" {
			fn = layerFunctionToken(l.Function)
		}
		f.Ecad.CadData.Layers = append(f.Ecad.CadData.Layers, layerEl{
			Name: l.Name, Function: fn, Side: l.Attributes["side"], Polarity: l.Attributes["polarity"],
		})
	}
	writeStack(&f, stack)
	if len(d.Bom) > 0 {
		b := &bomEl{}
		for _, bl := range d.Bom {
			it := bomItemEl{
				Quantity: int(bl.Quantity), InternalPartNumber: bl.Mpn,
				PackageRef: bl.Attributes["package_ref"], Category: bl.Attributes["category"],
			}
			for _, rd := range bl.RefDes {
				it.RefDes = append(it.RefDes, refDesEl{Name: rd})
			}
			b.Items = append(b.Items, it)
		}
		f.Bom = b
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(f); err != nil {
		return err
	}
	return enc.Close()
}

// layerFunctionToken maps a normalized LayerFunction back to a representative IPC-2581
// layerFunction token, used only when the reader did not preserve the source token.
func layerFunctionToken(f ir.LayerFunction) string {
	switch f {
	case ir.LayerFunction_LAYER_FUNCTION_SIGNAL:
		return "CONDUCTOR"
	case ir.LayerFunction_LAYER_FUNCTION_PLANE:
		return "PLANE"
	case ir.LayerFunction_LAYER_FUNCTION_DIELECTRIC:
		return "DIELECTRIC"
	case ir.LayerFunction_LAYER_FUNCTION_SOLDER_MASK:
		return "SOLDERMASK"
	case ir.LayerFunction_LAYER_FUNCTION_SILKSCREEN:
		return "SILKSCREEN"
	case ir.LayerFunction_LAYER_FUNCTION_PASTE:
		return "SOLDERPASTE"
	default:
		return ""
	}
}

// writeStack puts the board's stack into f: one StackupLayer per row, in order, a Layer for each
// row the design does not already declare, and a Spec for each row with a dielectric constant or
// loss tangent.
func writeStack(f *emitFile, stack *geom.Stackup) {
	if stack == nil || len(stack.GetLayers()) == 0 {
		return
	}
	// The board names copper by its place in the stack (layerNames), so the k-th copper row is the
	// design's k-th copper layer, under the name the design gives it.
	declared := map[string]bool{}
	var copperRefs []string
	for _, l := range f.Ecad.CadData.Layers {
		declared[l.Name] = true
		if kind, _ := stackKind(l.Function); kind == "copper" {
			copperRefs = append(copperRefs, l.Name)
		}
	}
	coppers := 0
	for _, row := range stack.GetLayers() {
		if row.GetKind() == "copper" {
			coppers++
		}
	}
	if coppers != len(copperRefs) {
		copperRefs = nil
	}
	copper := 0
	st := &stackupEl{}
	if stack.Thickness != nil {
		st.OverallThickness = strconv.FormatInt(stack.GetThickness(), 10)
	}
	var specs []emitSpecEl
	for i, row := range stack.GetLayers() {
		ref := row.GetLayer()
		if ref == "" {
			ref = row.GetName()
		}
		if row.GetKind() == "copper" {
			if copperRefs != nil {
				ref = copperRefs[copper]
			}
			copper++
		}
		if !declared[ref] {
			declared[ref] = true
			f.Ecad.CadData.Layers = append(f.Ecad.CadData.Layers, layerEl{Name: ref, Function: stackFunction(row)})
		}
		sl := stackupLayerEl{LayerRef: ref, Material: row.GetMaterial(), Sequence: i + 1}
		if row.Thickness != nil {
			sl.Thickness = strconv.FormatInt(row.GetThickness(), 10)
		}
		var props []emitSpecPropEl
		for _, p := range []struct {
			typ string
			v   *float64
		}{{"DIELECTRIC_CONSTANT", row.EpsilonR}, {"LOSS_TANGENT", row.LossTangent}} {
			if p.v != nil {
				prop := emitSpecPropEl{Type: p.typ}
				prop.Property.Value = strconv.FormatFloat(*p.v, 'g', -1, 64)
				props = append(props, prop)
			}
		}
		if len(props) > 0 {
			id := "SPEC_" + strconv.Itoa(i+1)
			specs = append(specs, emitSpecEl{Name: id, Dielectric: props})
			sl.SpecRef = &struct {
				ID string `xml:"id,attr"`
			}{ID: id}
		}
		st.Layers = append(st.Layers, sl)
	}
	f.Ecad.CadData.Stackup = st
	if len(specs) > 0 {
		f.Ecad.CadHeader = &cadHeaderEl{Units: "NANOMETER", Specs: specs}
	}
}

// stackFunction is the layerFunction a stack row's kind reads back as (stackKind's inverse).
func stackFunction(row *geom.StackLayer) string {
	switch row.GetKind() {
	case "copper":
		return "CONDUCTOR"
	case "dielectric":
		switch row.GetDielectricType() {
		case "core":
			return "DIELCORE"
		case "prepreg":
			return "DIELPREG"
		}
		return "DIELECTRIC"
	case "mask":
		return "SOLDERMASK"
	case "paste":
		return "PASTEMASK"
	case "silk":
		return "SILKSCREEN"
	}
	return "DOCUMENT"
}
