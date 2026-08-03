package check

import (
	"fmt"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// singlePinNet flags nets with fewer than two connections, except those that are intentionally
// unconnected. See Detail for the what/why/impact and the query structure.
var singlePinNet = &Rule{
	Name:       "single-pin-net",
	Severity:   "info",
	Summary:    "A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect.",
	Impact:     "A stub is a signal wired to one thing or nothing: the other end was meant to go somewhere and does not. It shows up only at bring-up, when a pin is silently dead. Catching it at capture is free; catching it on the bench is a debugging session.",
	Primitives: []string{"select", "count", "traverse", "exists", "pin-role"},
	Reads:      []string{"net.names", "net.pin_count", "pin.no_connect"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("single-pin-net"),
	Eval: func(m Model) []Finding {
		stubs := Select(m.Nets(), func(n *ir.Net) bool {
			return len(n.Connections) < 2 && !intentionallyUnconnected(m, n)
		})
		return Report(stubs, func(n *ir.Net) Finding {
			return Finding{
				Kind:    KindNet,
				Subject: n.Name,
				NetID:   n.GetId(),
				Message: fmt.Sprintf("net has %d connection(s); expected >= 2", len(n.Connections)),
				Prov:    n.Prov,
			}
		})
	},
}

// intentionallyUnconnected reports whether a net's lack of connections is deliberate: its name
// is a tool no-connect marker, or a connected pin resolves to the NO_CONNECT electrical type.
func intentionallyUnconnected(m Model, n *ir.Net) bool {
	switch name := strings.ToLower(n.Name); {
	case strings.HasPrefix(name, "unconnected"),
		strings.HasPrefix(name, "no_connect"),
		strings.HasPrefix(name, "nc_"):
		return true
	}
	return Exists(n.Connections, func(c *ir.Connection) bool {
		return m.PinDir(c.ComponentRef, c.PinRef) == ir.PinDirection_PIN_DIRECTION_NO_CONNECT
	})
}

// singlePinNetSpec is the rule's declarative twin (WS3-003): the count compare in the AST,
// the multi-clause no-connect heuristic through the FFI seam.
var singlePinNetSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Cmp{L: Fact{"net.pin_count"}, Op: "<", R: Lit{2}},
		Not{X: IsTrue{T: Call{Fn: "intentionally_unconnected"}}},
	}},
	Message: "net has {net.pin_count} connection(s); expected >= 2",
}
