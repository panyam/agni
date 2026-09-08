package intent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/ident"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The declared pin map checked against the netlist (agni issue 517).
//
// The failure this catches is ordinary rather than exotic. A pin assignment moves late, because
// layout wanted a swap or firmware hit a mux conflict, the map is updated, and one net does not get
// redrawn. The netlist stays self-consistent. Every electrical rule passes, correctly, because
// nothing is electrically wrong. The only thing wrong is that the board disagrees with a document
// nothing had ever read.
//
// THREE rules over one declaration, because a reviewer does something different about each answer: a
// net on the wrong pin is a schematic edit, a declared net the netlist does not have is usually a
// real disconnection, and a wrong far end is a routing question. Those last two get confused
// constantly and they are opposite defects, so they never share a verdict.
//
// Every comparison runs through core/ident and never through string equality. A map is authored in
// the vocabulary a datasheet uses and the netlist answers in package designators, and both documents
// carry zero-padded indices, invisible characters pasted out of a PDF, and cells naming several
// functions at once. We measured a shipped in-house checker of exactly this kind against a large
// production board: every warning it produced was a string-comparison artifact and none was a design
// defect. A checker whose warnings are all noise gets switched off, so the comparison is the part
// that decides whether any of this is worth running.

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
		Reads:               []string{"pin", "pin.name", "pin.net"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return ioMapPinVerdicts(m, d) },
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
		Eval:                func(m check.Model) []check.Verdict { return ioMapNetAbsentVerdicts(m, d) },
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
		Reads:               []string{"pin", "pin.name", "pin.net", "reaches"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return ioMapFarEndVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapPinVerdicts decides every row the map declares. The considered set is the DECLARATION, which
// is what an intent rule is for: it knows exactly what it was asked to look for, so it can say "all
// two hundred declared assignments are as drawn" rather than saying the nothing an undeclared map
// says.
func ioMapPinVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	for _, a := range d.IOMap {
		v := check.Verdict{Subjects: []check.Entity{check.PinEntity(a.Device, a.Pin)}}
		// The missing-net case belongs to io-map-net-absent. Reporting it here as well would put one
		// defect under two review items, which is the partition railBudgetMarginRule keeps next door.
		if netNamed(m, a.Net) == nil {
			v.Outcome, v.Reason = check.NotConsidered, fmt.Sprintf(
				"the design carries no net named %q, which %s reports rather than this rule", a.Net, RuleIOMapNetAbsent)
			out = append(out, notEvaluatedFunction(v, a))
			continue
		}
		res := resolvePin(m, a.Device, a.Pin)
		if res.outcome != check.Pass {
			v.Outcome, v.Reason, v.Finding = res.outcome, res.reason, res.finding
			out = append(out, notEvaluatedFunction(v, a))
			continue
		}
		v.Subjects = []check.Entity{check.PinEntity(a.Device, res.designator)}
		v.Context = []check.ContextSubject{check.Ctx(check.NetNameEntity(a.Net), "declared net")}
		actual := m.PinNetName(a.Device, res.designator)
		terms := []check.WitnessTerm{
			{Label: "declared", Value: a.Net},
			{Label: "on the pin", Value: orNone(actual)},
		}
		// A net name is the design's OWN identifier, so it is compared at exact or normalized and
		// never fuzzily. A token-subsequence match is right for a vendor's inconsistent function
		// spellings and wrong here: DDR_CK_P and DDR_CK_T_P are two nets, not two spellings of one.
		if actual != "" && sameName(actual, a.Net) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s pin %s carries %q, as the IO map declares%s",
					a.Device, res.label(), actual, res.note()),
				Terms: terms,
			}
			out = append(out, notEvaluatedFunction(v, a))
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
			Prov: componentProv(m, a.Device),
		}
		out = append(out, notEvaluatedFunction(v, a))
	}
	return out
}

// ioMapNetAbsentVerdicts asks the other half of the pair, and it is a separate rule because the two
// are opposite defects that get confused constantly: a net the map declares and the netlist does not
// have is usually a real disconnection, where a net the netlist has and the map does not declare is
// an incomplete map. The second is a COVERAGE question and is not this rule's.
func ioMapNetAbsentVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	seen := map[string]bool{}
	for _, a := range d.IOMap {
		// One verdict per declared NET rather than per row. A map naming one net on several rows is
		// ordinary (a bus, a rail), and repeating the same absence per row would inflate the count a
		// reader uses to judge how bad the disagreement is.
		if seen[a.Net] {
			continue
		}
		seen[a.Net] = true
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(a.Net)}}
		n := netNamed(m, a.Net)
		if n == nil {
			if alt := nearestNet(m, a.Net); alt != "" {
				// The netlist has a net this one only differs from by spelling, which is a different
				// defect from an absent net and must not be reported as one.
				v.Outcome = check.Inconclusive
				v.Reason = fmt.Sprintf("no net is named %q, and the design has %q, which differs from it only by spelling", a.Net, alt)
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
// series parts between them, so a failure says what the net actually goes through and where it ends
// up rather than only that it is wrong (agni issue 518).
//
// EVERY row gets a verdict, including the ones declaring no far end. The far-end columns of a real
// map are filled sparsely, roughly a third of rows in the one we measured, and a rule that reported
// only the rows carrying them would show a clean result whose denominator was the empty ones.
func ioMapFarEndVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	for _, a := range d.IOMap {
		v := check.Verdict{Subjects: []check.Entity{check.PinEntity(a.Device, a.Pin)}}
		if a.To == nil {
			v.Outcome, v.Reason = check.NotConsidered, "this row declares no far end, so there is nothing to follow the net to"
			out = append(out, v)
			continue
		}
		v.Context = []check.ContextSubject{check.Ctx(check.PinEntity(a.To.Device, a.To.Pin), "declared far end")}
		near, far := resolvePin(m, a.Device, a.Pin), resolvePin(m, a.To.Device, a.To.Pin)
		if near.outcome != check.Pass || far.outcome != check.Pass {
			bad := near
			if near.outcome == check.Pass {
				bad = far
			}
			v.Outcome, v.Reason, v.Finding = bad.outcome, bad.reason, bad.finding
			out = append(out, v)
			continue
		}
		v.Subjects = []check.Entity{check.PinEntity(a.Device, near.designator)}
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
				Prov: componentProv(m, a.Device),
			}
		default:
			// An endpoint that resolved to a pin the design has but that sits on no net at all. The
			// walk could not start, which is not the same answer as "these two do not join", and
			// reporting it as a no-route would send a reader to look at the routing instead of at
			// the pin.
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

// note says what had to be assumed to match the map's spelling to the design's, and says nothing
// when the two agreed outright. A match a reader cannot check is the thing this whole layer exists
// to avoid, so an inferred one always announces itself.
func (p pinMatch) note() string {
	if p.via.Match == ident.Exact || p.via.Note == "" {
		return ""
	}
	return " (" + string(p.via.Match) + " pin-name match: " + p.via.Note + ")"
}

// resolvePin finds the design pin a map row names, accepting either the package designator or the
// part type's functional name.
//
// An author writes a map in the vocabulary the datasheet uses and should not have to know which of
// the two spellings the checker wants, so both are tried and both go through core/ident. The
// designator is preferred when both match, because it is the netlist's own key.
//
// The four non-matches are kept apart because they send a reader to four different places: a device
// the design does not have, a device the read gave no pins for, a pin no reading of the name finds,
// and a name that finds SEVERAL pins.
func resolvePin(m check.Model, refDes, declared string) pinMatch {
	out := pinMatch{declared: declared}
	if !m.HasComponent(refDes) {
		out.outcome = check.NotConsidered
		out.reason = fmt.Sprintf("the design carries no component %s, so there is no pin to look up", refDes)
		return out
	}
	var pins []string
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() == refDes {
			pins = append(pins, p.Designator)
		}
	}
	if len(pins) == 0 {
		out.outcome = check.NotConsidered
		out.reason = fmt.Sprintf("the read supplied no pin list for %s, so a declared pin cannot be resolved on it", refDes)
		return out
	}
	sort.Strings(pins)
	type hit struct {
		designator string
		via        ident.Result
	}
	var hits []hit
	for _, des := range pins {
		byDes := ident.Compare(declared, des)
		byName := ident.Result{Match: ident.None}
		if name := m.PinName(refDes, des); name != "" && name != "~" {
			byName = ident.Compare(declared, name)
		}
		if best := stronger(byDes, byName); best.Match != ident.None {
			hits = append(hits, hit{des, best})
		}
	}
	switch len(hits) {
	case 0:
		out.outcome = check.Fail
		out.reason = fmt.Sprintf("%s declares no pin %q, by designator or by name", refDes, declared)
		out.finding = &check.Finding{
			Subject: check.ComponentEntity(refDes),
			Message: fmt.Sprintf("the IO map names pin %q on %s, which declares no such pin by designator or by name", declared, refDes),
			Prov:    componentProv(m, refDes),
		}
		return out
	case 1:
		out.outcome, out.designator, out.via = check.Pass, hits[0].designator, hits[0].via
		return out
	}
	// Several pins answer to the name. A part type may declare one name on several pins (a device
	// with four GND pins is ordinary), so this is a property of the part rather than a defect, and
	// the comparison genuinely cannot decide which pin the map meant. That is inconclusive and it is
	// emphatically not a fail: the row may well be correct.
	var named []string
	for _, h := range hits {
		named = append(named, h.designator)
	}
	out.outcome = check.Inconclusive
	out.reason = fmt.Sprintf("%q matches %d pins on %s (%s), so which one the map means cannot be decided",
		declared, len(hits), refDes, strings.Join(named, ", "))
	out.finding = &check.Finding{
		Subject:      check.ComponentEntity(refDes),
		Message:      fmt.Sprintf("the IO map names pin %q on %s, which has %d pins of that name (%s); name the package designator to make the row decidable", declared, refDes, len(hits), strings.Join(named, ", ")),
		Inconclusive: true,
		Prov:         componentProv(m, refDes),
	}
	return out
}

// stronger picks the more confident of two readings of one pin, so a designator hit is never
// downgraded by a weaker name hit on the same pin.
func stronger(a, b ident.Result) ident.Result {
	if rank(a.Match) >= rank(b.Match) {
		return a
	}
	return b
}

func rank(m ident.Match) int {
	switch m {
	case ident.Exact:
		return 3
	case ident.Normalized:
		return 2
	case ident.Fuzzy:
		return 1
	}
	return 0
}

// sameName compares two names of the same THING (a net against a net), where a fuzzy reading is
// wrong. ident.Fuzzy factors out tokens one side carries and the other does not, which is the right
// reading of a vendor's inconsistent function spellings and the wrong reading of a design's own net
// names, since dropping a token there names a different net.
func sameName(a, b string) bool {
	switch ident.Compare(a, b).Match {
	case ident.Exact, ident.Normalized:
		return true
	}
	return false
}

// nearestNet returns the design's net whose name differs from the declared one only by spelling, or
// "" when none does.
//
// It exists so a misspelling is never reported as a disconnection. Those are opposite defects and
// the expensive one is the false alarm: a reviewer sent to look for a missing net that is actually
// present, under a name differing by an invisible character, stops trusting the tool.
func nearestNet(m check.Model, declared string) string {
	for _, n := range m.Nets() {
		if sameName(n.Name, declared) {
			return n.Name
		}
	}
	return ""
}

// notEvaluatedFunction stamps a row that declares a `function` with the fact that nothing read it.
//
// The field is carried so a map is authored once (see IOAssignment.Function), and this is what stops
// its presence reading as verification. An author who fills a column in believes it is being
// checked, which is why the rail-budget card argues against carrying an unread field at all; making
// it loud on every verdict is how both things can be true here.
func notEvaluatedFunction(v check.Verdict, a IOAssignment) check.Verdict {
	if a.Function == "" {
		return v
	}
	note := fmt.Sprintf("the declared function %q was NOT evaluated: deciding whether a function is legal on a pin needs the part's alternate-function table, which the contract has no shape for (agni issue 667)", a.Function)
	switch {
	case v.Witness != nil:
		v.Witness.Statement += ". " + note
	case v.Reason != "":
		v.Reason += ". " + note
	default:
		v.Reason = note
	}
	return v
}

// routeText renders a trace through the SHARED route renderer rather than joining the parts here.
//
// It used to build the string itself, which made a second implementation of a format core/model
// already owned, agreeing with it only because one person wrote both within a week. That is the
// hazard DECISIONS.md names under "A path is not a query column": once a path is rendered into a
// string, the rendering becomes a format nobody can change, and a second copy is how that begins.
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

func componentProv(m check.Model, refDes string) *ir.Provenance {
	for _, c := range m.Components() {
		if c.GetRefDes() == refDes {
			return c.GetProv()
		}
	}
	return nil
}
