package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// regulatorOutputExceedsAbsMax is the first CONNECTION-AWARE datasheet rule (WS3-028): it compares a
// parameter on one part against a parameter on ANOTHER part, across the net that joins them. Every
// datasheet rule before it read one spec against one rail.
//
// The check: a regulator's datasheet output voltage against the absolute-maximum supply rating of a
// part it feeds. Both numbers are vendor values, so the finding cites both documents.
//
// WHY THIS IS NOT supply-exceeds-abs-max AGAIN. That rule asks the same question but takes the rail
// voltage from the NET NAME (check.NominalVoltageFromName, so "+5V" means 5V). That is a naming
// convention, and it is silent on a net named VOUT_A or 5V_SW, wrong on a net whose name outlived a
// design change, and unavailable on a rail nobody named for its voltage. Reading the number off the
// regulator's own datasheet replaces a convention with evidence, which is the point of the whole
// connection-aware class rather than an incidental improvement.
//
// It does not supersede the older rule: a rail fed by something with no seeded spec (a connector, an
// unseeded module) still has only its name to go on, so both rules earn their place.
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
	// Source, rail, load, in the order the message names them. All three are needed: one regulator
	// feeding one part over VOUT_A and again over VOUT_B is two different answers, so a tuple without
	// the rail would give them one name.
	SubjectShape:        []string{check.KindComponent, check.KindNet, check.KindComponent},
	Eval:                regulatorOutputVerdicts,
	StatesConsideredSet: true,
}

// regulatorOutputVerdicts decides every (source, rail, load) triple the supply walk reaches, one
// verdict each.
//
// THE ARITY IS THREE AND THE THIRD ELEMENT IS THE ONE THAT GETS FORGOTTEN. A regulator and a load are
// not enough to name an answer: a part fed from one PMIC over two supplied rails is endangered twice,
// on different rails, with different numbers. Dropping the rail from the identity would merge them.
//
// TWO SILENCES BECOME NotConsidered, and both are corpus gaps rather than design facts. A load with no
// seeded datasheet, and a load whose datasheet states no absolute-maximum supply row, each took the
// same `continue` a safely-fed load took. On a partially-seeded corpus that is most of the board, and
// a reader who cannot tell "checked and fine" from "nobody has seeded this part" has no idea how much
// of the supply tree this rule actually covered.
//
// The SOURCE side is scope rather than an outcome: a part that states no output voltage is not a
// source, so it yields nothing at all instead of a verdict about a supply relationship that does not
// exist.
func regulatorOutputVerdicts(m check.Model) []check.Verdict {
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
		// The HIGHEST output a part can present is what a downstream part is exposed to: a
		// regulator with an adjustable or multi-rail output endangers its load at the top of
		// the range, not the middle.
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
					// supply-exceeds-abs-max: a part is endangered at its lowest rating.
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
						// The two entities the sentence names and the FINDING's subject is not: the
						// regulator doing the supplying, and the net it supplies over. Declared in the
						// order the message names them, so the panel's chips read like the sentence
						// (agni issue 349). The finding's subject stays the ENDANGERED part, because
						// that is what a reader has to change.
						Context: []check.ContextSubject{
							check.Ctx(check.ComponentEntity(src.RefDes), "source"),
							check.Ctx(check.NetEntity(n), "rail"),
						},
						// BOTH citations, load first: the finding's subject is the endangered part, so
						// its document is the one a reviewer opens. The review's data-trust gate reads
						// every entry and rates the finding by its weakest, which is the whole
						// reason this field is a slice (WS3-028).
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

// suppliedNets is the set of nets a part's output can reach: the nets it sits on, plus their
// neighbourhood within check.SupplyPathReachHops, so a bead or series resistor between a regulator
// and its load does not hide the connection. Deduplicated by name, since two of the part's pins
// commonly land on the same net.
//
// It deliberately does NOT walk far. Voltage does not fall off along a supply path the way a surge
// does, so a wide radius would make every part on the board look connected to every regulator and
// the rule would start reporting pairs that share no supply at all.
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
