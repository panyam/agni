package odb

import (
	"io/fs"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// Read parses the ODB++ job rooted at fsys into an ir.Design.
//
// Fidelity: lossy-bounded. We extract the netlist (each placed component, each pin's net from its
// toeprint record), the packages as footprints, every component property, and the board layer
// stack; copper geometry goes to the board sidecar (ReadBoardGeometry, C7/C8). A net is built from
// the pins on it, as the KiCad board reader builds one, so a net no pin lands on is dropped, and a
// pin on the `$NONE$` net is unconnected. sourceFile is provenance only.
func Read(fsys fs.FS, sourceFile string) (*ir.Design, error) {
	p, err := parseJob(fsys)
	if err != nil {
		return nil, err
	}
	return p.toDesign(sourceFile), nil
}

func (p *parsedJob) toDesign(src string) *ir.Design {
	prov := func() *ir.Provenance { return &ir.Provenance{SourceFile: src} }
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "odb++",
		Name:         p.jobName(),
		Attributes:   map[string]string{},
		Prov:         prov(),
	}
	putAttr(d.Attributes, "odb_step", p.step)
	putAttr(d.Attributes, "odb_source", p.info["ODB_SOURCE"])
	putAttr(d.Attributes, "odb_save_app", p.info["SAVE_APP"])
	if v := p.info["ODB_VERSION_MAJOR"]; v != "" {
		d.Attributes["odb_version"] = v + "." + p.info["ODB_VERSION_MINOR"]
	}

	for _, pkg := range p.eda.pkgs {
		d.Footprints = append(d.Footprints, &ir.Footprint{Name: pkg.name, Prov: prov()})
	}

	byNet := map[int]*ir.Net{}
	seen := map[*ir.Net]map[string]bool{}
	var order []int
	counts := map[string]int{}
	var refs []string
	for _, c := range p.allComponents() {
		if skipRefDes(c.ref) {
			continue
		}
		if counts[c.ref] == 0 {
			refs = append(refs, c.ref)
		}
		counts[c.ref]++
		pkgName := ""
		if c.pkg >= 0 && c.pkg < len(p.eda.pkgs) {
			pkgName = p.eda.pkgs[c.pkg].name
		}
		comp := &ir.Component{RefDes: c.ref, FootprintRef: pkgName, Attributes: map[string]string{}, Prov: prov()}
		props := map[string]string{}
		putAttr(comp.Attributes, "part", c.part)
		putAttr(comp.Attributes, "side", sideWord(c.back))
		for _, pr := range c.props {
			key := pr[0]
			// The Value key is what checks, the BOM and diff read, so a producer's spelling of it
			// (KiCad's Value, a Valor-lineage VALUE) is folded onto that one key.
			if strings.EqualFold(key, "value") {
				key = "Value"
			}
			putAttr(comp.Attributes, key, pr[1])
			putAttr(props, key, pr[1])
		}
		comp.Sections = []*ir.ComponentSection{{Index: 0, PartRef: pkgName, Attributes: props, Prov: prov()}}
		d.Components = append(d.Components, comp)

		for _, t := range c.pins {
			if p.eda.netName(t.net) == "" {
				continue
			}
			net := byNet[t.net]
			if net == nil {
				net = &ir.Net{Name: p.eda.netName(t.net), Prov: prov()}
				byNet[t.net] = net
				seen[net] = map[string]bool{}
				order = append(order, t.net)
			}
			key := c.ref + "." + t.name
			if seen[net][key] {
				continue
			}
			seen[net][key] = true
			net.Connections = append(net.Connections, &ir.Connection{ComponentRef: c.ref, PinRef: t.name, Prov: prov()})
		}
	}
	sort.Ints(order)
	for _, i := range order {
		d.Nets = append(d.Nets, byNet[i])
	}

	// A repeated designator is two placements claiming one name. Declared even when empty so
	// `duplicate-ref-des` knows this reader looked (agni issue 309).
	diag := &ir.InputDiagnostics{Supplied: []string{"ref_des_collisions"}}
	for _, ref := range refs {
		if counts[ref] < 2 {
			continue
		}
		col := &ir.RefDesCollision{RefDes: ref}
		for i := 0; i < counts[ref]; i++ {
			col.Instances = append(col.Instances, prov())
		}
		diag.RefDesCollisions = append(diag.RefDesCollisions, col)
	}
	d.InputDiagnostics = diag

	idx := int32(0)
	for _, l := range p.matrix.layers {
		if l.context != "BOARD" || l.typ == "COMPONENT" {
			continue
		}
		layer := &ir.Layer{Name: l.name, Index: idx, Function: layerFunction(l.typ), Attributes: map[string]string{}, Prov: prov()}
		idx++
		putAttr(layer.Attributes, "layer_function_raw", l.typ)
		putAttr(layer.Attributes, "polarity", l.polarity)
		d.Layers = append(d.Layers, layer)
	}
	return d
}

// skipRefDes reports whether a designator is absent or a placeholder the source has not annotated
// ("REF**", "C?1845"). The netlist and the board both skip through it, since they join on ref_des.
func skipRefDes(ref string) bool { return ref == "" || refdes.IsPlaceholder(ref) }

func sideWord(back bool) string {
	if back {
		return "bottom"
	}
	return "top"
}

// layerFunction normalizes a matrix layer TYPE onto ir.LayerFunction; the source word is kept in
// layer_function_raw beside it.
func layerFunction(typ string) ir.LayerFunction {
	switch typ {
	case "SIGNAL", "MIXED":
		return ir.LayerFunction_LAYER_FUNCTION_SIGNAL
	case "POWER_GROUND":
		return ir.LayerFunction_LAYER_FUNCTION_PLANE
	case "DIELECTRIC":
		return ir.LayerFunction_LAYER_FUNCTION_DIELECTRIC
	case "SOLDER_MASK":
		return ir.LayerFunction_LAYER_FUNCTION_SOLDER_MASK
	case "SILK_SCREEN":
		return ir.LayerFunction_LAYER_FUNCTION_SILKSCREEN
	case "SOLDER_PASTE":
		return ir.LayerFunction_LAYER_FUNCTION_PASTE
	}
	return ir.LayerFunction_LAYER_FUNCTION_UNSPECIFIED
}

// putAttr sets m[k]=v only when v is non-empty.
func putAttr(m map[string]string, k, v string) {
	if v != "" {
		m[k] = v
	}
}
