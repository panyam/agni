package diff

import (
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Tally counts one kind of entity in a design's two views. Matched is how many both views hold,
// Schematic how many the schematic holds, and Board how many the board holds.
type Tally struct {
	Matched, Schematic, Board int
}

// ViewReport says how well a design's schematic and its board agree (agni issue 1590). Report is
// the diff with the schematic as the old side and the board as the new one:
//   - ComponentsRemoved lists the parts only the schematic places, and ComponentsAdded the
//     footprints only the board places.
//   - A deleted net is only on the schematic, and a new net only on the board.
//   - A renamed net has the same pins under another name. OldName is the schematic's name and Name
//     the board's.
//   - A hard net has the same name on both, with different pins.
//
// ComponentsChanged is always empty, since the board holds no part fields to compare.
//
// Pins are compared on the parts both views place, as "refdes.pin" endpoints on a net. A part only
// one view places is already listed, and listing each of its pins too would bury the rest.
type ViewReport struct {
	Report            *Report
	Parts, Pins, Nets Tally
	// SchematicOnlyPins are pins on a net in the schematic whose pad joins no net on the board, or
	// is missing. BoardOnlyPins are the reverse. Both are sorted.
	SchematicOnlyPins, BoardOnlyPins []string
}

// Views compares a design's netlist with its board's footprints and pad nets, by the same
// partition comparison that Designs runs on two revisions. Parts match by ref-des. Nets match by
// name, then by identical pins when the names differ. Nothing is fuzzy, and the near-match pass
// does not run. A power symbol's pin is left out, as Designs leaves it out, and so is a net that
// joins no pin, since it connects nothing either view could show.
func Views(sch *ir.Design, b *geom.BoardGeometry) *ViewReport {
	s := connected(sch)
	bd := BoardNetlist(b)
	r := Designs(s, bd)
	r.ComponentsChanged = nil
	v := &ViewReport{Report: r}

	sParts, bParts := refSet(s), refSet(bd)
	v.Parts = tally(sParts, bParts)

	sPins, bPins := endpoints(s, sParts, bParts), endpoints(bd, sParts, bParts)
	v.Pins = tally(sPins, bPins)
	v.SchematicOnlyPins = setDiff(sPins, bPins)
	v.BoardOnlyPins = setDiff(bPins, sPins)

	unmatched := 0
	for _, nc := range r.Nets {
		switch nc.Kind {
		case NetDeleted, NetHard:
			unmatched++
		}
	}
	v.Nets = Tally{Matched: len(s.GetNets()) - unmatched, Schematic: len(s.GetNets()), Board: len(bd.GetNets())}
	return v
}

// BoardRenames maps each renamed net's board name to its schematic name. A cross-probe joins a
// board net to the schematic's by name, and this lets it join a net that the two views name
// differently.
func (v *ViewReport) BoardRenames() map[string]string {
	m := map[string]string{}
	for _, nc := range v.Report.Nets {
		if nc.Kind == NetRenamed {
			m[nc.Name] = nc.OldName
		}
	}
	return m
}

// BoardNetlist reads a board as a netlist. Each placement with a ref-des is a component, and each
// pad's net joins that pad's (ref-des, number) as a connection. Copper with no pad on it joins no
// pin, so a net drawn only as copper is left out.
func BoardNetlist(b *geom.BoardGeometry) *ir.Design {
	d := &ir.Design{}
	seen := map[string]bool{}
	nets := map[string]*ir.Net{}
	var order []string
	for _, pl := range b.GetPlacements() {
		ref := pl.GetRefDes()
		if ref == "" {
			continue
		}
		if !seen[ref] {
			seen[ref] = true
			d.Components = append(d.Components, &ir.Component{RefDes: ref})
		}
		for _, pad := range pl.GetPads() {
			name := pad.GetNet()
			if name == "" || pad.GetNumber() == "" {
				continue
			}
			n := nets[name]
			if n == nil {
				n = &ir.Net{Name: name}
				nets[name] = n
				order = append(order, name)
			}
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: pad.GetNumber()})
		}
	}
	sort.Strings(order)
	for _, name := range order {
		d.Nets = append(d.Nets, nets[name])
	}
	return d
}

// connected is d with only the nets that join at least one real pin. It shares d's components and
// nets rather than copying them.
func connected(d *ir.Design) *ir.Design {
	out := &ir.Design{Components: d.GetComponents(), SourceFormat: d.GetSourceFormat()}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			if !strings.HasPrefix(c.GetComponentRef(), "#") {
				out.Nets = append(out.Nets, n)
				break
			}
		}
	}
	return out
}

func refSet(d *ir.Design) map[string]bool {
	m := map[string]bool{}
	for _, c := range d.GetComponents() {
		if c.GetRefDes() != "" {
			m[c.GetRefDes()] = true
		}
	}
	return m
}

// endpoints are d's "refdes.pin" connections on parts both a and b hold.
func endpoints(d *ir.Design, a, b map[string]bool) map[string]bool {
	m := map[string]bool{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			if ref := c.GetComponentRef(); a[ref] && b[ref] {
				m[ref+"."+c.GetPinRef()] = true
			}
		}
	}
	return m
}

func tally(a, b map[string]bool) Tally {
	t := Tally{Schematic: len(a), Board: len(b)}
	for k := range a {
		if b[k] {
			t.Matched++
		}
	}
	return t
}
