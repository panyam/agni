// Package reviewquery bridges a review manifest's inline query bindings to the datalog engine. It is
// the only place the two meet: core/review holds the manifest vocabulary and no query language
// (C29), core/query holds the language and knows nothing about checklists, and this package imports
// both and registers one with the other.
//
// It exists as a separate package rather than as an import inside either half because the arrow would
// otherwise have to point somewhere wrong — a manifest package that parses datalog, or a general
// query engine that knows what a review item is. Blank-import it wherever a binary composes a
// checklist surface, the way stdlib/relations is blank-imported to install the built-in relations.
//
// A binary that omits it still loads a manifest; an inline query binding then fails at Load with a
// message naming this package, rather than resolving to no rules and reading as an item nothing
// checked.
package reviewquery

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/review"
)

// Compiler compiles a manifest's inline query as a datalog program.
type Compiler struct{}

// CompileQuery parses the manifest's match text and binds the result to the identity and presentation
// the manifest already fixed. The error names the query rather than the item, because Load wraps it
// with the item id and doubling that reads as two separate problems.
func (Compiler) CompileQuery(req review.QueryRequest) (*check.Rule, error) {
	prog, err := query.Parse(req.Query)
	if err != nil {
		return nil, fmt.Errorf("query does not parse: %w", err)
	}
	return query.RuleFromQuery(query.FindingQuery{
		Rule:        req.Rule,
		Query:       prog,
		Kind:        req.Kind,
		SubjectVar:  req.Subject,
		Message:     req.Message,
		ParamSymbol: req.ParamSymbol,
	}), nil
}

func init() { review.RegisterQueryCompiler(Compiler{}) }
