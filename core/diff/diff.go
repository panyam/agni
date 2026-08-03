// Package diff computes a semantic diff between two netlist IR Designs.
// It operates purely on the IR (CONSTRAINTS C1): no I/O, no platform calls, so the same
// diff runs over any format that reads into the IR. Design and rationale are in
// docs/18-semantic-diff.md.
package diff

import (
	"fmt"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ComponentChange records one changed field on a component present in both designs.
type ComponentChange struct {
	RefDes string
	Field  string
	Old    string
	New    string
}

// NetChangeKind classifies a net change, following the ground-truth taxonomy engineers use
// (New / Deleted / rename / connectivity vs attribute-only). Equal nets are not reported.
type NetChangeKind string

const (
	NetNew     NetChangeKind = "new"     // present only in the new design
	NetDeleted NetChangeKind = "deleted" // present only in the old design
	NetRenamed NetChangeKind = "renamed" // same connectivity, different name
	NetHard    NetChangeKind = "hard"    // connectivity (pin membership) changed
	NetSoft    NetChangeKind = "soft"    // attribute-only change; connectivity identical
)

// NetChange is one classified net-level change. Name is the net's name in the design where
// it exists (the new name for a rename); OldName is set only for renames. Added/Removed hold
// the "refdes.pin" connection deltas for a Hard change. OldProv/NewProv locate the net in
// each revision's source (nil on the side where the net does not exist).
type NetChange struct {
	Kind    NetChangeKind
	Name    string
	OldName string
	Added   []string
	Removed []string
	OldProv *ir.Provenance
	NewProv *ir.Provenance
}

// Report is the result of diffing two designs.
type Report struct {
	ComponentsAdded   []string
	ComponentsRemoved []string
	ComponentsChanged []ComponentChange
	Nets              []NetChange
}

// Designs diffs a (old) against b (new).
func Designs(a, b *ir.Design) *Report {
	r := &Report{}
	diffComponents(a, b, r)
	diffNets(a, b, r)
	return r
}

// diffComponents fills the report's component add/remove/change sets by matching
// components on ref_des between the two designs.
func diffComponents(a, b *ir.Design, r *Report) {
	ai := indexComponents(a)
	bi := indexComponents(b)
	for ref := range ai {
		if _, ok := bi[ref]; !ok {
			r.ComponentsRemoved = append(r.ComponentsRemoved, ref)
		}
	}
	for ref, bc := range bi {
		ac, ok := ai[ref]
		if !ok {
			r.ComponentsAdded = append(r.ComponentsAdded, ref)
			continue
		}
		r.ComponentsChanged = append(r.ComponentsChanged, componentFieldChanges(ref, ac, bc)...)
	}
	sort.Strings(r.ComponentsAdded)
	sort.Strings(r.ComponentsRemoved)
	sort.Slice(r.ComponentsChanged, func(i, j int) bool {
		if r.ComponentsChanged[i].RefDes != r.ComponentsChanged[j].RefDes {
			return r.ComponentsChanged[i].RefDes < r.ComponentsChanged[j].RefDes
		}
		return r.ComponentsChanged[i].Field < r.ComponentsChanged[j].Field
	})
}

// componentFieldChanges returns the per-field changes (part references, value, and the
// fabrication/footprint attributes captured in WS1-037) for a component present in both
// designs. Because one ref_des may have several sections, the part reference is compared as
// the SET of "library/part" over all sections, not a single value; this avoids false diffs
// from section ordering (WS2-001 learning). dnp is compared so a populated-vs-not flip is
// visible in a diff (a DNP part is not fabricated, so flipping it changes the built board).
func componentFieldChanges(ref string, a, b *ir.Component) []ComponentChange {
	var out []ComponentChange
	add := func(field, o, n string) {
		if o != n {
			out = append(out, ComponentChange{RefDes: ref, Field: field, Old: o, New: n})
		}
	}
	add("parts", partSet(a), partSet(b))
	add("Value", a.Attributes["Value"], b.Attributes["Value"])
	add("dnp", a.Attributes["dnp"], b.Attributes["dnp"])
	add("Footprint", a.Attributes["Footprint"], b.Attributes["Footprint"])
	return out
}

// partSet returns a component's distinct "library/part" references over all sections, as
// a sorted, comma-joined string suitable for equality comparison.
func partSet(c *ir.Component) string {
	seen := map[string]bool{}
	var refs []string
	for _, s := range c.Sections {
		key := s.LibraryRef + "/" + s.PartRef
		if !seen[key] {
			seen[key] = true
			refs = append(refs, key)
		}
	}
	sort.Strings(refs)
	return strings.Join(refs, ", ")
}

// netInfo is a net indexed for diffing: its connection set, a canonical signature of that
// set (for rename matching), a canonical key of its non-connectivity attributes (for Soft
// vs Hard), and its provenance.
type netInfo struct {
	conns map[string]bool
	sig   string
	meta  string
	prov  *ir.Provenance
}

// diffNets classifies every net change. Nets are matched by name first (Equal / Soft /
// Hard); the leftover deleted and added names are then paired by identical connection
// signature into renames, and whatever remains is New or Deleted.
func diffNets(a, b *ir.Design, r *Report) {
	an := indexNets(a)
	bn := indexNets(b)

	for name, ai := range an {
		bi, ok := bn[name]
		if !ok {
			continue
		}
		switch {
		case ai.sig != bi.sig:
			r.Nets = append(r.Nets, NetChange{
				Kind: NetHard, Name: name,
				Added:   setDiff(bi.conns, ai.conns),
				Removed: setDiff(ai.conns, bi.conns),
				OldProv: ai.prov, NewProv: bi.prov,
			})
		case ai.meta != bi.meta:
			r.Nets = append(r.Nets, NetChange{Kind: NetSoft, Name: name, OldProv: ai.prov, NewProv: bi.prov})
		}
	}

	var deleted, added []string
	for name := range an {
		if _, ok := bn[name]; !ok {
			deleted = append(deleted, name)
		}
	}
	for name := range bn {
		if _, ok := an[name]; !ok {
			added = append(added, name)
		}
	}

	// Rename detection: a deleted net and an added net whose connection signature is
	// identical AND unique on each side are the same net renamed. Requiring uniqueness (and
	// a non-empty signature) avoids mis-pairing unrelated nets that happen to share conns.
	delBySig := uniqueBySig(deleted, an)
	addBySig := uniqueBySig(added, bn)
	renamedDel, renamedAdd := map[string]bool{}, map[string]bool{}
	for sig, dName := range delBySig {
		aName, ok := addBySig[sig]
		if !ok {
			continue
		}
		r.Nets = append(r.Nets, NetChange{
			Kind: NetRenamed, Name: aName, OldName: dName,
			OldProv: an[dName].prov, NewProv: bn[aName].prov,
		})
		renamedDel[dName], renamedAdd[aName] = true, true
	}
	for _, name := range deleted {
		if !renamedDel[name] {
			r.Nets = append(r.Nets, NetChange{Kind: NetDeleted, Name: name, OldProv: an[name].prov})
		}
	}
	for _, name := range added {
		if !renamedAdd[name] {
			r.Nets = append(r.Nets, NetChange{Kind: NetNew, Name: name, NewProv: bn[name].prov})
		}
	}

	sort.Slice(r.Nets, func(i, j int) bool {
		if r.Nets[i].Kind != r.Nets[j].Kind {
			return r.Nets[i].Kind < r.Nets[j].Kind
		}
		return r.Nets[i].Name < r.Nets[j].Name
	})
}

// uniqueBySig maps connection-signature -> net name, keeping only signatures that are
// non-empty and belong to exactly one net in the set (so a rename pairing is unambiguous).
func uniqueBySig(names []string, idx map[string]*netInfo) map[string]string {
	count := map[string]int{}
	for _, n := range names {
		count[idx[n].sig]++
	}
	out := map[string]string{}
	for _, n := range names {
		if s := idx[n].sig; s != "" && count[s] == 1 {
			out[s] = n
		}
	}
	return out
}

// indexComponents maps ref_des -> component for one design. Components are already
// grouped by ref_des in the IR, so this no longer loses sections to last-wins.
func indexComponents(d *ir.Design) map[string]*ir.Component {
	m := make(map[string]*ir.Component, len(d.Components))
	for _, c := range d.Components {
		if c.RefDes != "" {
			m[c.RefDes] = c
		}
	}
	return m
}

// indexNets maps net name -> netInfo for one design.
func indexNets(d *ir.Design) map[string]*netInfo {
	m := make(map[string]*netInfo, len(d.Nets))
	for _, n := range d.Nets {
		conns := make(map[string]bool, len(n.Connections))
		keys := make([]string, 0, len(n.Connections))
		for _, c := range n.Connections {
			k := c.ComponentRef + "." + c.PinRef
			if !conns[k] {
				conns[k] = true
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		m[n.Name] = &netInfo{
			conns: conns,
			sig:   strings.Join(keys, ","),
			meta:  n.NetClass + "\x00" + attrsKey(n.Attributes),
			prov:  n.Prov,
		}
	}
	return m
}

// attrsKey renders an attribute map as a canonical, order-independent string for equality.
func attrsKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s;", k, m[k])
	}
	return b.String()
}

// setDiff returns elements in a but not in b, sorted.
func setDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Render returns a human-readable report. limit caps how many items are listed per
// section; a non-positive limit lists everything. The caller (e.g. the CLI) chooses
// the limit, so the diff package holds no display policy of its own.
func (r *Report) Render(limit int) string {
	var counts [5]int // indexed by kind, see below
	kindIdx := map[NetChangeKind]int{NetNew: 0, NetDeleted: 1, NetRenamed: 2, NetHard: 3, NetSoft: 4}
	for _, nc := range r.Nets {
		counts[kindIdx[nc.Kind]]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Components: +%d  -%d  ~%d\n",
		len(r.ComponentsAdded), len(r.ComponentsRemoved), len(r.ComponentsChanged))
	fmt.Fprintf(&b, "Nets:       new %d  deleted %d  renamed %d  hard %d  soft %d\n",
		counts[0], counts[1], counts[2], counts[3], counts[4])

	section(&b, "Components added", r.ComponentsAdded, limit)
	section(&b, "Components removed", r.ComponentsRemoved, limit)
	if len(r.ComponentsChanged) > 0 {
		fmt.Fprintf(&b, "\nComponents changed (%d):\n", len(r.ComponentsChanged))
		for _, c := range capItems(r.ComponentsChanged, limit) {
			fmt.Fprintf(&b, "  %s %s: %q -> %q\n", c.RefDes, c.Field, c.Old, c.New)
		}
	}
	if len(r.Nets) > 0 {
		fmt.Fprintf(&b, "\nNets changed (%d):\n", len(r.Nets))
		for _, nc := range capItems(r.Nets, limit) {
			switch nc.Kind {
			case NetRenamed:
				fmt.Fprintf(&b, "  [renamed] %s -> %s\n", nc.OldName, nc.Name)
			case NetHard:
				fmt.Fprintf(&b, "  [hard]    %s: +%v -%v\n", nc.Name, nc.Added, nc.Removed)
			case NetSoft:
				fmt.Fprintf(&b, "  [soft]    %s: attributes changed\n", nc.Name)
			case NetNew:
				fmt.Fprintf(&b, "  [new]     %s\n", nc.Name)
			case NetDeleted:
				fmt.Fprintf(&b, "  [deleted] %s\n", nc.Name)
			}
		}
	}
	return b.String()
}

// section appends a titled list of items to b, listing at most limit of them
// (limit <= 0 lists all).
func section(b *strings.Builder, title string, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s (%d):\n", title, len(items))
	for _, it := range capItems(items, limit) {
		fmt.Fprintf(b, "  %s\n", it)
	}
	if limit > 0 && len(items) > limit {
		fmt.Fprintf(b, "  ... and %d more\n", len(items)-limit)
	}
}

// capItems returns the first limit items, or all of them when limit <= 0.
func capItems[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
