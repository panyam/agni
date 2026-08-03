package check

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// This file holds the shared guard vocabulary: name heuristics, protection/reach
// walks, and scope/Citation helpers that BOTH the built-in rules and the Spec FFI
// builtins (spec_funcs.go) depend on. It lives in a framework file (not a rule_*.go)
// so it survives the rule-catalog extraction to stdlib (issue #4, phase 2b): the
// checks are content, but this vocabulary is a primitive the framework itself uses.

// Citation renders the datasheet side of a finding's dual provenance as a message string: which
// document revision, page, and table the limit came from, and how it was extracted. The design side
// travels in Finding.Prov; the same facts also travel structured in Finding.DatasheetProv (built via
// DatasheetCitationOf, which this shares), so a renderer can column them instead of parsing this text.
func Citation(spec *parampb.PartSpec, p *parampb.Parameter) string {
	c := DatasheetCitationOf(spec, p)
	doc := c.Doc
	if doc == "" {
		doc = "unknown source"
	}
	return fmt.Sprintf("datasheet %q page %d, %q (%s, confidence %g)",
		doc, c.Page, c.Section, c.Method, c.Confidence)
}

// DiffConventionPresent reports whether the design uses differential-pair naming at all: at
// least one net has its expected complement present (a complete X_P/X_N pair). It is the
// pair-population evidence that gates orphan reporting, so a design with no differential pairs
// stays silent even when names coincidentally carry a _P/_DP/+ suffix.
func DiffConventionPresent(m Model) bool {
	for _, n := range m.Nets() {
		if neg, ok := ExpectedDiffNegative(n.Name); ok && m.HasNetName(neg) {
			return true
		}
	}
	return false
}

// ExpectedDiffNegative returns the complementary negative net name for a differential-pair
// positive member, and ok=false when the name is not a positive member. Suffix families:
// "_P"/"_N", "_DP"/"_DN", and trailing "+"/"-". Matching is case-insensitive; the returned name
// preserves the source casing so the message reads naturally.
func ExpectedDiffNegative(name string) (string, bool) {
	up := strings.ToUpper(name)
	switch {
	case strings.HasSuffix(up, "_DP"), strings.HasSuffix(up, "_P"):
		// Both end in P; flip only the trailing P/p to N/n, preserving case.
		last := name[len(name)-1]
		flip := "N"
		if last == 'p' {
			flip = "n"
		}
		return name[:len(name)-1] + flip, true
	case strings.HasSuffix(name, "+"):
		return name[:len(name)-1] + "-", true
	}
	return "", false
}

// ICESDRated reports whether a component on the net (or within its 2-hop series reach) declares a
// datasheet ESD rating at or above the credit floor — the IC-integrated ESD that protects a
// connector-facing signal without a discrete TVS (WS3-073). Silent without a seeded param set
// (m.PartSpec is nil), so esd behaves exactly as before on a design read with no datasheets.
func ICESDRated(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		for _, c := range rn.Connections {
			if spec := m.PartSpec(c.ComponentRef); spec != nil && len(esdRatingLimits(spec)) > 0 {
				return true
			}
		}
	}
	return false
}

// IntentionallyUnconnected reports whether a net's lack of connections is deliberate: its name
// is a tool no-connect marker, or a connected pin resolves to the NO_CONNECT electrical type.
func IntentionallyUnconnected(m Model, n *ir.Net) bool {
	switch name := strings.ToLower(n.Name); {
	case strings.HasPrefix(name, "unconnected"),
		strings.HasPrefix(name, "no_connect"),
		strings.HasPrefix(name, "nc_"):
		return true
	}
	return Exists(n.Connections, func(c *ir.Connection) bool {
		return m.PinDir(c.ComponentRef, c.PinRef) == ir.PinDirection_PIN_DIRECTION_NO_CONNECT
	})
}

// IsGroundName matches the ground-rail naming conventions (GND and variants, VSS, EARTH). Ground pins
// read as power_in, but decoupling is asserted on the supply side; matching by name keeps the rule from
// double-reporting every cap-less design on its ground net too. The pattern set lives in the active
// naming lexicon (WS3-069), so a project can extend it via --conventions.
func IsGroundName(name string) bool { return classify.ActiveRoleVocab().IsGround(name) }

// IsPowerRailName reports whether a net name follows a supply-rail convention (VCC, VDD, VBUS, VIN,
// +3V3, 12V, ...). It is the rail-identity fallback for sources that carry no pin directions or power
// symbols (an EDIF netlist), where the name is the only evidence; rail nets are input-protection's and
// bulk-cap's concern, not ESD's. The pattern set lives in the active naming lexicon (WS3-069), so a
// project can extend it via --conventions; hierarchical sheet prefixes ("/psu/12V") are stripped first.
func IsPowerRailName(name string) bool { return classify.ActiveRoleVocab().IsRail(name) }

// PowerPinReachable reports a power-direction pin on the net or on any net in its 2-hop
// series reach (WS3-011): the esd/input-protection turf split must not depend on whether
// a bead sits between the connector and the regulator.
func PowerPinReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if CountDir(NetDirs(m, rn), func(d ir.PinDirection) bool {
			return d == ir.PinDirection_PIN_DIRECTION_POWER_IN || d == ir.PinDirection_PIN_DIRECTION_POWER_OUT
		}) >= 1 {
			return true
		}
	}
	return false
}

// ScopeOf splits a KiCad-style qualified name into its sheet scope and leaf: "/amp1/SIG"
// -> ("/amp1", "SIG"), bare root names -> ("", name).
func ScopeOf(name string) (scope, leaf string) {
	if !strings.HasPrefix(name, "/") {
		return "", name
	}
	i := strings.LastIndex(name, "/")
	return name[:i], name[i+1:]
}

// TVSReachable reports a TVS on the net or on any net in its 2-hop series reach
// (WS3-011): ESD structures commonly put a series resistor between the connector and
// the clamped node, which splits the net and hid the clamp from the pre-reach rule.
func TVSReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassTVS)
		}) {
			return true
		}
	}
	return false
}

// UnprotectedPowerReach walks the connector net's series neighborhood (WS3-011) and
// reports whether SOME reached net carries a real power-input pin with neither a fuse
// crossed on the way there nor a TVS on any net along that path. The per-target path
// check matters: a board can have a protected 5V path and an unprotected 3V3 path off
// one connector, and protection on one must not excuse the other.
func UnprotectedPowerReach(m Model, n *ir.Net) bool {
	r := m.Reach(n, 3)
	protectorOn := func(net *ir.Net) bool {
		return Exists(net.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassFuse) || m.HasClass(c.ComponentRef, ClassTVS)
		})
	}
	for _, target := range r.Nets {
		hasPowerIn := Exists(target.Connections, func(c *ir.Connection) bool {
			return !IsVirtualRef(c.ComponentRef) && ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
		})
		if !hasPowerIn {
			continue
		}
		protected := false
		for _, ref := range r.ThroughOnPath(target) {
			if m.HasClass(ref, ClassFuse) {
				protected = true // a fuse sits on this path as a series element
				break
			}
		}
		if !protected {
			// A protector as a MEMBER of a path net also counts — the pre-reach rule's
			// (conservative) reading, kept so no previously-quiet board starts firing.
			for _, pn := range r.PathTo(target) {
				if protectorOn(pn) {
					protected = true
					break
				}
			}
		}
		if !protected {
			return true
		}
	}
	return false
}

// ZenerReachable reports a Zener clamp on the net or on any net in its 2-hop series reach — the
// same reach the TVS check walks (WS3-011), so a series-split clamp is not hidden. A Zener is
// distinct from a TVS (a slower clamp), so esd-protection does not count it as ESD protection;
// esd-clamp-not-tvs (WS3-078) reports its presence separately for the review to weigh.
func ZenerReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassZener)
		}) {
			return true
		}
	}
	return false
}
