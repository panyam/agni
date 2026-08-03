package check

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The Go-eval'd connectivity rules (floating-input, power-input-not-driven, and the
// protection batch) classify a net's pins by electrical role, so the predicates live here
// in one place. Role comes
// from ir.PinDirection via Model.PinDir; a reader that does not type its pins yields UNSPECIFIED,
// so these predicates are false and the rules simply do not fire (absent-tolerant, same posture as
// the rest of check).

// IsDriver reports whether a pin can source a signal onto its net: a plain output, a power source
// (a regulator output or a PWR_FLAG, mapped to POWER_OUT), or a bidirectional pin (it drives when
// enabled). INOUT is a driver for "is this net driven?" but not counted as a hard driver for the
// two-drivers conflict, where a shared bus of INOUT pins is legal.
func IsDriver(d ir.PinDirection) bool {
	switch d {
	case ir.PinDirection_PIN_DIRECTION_OUTPUT,
		ir.PinDirection_PIN_DIRECTION_POWER_OUT,
		ir.PinDirection_PIN_DIRECTION_INOUT:
		return true
	}
	return false
}

// NetDirs resolves the electrical direction of every connection on a net, in connection order.
func NetDirs(m Model, n *ir.Net) []ir.PinDirection {
	out := make([]ir.PinDirection, 0, len(n.Connections))
	for _, c := range n.Connections {
		out = append(out, ConnDir(m, c))
	}
	return out
}

// ConnDir resolves one connection's electrical direction: the connection-level
// "direction" attribute wins (a virtual power-symbol pin, whose component is not in
// Components so no part-type pin exists — WS1-014), then the part-type pin via PinDir.
func ConnDir(m Model, c *ir.Connection) ir.PinDirection {
	switch c.GetAttributes()["direction"] {
	case "power_in":
		return ir.PinDirection_PIN_DIRECTION_POWER_IN
	case "power_out":
		return ir.PinDirection_PIN_DIRECTION_POWER_OUT
	}
	return m.PinDir(c.ComponentRef, c.PinRef)
}

// IsVirtualRef reports whether a connection's component is a virtual connectivity symbol
// (a KiCad #PWR/#FLG), which contributes power evidence but is not a physical part:
// consumer-intent guards (a decoupling rule asking "does a real part draw from this
// rail") must not count it, while driver/ERC semantics (power-input-not-driven, the
// driver conflict) must.
func IsVirtualRef(ref string) bool {
	return strings.HasPrefix(ref, "#")
}

// CountDir returns how many of dirs satisfy pred.
func CountDir(dirs []ir.PinDirection, pred func(ir.PinDirection) bool) int {
	n := 0
	for _, d := range dirs {
		if pred(d) {
			n++
		}
	}
	return n
}

// IsPassiveClass reports whether a component class is a two-terminal passive (plus test
// points): parts whose pins conduct rather than listen or drive, so direction-based rules
// treat them as transparent — some libraries type a passive's pins INPUT (the Mentor EDIF
// corpus does for capacitors), and counting those as logic inputs is a false positive.
func IsPassiveClass(c ComponentClass) bool {
	switch c {
	case ClassResistor, ClassCapacitor, ClassInductor, ClassFerrite, ClassFuse, ClassTestPoint:
		return true
	}
	return false
}

// formatTypesPowerOut reports whether a source format's reader classifies power-OUTPUT pins. EDIF
// carries only INPUT/OUTPUT/INOUT (no power_out) and IPC-2581 is a board format with no pin electrical
// types; every other reader (KiCad, gEDA, xschem) types power outputs. It is the capability
// power-input-not-driven gates on: that rule infers "unpowered" from the absence of a typed driver, and
// on a format that types no power outputs a rail's driver reads as a plain input, so the absence is
// meaningless. Prefix-matched because SourceFormat carries a version ("edif-2.0.0"). WS3-072 PR2 stamps
// the power_IN side; the symmetric power_out stamp (PR3) lifts this gate.
func formatTypesPowerOut(sourceFormat string) bool {
	return !strings.HasPrefix(sourceFormat, "edif") && !strings.HasPrefix(sourceFormat, "ipc")
}

// classifyPinRole derives a pin's role from its declared name, gated by device class for
// the polarity roles. Matching is deliberately exact-token (not substring): pin names are
// short vocabulary words, and "CLKA" must not read as an anode.
func classifyPinRole(name string, class ComponentClass) PinRole {
	u := strings.ToUpper(strings.TrimSpace(name))
	if u == "" || u == "~" {
		return RoleUnknown
	}
	switch class {
	case ClassDiode, ClassLED, ClassTVS, ClassZener:
		switch u {
		case "A", "AN", "ANODE", "+":
			return RoleAnode
		case "K", "KATHODE", "CATHODE", "CAT", "-":
			return RoleCathode
		}
	}
	if IsGroundName(u) {
		return RoleGround
	}
	if IsPowerRailName(u) {
		return RolePower
	}
	return RoleUnknown
}
