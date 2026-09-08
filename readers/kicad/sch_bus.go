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
		add("bus_alias", unescapeName(atomOf(a.Arg(1))), aliasMembers(a))
	}
	return out
}

// aliasMembers reads the member list off one `(bus_alias ...)` node.
func aliasMembers(a *node) []string {
	mn := a.Child("members")
	if mn == nil {
		return nil
	}
	var out []string
	for _, m := range mn.Kids[1:] { // Kids[0] is the "members" head
		out = append(out, unescapeName(m.Text()))
	}
	return out
}

// busAliases reads a sheet's `(bus_alias "NAME" (members "A" "B" ...))` declarations into a table.
//
// The table is what makes a group bus recognizable at all. A label `I2C0{I2C}` names the bus whose
// members the alias `I2C` lists, and the SAME shape is how KiCad spells a subscript, so `3V3_{OUT}`
// and `A_{1}` are ordinary scalar labels no pattern can tell apart from a bus. Membership here is the
// test, and it is exact where a tighter pattern could only guess: the jetson baseboard carries over a
// hundred distinct subscript groups (`1`, `CC`, `CLR`, `CS0`) and not one of them names an alias.
//
// Aliases are per FILE rather than per project, so a sheet that uses one declares it.
func busAliases(root *node) map[string][]string {
	var out map[string][]string
	for _, a := range root.Children("bus_alias") {
		name := unescapeName(atomOf(a.Arg(1)))
		if name == "" {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		out[name] = aliasMembers(a)
	}
	return out
}
