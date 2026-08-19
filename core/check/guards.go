package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/netgraph"
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
	return citationText(spec, p.GetProv())
}

// PinCitation renders the datasheet citation for a PIN's declaration, the same way Citation does for
// a parameter's. A pin function is an extracted claim like any other and param.Validate requires
// provenance on it, so a pin-level fact is as verifiable against a page as a value-level one. The
// two share a formatter rather than one wrapping the other, because a Pin is not a Parameter and
// nothing about the citation depends on which of them carried the provenance.
func PinCitation(spec *parampb.PartSpec, pin *parampb.Pin) string {
	return citationText(spec, pin.GetProv())
}

// RelationCitation renders the datasheet citation for a PIN RELATION, the third carrier of
// ParamProvenance alongside a parameter and a pin. Same reasoning as PinCitation: a relation is an
// extracted claim, param.Validate requires provenance on it, and nothing about the citation depends
// on which kind of row carried the provenance.
func RelationCitation(spec *parampb.PartSpec, rel *parampb.PinRelation) string {
	return citationText(spec, rel.GetProv())
}

// citationText formats one ParamProvenance against its spec. An unresolvable doc_ref renders as
// "unknown source" rather than an empty pair of quotes, so a citation is never silently blank.
//
// A document with no recorded identity names the PART and says so, rather than borrowing the part
// name and presenting it as the document's. The distinction is the point of agni issue 290: page
// numbers move between revisions, so "page 4" is an instruction to look somewhere that may not exist
// in the copy the reader holds, and a citation that cannot name the revision should say which
// question it cannot answer instead of reading like a complete answer.
func citationText(spec *parampb.PartSpec, prov *parampb.ParamProvenance) string {
	where := documentRef(spec, prov.GetDocRef())
	return fmt.Sprintf("%s page %d, %q (%s, confidence %g)",
		where, prov.GetPage(), prov.GetTableOrFigure(), prov.GetMethod(), prov.GetConfidence())
}

// documentRef names the document a citation points at, degrading in named steps rather than to a
// blank: the printed identity when the corpus has it, else the part with the gap stated, else an
// unresolvable reference.
func documentRef(spec *parampb.PartSpec, docRef string) string {
	if title := DocTitle(spec, docRef); title != "" {
		return fmt.Sprintf("datasheet %q", title)
	}
	if mpn := spec.GetMpn(); mpn != "" {
		return fmt.Sprintf("datasheet for %s (revision unrecorded)", mpn)
	}
	return `datasheet "unknown source"`
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
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		for _, c := range rn.Connections {
			if spec := m.PartSpec(c.ComponentRef); spec != nil && len(EsdRatingLimits(spec)) > 0 {
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
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
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
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassTVS)
		}) {
			return true
		}
	}
	return false
}

// ExternalSignalNet reports the scope the ESD rules share: a connector-facing SIGNAL net that is not
// a rail or ground (by name or by fact), not a deliberately unconnected pad, and not on a power path
// (input-protection's turf, WS3-011). esd-protection and esd-clamp-not-tvs partition these nets by
// what protects them (nothing / a Zener clamp).
//
// It lives here rather than beside either rule because it is now a THIRD consumer's vocabulary too:
// the external_signal_net query relation projects it, so a datalog-authored ESD check scopes itself
// exactly as the Go rules do instead of reassembling six guards by hand and getting one wrong
// (WS3-061). Its guards read net ATTRIBUTES that have no relation of their own, which is why the
// scope could not simply be composed in datalog the way the protection predicates now are.
func ExternalSignalNet(m Model, n *ir.Net) bool {
	a := n.Attributes
	if a[netgraph.AttrExternal] == "true" || a[netgraph.AttrGlobal] == "true" ||
		a[netgraph.AttrPowerDriven] == "true" || m.IsGroundNet(n) || m.IsRailNet(n) {
		return false
	}
	if IntentionallyUnconnected(m, n) {
		return false
	}
	hasConn := Exists(n.Connections, func(c *ir.Connection) bool {
		return m.HasClass(c.ComponentRef, ClassConnector)
	})
	if !hasConn {
		return false
	}
	return !PowerPinReachable(m, n)
}

// NetBias reports which way a net is held by a bias resistor: toward a rail (up), toward ground
// (down), or neither. It is the "is this net pulled" predicate, in one place (WS3-088).
//
// TWO CLAUSES, and the second one is the one that gets forgotten. A bias resistor commonly sits
// directly between the net and its rail, but it may also reach the rail through further passives, and
// a rule that only checked the direct form silently misses those. profiles.pullupRule learned this the
// hard way (WS3-108): its reaches-only form could not see a wide rail at all, so a direct clause had
// to be added beside it. Both live here now so a third consumer inherits both rather than
// reimplementing one.
//
// Both directions can hold at once on a divider, which sets an intermediate level rather than holding
// either rail — so that reports NEITHER, and a caller asking "is this held asserted" gets the honest
// answer instead of a coin flip.
// NetBiasResistors returns the ref-designators of the resistors that bias n toward a rail or ground,
// sorted, alongside the same direction NetBias reports. It is NetBias with the evidence kept rather
// than collapsed, for a caller that has to say something ABOUT the resistor (its value) instead of
// only about the net.
//
// The refs are returned even when the direction is neither, which happens on a divider: both pull
// resistors are real and identifiable, and only the net's resulting LEVEL is ambiguous. A caller
// checking resistance still has something to check; a caller asking which way the net is held gets
// the same honest "neither" NetBias gives.
func NetBiasResistors(m Model, n *ir.Net) (refs []string, up, down bool) {
	up, down = netBias(m, n, &refs)
	sort.Strings(refs)
	return refs, up, down
}

// NetBias reports which way a net is held by a bias resistor: toward a rail (up), toward ground
// (down), or neither. See netBias for the walk; NetBiasResistors is the form that also names the
// resistors.
func NetBias(m Model, n *ir.Net) (up, down bool) { return netBias(m, n, nil) }

// netBias is the shared walk. found, when non-nil, collects the biasing resistors' refs.
func netBias(m Model, n *ir.Net, found *[]string) (up, down bool) {
	if isSupplyNet(m, n) {
		return false, false
	}
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if !m.HasClass(ref, ClassResistor) {
			continue
		}
		for _, far := range m.Nets() {
			if far.GetName() == n.GetName() || !connects(far, ref) {
				continue
			}
			u, d := railOrGround(m, far)
			// The far side may reach its rail through more passives, so walk from it too.
			if !u && !d {
				for _, hop := range m.Reach(far, ProtectionReachHops).Nets {
					if hu, hd := railOrGround(m, hop); hu || hd {
						u, d = u || hu, d || hd
					}
				}
			}
			if (u || d) && found != nil {
				*found = append(*found, ref)
			}
			up, down = up || u, down || d
		}
	}
	if up && down {
		return false, false // a divider holds neither rail; its resistors are still named in found
	}
	return up, down
}

// railOrGround classifies a net as a supply rail or a ground, the two things a bias can pull toward.
func railOrGround(m Model, n *ir.Net) (rail, ground bool) {
	if m.IsGroundNet(n) {
		return false, true
	}
	return m.IsPowerRail(n.GetName()), false
}

// ACCoupled reports whether a SERIES capacitor carries the net — the structural signature of AC
// coupling (WS3-088).
//
// The whole check is telling a coupling cap from a decoupling one, since both are "a capacitor on the
// net". The difference is the far side: a decoupling cap returns to ground or a rail and the signal
// does not pass through it, while a coupling cap's far side is another signal and the signal does.
func ACCoupled(m Model, n *ir.Net) bool {
	if isSupplyNet(m, n) {
		return false
	}
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if !m.HasClass(ref, ClassCapacitor) {
			continue
		}
		for _, far := range m.Nets() {
			if far.GetName() == n.GetName() || !connects(far, ref) {
				continue
			}
			if rail, gnd := railOrGround(m, far); !rail && !gnd {
				return true
			}
		}
	}
	return false
}

// isSupplyNet reports whether n is itself a rail or a ground, in which case neither derived property
// is meaningful and both predicates decline to answer.
//
// Found by running the relations on a real board rather than by a fixture. A rail read as "biased
// high" because a pull-up resistor connects it to the signal it pulls, and GROUND read as AC-coupled
// because a crystal load capacitor puts a signal on its far side. Both are the predicate answered from
// the wrong end: a rail is not held at a level, it IS the level, and ground is not a coupled signal.
// Excluding the far side was never enough — the SUBJECT has to be a signal too.
func isSupplyNet(m Model, n *ir.Net) bool {
	return m.IsGroundNet(n) || m.IsPowerRail(n.GetName())
}

// connects reports whether refDes has a connection on n.
func connects(n *ir.Net, refDes string) bool {
	return Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
}

// UnprotectedPowerReach walks the connector net's series neighborhood (WS3-011) and
// reports whether SOME reached net carries a real power-input pin with neither a fuse
// crossed on the way there nor a TVS on any net along that path. The per-target path
// check matters: a board can have a protected 5V path and an unprotected 3V3 path off
// one connector, and protection on one must not excuse the other.
func UnprotectedPowerReach(m Model, n *ir.Net) bool {
	r := m.Reach(n, PowerPathReachHops)
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
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassZener)
		}) {
			return true
		}
	}
	return false
}

// PullUpReachHops bounds the walk from an I2C bus to the rail its pull-up returns it to.
//
// The number is ELECTRICAL, not a search budget, like the other hop constants in reach.go. A pull-up
// sitting directly on the bus is one crossing. A bus segment separated from its pull-up by a series
// isolation or termination resistor is two, which is an ordinary topology and the reason this rule
// cannot ask at one. Two series elements between a bus and its rail is three and already unusual.
// Past that the accumulated series resistance is comparable to the pull-up itself, so the node no
// longer returns high in the time the bus needs and there is nothing left worth crediting: a pull-up
// four resistors away does not pull this bus up. Widening it would start passing buses that are
// electrically unheld, which is the silent direction.
const PullUpReachHops = 3

// PullUpReachesRail reports whether a rail is reachable from n by crossing RESISTORS only, within
// PullUpReachHops, without passing through ground.
//
// WHY THIS DOES NOT USE Reach. The shared series walk drops any bus-like net outright
// (`if IsBusLike(m, o) { continue }`), and IsBusLike counts a net with more than maxWalkFan
// connections. A supply rail on a real board is exactly that, so Reach cannot arrive at the thing a
// pull-up terminates on. The exclusion is right for the protection rules, which walk THROUGH series
// paths and must not leak into a rail; it is wrong for a question whose whole answer is "did we land
// on a rail". Rather than widen a shared predicate for one caller, this walk keeps the rail as a
// legal DESTINATION and still refuses to continue through one.
//
// Resistors only. A ferrite or a fuse is a legitimate series element elsewhere, but a pull-up is a
// resistor to a rail by definition, and crediting a bead would start passing buses on the strength
// of a filter.
//
// Ground is never crossed. A resistor to ground is a pull-DOWN, and counting it would pass exactly
// the bus this rule exists to catch. Ground is also a plane, so traversing it would make everything
// reachable from everything.
func PullUpReachesRail(m Model, n *ir.Net) bool {
	if n == nil {
		return false
	}
	resistorNets := resistorNetIndex(m)

	type step struct {
		net   *ir.Net
		depth int
		used  map[string]bool
	}
	seen := map[string]bool{n.Name: true}
	queue := []step{{net: n, depth: 0, used: map[string]bool{}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= PullUpReachHops {
			continue
		}
		for _, c := range cur.net.Connections {
			ref := c.ComponentRef
			// A resistor may appear once per PATH, not once per walk: two bus segments can
			// legitimately reach the same rail through different resistors.
			if cur.used[ref] || !m.HasClass(ref, ClassResistor) {
				continue
			}
			for _, other := range resistorNets[ref] {
				if other.Name == cur.net.Name || m.IsGroundNet(other) {
					continue
				}
				if m.IsRailNet(other) {
					return true
				}
				if seen[other.Name] {
					continue
				}
				seen[other.Name] = true
				used := map[string]bool{ref: true}
				for k := range cur.used {
					used[k] = true
				}
				queue = append(queue, step{net: other, depth: cur.depth + 1, used: used})
			}
		}
	}
	return false
}

// resistorNetIndex maps a resistor's ref-des to the distinct nets it touches.
//
// Built per call rather than read off the model: the model's equivalent index is unexported and
// covers every pass class, and a rule-local build is one pass over the nets. The cost is bounded by
// the number of I2C nets on the design, which is single digits to low tens on a real board.
func resistorNetIndex(m Model) map[string][]*ir.Net {
	idx := map[string][]*ir.Net{}
	for _, n := range m.Nets() {
		for _, c := range n.Connections {
			ref := c.ComponentRef
			if !m.HasClass(ref, ClassResistor) {
				continue
			}
			if slicesContainsNet(idx[ref], n.Name) {
				continue
			}
			idx[ref] = append(idx[ref], n)
		}
	}
	return idx
}

func slicesContainsNet(ns []*ir.Net, name string) bool {
	for _, n := range ns {
		if n.Name == name {
			return true
		}
	}
	return false
}
