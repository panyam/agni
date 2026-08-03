package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// wireNoJunction flags a wire endpoint dropped onto the BODY of another wire with no
// junction dot — the T-tap that looks connected and is not. See Detail.
var wireNoJunction = &Rule{
	Name:       "wire-no-junction",
	Severity:   "warning",
	Summary:    "A wire endpoint lands mid-span on another wire with no junction dot.",
	Impact:     "The drawing shows a T-tap; the tool sees two separate nets. The author believes net A and net B are one, every visual review confirms it, and the board ships with the connection missing. It is the most dangerous silent connectivity slip in schematic capture.",
	Primitives: []string{"select"},
	Reads:      []string{"wire.endpoint", "wire.junction"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
		KeySite:         SiteDiagnostic, // detected by the reader from wire geometry (docs/19)
	},
	Detail: ruleDoc("wire-no-junction"),
	Eval: func(m Model) []Finding {
		return Report(m.NoJunctionEndpoints(), func(e *ir.DanglingEndpoint) Finding {
			return Finding{
				Kind:    KindEndpoint,
				Subject: fmt.Sprintf("%d,%d", e.X, e.Y),
				Message: "wire endpoint touches another wire mid-span with no junction dot",
				Prov:    e.Prov,
			}
		})
	},
}

// wireNoJunctionSpec is the rule's declarative twin (WS3-003); the over-set is new
// vocabulary, so the Go Eval stays canonical until it soaks (docs/19).
var wireNoJunctionSpec = &Spec{
	Over:    "no_junction_endpoints",
	Message: "wire endpoint touches another wire mid-span with no junction dot",
}
