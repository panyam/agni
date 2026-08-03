package check

import "github.com/panyam/agni/internal/netgraph"

// LocateReason codes (WS9-039): why the entity named by (kind, subject) may not be highlightable
// in a faithful view. The empty string is "a normal entity, expected to highlight". These are
// netlist facts, so they are layout-independent — the UI pairs the reason with an actual on-screen
// resolution check before showing it, so a rail that DOES highlight on an auto-layout never
// surfaces a note. The values are the wire codes the webapi LocateReason enum mirrors.
const (
	LocatableNormal   = ""               // a normal entity; no explanation
	LocateVirtual     = "virtual_symbol" // a `#`-ref power port / flag (#PWR/#FLG), not a placed part
	LocatePowerRail   = "power_rail"     // a power/ground rail distributed via taps, no drawn wire
	LocateNotInDesign = "not_in_design"  // the ref/net is absent from the loaded netlist
)

// LocateReason classifies why the entity named by (kind, subject) may not highlight, from the
// design Model's indexed reads (never a raw-proto scan): a `#`-ref virtual power/flag symbol, a
// power/ground rail (no drawn wire on a faithful schematic), or a ref/net not in the design. It
// returns LocatableNormal ("") for an entity that should highlight. kind is check.KindComponent or
// check.KindNet; any other kind is LocatableNormal. Over-classifying a rail is safe because the
// caller only shows the reason when a highlight actually resolves nothing.
func LocateReason(m Model, kind, subject string) string {
	switch kind {
	case KindComponent:
		if IsVirtualRef(subject) {
			return LocateVirtual
		}
		if !m.HasComponent(subject) {
			return LocateNotInDesign
		}
	case KindNet:
		if !m.HasNetName(subject) {
			return LocateNotInDesign
		}
		if m.IsPowerRail(subject) {
			return LocatePowerRail
		}
	}
	return LocatableNormal
}

// HasComponent reports whether ref_des is known to the design — a listed component (every listed
// ref has a classSet entry, even an empty one for an unclassified part) or a connection ref. The
// connection fallback matters because component-on-net derives refs from connectivity, and some
// netlist reads carry connections without a separate component list.
func (m *irModel) HasComponent(refDes string) bool {
	if _, listed := m.classSet[refDes]; listed {
		return true
	}
	return m.connected[refDes]
}

// IsPowerRail reports whether the named net is a power/ground rail (WS9-039): asserted-driven or
// global, or a ground / power-rail name. Such rails are distributed by power-symbol taps rather
// than drawn wires, so a faithful schematic has nothing to stroke for them. An unknown net is false.
func (m *irModel) IsPowerRail(name string) bool {
	n := m.netByName[name]
	if n == nil {
		return false
	}
	a := n.GetAttributes()
	return a[netgraph.AttrPowerDriven] == "true" || a[netgraph.AttrGlobal] == "true" ||
		netHasRole(n, NetRoleGround, IsGroundName) || netHasRole(n, NetRoleRail, IsPowerRailName)
}
