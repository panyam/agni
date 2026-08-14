package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// wireNoJunction flags a wire endpoint dropped onto the BODY of another wire with no
// junction dot, the T-tap that looks connected and is not. See Detail.
var wireNoJunction = &check.Rule{
	Name:       "wire-no-junction",
	Severity:   "warning",
	Summary:    "A wire endpoint lands mid-span on another wire with no junction dot.",
	Impact:     "The drawing shows a T-tap; the tool sees two separate nets. The author believes net A and net B are one, every visual review confirms it, and the board ships with the connection missing. It is the most dangerous silent connectivity slip in schematic capture.",
	Primitives: []string{"select"},
	Reads:      []string{"wire.endpoint", "wire.junction"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // reader detects it from wire geometry (docsite/content/architecture/rules-and-checks.md)
	},
	Detail: ruleDoc("wire-no-junction"),
	Eval: func(m check.Model) []check.Finding {
		return check.Report(m.NoJunctionEndpoints(), func(e *ir.DanglingEndpoint) check.Finding {
			return check.Finding{
				Kind:    check.KindEndpoint,
				Subject: fmt.Sprintf("%d,%d", e.X, e.Y),
				Message: "wire endpoint touches another wire mid-span with no junction dot",
				Prov:    e.Prov,
			}
		})
	},
}

// wireNoJunctionSpec is the rule's declarative twin (WS3-003); the no_junction_endpoints over-set
// is new vocabulary, so the Go Eval stays canonical (twin discipline:
// docsite/content/build/check-rule.md).
var wireNoJunctionSpec = &check.Spec{
	Over:    "no_junction_endpoints",
	Message: "wire endpoint touches another wire mid-span with no junction dot",
}
