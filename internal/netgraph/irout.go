package netgraph

import (
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Net fact attributes (ir.Net.attributes keys) emitted and resolved here. They are FACTS
// (annotations the check layer reads), kept in the open attributes map rather than typed
// Net fields: C9 admits a typed semantic field only when a second format populates it, and
// the fact vocabulary is deliberately open (it grows with analysis layers; WS3-004 designs
// the typed fact-base properly). The constants make producers and consumers typo-safe
// without closing the schema.
const (
	// AttrPowerDriven marks a net asserted as fed (a PWR_FLAG or equivalent directive).
	AttrPowerDriven = "power_driven"
	// AttrExternal marks a net that continues into something the read did NOT cover: a
	// by-name connection mechanism (global label, power symbol) whose other ends may live
	// in unread files. It is about READ SCOPE, not sheet membership: a net spanning ten
	// sheets of a completely-read design carries no marking ("which sheets does this net
	// touch" is derivable topology, not a fact to stamp).
	AttrExternal = "external"
	// AttrGlobal marks a named rail on a completely-read design (AttrExternal, resolved).
	AttrGlobal = "global"
	// AttrAliases carries every distinct label a net arrived with, when there is more than
	// one: "rank:name" entries joined by the US separator (names contain commas and
	// slashes; \x1f does not occur in EDA names). Rank is the label's Anchor.Rank — for
	// KiCad, 0 = design-wide (global label / power rail), 1 = sheet-scoped. The naming
	// pass collapses aliases to one Net.Name; the conflict rules (duplicate labels, rival
	// rail taps) read this to see what was collapsed. Parse with ParseAliases.
	AttrAliases = "aliases"
	// AttrSheets lists the sheet instance ids a net touches, in sheet order, joined by the US
	// separator (a KiCad Sheetname may contain a comma; \x1f does not occur in EDA names).
	// This is the one membership fact the solver DOES stamp, and it is a deliberate exception
	// to AttrExternal's "membership is derivable topology" note (WS9-028): it is NOT cheaply
	// derivable downstream — a wireless single-pin net on a sub-sheet has no wire geometry to
	// join, so the finding-panel badge (which sheet does this net's violation live on?) has no
	// other source. Only a multi-sheet read populates it (the hierarchy walk knows each
	// connection point's sheet band); a single-sheet design carries no marking, so the badge
	// layer, which hides badges below two sheets, sees the same "nothing to stamp" as before.
	// Parse with ParseSheets.
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
// to nil (a single-sheet design, or any non-hierarchical read, carries no attribute).
func ParseSheets(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, sheetSep)
}

// EncodeSheets joins sheet ids into an AttrSheets value; empty in, empty out (the caller then
// stamps nothing). Exported so the hierarchy reader, which owns the band->sheet decode, writes
// the same encoding ParseSheets reads.
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

// IRNets maps assembled nets onto the IR wire form, the one place a solver Net becomes an
// ir.Net (previously each reader carried its own copy, and two of the three dropped the
// solver's flags). Driven/External surface as the power_driven / external attributes the
// check rules read; a reader whose anchors never set those flags emits no attributes, same
// as before. Nets with no connections are kept (a labeled wire whose pins did not resolve
// still names a net); a reader whose source tool omits pinless nets filters before calling.
// StampNetIDs fills ir.Net.id (WS9) for every net that lacks one, hashing its connection set the
// same way the solver does (hashPairs), so a reader that builds ir.Net DIRECTLY — EDIF declares its
// nets, it does not run the geometry solver — gets the same format-neutral per-instance identity a
// netgraph-based reader gets from IRNets. Idempotent: a net whose id is already set (a netgraph
// reader) is left alone, and re-running produces the same ids. A pinless net keeps its empty id.
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

// ResolveExternal downgrades external to global on every net of a completely-read
// design (WS1-017). external, emitted above from the solver's External flag, means
// "this net continues into something we did NOT read"; when a reader knows its read
// covered the whole design (e.g. a KiCad .kicad_pro whose root has no unread sub-sheets),
// the marking is stale and would guard rules (decoupling-present, power-input-not-driven,
// floating-input) off exactly the rails they exist for. The global attribute keeps "is a
// named rail" queryable. The pass is format-neutral and lives here, beside the emission,
// so the attribute's whole lifecycle is one file; only the completeness JUDGMENT stays in
// each reader, because only the reader knows what its source references versus what was
// actually opened.
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
