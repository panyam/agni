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
