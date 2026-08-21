package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// fetVdssBelowRail flags a MOSFET connected to a power rail whose voltage is at or above the part's
// datasheet drain-source breakdown rating (WS3-116). A connection-aware datasheet rule in the class
// WS3-028 established.
//
// Above VDSS the part stops behaving like a switch: it can avalanche, conduct when it is supposed to
// block, or fail SHORT. The short is the direction that matters. A high-side switch that fails short
// hands the full rail to whatever it was protecting, so the FET's failure becomes the load's failure.
//
// PRECISION LIMIT, stated because it shapes what the rule can claim. VDSS is a DRAIN-SOURCE rating,
// but the pin-role vocabulary has no drain or source (anode/cathode/power/ground only), so the rule
// cannot tell which of the FET's nets sits across those two terminals. It therefore compares against
// every RAIL the part touches and reports the highest. Rails are the right filter rather than every
// net: a gate-drive net is not a rail, so the common false pairing is structurally excluded rather
// than avoided by luck. A gate deliberately tied to a rail is the residual case, and there the
// binding limit is VGSS rather than VDSS — the finding would name the wrong parameter for a condition
// that is usually still a defect. WS3-117 (FET pin roles) is what makes this exact.
var fetVdssBelowRail = &check.Rule{
	Name:       "fet-vdss-below-switched-rail",
	Severity:   "error",
	Summary:    "A MOSFET sits on a rail at or above its datasheet drain-source breakdown voltage.",
	Impact:     "Past its breakdown rating a switching FET no longer blocks: it can avalanche or fail short, and a shorted high-side switch applies the full rail to the load it was protecting. The rating is the vendor's own number, cited with the page it came from.",
	Remedy:     "Fit a FET whose drain-source rating clears the rail with margin for switching overshoot, checking it against the rail's transient peak rather than its nominal.",
	Primitives: []string{"select", "traverse", "param-join"},
	Reads:      []string{"param.fet_breakdown", "param.output_voltage", "net.max_voltage", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("fet-vdss-below-switched-rail"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		var out []check.Finding
		for _, c := range m.Components() {
			spec := m.PartSpec(c.RefDes)
			if spec == nil {
				continue
			}
			limits := check.FetBreakdownLimits(spec)
			if len(limits) == 0 {
				continue
			}
			// The LOWEST breakdown row binds: a part is endangered at its weakest rating.
			vdss := limits[0]
			for _, p := range limits[1:] {
				if p.Value.GetMax() < vdss.Value.GetMax() {
					vdss = p
				}
			}
			for _, n := range m.Nets() {
				if !onNet(n, c.RefDes) || !m.IsPowerRail(n.Name) || m.IsGroundNet(n) {
					continue
				}
				volts, src, ok := railVolts(m, n)
				if !ok || volts < vdss.Value.GetMax() {
					continue
				}
				f := check.Finding{
					Kind:    check.KindComponent,
					Subject: c.RefDes,
					Message: fmt.Sprintf("%s sits on rail %q at %gV (%s), at or above its breakdown %s %gV — %s",
						c.RefDes, n.Name, volts, src.how, vdss.Symbol, vdss.Value.GetMax(),
						check.Citation(spec, vdss)),
					Prov:          c.Prov,
					DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(spec, vdss)},
					// The rail the message names. The subject is the FET, because that is the part to
					// change, so without this the reader cannot get to the net from the finding.
					Context: []check.ContextSubject{
						{Kind: check.KindNet, Subject: n.Name, NetID: n.GetId(), Role: "rail"},
					},
				}
				// A rail voltage read off a DRIVING PART's datasheet is a second vendor value the
				// conclusion rests on, so it earns a citation and the data-trust gate weighs it.
				// A name-derived rail is a design convention, not a document, so it gets none — and
				// the message says which, because "5V" from a datasheet and "5V" from a net name are
				// not equally trustworthy and the report should not flatten them.
				if src.cite != nil {
					f.DatasheetProv = append(f.DatasheetProv, src.cite)
				}
				out = append(out, f)
			}
		}
		return out
	}),
}

// railEvidence records where a rail's voltage came from: the human-readable provenance for the
// message, and a citation when the number was a datasheet value rather than a naming convention.
type railEvidence struct {
	how  string
	cite *check.DatasheetCitation
}

// railVolts resolves a rail's voltage, preferring DATASHEET evidence over the net's name. A part on
// the rail whose spec states an output voltage is the strongest evidence available; the name-derived
// nominal is the fallback a directionless netlist leaves. ok is false when neither yields a number,
// so the rule skips rather than guessing.
func railVolts(m check.Model, n *ir.Net) (float64, railEvidence, bool) {
	best, found := 0.0, false
	var ev railEvidence
	for _, conn := range n.GetConnections() {
		spec := m.PartSpec(conn.GetComponentRef())
		if spec == nil {
			continue
		}
		for _, p := range check.OutputVoltageLimits(spec) {
			if v := p.Value.GetMax(); !found || v > best {
				best, found = v, true
				ev = railEvidence{
					how:  fmt.Sprintf("%s datasheet output", conn.GetComponentRef()),
					cite: check.DatasheetCitationOf(spec, p),
				}
			}
		}
	}
	if found {
		return best, ev, true
	}
	if v, ok := check.RailMaxVoltage(n, n.Name); ok {
		return v, railEvidence{how: "from the net name"}, true
	}
	return 0, railEvidence{}, false
}
