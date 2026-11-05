package allegro

import (
	"path"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// placed is one placed footprint that carries a reference designator.
type placed struct {
	inst   int // the 0x2D
	comp   int // its 0x07
	def    int // the 0x06 the 0x07 belongs to, or -1
	ref    string
	fpName string
}

// placements lists every placed footprint with a reference designator, in the order the footprint
// definitions list them. A placement with no 0x07, such as a dimension or a fiducial mark drawn as a
// symbol, has no designator and is not a component.
func (p *board) placements() []placed {
	var out []placed
	p.list(listFootDefs, func(def int) {
		fp := p.str(p.u32(def, 8))
		defKey := p.u32(def, 4)
		p.chain(p.u32(def, 36), defKey, next8, func(inst int) {
			if p.b[inst] != tagFootInst {
				return
			}
			comp, ok := p.at(p.u32(inst, 40), tagCompInst)
			if !ok {
				return
			}
			ref := p.str(p.u32(comp, 28))
			if ref == "" || refdes.IsPlaceholder(ref) {
				return
			}
			cdef, ok := p.at(p.u32(comp, 12), tagCompDef)
			if !ok {
				cdef = -1
			}
			out = append(out, placed{inst: inst, comp: comp, def: cdef, ref: ref, fpName: fp})
		})
	})
	return out
}

func next8(byte) int { return 8 }

// fieldText is the text of the 0x03 field with the given code on the chain starting at head, or "".
func (p *board) fieldText(head, owner uint32, code uint16) string {
	text := ""
	p.chain(head, owner, next8, func(off int) {
		if text != "" || p.b[off] != tagField || le16(p.b, off+2) != code || p.b[off+16] != 0x68 {
			return
		}
		size := int(le16(p.b, off+18))
		if off+24+size > len(p.b) {
			return
		}
		text = cstring(p.b[off+24 : off+24+size])
	})
	return text
}

// padNumber is the pin number of a placed pad, from the footprint pad it copies.
func (p *board) padNumber(placedPad int) string {
	pad, ok := p.at(p.u32(placedPad, 36), tagPad)
	if !ok {
		return ""
	}
	return p.str(p.u32(pad, 8))
}

// padNet is the name of the net a placed pad, via, track or copper shape is on, through its net
// assignment, or "" for none.
func (p *board) padNet(item int) string {
	asg, ok := p.at(p.u32(item, 12), tagNetAssign)
	if !ok {
		return ""
	}
	net, ok := p.at(p.u32(asg, 12), tagNet)
	if !ok {
		return ""
	}
	return p.str(p.u32(net, 12))
}

func (p *board) toDesign(src string) *ir.Design {
	prov := func() *ir.Provenance { return &ir.Provenance{SourceFile: src} }
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "allegro",
		Name:         designName(src),
		Attributes:   map[string]string{"allegro_format": "17.2"},
		Prov:         prov(),
	}
	putAttr(d.Attributes, "allegro_version", p.version)

	fps := map[string]bool{}
	counts := map[string]int{}
	var refs []string
	byRef := map[uint32]string{} // 0x07 key -> ref-des, for the net walk
	for _, pl := range p.placements() {
		if !fps[pl.fpName] {
			fps[pl.fpName] = true
			d.Footprints = append(d.Footprints, &ir.Footprint{Name: pl.fpName, Prov: prov()})
		}
		if counts[pl.ref] == 0 {
			refs = append(refs, pl.ref)
		}
		counts[pl.ref]++
		byRef[p.u32(pl.comp, 4)] = pl.ref

		comp := &ir.Component{RefDes: pl.ref, FootprintRef: pl.fpName, Attributes: map[string]string{}, Prov: prov()}
		props := map[string]string{}
		side := "top"
		if p.b[pl.inst+2] == 1 {
			side = "bottom"
		}
		putAttr(comp.Attributes, "side", side)
		if pl.def >= 0 {
			defKey := p.u32(pl.def, 4)
			// The value is a field on the part's definition. Checks, the BOM and diff read the Value key.
			value := p.fieldText(p.u32(pl.def, 32), defKey, 0x02)
			device := p.str(p.u32(pl.def, 12))
			putAttr(comp.Attributes, "Value", value)
			putAttr(props, "Value", value)
			putAttr(comp.Attributes, "device_type", device)
			putAttr(props, "device_type", device)
		}
		comp.Sections = []*ir.ComponentSection{{Index: 0, PartRef: pl.fpName, Attributes: props, Prov: prov()}}
		d.Components = append(d.Components, comp)
	}

	// A net is built from the placed pads on it, as the ODB++ and KiCad board readers build one. An
	// unnamed net holds an unconnected pin, so it is left out and the pin reads as unconnected.
	p.list(listNets, func(net int) {
		name := p.str(p.u32(net, 12))
		if name == "" {
			return
		}
		n := &ir.Net{Name: name, Prov: prov()}
		seen := map[string]bool{}
		netKey := p.u32(net, 4)
		p.chain(p.u32(net, 28), netKey, next8, func(asg int) {
			if p.b[asg] != tagNetAssign {
				return
			}
			p.chain(p.u32(asg, 16), p.u32(asg, 4), next8, func(item int) {
				if p.b[item] != tagPlacedPad {
					return
				}
				ref := byRef[p.u32(item, 56)]
				pin := p.padNumber(item)
				if ref == "" || pin == "" || seen[ref+"."+pin] {
					return
				}
				seen[ref+"."+pin] = true
				n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: pin, Prov: prov()})
			})
		})
		if len(n.Connections) > 0 {
			d.Nets = append(d.Nets, n)
		}
	})

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

	for i, l := range p.copperLayers() {
		fn := ir.LayerFunction_LAYER_FUNCTION_SIGNAL
		if l.plane {
			fn = ir.LayerFunction_LAYER_FUNCTION_PLANE
		}
		layer := &ir.Layer{Name: l.name, Index: int32(i), Function: fn, Attributes: map[string]string{}, Prov: prov()} // #nosec G115 -- a board has at most 256 layers, the width of a subclass byte
		putAttr(layer.Attributes, "allegro_layer", l.source)
		d.Layers = append(d.Layers, layer)
	}
	return d
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
