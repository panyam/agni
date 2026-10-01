package query

import (
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/datalog"
)

// init makes Datalog a module language of the fact layer's vocabulary, so a module of derived
// relations registered there is read by this engine. Every predicate a query calls, the string tests
// and the circuit walks included, is registered in the fact layer at its path; this package adds
// none of its own.
func init() {
	facts.RegisterLanguage(datalog.Language)
}

// GeneratorFirstRules reports the rules that OPEN their body with a value-producing generator whose
// own input argument is unbound, naming each offender by its head relation. A shipped profile rule
// opened with `net.reaches(?n, ?rn, ?h)` and took `agni check` from 13s to not finishing at all on a real
// design (WS3-114). See the engine's documentation for what it does and does not catch.
func GeneratorFirstRules(q Query) []string {
	return datalog.GeneratorFirstRules(q, facts.DefaultRegistry().Vocabulary())
}
