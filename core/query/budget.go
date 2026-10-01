package query

import (
	"context"

	"github.com/panyam/jaala/datalog"
)

// BudgetExceeded is the error an evaluation stops with once its work passes its budget. A host tells
// it apart from a query that is wrong with errors.As, since running out of budget says the question
// was too expensive here, not that it was badly asked.
type BudgetExceeded = datalog.BudgetExceeded

type budgetKey struct{}

// WithBudget returns ctx carrying a work budget for every query evaluated under it (agni issue 792):
// a served query's own, and each query a check or review's query-backed rules evaluate. Work is what
// Base.Work counts, candidate comparisons plus each row a generator emits. Zero or less carries no
// budget, so an unbudgeted context stays unbudgeted.
//
// A budget applies per evaluation, so each query in a set, and each rule in a run, has the whole of
// it rather than sharing one.
func WithBudget(ctx context.Context, work int64) context.Context {
	if work <= 0 {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, work)
}

// BudgetOf is the work budget ctx carries, or 0 for none.
func BudgetOf(ctx context.Context) int64 {
	n, _ := ctx.Value(budgetKey{}).(int64)
	return n
}

// NarrowBudget returns ctx with its budget lowered to work when work is smaller, or set to work when
// ctx carries none. It never raises a budget, which is what lets a request ask for less than a
// deployment allows and never more. Zero or less leaves ctx as it is.
func NarrowBudget(ctx context.Context, work int64) context.Context {
	if work <= 0 {
		return ctx
	}
	if cur := BudgetOf(ctx); cur > 0 && cur <= work {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, work)
}

// EvalOptions are the engine options the context asks for: its budget, when it carries one. Every
// evaluation agni runs for a caller passes them, so a budget set once on the context reaches each.
func EvalOptions(ctx context.Context) []datalog.Option {
	if n := BudgetOf(ctx); n > 0 {
		return []datalog.Option{datalog.Budget(n)}
	}
	return nil
}
