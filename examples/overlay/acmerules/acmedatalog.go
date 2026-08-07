package acmerules

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// This file is the half of the overlay story the Go rule beside it cannot tell: a private rule
// authored as DATALOG over the engine's public relations, from a separate module, with no change to
// the engine (WS3-038). The Go rule writes an Eval closure and walks the Model itself; this one
// declares a query and lets query.RuleFromQuery turn the rows into findings with severity, message
// and provenance. Both arrive in the catalog the same way, through check.RegisterSource.
//
// Why an overlay author would want the datalog form: the rule is a value, not code. It can be
// reviewed by someone who does not read Go, printed in a report next to the finding it produced,
// and moved into a config file later without rewriting the engine's half of the contract.
//
// THE IMPORT THAT IS EASY TO MISS. A datalog rule evaluates against the engine's fact base, which
// is installed by stdlib/relations' init. A composing binary that does not blank-import that
// package gets an EMPTY fact base, so this rule matches nothing and reports clean — no error, no
// warning, just a quiet pass on a design that may well be violating it. See main.go, where the
// import is spelled out with that consequence.

// experimentalOnPowerNet is a house rule with no counterpart in the core catalog: an experimental
// (X-prefixed) part must not share a net with a production part's power pin. The engine ships
// nothing like it because it encodes a policy, not a law of electronics — which is exactly the kind
// of rule that belongs in an overlay rather than upstream.
//
// The query is four atoms and reads in the order an engineer would say it out loud: find a declared
// power pin, find the net it sits on, find another part on that same net, and keep it if that part's
// ref-des marks it experimental. It composes two PIN relations with a NET-level one, which is the
// join a pin rule needs and the reason pin.role / pin.net had to exist as relations at all.
//
// `prefix` is an engine built-in string predicate, so the whole rule is expressible without the
// overlay registering any relation or predicate of its own.
//
// The `?ref != ?x` clause is not decoration. Datalog matches by homomorphism, so without it the two
// variables may bind the SAME component and an experimental part's own power pin would satisfy the
// rule against itself — the finding would then say a part conflicts with itself. Any pattern naming
// two parts that must be distinct needs this, and forgetting it produces a wrong answer rather than
// an error, which is what makes it worth pointing at here.
var experimentalOnPowerNet = query.FindingQuery{
	Rule: check.Rule{
		Name:     "experimental-on-power-net",
		Severity: "warning",
		Summary:  "ACME house rule: an experimental part shares a net with a production power pin",
		Impact:   "a breadboard part on a production supply can pull the rail down or inject noise into every device on it, and the failure looks like an unrelated part misbehaving",
		Tags:     map[string]string{check.KeyCategory: "house-style"},
	},
	Query: query.MustParse(`
		exp_on_power(?net) :- pin.role(?ref, ?pin, "power"),
		                      pin.net(?ref, ?pin, ?net),
		                      component-on-net(?x, ?net),
		                      prefix(?x, "X"),
		                      ?ref != ?x;
		exp_on_power(?net) => ?net`),
	Kind:       check.KindNet,
	SubjectVar: "net",
	Message:    "net {net} carries a production power pin and an experimental (X-prefixed) part",
}
