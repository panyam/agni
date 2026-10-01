package netgraph

import (
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Net fact attributes (ir.Net.attributes keys) emitted and resolved here. They are FACTS the check
// layer reads, kept in the open attributes map rather than typed Net fields because C9 admits a
// typed field only once a second format populates it (WS3-004 designs the typed fact base). The
// constants keep producers and consumers typo-safe without closing the schema.
const (
	// AttrPowerDriven marks a net asserted as fed (a PWR_FLAG or equivalent directive).
	AttrPowerDriven = "power_driven"
	// AttrExternal marks a net that continues into something the read did NOT cover, a by-name
	// connection (global label, power symbol) whose other ends may live in unread files. It is
	// about READ SCOPE, not sheet membership, so a net spanning ten sheets of a completely-read
	// design carries no marking.
	AttrExternal = "external"
	// AttrGlobal marks a named rail on a completely-read design (AttrExternal, resolved).
	AttrGlobal = "global"
	// AttrAliases carries every distinct label a net arrived with, when there is more than one,
	// as "rank:name" entries joined by the US separator (names contain commas and slashes, and
	// \x1f does not occur in EDA names). Rank is the label's Anchor.Rank, which for KiCad is 0
	// for design-wide (global label, power rail) and 1 for sheet-scoped. The naming pass
	// collapses aliases to one Net.Name, and the conflict rules (duplicate labels, rival rail
	// taps) read this to see what was collapsed. Parse with ParseAliases.
	AttrAliases = "aliases"
	// AttrSheets lists the sheet instance ids a net touches, in sheet order, joined by the US
	// separator (a KiCad Sheetname may contain a comma). It is the one membership fact the
	// solver stamps (WS9-028), because a wireless single-pin net on a sub-sheet has no wire
	// geometry to join, so the finding-panel sheet badge has no other source. Only a
	// multi-sheet read populates it, from the hierarchy walk's sheet bands. Parse with
	// ParseSheets.
	AttrSheets = "sheets"
)

// aliasSep separates AttrAliases entries; aliasRankSep splits rank from name (first cut
// only: names may contain colons). sheetSep separates AttrSheets entries (same US byte).
const (
	aliasSep     = "\x1f"
	aliasRankSep = ":"
	sheetSep     = "\x1f"
)

// ParseSheets decodes an AttrSheets value into its ordered sheet ids. Empty or absent decodes
// to nil, as for any single-sheet or non-hierarchical read.
func ParseSheets(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, sheetSep)
}

// EncodeSheets joins sheet ids into an AttrSheets value, empty for none (the caller then stamps
// nothing). The hierarchy reader uses it so it writes the encoding ParseSheets reads.
func EncodeSheets(ids []string) string {
	return strings.Join(ids, sheetSep)
}

// ParseAliases decodes an AttrAliases value back into the solver's alias list. Empty or
// absent values decode to nil (single-named nets carry no attribute).
func ParseAliases(v string) []Alias {
	if v == "" {
		return nil
	}
	var out []Alias
	for _, e := range strings.Split(v, aliasSep) {
		rank, name, ok := strings.Cut(e, aliasRankSep)
		if !ok {
			continue
		}
		r, err := strconv.Atoi(rank)
		if err != nil {
			continue
		}
		out = append(out, Alias{Name: name, Rank: r})
	}
	return out
}

func encodeAliases(as []Alias) string {
	parts := make([]string, len(as))
	for i, a := range as {
		parts[i] = strconv.Itoa(a.Rank) + aliasRankSep + a.Name
	}
	return strings.Join(parts, aliasSep)
}

// StampNetIDs fills ir.Net.id (WS9) for every net that lacks one, hashing its connection set the
// same way the solver does (hashPairs). A reader that builds ir.Net directly, as EDIF does, gets
// the same per-instance identity a netgraph-based reader gets from IRNets. It is idempotent and
// leaves a set id alone. A pinless net keeps its empty id.
func StampNetIDs(d *ir.Design) {
	for _, n := range d.GetNets() {
		if n.GetId() != "" {
			continue
		}
		pairs := make([][2]string, 0, len(n.GetConnections()))
		for _, c := range n.GetConnections() {
			pairs = append(pairs, [2]string{c.GetComponentRef(), c.GetPinRef()})
		}
		n.Id = hashPairs(pairs)
	}
}

// IRNets maps assembled nets onto the IR wire form, the one place a solver Net becomes an ir.Net.
// Driven, External and multiple Aliases surface as the power_driven, external and aliases
// attributes the check rules read, and a net with none of them carries no attributes. Nets with no
// connections are kept, since a labeled wire whose pins did not resolve still names a net. A reader
// whose source tool omits pinless nets filters before calling.
func IRNets(nets []Net, src string) []*ir.Net {
	var out []*ir.Net
	for _, n := range nets {
		net := &ir.Net{Name: n.Name, Id: n.ID, Prov: &ir.Provenance{SourceFile: src}}
		if n.Driven || n.External || len(n.Aliases) > 1 {
			net.Attributes = map[string]string{}
			if n.Driven {
				net.Attributes[AttrPowerDriven] = "true"
			}
			if n.External {
				net.Attributes[AttrExternal] = "true"
			}
			if len(n.Aliases) > 1 {
				net.Attributes[AttrAliases] = encodeAliases(n.Aliases)
			}
		}
		for _, c := range n.Conns {
			conn := &ir.Connection{ComponentRef: c.Comp, PinRef: c.Pin}
			if c.Dir != "" {
				conn.Attributes = map[string]string{"direction": c.Dir}
			}
			net.Connections = append(net.Connections, conn)
		}
		out = append(out, net)
	}
	return out
}

// ResolveExternal downgrades external to global on every net of a completely-read design
// (WS1-017). Once a read covered the whole design (a KiCad .kicad_pro whose root has no unread
// sub-sheets), external is stale and would guard rules such as decoupling-present off the rails
// they exist for, while global keeps "is a named rail" queryable. Only the completeness judgment
// stays in each reader, since only the reader knows what its source references versus what it
// opened. See docsite/content/architecture/net-solving.md#the-hierarchy-walk.
func ResolveExternal(d *ir.Design) {
	for _, n := range d.Nets {
		if n.Attributes[AttrExternal] == "true" {
			delete(n.Attributes, AttrExternal)
			n.Attributes[AttrGlobal] = "true"
		}
	}
}

// IRDangles maps the solver's dangling endpoints onto the IR diagnostic. idKind names the
// wire id's format (e.g. "kicad-uuid"); it and the id are omitted for formats with no
// per-wire id.
func IRDangles(ds []Dangle, src, idKind string) []*ir.DanglingEndpoint {
	var out []*ir.DanglingEndpoint
	for _, d := range ds {
		prov := &ir.Provenance{SourceFile: src}
		if d.WireId != "" {
			prov.NativeId, prov.NativeIdKind = d.WireId, idKind
		}
		out = append(out, &ir.DanglingEndpoint{X: d.At.X, Y: d.At.Y, Prov: prov})
	}
	return out
}
