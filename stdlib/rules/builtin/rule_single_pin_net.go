package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
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
	Detail:              ruleDoc("single-pin-net"),
	Eval:                singlePinNetVerdicts,
	StatesConsideredSet: true,
}

// singlePinNetVerdicts decides every net in the design and returns one verdict each, which IS this
// rule's considered set. Nothing is NotConsidered: a net's connection count is readable on any
// netlist, so every subject reaches a decision and none is dropped on the way.
//
// THE NO-CONNECT EXEMPTION BECOMES A PASS, and that is the substance of the conversion rather than a
// detail. A one-pin net the author deliberately marked no-connect used to leave the rule through the
// same silent `return` as a net the rule never looked at, so the two were indistinguishable
// downstream. Stating it as a pass with its own reason answers the question a reviewer actually has
// about a stub: not "did you find anything" but "there is a one-pin net here, do you know about it".
//
// The witness carries the count on every branch, including the exemption, so it tracks the fact it
// rests on. Wire a second pin to a stub and the statement changes with it; that is the property
// build/evidence.md asks for, and the reason the count is a Term rather than only prose.
func singlePinNetVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		count := len(n.Connections)
		terms := []check.WitnessTerm{{Label: "connections", Value: strconv.Itoa(count)}}
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}, Outcome: check.Pass}
		switch {
		case count >= 2:
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("net reaches %d connections, so it is not a stub", count),
				Terms:     terms,
			}
		case check.IntentionallyUnconnected(m, n):
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("net has %d connection(s), and the design marks it an intentional no-connect", count),
				Terms:     terms,
			}
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("net has %d connection(s) and nothing marks it no-connect", count),
				Terms:     terms,
			}
			f := check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}, Message: fmt.Sprintf("net has %d connection(s); expected >= 2", count), Prov: n.Prov}
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
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
