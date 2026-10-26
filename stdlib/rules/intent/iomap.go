package intent

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/ident"
)

// The declared pin map checked against the netlist (agni issue 517).
//
// It catches a pin assignment that moved late, after layout wanted a swap or firmware hit a mux
// conflict, where the map was updated and one net was not redrawn. The netlist stays self-consistent
// and every electrical rule passes, because nothing is electrically wrong.
//
// THREE rules here over one declaration, because a reviewer acts differently on each answer. A net on
// the wrong pin is a schematic edit, a declared net the netlist lacks is usually a real
// disconnection, and a wrong far end is a routing question. The last two are often confused, so they
// never share a verdict. The fourth rule, io-map-coverage, is in iomapcoverage.go, and the fifth,
// io-map-function, which checks a row's declared function against the datasheet, in
// iomapfunction.go.
//
// Every comparison goes through core/ident, never string equality. A map uses the datasheet's
// vocabulary and the netlist answers in package designators, and both carry zero-padded indices,
// invisible characters pasted out of a PDF, and cells naming several functions at once. Measured on
// a large production board, every warning a shipped in-house checker of this kind produced was a
// string-comparison artifact and none was a design defect.

// ioMapPinRule: the declared net is not on the pin the map names.
func ioMapPinRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleIOMapPin,
		Severity: "error",
		Summary:  "a net is not on the pin the design intent's IO map assigns it to",
		Detail:   intentDoc(RuleIOMapPin),
		Impact: "the board disagrees with the pin assignment the firmware and the pin-mux configuration were " +
			"written against. Nothing is electrically wrong, so every other rule passes; the part is simply " +
			"wired to a different peripheral than the software expects, which surfaces at bring-up as a " +
			"peripheral that never responds.",
		Remedy:              intentRemedy(RuleIOMapPin),
		Reads:               []string{"component.pin", "pin.name", "pin.net"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return ioMapPinVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapNetAbsentRule: the map declares a net the netlist does not have.
func ioMapNetAbsentRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleIOMapNetAbsent,
		Severity: "error",
		Summary:  "the design intent's IO map declares a net the netlist does not have",
		Detail:   intentDoc(RuleIOMapNetAbsent),
		Impact: "a net the architecture says exists is not in the design at all. That is usually a real " +
			"disconnection rather than a naming difference, and it is the opposite defect from a net the " +
			"netlist has and the map does not declare, which is an incomplete map.",
		Remedy:              intentRemedy(RuleIOMapNetAbsent),
		Reads:               []string{"on_net"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return ioMapNetAbsentVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapFarEndRule: the net does not reach the far end the map declares for it.
func ioMapFarEndRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleIOMapFarEnd,
		Severity: "error",
		Summary:  "a net does not reach the far-end device pin the design intent's IO map declares",
		Detail:   intentDoc(RuleIOMapFarEnd),
		Impact: "the signal leaves the pin the map assigns and arrives somewhere other than the device the " +
			"map says it drives. A net can be on the right pin at one end and land on the wrong part at the " +
			"other, which no per-net check can see.",
		Remedy:              intentRemedy(RuleIOMapFarEnd),
		Reads:               []string{"component.pin", "pin.name", "pin.net", "net.reaches"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return ioMapFarEndVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapPinVerdicts decides every row the map declares. The considered set is the DECLARATION, so a
// clean result states how many declared assignments were checked.
func ioMapPinVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	for _, a := range d.IOMap {
		res := resolvePin(m, a.Device, a.Pin)
		actual := ""
		if res.outcome == check.Pass {
			actual = m.PinNetName(a.Device, res.designator)
		}
		v := check.Verdict{Subjects: []check.Entity{res.subject(a.Device)}, Context: rowContext(a, res)}
		if actual != "" {
			v.Context = append(v.Context, check.Ctx(check.NetNameEntity(actual), RoleOnThePin))
		}
		// io-map-net-absent owns the missing-net case. Reporting it here too would put one defect
		// under two review items (railBudgetMarginRule splits the same way).
		if netNamed(m, a.Net) == nil {
			v.Outcome, v.Reason = check.NotConsidered, fmt.Sprintf(
				"the design carries no net named %q, which %s reports rather than this rule", a.Net, RuleIOMapNetAbsent)
			out = append(out, v)
			continue
		}
		if res.outcome != check.Pass {
			res.decide(&v)
			out = append(out, v)
			continue
		}
		terms := []check.WitnessTerm{
			{Label: "declared", Value: a.Net},
			{Label: "on the pin", Value: orNone(actual)},
		}
		// Net names compare exact or normalized, never fuzzy (see sameName), since DDR_CK_P and
		// DDR_CK_T_P are two different nets.
		if actual != "" && sameName(actual, a.Net) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s pin %s carries %q, as the IO map declares%s",
					a.Device, res.label(), actual, res.note()),
				Terms: terms,
			}
			out = append(out, v)
			continue
		}
		v.Outcome = check.Fail
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("%s pin %s carries %s, and the IO map declares %q%s",
				a.Device, res.label(), orNone(actual), a.Net, res.note()),
			Terms: terms,
		}
		v.Finding = &check.Finding{
			Subject: check.PinEntity(a.Device, res.designator),
			Message: fmt.Sprintf("the IO map assigns %q to %s pin %s, which carries %s",
				a.Net, a.Device, res.label(), orNone(actual)),
			Prov: check.ComponentProv(m, a.Device),
		}
		out = append(out, v)
	}
	return out
}

// rowContext names the map row a per-row verdict answers: the declared net, and the declared pin
// when the design spells it differently from the subject. With the subject's device that is the
// row's key, so a consumer laying the map out one row per pin joins each rule's verdict back to its
// row without a model (agni issue 1188).
func rowContext(a IOAssignment, res pinMatch) []check.ContextSubject {
	ctx := []check.ContextSubject{check.Ctx(check.NetNameEntity(a.Net), RoleDeclaredNet)}
	if res.outcome == check.Pass && res.designator != a.Pin {
		ctx = append(ctx, check.Ctx(check.PinEntity(a.Device, a.Pin), RoleDeclaredPin))
	}
	return ctx
}

// ioMapNetAbsentVerdicts reports each declared net the netlist lacks, which is usually a real
// disconnection. The reverse, a net the netlist has and the map omits, is an incomplete map and
// belongs to io-map-coverage.
func ioMapNetAbsentVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	seen := map[string]bool{}
	for _, a := range d.IOMap {
		// One verdict per declared NET, not per row. A net on several rows is ordinary (a bus, a
		// rail), and a per-row verdict would inflate the absence count.
		if seen[a.Net] {
			continue
		}
		seen[a.Net] = true
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(a.Net)}}
		n := netNamed(m, a.Net)
		if n == nil {
			if alt := nearestNet(m, a.Net); alt != "" {
				// A net differing only by spelling is a misspelling, not an absent net.
				v.Outcome = check.Inconclusive
				v.Witness = &check.Witness{Statement: fmt.Sprintf("no net is named %q, and the design has %q, which differs from it only by spelling", a.Net, alt)}
				v.Finding = &check.Finding{
					Subject:      check.NetNameEntity(alt),
					Message:      fmt.Sprintf("the IO map declares net %q and the design has %q; one of the two documents is misspelling the other", a.Net, alt),
					Inconclusive: true,
					Prov:         netNamed(m, alt).GetProv(),
				}
				out = append(out, v)
				continue
			}
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: fmt.Sprintf("the design has no net named %q", a.Net)}
			v.Finding = &check.Finding{
				Subject: check.NetNameEntity(a.Net),
				Message: fmt.Sprintf("the IO map declares net %q and the netlist has no such net", a.Net),
			}
			out = append(out, v)
			continue
		}
		v.Subjects = []check.Entity{check.NetEntity(n)}
		v.Outcome = check.Pass
		v.Witness = &check.Witness{Statement: fmt.Sprintf("the design carries net %q, as the IO map declares", n.Name)}
		out = append(out, v)
	}
	return out
}

// ioMapFarEndVerdicts follows the net from the declared pin to the declared far end through the
// series parts between them, so a failure names what the net goes through and where it ends up
// (agni issue 518).
//
// EVERY row gets a verdict, including rows with no far end, so a clean result's denominator is the
// whole map. Real maps fill the far-end columns sparsely (roughly a third of rows in the one we
// measured); see docsite/content/guide/design-intent.md#the-nine-forms.
func ioMapFarEndVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	for _, a := range d.IOMap {
		near := resolvePin(m, a.Device, a.Pin)
		v := check.Verdict{Subjects: []check.Entity{near.subject(a.Device)}, Context: rowContext(a, near)}
		if a.To == nil {
			v.Outcome, v.Reason = check.NotConsidered, "this row declares no far end, so there is nothing to follow the net to"
			out = append(out, v)
			continue
		}
		v.Context = append(v.Context, check.Ctx(check.PinEntity(a.To.Device, a.To.Pin), RoleDeclaredFarEnd))
		far := resolvePin(m, a.To.Device, a.To.Pin)
		if near.outcome != check.Pass || far.outcome != check.Pass {
			bad := near
			if near.outcome == check.Pass {
				bad = far
			}
			bad.decide(&v)
			out = append(out, v)
			continue
		}
		t := check.TracePins(m, check.Endpoint{RefDes: a.Device, Pin: near.designator},
			check.Endpoint{RefDes: a.To.Device, Pin: far.designator}, 0)
		switch t.Outcome {
		case check.TraceRouted:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s reaches %s, as the IO map declares: %s",
					near.endpoint(a.Device), far.endpoint(a.To.Device), routeText(t)),
				Terms: []check.WitnessTerm{{Label: "route", Value: routeText(t)}},
			}
		case check.TraceNoRoute:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: fmt.Sprintf("%s does not reach %s within %d crossings",
				near.endpoint(a.Device), far.endpoint(a.To.Device), t.Radius)}
			v.Finding = &check.Finding{
				Subject: check.PinEntity(a.Device, near.designator),
				Message: fmt.Sprintf("the IO map declares %s reaches %s, and it does not: %s sits on %q and %s sits on %q",
					near.endpoint(a.Device), far.endpoint(a.To.Device),
					near.endpoint(a.Device), t.From.Net, far.endpoint(a.To.Device), t.To.Net),
				Prov: check.ComponentProv(m, a.Device),
			}
		default:
			// An endpoint pin on no net, so the walk could not start. That is not a no-route, and
			// reporting it as one would send a reader to the routing instead of the pin.
			v.Outcome, v.Reason = check.NotConsidered, t.Reason
		}
		out = append(out, v)
	}
	return out
}

// pinMatch is one resolved declaration-to-design pin lookup: which pin it landed on, how sure that
// is, and the verdict when it did not land at all.
type pinMatch struct {
	outcome    check.Outcome
	reason     string
	finding    *check.Finding
	designator string // the design's own spelling, which every later lookup uses
	declared   string // the map's spelling, kept so a message can show both
	via        ident.Result
}

// label renders the pin the way a reader checking the finding needs to see it: the design's
// designator, plus the map's spelling when the two differ.
func (p pinMatch) label() string {
	if p.designator == p.declared {
		return p.designator
	}
	return fmt.Sprintf("%s (declared %q)", p.designator, p.declared)
}

func (p pinMatch) endpoint(refDes string) string { return refDes + "." + p.label() }

// subject is the pin a per-row verdict is about: the design's designator once the lookup landed, and
// the map's own spelling when it did not, since then there is no design pin to name.
func (p pinMatch) subject(refDes string) check.Entity {
	if p.outcome == check.Pass {
		return check.PinEntity(refDes, p.designator)
	}
	return check.PinEntity(refDes, p.declared)
}

// decide gives a verdict the outcome of a lookup that did not land. A failed or undecided lookup is
// stated as the witness and an unconsidered one as the reason, since the contract keeps the reason
// for the subjects a rule could not judge.
func (p pinMatch) decide(v *check.Verdict) {
	v.Outcome, v.Finding = p.outcome, p.finding
	if p.outcome == check.NotConsidered {
		v.Reason = p.reason
		return
	}
	v.Witness = &check.Witness{Statement: p.reason}
}

// note says what was assumed to match the map's spelling to the design's, and is empty when the two
// agreed outright. An inferred match always announces itself so a reader can check it.
func (p pinMatch) note() string {
	if p.via.Match == ident.Exact || p.via.Note == "" {
		return ""
	}
	return " (" + string(p.via.Match) + " pin-name match: " + p.via.Note + ")"
}

// resolvePin finds the design pin a map row names through check.LookupPin, the lookup trace shares,
// and turns each kind of non-match into the verdict this rule gives it.
func resolvePin(m check.Model, refDes, declared string) pinMatch {
	out := pinMatch{declared: declared}
	l := check.LookupPin(m, refDes, declared)
	switch l.Kind {
	case check.PinNoComponent:
		out.outcome = check.NotConsidered
		out.reason = fmt.Sprintf("the design carries no component %s, so there is no pin to look up", refDes)
	case check.PinNoPinList:
		out.outcome = check.NotConsidered
		out.reason = fmt.Sprintf("the read supplied no pin list for %s, so a declared pin cannot be resolved on it", refDes)
	case check.PinUnknown:
		out.outcome = check.Fail
		out.reason = fmt.Sprintf("%s declares no pin %q, by designator or by name", refDes, declared)
		out.finding = &check.Finding{
			Subject: check.ComponentEntity(refDes),
			Message: fmt.Sprintf("the IO map names pin %q on %s, which declares no such pin by designator or by name", declared, refDes),
			Prov:    check.ComponentProv(m, refDes),
		}
	case check.PinFound:
		out.outcome, out.designator, out.via = check.Pass, l.Designator, l.Via
	case check.PinAmbiguous:
		// A part may declare one name on several pins (four GND pins is ordinary), so the
		// comparison cannot decide which pin the map meant. That is inconclusive and not a fail,
		// since the row may be correct.
		out.outcome = check.Inconclusive
		out.reason = fmt.Sprintf("%q matches %d pins on %s (%s), so which one the map means cannot be decided",
			declared, len(l.Candidates), refDes, strings.Join(l.Candidates, ", "))
		out.finding = &check.Finding{
			Subject:      check.ComponentEntity(refDes),
			Message:      fmt.Sprintf("the IO map names pin %q on %s, which has %d pins of that name (%s); name the package designator to make the row decidable", declared, refDes, len(l.Candidates), strings.Join(l.Candidates, ", ")),
			Inconclusive: true,
			Prov:         check.ComponentProv(m, refDes),
		}
	}
	return out
}

// sameName compares two names of the same THING (a net against a net), where a fuzzy reading is
// wrong. ident.Fuzzy drops tokens one side carries and the other does not, which suits a vendor's
// inconsistent function spellings, but dropping a token from a net name names a different net.
func sameName(a, b string) bool {
	switch ident.Compare(a, b).Match {
	case ident.Exact, ident.Normalized:
		return true
	}
	return false
}

// nearestNet returns the design's net whose name differs from the declared one only by spelling, or
// "" when none does. It keeps a misspelling from being reported as a disconnection, since a
// reviewer sent after a missing net that is present under a name differing by an invisible
// character stops trusting the tool.
func nearestNet(m check.Model, declared string) string {
	for _, n := range m.Nets() {
		if sameName(n.Name, declared) {
			return n.Name
		}
	}
	return ""
}

// routeText renders a trace through check.RenderRoute, the one route renderer, so the IO-map
// witness and the query column cannot diverge (#664). Do not join the parts here. DECISIONS.md, "A
// path is not a query column", says why the format has one owner.
func routeText(t check.Trace) string {
	hops := make([]check.RouteHop, len(t.Crossings))
	for i, c := range t.Crossings {
		hops[i] = check.RouteHop{Through: c.RefDes, To: c.ToNet}
	}
	return check.RenderRoute(t.From.Net, hops)
}

func orNone(net string) string {
	if net == "" {
		return "no net"
	}
	return fmt.Sprintf("%q", net)
}
