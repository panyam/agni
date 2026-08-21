package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// singlePinNet flags nets with fewer than two connections, except those that are intentionally
// unconnected. See Detail for the what/why/impact and the query structure.
var singlePinNet = &check.Rule{
	Name:       "single-pin-net",
	Severity:   "info",
	Summary:    "A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect.",
	Impact:     "A stub is a signal wired to one thing or nothing: the other end was meant to go somewhere and does not. It shows up only at bring-up, when a pin is silently dead. Catching it at capture is free; catching it on the bench is a debugging session.",
	Remedy:     "Wire the stub to whatever it was meant to reach, or mark the pin no-connect if it is genuinely unused, so the intent is recorded rather than left to be guessed at.",
	Primitives: []string{"select", "count", "traverse", "exists", "pin-role"},
	Reads:      []string{"net.names", "net.pin_count", "pin.no_connect"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("single-pin-net"),
	Eval: func(m check.Model) []check.Finding {
		stubs := check.Select(m.Nets(), func(n *ir.Net) bool {
			return len(n.Connections) < 2 && !check.IntentionallyUnconnected(m, n)
		})
		return check.Report(stubs, func(n *ir.Net) check.Finding {
			return check.Finding{
				Kind:    check.KindNet,
				Subject: n.Name,
				NetID:   n.GetId(),
				Message: fmt.Sprintf("net has %d connection(s); expected >= 2", len(n.Connections)),
				Prov:    n.Prov,
			}
		})
	},
}

// singlePinNetSpec is the rule's declarative twin (WS3-003): the count compare in the AST,
// the multi-clause no-connect heuristic through the FFI seam.
var singlePinNetSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: "<", R: check.Lit{V: 2}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "intentionally_unconnected"}}},
	}},
	Message: "net has {net.pin_count} connection(s); expected >= 2",
}
