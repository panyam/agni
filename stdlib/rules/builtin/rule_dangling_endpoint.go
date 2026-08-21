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
