package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// danglingEndpoint flags schematic wire endpoints that terminate on nothing. See Detail for the
// what/why/impact and the query structure.
var danglingEndpoint = &Rule{
	Name:       "dangling-endpoint",
	Severity:   "warning",
	Summary:    "A wire endpoint terminates on nothing (no pin, junction, label, or other wire).",
	Impact:     "A wire drawn to nothing is a connection the author intended but did not complete. Unlike a stub net it has no net to show up on: the wire is just there, contributing nothing, and whatever it was meant to reach is left unconnected. It is invisible at capture and only bites at bring-up.",
	Primitives: []string{"select"},
	Reads:      []string{"wire.endpoint", "wire.junction"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
		KeySite:         SiteDiagnostic, // detected by the reader from wire geometry (docs/19)
	},
	Detail: ruleDoc("dangling-endpoint"),
	Eval: func(m Model) []Finding {
		return Report(m.DanglingEndpoints(), func(e *ir.DanglingEndpoint) Finding {
			return Finding{
				Kind:    KindEndpoint,
				Subject: fmt.Sprintf("%d,%d", e.X, e.Y),
				Message: "wire endpoint connects to nothing",
				Prov:    e.Prov,
			}
		})
	},
}

// danglingEndpointSpec is the rule's declarative twin (WS3-003); parity with Eval is
// asserted by TestSpecParity.
var danglingEndpointSpec = &Spec{
	Over:    "dangling_endpoints",
	Message: "wire endpoint connects to nothing",
}
