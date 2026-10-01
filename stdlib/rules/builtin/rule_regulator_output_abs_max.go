package builtin

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// regulatorOutputExceedsAbsMax is a CONNECTION-AWARE datasheet rule (WS3-028). It compares a
// parameter on one part against a parameter on ANOTHER part, across the net that joins them, where
// the older datasheet rules read one spec against one rail.
//
// It checks a regulator's datasheet output voltage against the absolute-maximum supply rating of a
// part it feeds. Both numbers are vendor values, so the finding cites both documents.
//
// WHY THIS IS NOT supply-exceeds-abs-max AGAIN. That rule asks the same question but takes the rail
// voltage from the NET NAME (check.NominalVoltageFromName, so "+5V" means 5V). That is a naming
// convention, and it is silent on a net named VOUT_A or 5V_SW, wrong on a net whose name outlived a
// design change, and unavailable on a rail nobody named for its voltage. Reading the number off the
// regulator's own datasheet replaces a convention with evidence.
//
// It does not supersede the older rule, because a rail fed by something with no seeded spec (a
// connector, an unseeded module) still has only its name to go on.
var regulatorOutputExceedsAbsMax = &check.Rule{
	Name:       "regulator-output-exceeds-abs-max",
	Severity:   "error",
	Summary:    "A regulator's datasheet output voltage exceeds the absolute-maximum supply rating of a part it feeds.",
	Impact:     "The downstream part is driven past the vendor's stress envelope by a supply the design itself creates. It may fail immediately or degrade in the field, and because both numbers are vendor values rather than a name-derived guess, the finding is actionable without further verification.",
	Remedy:     "Reprogram the regulator's output to a voltage the downstream part is rated for, or move that part to a rail that already is. Both numbers come from vendor documents, so a design change settles this without needing a measurement.",
	Primitives: []string{"select", "traverse", "reach", "param-join"},
	Reads:      []string{"param.supply_abs_max", "param.output_voltage", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("regulator-output-exceeds-abs-max"),
	// Source, rail, load, in the order the message names them. All three are needed, because one
	// regulator feeding one part over VOUT_A and again over VOUT_B gives two different answers, and a
	// tuple without the rail would give them one name.
	SubjectShape:        []string{check.KindComponent, check.KindNet, check.KindComponent},
	Eval:                regulatorOutputVerdicts,
	StatesConsideredSet: true,
}

// regulatorOutputVerdicts decides every (source, rail, load) triple the supply walk reaches, one
// verdict each.
//
// THE ARITY IS THREE, and the rail is the element that gets forgotten. A part fed from one PMIC over
// two supplied rails is endangered twice, with different numbers, so dropping the rail from the
// identity would merge them.
//
// TWO CORPUS GAPS ARE NotConsidered rather than silent: a load with no seeded datasheet, and a load
// whose datasheet states no absolute-maximum supply row. On a partially-seeded corpus that is most of
// the board, and a reader who cannot tell "checked and fine" from "nobody has seeded this part"
// cannot tell how much of the supply tree this rule covered.
//
// The SOURCE side is scope rather than an outcome. A part that states no output voltage is not a
// source, so it yields no verdict at all.
func regulatorOutputVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, src := range m.Components() {
		srcSpec := m.PartSpec(src.RefDes)
		if srcSpec == nil {
			continue // not a seeded part, so nothing says it supplies anything
		}
		outputs := check.OutputVoltageLimits(srcSpec)
		if len(outputs) == 0 {
			continue // states no output voltage, so it is not a source and this rule is not about it
		}
		// The HIGHEST output a part can present is what a downstream part is exposed to,
		// since a regulator with an adjustable or multi-rail output endangers its load at the
		// top of the range.
		srcParam := outputs[0]
		for _, p := range outputs[1:] {
			if p.Value.GetMax() > srcParam.Value.GetMax() {
				srcParam = p
			}
		}

		for _, n := range suppliedNets(m, src) {
			for _, conn := range n.GetConnections() {
				ref := conn.GetComponentRef()
				if ref == src.RefDes || check.IsVirtualRef(ref) {
					continue // the source itself, or a power symbol rather than a part
				}
				v := check.Verdict{Subjects: []check.Entity{
					check.ComponentEntity(src.RefDes), check.NetEntity(n), check.ComponentEntity(ref),
				}}
				loadSpec := m.PartSpec(ref)
				var load *parampb.Parameter
				if loadSpec != nil {
					// The most restrictive abs-max row is the binding one, matching
					// supply-exceeds-abs-max, since a part is endangered at its lowest rating.
					for _, p := range check.SupplyAbsMaxLimits(loadSpec) {
						if load == nil || p.Value.GetMax() < load.Value.GetMax() {
							load = p
						}
					}
				}
				switch {
				case loadSpec == nil:
					v.Outcome = check.NotConsidered
					v.Reason = ref + " carries no seeded datasheet, so it states no supply rating to compare this rail against"
				case load == nil:
					v.Outcome = check.NotConsidered
					v.Reason = "the datasheet for " + ref + " states no comparable absolute-maximum supply row"
				default:
					outcome, w := check.CompareToBound(srcParam.Value.GetMax(), "V",
						check.Bound{Max: load.Value.Max}, src.RefDes+" output", ref+" absolute maximum")
					if w != nil {
						w.Datasheet = []*check.DatasheetCitation{
							check.DatasheetCitationOf(loadSpec, load),
							check.DatasheetCitationOf(srcSpec, srcParam),
						}
					}
					v.Outcome, v.Witness = outcome, w
					if outcome != check.Fail {
						break
					}
					v.Finding = &check.Finding{
						Subject: check.ComponentEntity(ref),
						Message: fmt.Sprintf(
							"%s supplies net %q at %s %gV, above %s's absolute-maximum %s %gV — %s; %s",
							src.RefDes, n.GetName(), srcParam.Symbol, srcParam.Value.GetMax(),
							ref, load.Symbol, load.Value.GetMax(),
							check.Citation(loadSpec, load), check.Citation(srcSpec, srcParam)),
						Prov: conn.GetProv(),
						// The regulator doing the supplying and the net it supplies over, which the
						// sentence names and the FINDING's subject does not. Declared in the order the
						// message names them, so the panel's chips read like the sentence
						// (agni issue 349). The finding's subject stays the ENDANGERED part, because
						// that is what a reader has to change.
						Context: []check.ContextSubject{
							check.Ctx(check.ComponentEntity(src.RefDes), "source"),
							check.Ctx(check.NetEntity(n), "rail"),
						},
						// BOTH citations, load first, because the finding's subject is the endangered
						// part and its document is the one a reviewer opens. The review's data-trust
						// gate reads every entry and rates the finding by its weakest, so the field is
						// a slice (WS3-028).
						DatasheetProv: []*check.DatasheetCitation{
							check.DatasheetCitationOf(loadSpec, load),
							check.DatasheetCitationOf(srcSpec, srcParam),
						},
					}
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// suppliedNets returns the nets a part's output can reach, meaning the nets it sits on plus their
// neighbourhood within check.SupplyPathReachHops, so a bead or series resistor between a regulator
// and its load does not hide the connection. Deduplicated by name, since two of the part's pins
// commonly land on the same net.
//
// It does NOT walk far. Voltage does not fall off along a supply path the way a surge does, so a wide
// radius would make every part on the board look connected to every regulator and the rule would
// report pairs that share no supply at all. See
// docsite/content/architecture/net-solving.md#how-far-a-walk-crosses-series-parts.
func suppliedNets(m check.Model, c *ir.Component) []*ir.Net {
	seen := map[string]bool{}
	var out []*ir.Net
	for _, n := range m.Nets() {
		if !onNet(n, c.RefDes) {
			continue
		}
		for _, rn := range m.Reach(n, check.SupplyPathReachHops).Nets {
			if !seen[rn.GetName()] {
				seen[rn.GetName()] = true
				out = append(out, rn)
			}
		}
	}
	return out
}

// onNet reports whether refDes has a connection on n.
func onNet(n *ir.Net, refDes string) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
}
