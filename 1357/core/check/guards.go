package check

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/ident"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/netgraph"
)

// This file holds the guard vocabulary. BOTH the built-in rules and the Spec FFI builtins
// (spec_funcs.go) use it, so it lives in the framework rather than in a rule_*.go (issue #4). It
// covers name heuristics, protection and reach walks, and the scope and citation helpers.

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
// it, so a design with no differential pairs reports nothing even when names happen to end in _P,
// _DP or +.
func DiffConventionPresent(m Model) bool {
	for _, n := range m.Nets() {
		if neg, ok := m.DiffNegative(n.Name); ok && m.HasNetName(neg) {
			return true
		}
	}
	return false
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
			if spec == nil || !m.IsFitted(c.ComponentRef) {
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

// IntentionallyUnconnected reports whether a net's lack of connections is deliberate: a pin on it
// carries the format's no-connect flag, a connected pin resolves to the NO_CONNECT electrical type,
// or the designer named it as a no-connect. A tool's own stub name is not evidence, because KiCad
// names every unlabelled one-pin net "unconnected-(...)", flagged or not (agni issue 1019).
func IntentionallyUnconnected(m Model, n *ir.Net) bool {
	if m.NoConnectMarked(n) {
		return true
	}
	return Exists(n.Connections, func(c *ir.Connection) bool {
		return m.PinDir(c.ComponentRef, c.PinRef) == ir.PinDirection_PIN_DIRECTION_NO_CONNECT
	})
}

// DeliberatelyOpen reports whether a net is one pin the design marks as left unconnected on purpose,
// such as a KiCad pin under a no-connect X. A rule that would demand something of the net (a pull-up,
// a driver, a capacitor) has nothing to judge there, since the designer already said the pin is
// unused (agni issue 1244). A flagged pin that is also wired to another pin is a design error rather
// than a statement, so a net of two or more connections is never deliberately open.
func DeliberatelyOpen(m Model, n *ir.Net) bool {
	return len(n.Connections) == 1 && IntentionallyUnconnected(m, n)
}

// DeliberatelyOpenReason is the reason a rule gives when DeliberatelyOpen holds for a net.
const DeliberatelyOpenReason = "the design marks the net's only pin as deliberately unconnected, so there is nothing on it to judge"

// ToolNetNamePattern matches the names a tool makes up when no designer named a net. Examples are an
// EDIF or synthetic "N$1", "$N1", and KiCad's "Net-(R1-Pad2)" and "unconnected-(J1A-MID0-PadK40)".
// KiCad builds such a name from a pin's own name. So a rule that reads meaning from a net's name must
// skip these, or the pin name inside reads as the designer's (agni issue 1019).
const ToolNetNamePattern = `^(N\$|unconnected-\(|Net-\(|\$)`

var toolNetName = regexp.MustCompile(ToolNetNamePattern)

// IsToolNetName reports whether a tool made up a net's name, rather than a designer giving it one.
func IsToolNetName(name string) bool { return toolNetName.MatchString(name) }

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

// Unpopulated reports whether every part on a net is marked do-not-populate, so the assembled board
// has the copper and no parts on it (agni issue 961). Power flags and power symbols count for neither
// side, and a net with no real part on it is not unpopulated. Some rules demand a capacitor or a clamp
// of a net, and such a rule leaves an unpopulated net out. The rule credits no unfitted part, so
// judging the unfitted circuit anyway would report a fault on hardware that is never built.
func Unpopulated(m Model, n *ir.Net) bool {
	real := false
	for _, c := range n.Connections {
		if IsVirtualRef(c.ComponentRef) {
			continue
		}
		if m.IsFitted(c.ComponentRef) {
			return false
		}
		real = true
	}
	return real
}

// PowerPinReachable reports a power-direction pin on the net or on any net in its 2-hop
// series reach (WS3-011), so the split between esd and input-protection does not depend on
// whether a bead sits between the connector and the regulator.
func PowerPinReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, ProtectionReachHops).Nets {
		for _, c := range rn.Connections {
			// A part marked do-not-populate has no power pin on the assembled board (agni issue 961).
			if !m.IsFitted(c.ComponentRef) {
				continue
			}
			if d := ConnDir(m, c); d == ir.PinDirection_PIN_DIRECTION_POWER_IN || d == ir.PinDirection_PIN_DIRECTION_POWER_OUT {
				return true
			}
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
			// A clamp marked do-not-populate protects nothing on the assembled board (agni issue 961).
			if m.HasClass(c.ComponentRef, class) && m.IsFitted(c.ComponentRef) {
				return c.ComponentRef
			}
		}
	}
	return ""
}

// ExternalSignalNet reports whether a net is a connector-facing SIGNAL net, the scope both ESD rules
// share. Such a net is not a rail or ground (by name or by fact), not a deliberately unconnected pad,
// and not on a power path (input-protection's, WS3-011). esd-protection and esd-clamp-not-tvs split
// these nets by what protects them (nothing, or a Zener clamp).
//
// The net.connector_signal query relation projects it too, so a datalog ESD check scopes itself
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
		return m.ExposedConnector(c.ComponentRef)
	})
	if !hasConn {
		return false
	}
	return !PowerPinReachable(m, n)
}

// NetBiasResistors returns the sorted ref-designators of the resistors that bias n toward a rail or
// ground, each once, with the same direction NetBias reports. A resistor array can reach a rail through
// more than one far net and is still one part. Use it when a caller has to say something ABOUT
// the resistor (its value) rather than only about the net. On a divider the refs are still returned
// while the direction is neither, since both resistors are real and only the net's LEVEL is ambiguous.
func NetBiasResistors(m Model, n *ir.Net) (refs []string, up, down bool) {
	up, down = netBias(m, n, &refs)
	sort.Strings(refs)
	return slices.Compact(refs), up, down
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
		// An unfitted resistor biases nothing on the assembled board (agni issue 961).
		if !m.HasClass(ref, ClassResistor) || !m.IsFitted(ref) {
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

// ACCoupled reports whether the net passes through a SERIES capacitor, the structural signature of AC
// coupling (WS3-088). It tells a coupling cap from a decoupling one by the far side. A decoupling
// cap returns to ground or a rail, while a coupling cap's far side is another signal.
func ACCoupled(m Model, n *ir.Net) bool {
	if isSupplyNet(m, n) {
		return false
	}
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		// An unfitted series capacitor couples nothing on the assembled board (agni issue 961).
		if !m.HasClass(ref, ClassCapacitor) || !m.IsFitted(ref) {
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

// UnprotectedPowerReach reports whether the connector net reaches SOME power input that is
// unprotected. The spec FFI and the declarative twin need this bool. PowerPathProtection runs the
// same walk and keeps the evidence.
func UnprotectedPowerReach(m Model, n *ir.Net) bool {
	return PowerPathProtection(m, n).Unprotected != ""
}

// PowerPathReport is what the power-entry walk found from one connector net.
//
// Check Loads before calling a net protected. UnprotectedPowerReach is false both for a protected 5V
// entry and for a USB data pair that reaches no supply pin at all. The first is a pass, and the second
// is not a subject of a power-entry rule.
type PowerPathReport struct {
	// Loads counts the reached nets that have a real power-input pin. Each one is a power entry this
	// connector net feeds. Zero means the walk found no power path.
	Loads int
	// Unprotected names the first reached load with neither a fuse crossed on the way nor a
	// protector on any net along that path, "" when every load is protected.
	Unprotected string
	// Protector names the first fuse or TVS credited on a protected path, "" when there is none to
	// credit. It is the pass's evidence, the part a reviewer should go and look at.
	Protector string
	// ProtectorWitness is the datasheet evidence when Protector is a current-limiting switch rather
	// than a fuse or TVS (CurrentLimitCredit), nil otherwise.
	ProtectorWitness *Witness
	// Unread names an IC at the Unprotected entry when the run has no datasheet for it. That IC may be
	// a current-limiting switch (UnreadIC). Unread is "" when there is none.
	Unread string
}

// PowerPathProtection walks the connector net's series neighborhood (WS3-011) and reports, per
// reached power input, whether a fuse was crossed on the way there or a protector sits on a net
// along that path. The check is per target because one connector can feed a protected 5V path and an
// unprotected 3V3 path, and protection on one must not excuse the other.
func PowerPathProtection(m Model, n *ir.Net) PowerPathReport {
	r := m.Reach(n, PowerPathReachHops)
	protectorOn := func(net *ir.Net) string {
		for _, c := range net.Connections {
			// An unfitted fuse or TVS protects nothing on the assembled board (agni issue 961).
			if (m.HasClass(c.ComponentRef, ClassFuse) || m.HasClass(c.ComponentRef, ClassTVS)) && m.IsFitted(c.ComponentRef) {
				return c.ComponentRef
			}
		}
		return ""
	}
	var out PowerPathReport
	for _, target := range r.Nets {
		// A load marked do-not-populate draws nothing on the assembled board (agni issue 961).
		hasPowerIn := Exists(target.Connections, func(c *ir.Connection) bool {
			return !IsVirtualRef(c.ComponentRef) && m.IsFitted(c.ComponentRef) && ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
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
		// Suppose a current-limiting switch is the entry's only load. Then the switch is the entry's
		// overcurrent protection, and only its datasheet can say so (agni issue 1248).
		var credit *Witness
		if protector == "" {
			protector, credit = CurrentLimitCredit(m, target)
		}
		switch {
		case protector == "" && out.Unprotected == "":
			out.Unprotected = target.Name
			out.Unread = UnreadIC(m, target, "")
		case protector != "" && out.Protector == "":
			out.Protector, out.ProtectorWitness = protector, credit
		}
	}
	return out
}

// ZenerReachable reports a Zener clamp on the net or on any net in its 2-hop series reach, the
// same reach TVSReachable walks (WS3-011). A Zener is a slower clamp than a TVS, so esd-protection
// does not count it, and esd-clamp-not-tvs (WS3-078) reports it separately for the review to weigh.
func ZenerReachable(m Model, n *ir.Net) bool { return ReachableOfClass(m, n, ClassZener) != "" }

// PullUpReachHops is the DEFAULT bound on the walk from an I2C bus to its pull-up's rail. Like the
// hop constants in reach.go it is ELECTRICAL, not a search budget. A pull-up behind a
// series isolation resistor is an ordinary two hops, and past three the series resistance is
// comparable to the pull-up, so the bus no longer returns high in time. Widening it for every design
// passes buses that are electrically unheld.
//
// A design may declare a different reach for one net (`pull_up_reach_hops` in its intent, agni issue
// 820). The designer is then saying that bus's series network is sized for that reach. The walks
// read the limit through Model.PullUpReachLimit, never this constant. See
// docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
const PullUpReachHops = 3

// PullUpReachesRail reports whether a rail is reachable from n by crossing RESISTORS only, within
// the net's pull-up reach (Model.PullUpReachLimit), without passing through ground.
//
// Not Reach, because Reach drops a bus-like net (more than maxWalkFan connections) outright, and a
// supply rail on a real board is one. This walk keeps the rail as a legal DESTINATION and never
// continues through it. ReachToTerminus does that too but crosses every pass class (agni issue 374).
//
// RESISTORS only, since a pull-up is a resistor to a rail, and crediting a bead would pass buses on
// the strength of a filter. Ground is never crossed, since a resistor to ground is a pull-DOWN and
// ground is a plane that would make everything reachable.
//
// It is the boolean projection of PullUpPathToRail, which runs the walk.
//
// Sometimes a part's datasheet binds an internal pull-up to a pin. That pin counts as pulled up too
// (IntegratedPullUpCredit, agni issue 1298). PullUpVerdict passes it on the same evidence, so the
// spec twins and the coverage panel agree with the rules.
func PullUpReachesRail(m Model, n *ir.Net) bool {
	if PullUpPathToRail(m, n) != nil {
		return true
	}
	ref, _ := IntegratedPullUpCredit(m, n)
	return ref != ""
}

// PullUpHop is one leg of the walk from an I2C net to a rail. It holds the resistor crossed and the
// net on its far side. It names the resistor rather than the pin because a reader locating the
// pull-up wants the part.
type PullUpHop struct {
	Resistor string // ref-des of the resistor crossed
	Net      string // the net reached by crossing it
}

// PullUpPathToRail returns the hops from n to the first rail reachable under PullUpReachesRail's
// rules, and nil when none is. The last hop's Net is the rail.
//
// A passing bus keeps the path as its proof, so a reviewer can ask "show me the pull-up" (#387).
func PullUpPathToRail(m Model, n *ir.Net) []PullUpHop {
	if n == nil {
		return nil
	}
	resistorNets := resistorNetIndex(m)
	limit := m.PullUpReachLimit(n.Name)

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
		if cur.depth >= limit {
			continue
		}
		for _, c := range cur.net.Connections {
			ref := c.ComponentRef
			// A resistor may appear once per PATH, not once per walk, since two bus segments can
			// reach the same rail through different resistors.
			// An unfitted resistor pulls nothing, so a do-not-populate alternative is no pull-up (agni 938).
			if cur.used[ref] || !m.HasClass(ref, ClassResistor) || !m.IsFitted(ref) {
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

// PullUpTermination is one resistor that takes a net to a rail, with the rail it reaches and the
// hops to that rail.
type PullUpTermination struct {
	Resistor string      // the resistor whose far side is the rail
	Rail     string      // the rail it landed on
	Path     []PullUpHop // from the subject net to the rail, the last hop crossing Resistor
}

// AllPullUpPathsToRail returns every DISTINCT resistor that takes n to a rail, with the rail each
// reaches. PullUpPathToRail answers whether there is one. This answers how many, and a second
// pull-up shows up in that count (agni issue 516). Two 2.2k in parallel are an effective 1.1k, so the
// bus sinks about twice the current it was sized for, and reachability alone cannot see it.
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
	limit := m.PullUpReachLimit(n.Name)

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
		if cur.depth >= limit {
			continue
		}
		for _, c := range cur.net.Connections {
			ref := c.ComponentRef
			// An unfitted resistor pulls nothing, so a do-not-populate alternative is no pull-up (agni 938).
			if cur.used[ref] || !m.HasClass(ref, ClassResistor) || !m.IsFitted(ref) {
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
// A FAIL carries a witness too, stating the hop limit in effect for the net (its declared reach, else
// PullUpReachHops). A pull-up four hops away and no pull-up at all are different situations.
//
// A PASS carries every distinct path to a rail within that limit (agni issue 821), one per
// terminating resistor as AllPullUpPathsToRail finds them, in SortPullUpTerminations' order, with the
// proof PullUpEvidence builds. A net with one path gets exactly the witness it got before PASS listed
// more than one.
//
// The statement names a one-hop path by its resistor and spells a longer one as RenderRoute does,
// with every net it crosses, so the reason column of an exported verdict table reads as the route
// on its own (agni issue 1240).
func PullUpVerdict(m Model, n *ir.Net) (Outcome, *Witness, []ContextSubject) {
	if n == nil {
		return Fail, nil, nil
	}
	paths := AllPullUpPathsToRail(m, n)
	if len(paths) == 0 {
		// Sometimes a part's datasheet binds an internal pull-up to its pin on n. That part pulls n up
		// with nothing on the board, as a level translator's ports do (agni issue 1298).
		if ref, w := IntegratedPullUpCredit(m, n); ref != "" {
			return Pass, w, []ContextSubject{{Entity: Entity{Kind: KindComponent, Ref: ref}, Role: "integrated pull-up"}}
		}
		// No resistor or rail was found and the searched-from net is the subject, so the proof is
		// the bound searched to and the context is empty.
		limit := m.PullUpReachLimit(n.Name)
		// Unlike a crystal terminal, a failure names no unread part. Nearly every bus has a
		// microcontroller with no datasheet in the run, and its weak internal pull-up is not the
		// pull-up a bus needs, so the note would sit on almost every finding and mislead.
		return Fail, &Witness{
			Statement: fmt.Sprintf("no rail is reachable from %s through a resistor within %d hops",
				n.Name, limit),
			Terms: []WitnessTerm{{Label: "hop limit", Value: fmt.Sprintf("%d", limit)}},
		}, nil
	}
	SortPullUpTerminations(paths)
	clauses := make([]string, 0, len(paths))
	for _, p := range paths {
		if len(p.Path) == 1 {
			clauses = append(clauses, fmt.Sprintf("rail %s through %s", p.Rail, p.Path[0].Resistor))
			continue
		}
		clauses = append(clauses, fmt.Sprintf("rail %s along %s", p.Rail, RenderRoute(n.Name, pullUpRouteHops(p))))
	}
	w := &Witness{Statement: fmt.Sprintf("%s reaches %s", n.Name, strings.Join(clauses, ", and "))}
	ctx, terms := PullUpEvidence(n.Name, paths)
	w.Terms = terms
	return Pass, w, ctx
}

// SortPullUpTerminations orders paths by hop count, then resistor, then rail, so evidence reads the
// same on every run. Names compare naturally (ident.CompareNatural), so R2 lists before R10 as it
// does in every table (agni issue 1165).
func SortPullUpTerminations(paths []PullUpTermination) {
	sort.SliceStable(paths, func(i, j int) bool {
		a, b := paths[i], paths[j]
		if len(a.Path) != len(b.Path) {
			return len(a.Path) < len(b.Path)
		}
		if c := ident.CompareNatural(a.Resistor, b.Resistor); c != 0 {
			return c < 0
		}
		return ident.CompareNatural(a.Rail, b.Rail) < 0
	})
}

// PullUpEvidence builds the proof for the paths found from net, in the order given. Every pull-up
// rule attaches it, so i2c-pull-up and the two count rules show a bus the same way (agni issues 821,
// 1165).
//
// The hops go in as CONTEXT, since every hop is an entity. A reader can be sent to it, and a
// highlight joins on its Kind. The subject net is left out as it is already the verdict's subject.
// Paths can share a segment or a rail, so an entity already listed is not listed again. The context
// is then a flat set of things to light up.
//
// One path is fully stated by its context, so it returns NO terms, since a term would duplicate an
// entity already there. Several paths need the grouping the flat context loses, so each becomes a
// `path` term rendered by RenderRoute.
func PullUpEvidence(net string, paths []PullUpTermination) ([]ContextSubject, []WitnessTerm) {
	var ctx []ContextSubject
	listed := map[Entity]bool{}
	add := func(e Entity, role string) {
		if !listed[e] {
			listed[e] = true
			ctx = append(ctx, ContextSubject{Entity: e, Role: role})
		}
	}
	for _, p := range paths {
		for i, h := range p.Path {
			role := "segment"
			if i == len(p.Path)-1 {
				role = "rail"
			}
			add(Entity{Kind: KindComponent, Ref: h.Resistor}, "pull-up")
			add(Entity{Kind: KindNet, Ref: h.Net}, role)
		}
	}
	if len(paths) < 2 {
		return ctx, nil
	}
	terms := make([]WitnessTerm, 0, len(paths))
	for _, p := range paths {
		terms = append(terms, WitnessTerm{Label: "path", Value: RenderRoute(net, pullUpRouteHops(p))})
	}
	return ctx, terms
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

func pullUpRouteHops(p PullUpTermination) []RouteHop {
	hops := make([]RouteHop, len(p.Path))
	for i, h := range p.Path {
		hops[i] = RouteHop{Through: h.Resistor, To: h.Net}
	}
	return hops
}
