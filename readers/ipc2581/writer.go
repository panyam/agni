package ipc2581

import (
	"encoding/xml"
	"io"
	"strconv"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// emitFile mirrors the reader's nesting for marshaling, reusing the reader's leaf element
// structs (layerEl, compEl, ...) so read and write stay symmetric. Only the wrapper elements
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
	CadData emitCadData `xml:"CadData"`
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

// Write emits an IPC-2581 document from an ir.Design (the inverse of Read).
//
// Fidelity: lossy-bounded, matching the reader -- it writes the netlist and physical tier
// (components, nets, footprints, layers, stackup, BOM) but not copper/pad geometry, and does
// not reproduce the source's byte layout or XML namespace (the reader matches by local name,
// so the output round-trips at the IR level). Byte/spec-lossless output is deferred to WS1-006
// (board geometry) plus the FidelityFragment layer. Units are emitted as NANOMETER with raw
// thickness_nm values, so a read-back recovers the same IR exactly.
func Write(w io.Writer, d *ir.Design) error {
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
		// netclass_raw is the source term kept verbatim by the reader; emitting it back keeps the
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
	if d.Stackup != nil {
		st := &stackupEl{OverallThickness: d.Stackup.Attributes["overall_thickness"]}
		for _, sl := range d.Stackup.Layers {
			th := sl.Attributes["thickness_raw"]
			if sl.ThicknessNm != 0 {
				th = strconv.FormatInt(sl.ThicknessNm, 10) // paired with units=NANOMETER above
			}
			st.Layers = append(st.Layers, stackupLayerEl{LayerRef: sl.LayerRef, Material: sl.Material, Thickness: th})
		}
		f.Ecad.CadData.Stackup = st
	}
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
