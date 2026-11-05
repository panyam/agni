package altium

import (
	"path"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// electrical reports whether a component belongs in the netlist: a standard part with a designator
// and at least one named pad. Altium's own ODB++ export draws the same line, leaving out mounting
// holes, fiducials and layer markers, which are mechanical or graphical parts or have unnamed pads.
func (c *comp) electrical() bool {
	return !c.mechanical && c.namedPads > 0 && c.ref != "" && !refdes.IsPlaceholder(c.ref)
}

// footprint is the component's footprint name, without the library path Altium may keep in it.
func (c *comp) footprint() string {
	f := strings.ReplaceAll(c.p["PATTERN"], "\\", "/")
	return path.Base(f)
}

func (p *pcb) toDesign(src string) *ir.Design {
	prov := func() *ir.Provenance { return &ir.Provenance{SourceFile: src} }
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "altium",
		Name:         designName(src),
		Attributes:   map[string]string{},
		Prov:         prov(),
	}
	putAttr(d.Attributes, "altium_version", p.board["VERSION"])

	fps := map[string]bool{}
	counts := map[string]int{}
	var refs []string
	for i := range p.comps {
		c := &p.comps[i]
		if !c.electrical() {
			continue
		}
		fp := c.footprint()
		if fp != "" && !fps[fp] {
			fps[fp] = true
			d.Footprints = append(d.Footprints, &ir.Footprint{Name: fp, Prov: prov()})
		}
		if counts[c.ref] == 0 {
			refs = append(refs, c.ref)
		}
		counts[c.ref]++

		comp := &ir.Component{RefDes: c.ref, FootprintRef: fp, Attributes: map[string]string{}, Prov: prov()}
		props := map[string]string{}
		side := "top"
		if strings.EqualFold(c.p["LAYER"], "BOTTOM") {
			side = "bottom"
		}
		putAttr(comp.Attributes, "side", side)
		// The comment usually carries the value, and "Comment" is Altium's placeholder for none.
		if c.comment != "" && !strings.EqualFold(c.comment, "Comment") && !strings.HasPrefix(c.comment, ".") {
			putAttr(comp.Attributes, "Value", c.comment)
			putAttr(props, "Value", c.comment)
		}
		// Schematic parameters, Part Number among them, go in under their own names. The shared
		// ingestion pass promotes the part number (classify.StampMPN).
		for k, v := range c.params {
			putAttr(comp.Attributes, k, v)
			putAttr(props, k, v)
		}
		comp.Sections = []*ir.ComponentSection{{Index: 0, PartRef: fp, Attributes: props, Prov: prov()}}
		d.Components = append(d.Components, comp)
	}

	// A net is built from the named pads on it, as the other board readers build one. Several pads
	// of one part can share a name (three SH pads on a connector shell), and they make one pin.
	byNet := map[int]*ir.Net{}
	seen := map[int]map[string]bool{}
	for _, pd := range p.pads {
		if pd.net < 0 || pd.net >= len(p.nets) || p.nets[pd.net] == "" || pd.name == "" || pd.comp < 0 || pd.comp >= len(p.comps) {
			continue
		}
		c := &p.comps[pd.comp]
		if !c.electrical() {
			continue
		}
		n := byNet[pd.net]
		if n == nil {
			n = &ir.Net{Name: p.nets[pd.net], Prov: prov()}
			byNet[pd.net] = n
			seen[pd.net] = map[string]bool{}
		}
		key := c.ref + "." + pd.name
		if seen[pd.net][key] {
			continue
		}
		seen[pd.net][key] = true
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c.ref, PinRef: pd.name, Prov: prov()})
	}
	for i := range p.nets {
		if n := byNet[i]; n != nil {
			d.Nets = append(d.Nets, n)
		}
	}

	// A repeated designator is two placements claiming one name. The list is declared even when
	// empty, so `duplicate-ref-des` knows this reader looked (agni issue 309).
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

	for i, id := range p.stack {
		fn := ir.LayerFunction_LAYER_FUNCTION_SIGNAL
		if id >= 39 && id <= 54 {
			fn = ir.LayerFunction_LAYER_FUNCTION_PLANE
		}
		layer := &ir.Layer{Name: copperName(i, len(p.stack)), Index: int32(i), Function: fn, Attributes: map[string]string{}, Prov: prov()} // #nosec G115 -- at most 82 layer ids
		putAttr(layer.Attributes, "altium_layer", p.board.layerName(id))
		d.Layers = append(d.Layers, layer)
	}
	return d
}

func (b props) layerName(id int) string {
	return b["LAYER"+itoa(id)+"NAME"]
}

// designName is the file's name without its directory or extension.
func designName(src string) string {
	base := path.Base(strings.ReplaceAll(src, "\\", "/"))
	return strings.TrimSuffix(base, path.Ext(base))
}

// putAttr sets m[k]=v only when v is non-empty.
func putAttr(m map[string]string, k, v string) {
	if v != "" {
		m[k] = v
	}
}
