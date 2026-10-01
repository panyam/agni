package acmerules

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// This file holds a private rule authored as DATALOG over the engine's public relations, from a
// separate module, with no change to the engine (WS3-038). The Go rule in acmerules.go writes an
// Eval closure and walks the Model itself. This one declares a query and lets
// query.RuleFromQuery turn the rows into findings with severity, message and provenance. Both
// register through check.RegisterSource.
//
// The datalog form is a value, not code. Someone who does not read Go can review it, a report can
// print it next to its finding, and it can move into a config file later.
//
// THE IMPORT THAT IS EASY TO MISS. A datalog rule evaluates against the fact base that
// stdlib/relations' init installs. A binary that does not blank-import that package gets an EMPTY
// fact base, so this rule matches nothing and reports clean with no error or warning. main.go
// carries the import, and agni.New refuses an empty fact base.

// experimentalOnPowerNet is a house rule with no counterpart in the core catalog. An experimental
// (X-prefixed) part must not share a net with a production part's power pin. It encodes a policy
// rather than a law of electronics, so it belongs in an extension rather than upstream.
//
// The query joins a NET-level relation with two PIN relations. It finds an experimental part and a
// net it sits on, then asks whether some other part's declared POWER pin is on that same net.
//
// The body leads with the experimental part, which binds ?net from the most selective set on the board
// (a handful of X-prefixed parts). agni's evaluator (query.Default) plans each body, so the order is
// for the reader rather than for speed: it starts from the most bound literal whatever is written
// first. Under the engine's reference evaluator, Naive, which runs literals as written, opening with
// pin.role would scan every power pin in the design first, which is how a shipped profile rule once
// made `agni check` non-terminating (WS3-114).
//
// `str.prefix` is one of the standard string predicates every vocabulary holds, so the extension
// registers no relation or predicate of its own.
//
// The `?ref != ?x` clause matters. Datalog matches by homomorphism, so without it the two variables
// may bind the SAME component and an experimental part's own power pin satisfies the rule against
// itself. Any pattern naming two parts that must be distinct needs this, and forgetting it produces
// a wrong answer rather than an error.
var experimentalOnPowerNet = query.FindingQuery{
	Rule: check.Rule{
		Name:     "experimental-on-power-net",
		Severity: "warning",
		Summary:  "ACME house rule: an experimental part shares a net with a production power pin",
		Impact:   "a breadboard part on a production supply can pull the rail down or inject noise into every device on it, and the failure looks like an unrelated part misbehaving",
		Tags:     map[string]string{check.KeyCategory: "house-style"},
	},
	Query: query.MustParse(`
		exp_on_power(?net) :- component.net(?x, ?net),
		                      str.prefix(?x, "X"),
		                      pin.net(?ref, ?pin, ?net),
		                      pin.role(?ref, ?pin, "power"),
		                      ?ref != ?x;
		exp_on_power(?net) => ?net`),
	Kind:       check.KindNet,
	SubjectVar: "net",
	Message:    "net {net} carries a production power pin and an experimental (X-prefixed) part",
}
