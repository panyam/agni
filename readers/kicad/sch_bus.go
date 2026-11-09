package kicad

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// collectBuses records the NAMED buses on one schematic sheet as BusNotModeled diagnostics (WS1-034).
// Each diagnostic lists its bus's member signals. The bus-not-modeled rule uses that set to tell a
// resolved bus from one whose members are unmodeled. In a resolved bus every member is already a net,
// through the member labels KiCad requires on the taps. A range-bus LABEL names a bus, and its
// members expand from the range. The `bus`/`bus_entry` wire geometry has no name of its own, so it is
// not flagged directly. Buses are deduped by name within the sheet.
//
// A `bus_alias` declaration is not a bus. It is a member list that a group bus uses (`I2C7{I2C}`).
// Its members become nets only as `PREFIX.MEMBER` where a group uses it, never under their bare names.
// When the reader recorded each declaration as a bus, it qualified the members by the declaring sheet
// (`/M.2/TX0+`). No net ever matches such a name, so every alias on the jetson baseboard read as 16
// unmodeled buses (agni issue 939). Group buses are not recorded here at all. Judging one would mean
// checking each PREFIX.MEMBER, and KiCad lets a group leave members unbroken-out.
//
// The label test is isKiCadBusLabel, KiCad's own `DATA[0..7]` and nothing else. xschem's `DATA[7:0]`
// is a plain scalar to KiCad, so it is no bus here, and rangeLabels records it for
// bus-spelling-not-a-bus instead (agni issue 605).
//
// qualify maps each MEMBER name into the net-name space of the sheet's member NETS. On a sub-sheet
// instance a member label `DATA0` becomes the net `/amp1/DATA0` (sheetScope.local), so bare members
// would never match and the flag would fire on a fully-tapped hierarchical bus for the wrong reason
// (WS1-034 Phase 2). The hierarchy walk passes `sheetScope.local`; the single-file/root read passes
// nil (identity, so root locals are left bare). The bus LABEL keeps the raw bus name, because it is
// the finding subject and the WS7-042 highlight join key. Only members are qualified.
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
	// Range-bus labels. Both local and global labels can name a bus, and the members are the range
	// expansion.
	for _, tag := range []string{"label", "global_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			if isKiCadBusLabel(name) {
				add("bus", name, netgraph.ExpandBusName(name))
			}
		}
	}
	return out
}

// isKiCadBusLabel reports whether a label is a bus to KiCad, which reads only its vector spelling
// `PREFIX[first..last]`. netgraph.IsBusName also accepts `[hi:lo]` because xschem and gEDA write it.
// collectBuses reads KiCad labels, and asks this function instead.
func isKiCadBusLabel(name string) bool { return kicadBusVectorRe.MatchString(name) }

// rangeLabels records every label on one sheet spelled with an index range in either dialect, and
// whether KiCad reads it as a bus. A colon-spelled label is one scalar net to KiCad, so a user who
// meant a bus by it has merged its members. bus-spelling-not-a-bus reports that merge. Deduped by
// name within the sheet, as collectBuses is.
func rangeLabels(root *node, src string) []*ir.RangeLabel {
	var out []*ir.RangeLabel
	seen := map[string]bool{}
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			if seen[name] || !netgraph.IsBusName(name) {
				continue
			}
			seen[name] = true
			out = append(out, &ir.RangeLabel{Label: name, ReadsAsBus: isKiCadBusLabel(name), Prov: &ir.Provenance{SourceFile: src}})
		}
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
// Without the table a group bus is not recognizable at all. A label `I2C0{I2C}` names a bus, and the
// alias `I2C` lists its members. KiCad spells a subscript with the SAME shape, so `3V3_{OUT}` and
// `A_{1}` are ordinary scalar labels that no pattern can tell apart from a bus. So the test is
// membership in this table, where a tighter pattern could only guess. The jetson baseboard has over a
// hundred distinct subscript groups (`1`, `CC`, `CLR`, `CS0`) and not one of them names an alias.
//
// This reads ONE file's declarations. An alias resolves project-wide, so the hierarchy walk unions
// every sheet's table through projectBusAliases.
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

// collectGroupBuses records each group bus labelled on one sheet, KiCad's `PREFIX{ALIAS}` or
// `PREFIX{A B C}`, with the nets it carries (agni issue 1435). A member is the net `PREFIX.MEMBER`,
// or `MEMBER` under an empty prefix, as KiCad names the taps. qualify maps it into the sheet's net
// names, as collectBuses qualifies its members. A drawn bus takes the same label as its name
// (drawnBuses), so a reader clicking the bus finds its members here. The list is apart from
// collectBuses's, since a group bus is no gap in the read (agni issue 939). Deduped by label within
// the sheet.
func collectGroupBuses(root *node, src string, aliases map[string][]string, qualify func(string) string) []*ir.GroupBus {
	if qualify == nil {
		qualify = func(s string) string { return s }
	}
	var out []*ir.GroupBus
	seen := map[string]bool{}
	for _, tag := range []string{"label", "global_label", "hierarchical_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			prefix, members, ok := groupBus(name, aliases)
			if !ok || seen[name] {
				continue
			}
			seen[name] = true
			g := &ir.GroupBus{Label: name, Prov: &ir.Provenance{SourceFile: src}}
			for _, m := range members {
				if prefix != "" {
					m = prefix + "." + m
				}
				g.Members = append(g.Members, qualify(m))
			}
			out = append(out, g)
		}
	}
	return out
}

// carriedBy renames each group-bus member to the net that carries it. A member broken out on one
// sheet is often named by a label on another, since KiCad names a net for its strongest driver, so
// `/USB Debug, PD/UART3_DEBUG.TX` is carried by the net `/SoM_IO/UART3_DEBUG.TX`. A net keeps every
// name it was reached by (Net.Aliases), and a member found among exactly one net's names takes that
// net's name. A member no net carries keeps its own, since KiCad lets a group leave a member
// unbroken-out, and a reader then sees it named and unconnected.
func carriedBy(groups []*ir.GroupBus, nets []netgraph.Net) {
	byAlias := map[string]string{}
	shared := map[string]bool{}
	for _, n := range nets {
		for _, a := range n.Aliases {
			if prev, ok := byAlias[a.Name]; ok && prev != n.Name {
				shared[a.Name] = true
			}
			byAlias[a.Name] = n.Name
		}
	}
	for _, g := range groups {
		for i, m := range g.Members {
			if name, ok := byAlias[m]; ok && !shared[m] {
				g.Members[i] = name
			}
		}
	}
}
