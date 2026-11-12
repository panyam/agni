package query

import (
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/datalog"
)

// init makes Datalog a module language of the fact layer's vocabulary, so this engine reads a
// module of derived relations registered there. The fact layer registers every predicate that a
// query can call at its path, including the string tests and the circuit walks. This package adds
// none of its own.
func init() {
	facts.RegisterLanguage(datalog.Language)
}
