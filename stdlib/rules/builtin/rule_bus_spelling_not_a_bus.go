package builtin

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
)

// busSpellingNotABus flags a label spelled like a bus that the format reads as one plain net (agni
// issue 605). KiCad reads only `DATA[0..7]` as a bus, so a KiCad label written `DATA[7:0]`, the
// xschem and gEDA spelling, merges every signal its author meant to keep apart. See Detail.
var busSpellingNotABus = &check.Rule{
	Name:       "bus-spelling-not-a-bus",
	Severity:   "warning",
	Summary:    "A label is spelled like a bus, and this format reads it as one plain net.",
	Impact:     "The author drew a bus and the tool made a single net under a bus-shaped name, so the signals they meant to keep apart are not separate wherever that label joins them. The schematic reads correctly by eye and the netlist does not.",
	Remedy:     "Respell the label in the format's own bus syntax (in KiCad, `DATA[0..7]` rather than `DATA[7:0]`) if it names a bus, or rename it without the range if it names one signal.",
	Primitives: []string{"select"},
	Reads:      []string{"bus.construct"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // the reader decides it with its own format's bus syntax
	},
	Detail: ruleDoc("bus-spelling-not-a-bus"),
	// Only the KiCad reader records how it reads a range-spelled label, so elsewhere the considered
	// set would be empty and read as a pass.
	RequiresCapability:  []check.Capability{check.CapRangeLabels},
	Eval:                busSpellingNotABusVerdicts,
	StatesConsideredSet: true,
	// Its subjects are labels, which no net or part names.
	Scoping: check.ScopeDesignWide,
}

// busSpellingNotABusVerdicts decides every range-spelled label the reader recorded. A label the
// format reads as a bus passes and one it reads as a plain net fails, so a sheet whose every bus is
// spelled correctly says so rather than reporting the nothing a sheet with no bus reports.
//
// A name spelled the same way on several sheets is one verdict, since respelling it is one change.
func busSpellingNotABusVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	seen := map[string]bool{}
	for _, r := range m.RangeLabels() {
		if seen[r.GetLabel()] {
			continue
		}
		seen[r.GetLabel()] = true
		v := check.Verdict{Subjects: []check.Entity{check.BusEntity(r.GetLabel())}}
		if r.GetReadsAsBus() {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{Statement: fmt.Sprintf("%s is in this format's bus syntax, so it names a bus", r.GetLabel())}
		} else {
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: fmt.Sprintf("%s carries a range this format does not read as a bus, so it names one net", r.GetLabel())}
			v.Finding = &check.Finding{
				Subject: check.Entity{Kind: check.KindBus, Ref: r.GetLabel()},
				Message: busSpellingNotABusMessage,
				Prov:    r.GetProv(),
			}
		}
		out = append(out, v)
	}
	return out
}

// busSpellingNotABusMessage is the finding message. The label lives in the finding's Subject.
const busSpellingNotABusMessage = "label is spelled like a bus and reads as one net (respell it in this format's bus syntax)"
