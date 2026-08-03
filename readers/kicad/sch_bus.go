package kicad

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// collectBuses records the NAMED buses on one schematic sheet as BusNotModeled diagnostics (WS1-034),
// each carrying its member-signal set so the bus-not-modeled rule can tell a resolved bus (every
// member already a net, via the member labels KiCad requires on the taps) from one whose members are
// unmodeled. Two name sources: a range-bus LABEL (`DATA[7:0]`, whose members expand from the range)
// and a `bus_alias` (an explicit member list). The `bus`/`bus_entry` wire geometry is recognized but
// carries no name of its own — the label/alias is what names the bus — so it is not flagged directly.
// Deduped by bus name within the sheet.
//
// qualify maps each MEMBER name into the same net-name space the sheet's member NETS live in. On a
// sub-sheet instance a member label `DATA0` becomes the net `/amp1/DATA0` (sheetScope.local), so bare
// members would never match and the flag would fire on a fully-tapped hierarchical bus for the wrong
// reason (WS1-034 Phase 2). The hierarchy walk passes `sheetScope.local`; the single-file/root read
// passes nil (identity, root locals stay bare). The bus LABEL stays the raw bus name (it is the
// finding subject and the WS7-042 highlight join key); only members are qualified.
func collectBuses(root *node, src string, qualify func(string) string) []*ir.BusNotModeled {
	if qualify == nil {
		qualify = func(s string) string { return s }
	}
	var out []*ir.BusNotModeled
	seen := map[string]bool{}
	add := func(kind, name string, members []string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		var qmembers []string
		if len(members) > 0 {
			qmembers = make([]string, len(members))
			for i, m := range members {
				qmembers[i] = qualify(m)
			}
		}
		out = append(out, &ir.BusNotModeled{
			Kind:    kind,
			Label:   name,
			Members: qmembers,
			Prov:    &ir.Provenance{SourceFile: src},
		})
	}
	// Range-bus labels (a bus's name is its `[hi:lo]` range syntax). Both local and global labels
	// can name a bus; the members are the range expansion.
	for _, tag := range []string{"label", "global_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			if netgraph.IsBusName(name) {
				add("bus", name, netgraph.ExpandBusName(name))
			}
		}
	}
	// bus_alias: `(bus_alias "NAME" (members "A" "B" ...))` — an explicitly-listed bus.
	for _, a := range root.Children("bus_alias") {
		name := unescapeName(atomOf(a.Arg(1)))
		var members []string
		if mn := a.Child("members"); mn != nil {
			for _, m := range mn.Kids[1:] { // Kids[0] is the "members" head
				members = append(members, unescapeName(m.Text()))
			}
		}
		add("bus_alias", name, members)
	}
	return out
}
