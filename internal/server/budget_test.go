package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// interceptOnce runs one request through the interceptor with next standing in for the service, and
// returns the context next saw and what was logged.
func interceptOnce(t *testing.T, b Budget, req *webapi.RunQueryRequest, next connect.UnaryFunc) (context.Context, []string) {
	t.Helper()
	var logged []string
	b.Log = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	var seen context.Context
	wrapped := b.Interceptor()(func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
		seen = ctx
		return next(ctx, r)
	})
	_, _ = wrapped(context.Background(), connect.NewRequest(req))
	return seen, logged
}

// TestTheInterceptorSetsTheBudgetAndLogsCostlyQueries covers agni issue 792 at the server. The
// deployment's budget reaches the service on the request's context, a query whose reported work
// passes the warning threshold is logged with a suggested cap, and a cheaper one is not.
func TestTheInterceptorSetsTheBudgetAndLogsCostlyQueries(t *testing.T) {
	answer := func(work int64) connect.UnaryFunc {
		return func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
			return connect.NewResponse(&webapi.RunQueryResponse{Query: "q(?x) => ?x", Work: work}), nil
		}
	}
	ctx, logged := interceptOnce(t, Budget{Enforce: 500, Warn: 100}, &webapi.RunQueryRequest{}, answer(250))
	if got := query.BudgetOf(ctx); got != 500 {
		t.Errorf("the service saw a budget of %d, want the deployment's 500", got)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "250 work units") || !strings.Contains(logged[0], "q(?x) => ?x") {
		t.Errorf("logged %q, want one line naming the query and its 250 units", logged)
	}
	_, logged = interceptOnce(t, Budget{Warn: 100}, &webapi.RunQueryRequest{}, answer(50))
	if len(logged) != 0 {
		t.Errorf("a query under the threshold was logged: %q", logged)
	}
	_, logged = interceptOnce(t, Budget{Warn: 100}, &webapi.RunQueryRequest{}, answer(250))
	if len(logged) != 1 || !strings.Contains(logged[0], "--query-budget 2500") {
		t.Errorf("with no budget enforced, the line should suggest one: %q", logged)
	}
}

func TestTheInterceptorLogsAQueryStoppedByTheBudget(t *testing.T) {
	stop := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, fmt.Errorf("%w: work passed its budget of 500", service.ErrResourceExhausted)
	}
	_, logged := interceptOnce(t, Budget{Enforce: 500}, &webapi.RunQueryRequest{Query: "expensive(?x) => ?x"}, stop)
	if len(logged) != 1 || !strings.Contains(logged[0], "expensive(?x)") || !strings.Contains(logged[0], "budget of 500") {
		t.Errorf("logged %q, want the stopped query named with the budget", logged)
	}
}
