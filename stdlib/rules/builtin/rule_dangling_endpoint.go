package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// danglingEndpoint flags schematic wire endpoints that terminate on nothing. See Detail for the
// what/why/impact and the query structure.
var danglingEndpoint = &check.Rule{
	Name:       "dangling-endpoint",
	Severity:   "warning",
	Summary:    "A wire endpoint terminates on nothing (no pin, junction, label, or other wire).",
	Impact:     "A wire drawn to nothing is a connection the author intended but did not complete. Unlike a stub net it has no net to show up on: the wire is just there, contributing nothing, and whatever it was meant to reach is left unconnected. It is invisible at capture and only bites at bring-up.",
	Remedy:     "Extend the wire to the pin, junction, or label it was meant to reach, or delete it. A wire that ends in space is never what the author intended.",
	Primitives: []string{"select"},
	Reads:      []string{"wire.endpoint", "wire.junction"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // detected by the reader from wire geometry (docsite/content/architecture/rules-and-checks.md)
	},
	Detail: ruleDoc("dangling-endpoint"),
	// THE READER SUPPLIES ONLY WHAT FAILED, so this rule cannot state a considered set (agni issue 391).
	//
	// `dangling_endpoints` holds the wire ends that terminate on nothing, and nothing counts the wire
	// ends that terminate on something. The Model has no set of everything the reader examined, so
	// there is nothing to map over: the verdicts would be the failure list again, which is exactly the
	// coverage claim StatesConsideredSet exists to withhold.
	//
	// bus-not-modeled is the diagnostic rule that CAN state one, and the difference is instructive.
	// `unmodeled_buses` holds every bus construct the reader saw and the rule partitions it, so a bus
	// whose members are already nets is a pass the reader made visible. Doing the same here means a
	// reader recording what it looked at, alongside the `supplied` flag that already records THAT it
	// looked. That is a reader-and-IR change rather than a rule conversion.
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		return check.Report(m.DanglingEndpoints(), func(e *ir.DanglingEndpoint) check.Finding {
			return check.Finding{
				Kind:    check.KindEndpoint,
				Subject: fmt.Sprintf("%d,%d", e.X, e.Y),
				Message: "wire endpoint connects to nothing",
				Prov:    e.Prov,
			}
		})
	}),
}

// danglingEndpointSpec is the rule's declarative twin (WS3-003).
var danglingEndpointSpec = &check.Spec{
	Over:    "dangling_endpoints",
	Message: "wire endpoint connects to nothing",
}
