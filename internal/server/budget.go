package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// Budget is the work budget a server applies to every query it evaluates, and how it reports the
// queries that cost the most (agni issue 792). Work is the unit a fact base counts: candidate
// comparisons plus each row a generator emits.
type Budget struct {
	// Enforce caps each evaluation's work. Zero enforces nothing, so a deployment can watch what its
	// queries cost (Warn) before it chooses a cap.
	Enforce int64
	// Warn is the work above which a query is logged with its cost and a suggested cap. Zero logs
	// nothing.
	Warn int64
	// Log receives each line. Nil discards them.
	Log func(format string, args ...any)
}

// Interceptor sets the budget on every request's context, where every query and every query-backed
// rule the request evaluates reads it (a request may narrow it, never raise it), and logs each query
// whose reported work passes Warn and each one stopped by the budget.
func (b Budget) Interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(query.WithBudget(ctx, b.Enforce), req)
			if b.Log == nil {
				return resp, err
			}
			name := req.Spec().Procedure
			if errors.Is(err, service.ErrResourceExhausted) {
				// The error names the budget that applied, which is the request's when it narrowed the
				// server's, so it is logged rather than b.Enforce.
				msg := err.Error()
				if ce := new(connect.Error); errors.As(err, &ce) {
					msg = ce.Message()
				}
				b.Log("query budget: %s stopped, %s: %s", name, msg, queryText(req))
			}
			if err == nil && b.Warn > 0 {
				for _, c := range queryCosts(resp) {
					if c.work > b.Warn {
						b.Log("query cost: %s spent %d work units, over the %d warning threshold; %s: %s",
							name, c.work, b.Warn, suggestion(b.Enforce, c.work), c.text)
					}
				}
			}
			return resp, err
		}
	}
}

// suggestion says what to do about a costly query, given what the server enforces.
func suggestion(enforced, work int64) string {
	if enforced <= 0 {
		return fmt.Sprintf("no budget is enforced, and --query-budget %d would allow ten times this cost", 10*work)
	}
	return fmt.Sprintf("the enforced budget is %d", enforced)
}

type queryCost struct {
	work int64
	text string
}

// queryCosts reads the work each answered query reports: one for RunQuery, one per query of a set.
func queryCosts(resp connect.AnyResponse) []queryCost {
	switch r := resp.Any().(type) {
	case interface {
		GetWork() int64
		GetQuery() string
	}:
		return []queryCost{{r.GetWork(), r.GetQuery()}}
	case interface {
		GetResults() []*webapi.NamedQueryResult
	}:
		var out []queryCost
		for _, res := range r.GetResults() {
			if a := res.GetResult(); a != nil {
				out = append(out, queryCost{a.GetWork(), res.GetName() + ": " + a.GetQuery()})
			}
		}
		return out
	}
	return nil
}

// queryText is the query a request asked, for a log line, or "" for one that carries none.
func queryText(req connect.AnyRequest) string {
	if q, ok := req.Any().(interface{ GetQuery() string }); ok {
		return q.GetQuery()
	}
	return ""
}
