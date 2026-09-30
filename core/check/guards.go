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

// This file holds the guard vocabulary that BOTH the built-in rules and the Spec FFI builtins
// (spec_funcs.go) use, so it stays in the framework rather than in a rule_*.go (issue #4): name
// heuristics, protection and reach walks, and the scope and citation helpers.

// Citation renders the datasheet side of a finding's provenance as a message string, naming the
// document revision, page, table and extraction method the limit came from. The design side is
// Finding.Prov. Finding.DatasheetProv carries these same facts structured (via DatasheetCitationOf),
// so a renderer should read that rather than parse this text.
func Citation(spec *parampb.PartSpec, p *parampb.Parameter) string {
	return citationText(spec, p.GetProv())
}

// PinCitation renders the datasheet citation for a PIN's declaration, as Citation does for a
// parameter. param.Validate requires provenance on a pin function too, so a pin-level fact is as
// checkable against a page as a value.
func PinCitation(spec *parampb.PartSpec, pin *parampb.Pin) string {
	return citationText(spec, pin.GetProv())
}

// RelationCitation renders the datasheet citation for a PIN RELATION, the third carrier of
// ParamProvenance after a parameter and a pin.
func RelationCitation(spec *parampb.PartSpec, rel *parampb.PinRelation) string {
	return citationText(spec, rel.GetProv())
}

// citationText formats one ParamProvenance against its spec, naming the document through documentRef.
func citationText(spec *parampb.PartSpec, prov *parampb.ParamProvenance) string {
	where := documentRef(spec, prov.GetDocRef())
	return fmt.Sprintf("%s page %d, %q (%s, confidence %g)",
		where, prov.GetPage(), prov.GetTableOrFigure(), prov.GetMethod(), prov.GetConfidence())
}

// documentRef names the document a citation points at, and never returns a blank. It gives the
// printed title when the corpus has one, else the PART with "revision unrecorded", else "unknown
// source". The missing revision is stated because page numbers move between revisions, so a bare
// "page 4" can point at a page the reader's copy does not have (agni issue 290).
func documentRef(spec *parampb.PartSpec, docRef string) string {
	if title := DocTitle(spec, docRef); title != "" {
		return fmt.Sprintf("datasheet %q", title)
	}
	if mpn := spec.GetMpn(); mpn != "" {
		return fmt.Sprintf("datasheet for %s (revision unrecorded)", mpn)
	}
	return `datasheet "unknown source"`
}

// DiffConventionPresent reports whether the design uses differential-pair naming at all, meaning at
// least one net has its expected complement (a complete X_P/X_N pair). Orphan reporting is gated on
// it, so a design with no differential pairs stays silent even when names happen to end in _P, _DP
// or +.
func DiffConventionPresent(m Model) bool {
	for _, n := range m.Nets() {
		if neg, ok := ExpectedDiffNegative(n.Name); ok && m.HasNetName(neg) {
			return true
		}
	}
	return false
}

// ExpectedDiffNegative returns the complementary negative net name for a differential-pair
// positive member, and ok=false when the name is not a positive member. The suffix families are
// "_P"/"_N", "_DP"/"_DN", and trailing "+"/"-". Matching is case-insensitive, and the returned name
// keeps the source casing.
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

// ICESDRated reports whether a component on the net or within its 2-hop series reach declares a
// datasheet ESD rating at or above the credit floor. That is IC-integrated ESD, which protects a
// connector-facing signal without a discrete TVS (WS3-073). It is false without a seeded param set
// (m.PartSpec returns nil), so esd is unchanged on a design read with no datasheets.
func ICESDRated(m Model, n *ir.Net) bool {
	ref, _ := ICESDCredit(m, n)
	return ref != ""
}

// ICESDCredit is ICESDRated with the EVIDENCE kept, returning the part whose datasheet credits the
// net and a witness stating the rating and where it was read. Deciding and justifying in one call
// means a rule cannot reach a pass and skip the justification. Returns "" and nil when nothing in
// reach carries a rating, which is every design read without --params.
func ICESDCredit(m Model, n *ir.Net) (string, *Witness) {
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		for _, c := range rn.Connections {
			spec := m.PartSpec(c.ComponentRef)
			if spec == nil {
				continue
			}
			limits := EsdRatingLimits(spec)
			if len(limits) == 0 {
				continue
			}
			p := limits[0]
			w := &Witness{
				Statement: fmt.Sprintf("%s declares a system-level ESD rating of %s in its datasheet",
					c.ComponentRef, fmtQty(p.GetValue().GetMax(), "V")),
				Terms: []WitnessTerm{
					{Label: "rated part", Value: c.ComponentRef},
					{Label: "ESD rating", Value: fmtQty(p.GetValue().GetMax(), "V")},
				},
			}
			if cit := DatasheetCitationOf(spec, p); cit != nil {
				w.Datasheet = []*DatasheetCitation{cit}
			}
			return c.ComponentRef, w
		}
	}
	return "", nil
}

// IntentionallyUnconnected reports whether a net's lack of connections is deliberate, meaning its
// name is a tool no-connect marker or a connected pin resolves to the NO_CONNECT electrical type.
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
// read as power_in while decoupling is asserted on the supply side, so matching by name keeps the rule
// from reporting every cap-less design a second time on its ground net. The patterns come from the
// active naming lexicon (WS3-069), so a project can extend them via --conventions.
func IsGroundName(name string) bool { return classify.ActiveRoleVocab().IsGround(name) }

// IsPowerRailName reports whether a net name follows a supply-rail convention (VCC, VDD, VBUS, VIN,
// +3V3, 12V, ...). It identifies rails in sources with no pin directions or power symbols (an EDIF
// netlist), where the name is the only evidence. Rail nets belong to input-protection and bulk-cap,
// not ESD. The patterns come from the active naming lexicon (WS3-069), so a project can extend them
// via --conventions, and hierarchical sheet prefixes ("/psu/12V") are stripped first.
func IsPowerRailName(name string) bool { return classify.ActiveRoleVocab().IsRail(name) }

// PowerPinReachable reports a power-direction pin on the net or on any net in its 2-hop
// series reach (WS3-011), so the split between esd and input-protection does not depend on
// whether a bead sits between the connector and the regulator.
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

// ScopeOf splits a KiCad-style qualified name into its sheet scope and leaf, so "/amp1/SIG"
// gives ("/amp1", "SIG") and a bare root name gives ("", name).
func ScopeOf(name string) (scope, leaf string) {
	if !strings.HasPrefix(name, "/") {
		return "", name
	}
	i := strings.LastIndex(name, "/")
	return name[:i], name[i+1:]
}

// TVSReachable reports a TVS on the net or on any net in its 2-hop series reach
// (WS3-011). ESD structures commonly put a series resistor between the connector and
// the clamped node, which splits the net.
func TVSReachable(m Model, n *ir.Net) bool { return ReachableOfClass(m, n, ClassTVS) != "" }

// ReachableOfClass names the first part of a class on n or within the protection reach, in walk
// order, or "" when there is none. It returns the REF rather than a bool so a witness can name the
// clamp a reviewer should go and look at.
func ReachableOfClass(m Model, n *ir.Net, class ComponentClass) string {
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		for _, c := range rn.Connections {
			if m.HasClass(c.ComponentRef, class) {
				return c.ComponentRef
			}
		}
	}
	return ""
}

// ExternalSignalNet reports the scope the ESD rules share, a connector-facing SIGNAL net that is not
// a rail or ground (by name or by fact), not a deliberately unconnected pad, and not on a power path
// (input-protection's, WS3-011). esd-protection and esd-clamp-not-tvs split these nets by what
// protects them (nothing, or a Zener clamp).
//
// The external_signal_net query relation projects it too, so a datalog ESD check scopes itself
// exactly as the Go rules do (WS3-061). It cannot be composed in datalog because its guards read net
// ATTRIBUTES that have no relation of their own.
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

// NetBiasResistors returns the sorted ref-designators of the resistors that bias n toward a rail or
// ground, with the same direction NetBias reports. Use it when a caller has to say something ABOUT
// the resistor (its value) rather than only about the net. On a divider the refs are still returned
// while the direction is neither, since both resistors are real and only the net's LEVEL is ambiguous.
func NetBiasResistors(m Model, n *ir.Net) (refs []string, up, down bool) {
	up, down = netBias(m, n, &refs)
	sort.Strings(refs)
	return refs, up, down
}

// NetBias reports which way a net is held by a bias resistor, toward a rail (up), toward ground
// (down), or neither (WS3-088). See netBias for the walk, and NetBiasResistors for the form that also
// names the resistors.
func NetBias(m Model, n *ir.Net) (up, down bool) { return netBias(m, n, nil) }

// netBias is the shared walk. found, when non-nil, collects the biasing resistors' refs.
//
// It checks TWO CLAUSES, a resistor directly between the net and its rail and one that reaches the
// rail through further passives. A rule checking only one misses the other, as profiles.pullupRule's
// reaches-only form missed every wide rail (WS3-108). A divider pulls both ways and sets an
// intermediate level, so it reports NEITHER direction.
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

// ACCoupled reports whether a SERIES capacitor carries the net, the structural signature of AC
// coupling (WS3-088). It tells a coupling cap from a decoupling one by the far side. A decoupling
// cap returns to ground or a rail, while a coupling cap's far side is another signal.
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
// is meaningful and both predicates decline to answer. Without it, on a real board, a rail read as
// "biased high" through the pull-up it feeds, and GROUND read as AC-coupled through a crystal load
// cap. Checking the far side is not enough, and the SUBJECT has to be a signal too.
func isSupplyNet(m Model, n *ir.Net) bool {
	return m.IsGroundNet(n) || m.IsPowerRail(n.GetName())
}

// connects reports whether refDes has a connection on n.
func connects(n *ir.Net, refDes string) bool {
	return Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
}

// UnprotectedPowerReach reports whether SOME power input the connector net reaches is unprotected.
// It is the bool the spec FFI and the declarative twin need, and PowerPathProtection is the same
// walk with the evidence kept.
func UnprotectedPowerReach(m Model, n *ir.Net) bool {
	return PowerPathProtection(m, n).Unprotected != ""
}

// PowerPathReport is what the power-entry walk found from one connector net.
//
// Check Loads before calling a net protected. UnprotectedPowerReach is false both for a protected 5V
// entry and for a USB data pair that reaches no supply pin at all. The first is a pass, and the second
// is not a subject of a power-entry rule.
type PowerPathReport struct {
	// Loads counts the reached nets carrying a real power-input pin, which is how many power entries
	// this connector net feeds. Zero means the walk found no power path.
	Loads int
	// Unprotected names the first reached load with neither a fuse crossed on the way nor a
	// protector on any net along that path, "" when every load is protected.
	Unprotected string
	// Protector names the first fuse or TVS credited on a protected path, "" when there is none to
	// credit. It is the pass's evidence, the part a reviewer should go and look at.
	Protector string
}

// PowerPathProtection walks the connector net's series neighborhood (WS3-011) and reports, per
// reached power input, whether a fuse was crossed on the way there or a protector sits on a net
// along that path. The check is per target because one connector can feed a protected 5V path and an
// unprotected 3V3 path, and protection on one must not excuse the other.
func PowerPathProtection(m Model, n *ir.Net) PowerPathReport {
	r := m.Reach(n, PowerPathReachHops)
	protectorOn := func(net *ir.Net) string {
		for _, c := range net.Connections {
			if m.HasClass(c.ComponentRef, ClassFuse) || m.HasClass(c.ComponentRef, ClassTVS) {
				return c.ComponentRef
			}
		}
		return ""
	}
	var out PowerPathReport
	for _, target := range r.Nets {
		hasPowerIn := Exists(target.Connections, func(c *ir.Connection) bool {
			return !IsVirtualRef(c.ComponentRef) && ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
		})
		if !hasPowerIn {
			continue
		}
		out.Loads++
		protector := ""
		for _, ref := range r.ThroughOnPath(target) {
			if m.HasClass(ref, ClassFuse) {
				protector = ref
				break
			}
		}
		if protector == "" {
			// A protector as a MEMBER of a path net also counts. Dropping this would start
			// firing on boards that are quiet today.
			for _, pn := range r.PathTo(target) {
				if p := protectorOn(pn); p != "" {
					protector = p
					break
				}
			}
		}
		switch {
		case protector == "" && out.Unprotected == "":
			out.Unprotected = target.Name
		case protector != "" && out.Protector == "":
			out.Protector = protector
		}
	}
	return out
}

// ZenerReachable reports a Zener clamp on the net or on any net in its 2-hop series reach, the
// same reach TVSReachable walks (WS3-011). A Zener is a slower clamp than a TVS, so esd-protection
// does not count it, and esd-clamp-not-tvs (WS3-078) reports it separately for the review to weigh.
func ZenerReachable(m Model, n *ir.Net) bool { return ReachableOfClass(m, n, ClassZener) != "" }

// PullUpReachHops bounds the walk from an I2C bus to the rail its pull-up returns it to. Like the
// hop constants in reach.go it is ELECTRICAL, not a search budget. A pull-up behind a series
// isolation resistor is an ordinary two hops, and past three the series resistance is comparable to
// the pull-up, so the bus no longer returns high in time. Widening it passes buses that are
// electrically unheld. See docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const PullUpReachHops = 3

// PullUpReachesRail reports whether a rail is reachable from n by crossing RESISTORS only, within
// PullUpReachHops, without passing through ground.
//
// Not Reach, because Reach drops a bus-like net (more than maxWalkFan connections) outright, and a
// supply rail on a real board is one. This walk keeps the rail as a legal DESTINATION and never
// continues through it. ReachToTerminus does that too but crosses every pass class (agni issue 374).
//
// RESISTORS only, since a pull-up is a resistor to a rail, and crediting a bead would pass buses on
// the strength of a filter. Ground is never crossed, since a resistor to ground is a pull-DOWN and
// ground is a plane that would make everything reachable.
//
// It is the boolean projection of PullUpPathToRail, where the walk lives.
func PullUpReachesRail(m Model, n *ir.Net) bool {
	return PullUpPathToRail(m, n) != nil
}

// PullUpHop is one leg of the walk from an I2C net to a rail, the resistor crossed and the net it
// landed on. It names the resistor rather than the pin because a reader locating the pull-up wants
// the part.
type PullUpHop struct {
	Resistor string // ref-des of the resistor crossed
	Net      string // the net reached by crossing it
}

// PullUpPathToRail returns the hops from n to the first rail reachable under PullUpReachesRail's
// rules, and nil when none is. The last hop's Net is the rail.
//
// The path is the proof a passing bus carries, so a reviewer can ask "show me the pull-up" (#387).
func PullUpPathToRail(m Model, n *ir.Net) []PullUpHop {
	if n == nil {
		return nil
	}
	resistorNets := resistorNetIndex(m)

	type step struct {
		net   *ir.Net
		depth int
		used  map[string]bool
		path  []PullUpHop
	}
	seen := map[string]bool{n.Name: true}
	queue := []step{{net: n, depth: 0, used: map[string]bool{}}}
	// extend copies rather than appending in place, because two queued steps can share a prefix and
	// appending to a shared backing array would let one branch overwrite another's path.
	extend := func(path []PullUpHop, h PullUpHop) []PullUpHop {
		out := make([]PullUpHop, len(path), len(path)+1)
		copy(out, path)
		return append(out, h)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= PullUpReachHops {
			continue
		}
		for _, c := range cur.net.Connections {
			ref := c.ComponentRef
			// A resistor may appear once per PATH, not once per walk, since two bus segments can
			// reach the same rail through different resistors.
			if cur.used[ref] || !m.HasClass(ref, ClassResistor) {
				continue
			}
			for _, other := range resistorNets[ref] {
				if other.Name == cur.net.Name || m.IsGroundNet(other) {
					continue
				}
				hop := PullUpHop{Resistor: ref, Net: other.Name}
				if m.IsRailNet(other) {
					return extend(cur.path, hop)
				}
				if seen[other.Name] {
					continue
				}
				seen[other.Name] = true
				used := map[string]bool{ref: true}
				for k := range cur.used {
					used[k] = true
				}
				queue = append(queue, step{
					net: other, depth: cur.depth + 1, used: used, path: extend(cur.path, hop),
				})
			}
		}
	}
	return nil
}

// PullUpTermination is one resistor that takes a net to a rail, with the rail it landed on and the
// hops it took to get there.
type PullUpTermination struct {
	Resistor string      // the resistor whose far side is the rail
	Rail     string      // the rail it landed on
	Path     []PullUpHop // from the subject net to the rail, the last hop crossing Resistor
}

// AllPullUpPathsToRail returns every DISTINCT resistor that takes n to a rail, with the rail each
// landed on. PullUpPathToRail answers whether there is one, and this answers how many, which is where
// a second pull-up shows up (agni issue 516). Two 2.2k in parallel are an effective 1.1k, so the bus
// sinks about twice the current it was sized for, and reachability alone cannot see it.
//
// DISTINCT BY RESISTOR, not by path and not by rail. Two routes through one resistor are one
// pull-up, and one resistor reaching two rails is still one part to remove.
//
// It is PullUpPathToRail's walk without the early return. The `seen` set still prunes revisits of
// INTERMEDIATE nets, which drops a second ROUTE but never a termination, because every net reached
// enumerates ALL resistors whose far side is a rail.
func AllPullUpPathsToRail(m Model, n *ir.Net) []PullUpTermination {
	if n == nil {
		return nil
	}
	resistorNets := resistorNetIndex(m)

	type step struct {
		net   *ir.Net
		depth int
		used  map[string]bool
		path  []PullUpHop
	}
	seen := map[string]bool{n.Name: true}
	queue := []step{{net: n, depth: 0, used: map[string]bool{}}}
	extend := func(path []PullUpHop, h PullUpHop) []PullUpHop {
		out := make([]PullUpHop, len(path), len(path)+1)
		copy(out, path)
		return append(out, h)
	}

	var out []PullUpTermination
	found := map[string]bool{} // resistor ref-des, so a part reached twice counts once
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= PullUpReachHops {
			continue
		}
		for _, c := range cur.net.Connections {
			ref := c.ComponentRef
			if cur.used[ref] || !m.HasClass(ref, ClassResistor) {
				continue
			}
			for _, other := range resistorNets[ref] {
				if other.Name == cur.net.Name || m.IsGroundNet(other) {
					continue
				}
				hop := PullUpHop{Resistor: ref, Net: other.Name}
				if m.IsRailNet(other) {
					if !found[ref] {
						found[ref] = true
						out = append(out, PullUpTermination{Resistor: ref, Rail: other.Name, Path: extend(cur.path, hop)})
					}
					continue // a rail is a destination, never a transit node
				}
				if seen[other.Name] {
					continue
				}
				seen[other.Name] = true
				used := map[string]bool{ref: true}
				for k := range cur.used {
					used[k] = true
				}
				queue = append(queue, step{
					net: other, depth: cur.depth + 1, used: used, path: extend(cur.path, hop),
				})
			}
		}
	}
	return out
}

// PullUpVerdict decides one I2C net and returns the outcome with its witness from a single call, as
// CompareToBound does for a limit, so a Pass always carries the path that justifies it.
//
// A FAIL carries a witness too, stating the hop limit, since a bus whose pull-up sits four hops away
// is a different situation from one with no pull-up at all.
func PullUpVerdict(m Model, n *ir.Net) (Outcome, *Witness, []ContextSubject) {
	if n == nil {
		return Fail, nil, nil
	}
	path := PullUpPathToRail(m, n)
	if path == nil {
		// No resistor or rail was found and the searched-from net is the subject, so the proof is
		// the bound searched to and the context is empty.
		return Fail, &Witness{
			Statement: fmt.Sprintf("no rail is reachable from %s through a resistor within %d hops",
				n.Name, PullUpReachHops),
			Terms: []WitnessTerm{{Label: "hop limit", Value: fmt.Sprintf("%d", PullUpReachHops)}},
		}, nil
	}
	// The path goes in as ordered CONTEXT rather than terms, since every hop is an entity a reader
	// can be sent to and carries the Kind a highlight joins on. The subject net is excluded as it is
	// already the verdict's subject. Roles repeat on a multi-hop path, so this is a list, not a map.
	// The witness carries NO terms, since every term would duplicate an entity already in Context.
	ctx := make([]ContextSubject, 0, len(path)*2)
	res := make([]string, 0, len(path))
	for i, h := range path {
		role := "segment"
		if i == len(path)-1 {
			role = "rail"
		}
		ctx = append(ctx,
			ContextSubject{Entity: Entity{Kind: KindComponent, Ref: h.Resistor}, Role: "pull-up"},
			ContextSubject{Entity: Entity{Kind: KindNet, Ref: h.Net}, Role: role})
		res = append(res, h.Resistor)
	}
	return Pass, &Witness{
		Statement: fmt.Sprintf("%s reaches rail %s through %s",
			n.Name, path[len(path)-1].Net, strings.Join(res, " then ")),
	}, ctx
}

// resistorNetIndex maps a resistor's ref-des to the distinct nets it touches.
//
// Built per call because the model's equivalent index is unexported and covers every pass class.
// It is one pass over the nets, run once per I2C net, which is single digits to low tens on a real
// board.
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
