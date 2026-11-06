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
	defOf := p.compDefs()
	p.list(listFootDefs, func(def int) {
		fp := p.str(p.u32(def, 8))
		defKey := p.u32(def, 4)
		p.chain(p.u32(def, 36), defKey, next8, func(inst int) {
			if p.b[inst] != tagFootInst {
				return
			}
			comp, ok := p.at(p.u32(inst, p.l.fiInstRef), tagCompInst)
			if !ok {
				return
			}
			ref := p.str(p.u32(comp, p.l.compRef))
			if ref == "" || refdes.IsPlaceholder(ref) {
				return
			}
			cdef, ok := defOf[p.u32(comp, 4)]
			if !ok {
				cdef = -1
			}
			out = append(out, placed{inst: inst, comp: comp, def: cdef, ref: ref, fpName: fp})
		})
	})
	return out
}

func next8(byte) int { return 8 }

// compDefs maps each component instance's (0x07) key to its definition (0x06). A 17.2 instance
// names its definition. A 16.6 instance does not, so each definition's chain of instances is
// followed instead.
func (p *board) compDefs() map[uint32]int {
	out := map[uint32]int{}
	if p.l.compDef != 0 {
		for _, off := range p.keys {
			if p.b[off] != tagCompInst {
				continue
			}
			if def, ok := p.at(p.u32(off, p.l.compDef), tagCompDef); ok {
				out[p.u32(off, 4)] = def
			}
		}
		return out
	}
	for _, def := range p.keys {
		if p.b[def] != tagCompDef {
			continue
		}
		p.chain(p.u32(def, 20), p.u32(def, 4), next8, func(inst int) {
			if p.b[inst] == tagCompInst {
				out[p.u32(inst, 4)] = def
			}
		})
	}
	return out
}

// fieldText is the text of the 0x03 field with the given code on the chain starting at head, or "".
func (p *board) fieldText(head, owner uint32, code uint16) string {
	text := ""
	p.chain(head, owner, next8, func(off int) {
		if text != "" || p.b[off] != tagField || le16(p.b, off+2) != code || p.b[off+p.l.fieldSub] != 0x68 {
			return
		}
		size := int(le16(p.b, off+p.l.fieldLen))
		body := off + p.l.fieldBody
		if body+size > len(p.b) {
			return
		}
		text = cstring(p.b[body : body+size])
	})
	return text
}

// padNumber is the pin number of a placed pad, from the footprint pad it copies.
func (p *board) padNumber(placedPad int) string {
	pad, ok := p.at(p.u32(placedPad, p.l.ppPad), tagPad)
	if !ok {
		return ""
	}
	return p.str(p.u32(pad, 8))
}

// pinName is the functional name of a placed pad's pin, through the pad's pin number (0x08) and that
// number's pin name (0x11), or "" when the part names none (agni issue 1361).
func (p *board) pinName(placedPad int) string {
	num, ok := p.at(p.u32(placedPad, p.l.ppPinNumber), tagPinNumber)
	if !ok {
		return ""
	}
	name, ok := p.at(p.u32(num, p.l.pinNumName), tagPinName)
	if !ok {
		return ""
	}
	return p.str(p.u32(name, 8))
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
		Attributes:   map[string]string{"allegro_format": p.l.release},
		Prov:         prov(),
	}
	putAttr(d.Attributes, "allegro_version", p.version)

	// A part type is a component definition (0x06), named by its device type. Its pins are the
	// placed pads of every placement of it, each with the pin name its pin number carries, so a pin
	// on no net is still a pin of the part.
	partOf := map[int]*ir.PartType{}
	partByName := map[string]*ir.PartType{}
	partPins := map[*ir.PartType]map[string]bool{}
	lib := &ir.PartLibrary{Name: "allegro", Prov: prov()}
	fps := map[string]bool{}
	counts := map[string]int{}
	var refs []string
	byRef := map[uint32]string{} // 0x2D key -> ref-des, for the net walk
	for _, pl := range p.placements() {
		if !fps[pl.fpName] {
			fps[pl.fpName] = true
			d.Footprints = append(d.Footprints, &ir.Footprint{Name: pl.fpName, Prov: prov()})
		}
		if counts[pl.ref] == 0 {
			refs = append(refs, pl.ref)
		}
		counts[pl.ref]++
		byRef[p.u32(pl.inst, 4)] = pl.ref

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
		partRef := pl.fpName
		if pt := partOf[pl.def]; pl.def >= 0 && pt == nil {
			if name := p.str(p.u32(pl.def, 12)); name != "" {
				if pt = partByName[name]; pt == nil {
					pt = &ir.PartType{Name: name, Prov: prov()}
					partByName[name] = pt
					partPins[pt] = map[string]bool{}
					lib.Parts = append(lib.Parts, pt)
				}
				partOf[pl.def] = pt
			}
		}
		if pt := partOf[pl.def]; pt != nil {
			partRef = pt.Name
			p.chain(p.u32(pl.inst, p.l.fiPads), p.u32(pl.inst, 4), func(byte) int { return p.l.ppNextInFp }, func(pp int) {
				num := p.padNumber(pp)
				if p.b[pp] != tagPlacedPad || num == "" || partPins[pt][num] {
					return
				}
				partPins[pt][num] = true
				pt.Pins = append(pt.Pins, &ir.Pin{Name: p.pinName(pp), Designator: num, Prov: prov()})
			})
		}
		sec := &ir.ComponentSection{Index: 0, PartRef: partRef, Attributes: props, Prov: prov()}
		if partRef != pl.fpName {
			sec.LibraryRef = lib.Name
		}
		comp.Sections = []*ir.ComponentSection{sec}
		d.Components = append(d.Components, comp)
	}

	if len(lib.Parts) > 0 {
		d.Libraries = []*ir.PartLibrary{lib}
	}

	sets := p.constraintSets()
	setName := map[uint32]string{}
	for _, s := range sets {
		setName[s.key] = s.name
	}
	d.Constraints = append(d.Constraints, netClassDefs(sets, p.copperLayers())...)

	// A net is built from the placed pads on it, as the ODB++ and KiCad board readers build one. An
	// unnamed net holds an unconnected pin, so it is left out and the pin reads as unconnected.
	p.list(listNets, func(net int) {
		name := p.str(p.u32(net, 12))
		if name == "" {
			return
		}
		n := &ir.Net{Name: name, Prov: prov()}
		if key, ok := p.netConstraintSet(net); ok && setName[key] != "" {
			n.NetClasses = []string{setName[key]}
		}
		seen := map[string]bool{}
		netKey := p.u32(net, 4)
		p.chain(p.u32(net, p.l.netAssign), netKey, next8, func(asg int) {
			if p.b[asg] != tagNetAssign {
				return
			}
			p.chain(p.u32(asg, 16), p.u32(asg, 4), next8, func(item int) {
				if p.b[item] != tagPlacedPad {
					return
				}
				ref := byRef[p.u32(item, p.l.ppParentFp)]
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
