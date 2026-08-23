// Package datalog holds check rules authored as datalog queries (via query.RuleFromQuery,
// WS3-038) rather than as Go or Spec. They register through check.RegisterSource, namespaced under
// the "dl" source, and live in their own package because a query-backed rule imports both check
// and query, and check must never import query. A binary that wants these rules blank-imports this
// package (cmd/agni does); they then appear in DefaultCatalog for both `agni check` and serve,
// like any other registered source.
package datalog

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/stdlib/relations"
)

// powerPinMistyped flags a pin whose NAME says power or ground (pin.role) but whose electrical TYPE
// is not power_in, sitting alone on its net. It is the datalog successor to the withdrawn Spec
// power-pin-unconnected (PR 217), and does not overlap power-input-not-driven, which needs the
// power_in type and so misses exactly this case. Keyed on net fan-out (net.pin_count < 2), NOT net
// membership, because KiCad stub-synthesizes a net for every bare pin, so "not on a net" is never
// true there but "alone on its net" is. Gated by has_nc_channel so it stays silent on formats that
// cannot express intentional no-connect.
var powerPinMistypedQ = query.FindingQuery{
	Rule: check.Rule{
		Name:     "power-pin-mistyped",
		Severity: "warning",
		Summary:  "A pin named like power/ground but not typed power_in sits alone on its net.",
		Impact:   "power-input-not-driven catches an unconnected power pin only when the symbol types it as a power input. A pin the symbol author named VDD or GND but left typed as a plain signal slips through it; if that pin is also wired to nothing, the part loses a supply or a ground silently. This is the gap, expressed in datalog over the pin relations.",
		Remedy:   "Type the pin as a power input in its symbol, then wire it to its rail. Fixing the symbol also restores power-input-not-driven over every other board that uses it.",
		Reads:    []string{relations.RelPinRole, relations.RelPinType, relations.RelPinNet, relations.RelNetPinCount, relations.RelHasNCChannel},
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryConnectivity,
			check.KeyTier:         "R",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("power-pin-mistyped"),
	},
	Query: query.MustParse(`
		bad(?ref, ?pin, ?net) :- pin.role(?ref, ?pin, "power"),  not pin.type(?ref, ?pin, "power_in"), pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?c), ?c < 2, has_nc_channel(?nc);
		bad(?ref, ?pin, ?net) :- pin.role(?ref, ?pin, "ground"), not pin.type(?ref, ?pin, "power_in"), pin.net(?ref, ?pin, ?net), net.pin_count(?net, ?c), ?c < 2, has_nc_channel(?nc);
		bad(?ref, ?pin, ?net) => ?ref, ?pin, ?net`),
	// The considered set: every pin the NAME says is a supply or a ground, on a format that can
	// express intentional no-connect. Both of the rule's tests drop out — the type test and the
	// fan-out comparison — because a pin that is correctly typed and a pin that is wired are both
	// pins this rule looked at and cleared.
	//
	// has_nc_channel stays. It is not a test the pin passes, it is whether the FORMAT can answer the
	// question at all, and a pin read from a format that cannot express no-connect was never judged.
	// Leaving it out would report every pin on an EDIF netlist as verified by a rule that is
	// structurally silent there, which is the false-pass shape the capability gate exists to prevent.
	Domain: &query.Domain{
		Query: query.MustParse(`
		scope(?ref, ?pin, ?net) :- pin.role(?ref, ?pin, "power"),  pin.net(?ref, ?pin, ?net), has_nc_channel(?nc);
		scope(?ref, ?pin, ?net) :- pin.role(?ref, ?pin, "ground"), pin.net(?ref, ?pin, ?net), has_nc_channel(?nc);
		scope(?ref, ?pin, ?net) => ?ref, ?pin, ?net`),
		Witness: "pin {pin} is named like a power/ground pin and is either typed power_in or wired to a shared net",
	},
	Kind:       check.KindPin,
	SubjectVar: "ref",
	PinVar:     "pin",
	Message:    "pin {pin} is named like a power/ground pin but is typed as a plain signal, alone on net {net} — a mistyped supply pin power-input-not-driven cannot catch",
	// The net, and only the net. {pin} is already the subject here (Kind is KindPin, so the subject is
	// the ref/pin pair), and repeating the subject as its own context would give the panel a chip that
	// navigates to the thing the reader is already looking at (agni issue 349).
	ContextVars: []query.ContextVar{{Var: "net", Kind: check.KindNet, Role: "net"}},
}

var powerPinMistyped = query.RuleFromQuery(powerPinMistypedQ)

// dlRules is the rule set registered under the "dl" source.
var dlRules = []*check.Rule{powerPinMistyped}

// DocRules returns the datalog source's documented rules for the docsite catalog generator
// (tools/catalogdocs). It is the same slice the source registers; callers must not mutate the
// returned rules.
func DocRules() []*check.Rule { return dlRules }

// Queries returns the datalog rule DECLARATIONS this package holds, registered and twin alike. It is
// the rule-definition contract's view of the package (WS3-103). RuleFromQuery yields a check.Rule
// whose Eval closure has no wire form, so a round-trip has to start from the declaration rather
// than the compiled rule. Callers must not mutate the returned values.
func Queries() []query.FindingQuery { return []query.FindingQuery{powerPinMistypedQ, crystalLoadCapsQ} }

func init() {
	check.RegisterSource(check.NewSource("dl", dlRules))
}
