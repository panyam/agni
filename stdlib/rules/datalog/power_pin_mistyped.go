// Package datalogrules holds check rules authored as datalog queries (via query.RuleFromQuery,
// WS3-038) rather than as Go or Spec. They live in their own package and register through
// check.RegisterSource — namespaced under the "dl" source — because a query-backed rule imports
// both check and query, and check must never import query. A binary that wants these rules blank-
// imports this package (cmd/agni does); they then appear in DefaultCatalog for both `agni check`
// and serve, exactly like any RegisterSource'd suite.
package datalog

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/stdlib/relations"
)

// powerPinMistyped flags a pin whose NAME says power or ground (pin.role) but whose electrical TYPE
// is not power_in, sitting alone on its net — a mistyped supply pin. It is the datalog successor to
// the withdrawn Spec power-pin-unconnected (PR 217), reframed to be NON-redundant: the existing
// power-input-not-driven needs the power_in type, so it misses exactly the mistyped case. Keyed on
// net fan-out (net.pin_count < 2), NOT net membership, because KiCad stub-synthesizes a net for
// every bare pin — so "not on a net" is never true there, but "alone on its net" is. Gated by
// has_nc_channel so it stays silent on formats that cannot express intentional no-connect.
var powerPinMistyped = query.RuleFromQuery(query.FindingQuery{
	Rule: check.Rule{
		Name:     "power-pin-mistyped",
		Severity: "warning",
		Summary:  "A pin named like power/ground but not typed power_in sits alone on its net.",
		Impact:   "power-input-not-driven catches an unconnected power pin only when the symbol types it as a power input. A pin the symbol author named VDD or GND but left typed as a plain signal slips through it; if that pin is also wired to nothing, the part loses a supply or a ground silently. This is the gap, expressed in datalog over the pin relations.",
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
	Kind:       check.KindPin,
	SubjectVar: "ref",
	PinVar:     "pin",
	Message:    "pin {pin} is named like a power/ground pin but is typed as a plain signal, alone on net {net} — a mistyped supply pin power-input-not-driven cannot catch",
})

// dlRules is the "dl" source's rule set — the datalog-authored rules this package registers.
// docs_test holds it 1:1 to the docs/ folder.
var dlRules = []*check.Rule{powerPinMistyped}

func init() {
	check.RegisterSource(check.NewSource("dl", dlRules))
}
