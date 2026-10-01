package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The tests here cover agni issues 795 (a run stops when its caller goes away) and 792 (a query stops
// at its work budget). Each pairs the stopped case with the same request under a live context or a
// budget it fits in, since a stop proven on a request that fails anyway proves nothing.

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestACancelledCheckStopsAsCancelled(t *testing.T) {
	svc := NewCheckService(fakeLoader{design: queryDesign()}, check.DefaultCatalog(), nil, "", nil, nil)
	req := &webapi.CheckDesignRequest{Uri: "mount://m/x.edn"}
	if _, err := svc.CheckDesign(cancelled(), req); !errors.Is(err, context.Canceled) {
		t.Errorf("CheckDesign with a cancelled context: err = %v, want context.Canceled", err)
	}
	if _, err := svc.CheckDesign(context.Background(), req); err != nil {
		t.Errorf("the same check with a live context failed, so the cancelled run proves nothing: %v", err)
	}
}

func TestACancelledReviewStopsAsCancelled(t *testing.T) {
	svc := newReviewSvc()
	req := &webapi.CreateReviewRequest{
		Manifest:  fixtureManifest(t, "review/mini.yaml"),
		DesignUri: "mount://m/review/can-broken.edn",
	}
	if _, err := svc.CreateReview(cancelled(), req); !errors.Is(err, context.Canceled) {
		t.Errorf("CreateReview with a cancelled context: err = %v, want context.Canceled", err)
	}
	if _, err := svc.CreateReview(context.Background(), req); err != nil {
		t.Errorf("the same review with a live context failed, so the cancelled run proves nothing: %v", err)
	}
}

const joinQuery = "component.net(?r, ?n), component.net(?r2, ?n) => ?r, ?r2"

func TestAQueryPastItsBudgetIsResourceExhausted(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: joinQuery, WorkBudget: 5})
	if !errors.Is(err, ErrResourceExhausted) || !strings.Contains(err.Error(), "budget of 5") {
		t.Errorf("a query past its budget: err = %v, want resource exhausted naming the budget of 5", err)
	}
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: joinQuery, WorkBudget: 10_000_000})
	if err != nil {
		t.Fatalf("the same query under a budget it fits in: %v", err)
	}
	if resp.GetWork() <= 5 {
		t.Errorf("work = %d, want the cost reported and above the budget that stopped it", resp.GetWork())
	}
}

// TestEachQueryInASetHasItsOwnBudget: a set's budget applies to each query rather than to the set,
// so a query past it is reported against its name and a cheaper one still answers.
func TestEachQueryInASetHasItsOwnBudget(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	const cheap = "component.net(?r, ?n) => ?r"
	cost := func(q string) int64 {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: q})
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return resp.GetWork()
	}
	c, j := cost(cheap), cost(joinQuery)
	if c >= j {
		t.Fatalf("the cheap query costs %d and the join %d, so no budget separates them", c, j)
	}
	set := &webapi.QuerySet{Title: "t", Queries: []*webapi.NamedQuery{{Name: "join", Query: joinQuery}, {Name: "cheap", Query: cheap}}}
	resp, err := svc.RunQueries(context.Background(), &webapi.RunQueriesRequest{Uri: "mount://m/x.edn", Set: set, WorkBudget: c})
	if err != nil {
		t.Fatalf("RunQueries: %v", err)
	}
	res := resp.GetResults()
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}
	if !strings.Contains(res[0].GetError(), "budget") {
		t.Errorf("the join past the budget: error = %q, want it named as over budget", res[0].GetError())
	}
	if res[1].GetError() != "" || len(res[1].GetResult().GetRows()) == 0 {
		t.Errorf("the cheap query: error = %q with %d rows, want it answered", res[1].GetError(), len(res[1].GetResult().GetRows()))
	}
}

// TestARequestCannotRaiseTheBudget: the deployment's budget rides the context (agni serve's
// interceptor sets it), and a request asking for more still gets the deployment's.
func TestARequestCannotRaiseTheBudget(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	_, err := svc.RunQuery(query.WithBudget(context.Background(), 5), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: joinQuery, WorkBudget: 10_000_000})
	if !errors.Is(err, ErrResourceExhausted) {
		t.Errorf("a request asking for more than the deployment's budget: err = %v, want resource exhausted", err)
	}
}

// TestAChecklistItemPastItsBudgetIsInconclusive: a review runs every item under the budget, and an
// item whose query passes it reads inconclusive, naming the budget, while the other items answer.
func TestAChecklistItemPastItsBudgetIsInconclusive(t *testing.T) {
	man := review.Manifest{Name: "t", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "q", Title: "shared nets", Binding: review.Binding{Query: &review.QueryBinding{Match: joinQuery, Subject: "r", Message: "{r} shares a net with {r2}"}}},
		{ID: "n", Title: "a manual item", Note: "checked by hand"},
	}}}}
	svc := NewReviewService(stubReviewLoader{design: queryDesign(), man: man}, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "", nil)
	ask := func(budget int64) *webapi.Review {
		t.Helper()
		resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{Manifest: ManifestProto(man), DesignUri: "mount://m/d", WorkBudget: budget})
		if err != nil {
			t.Fatalf("CreateReview with budget %d: %v", budget, err)
		}
		return resp
	}
	over := ask(5)
	if out, _ := outcomeOf(over, "q"); out != "inconclusive" {
		t.Errorf("the item past its budget = %q, want inconclusive", out)
	}
	if !strings.Contains(findingText(over, "q"), "budget of 5") {
		t.Errorf("the inconclusive item does not name the budget: %q", findingText(over, "q"))
	}
	if out, _ := outcomeOf(over, "n"); out != "not-automated" {
		t.Errorf("the other item = %q, want it answered as usual (not-automated)", out)
	}
	if out, _ := outcomeOf(ask(10_000_000), "q"); out == "inconclusive" {
		t.Error("the item is inconclusive under a budget it fits in too, so the over-budget run proves nothing")
	}
}

// TestACheckRulePastItsBudgetIsInconclusive: CheckDesign runs its query-backed rules under the
// request's budget, and a rule whose query passes it reports one inconclusive finding naming the
// budget rather than failing the whole check.
func TestACheckRulePastItsBudgetIsInconclusive(t *testing.T) {
	q, err := query.Parse(joinQuery)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := query.RuleFromQuery(query.FindingQuery{
		Rule:  check.Rule{Name: "shared-net", Severity: "warning"},
		Query: q, Kind: check.KindComponent, SubjectVar: "r", Message: "{r} shares a net with {r2}",
	})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := check.NewCatalog(check.NewSource("t", []*check.Rule{rule}))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewCheckService(fakeLoader{design: queryDesign()}, cat, nil, "", nil, nil)
	ask := func(budget int64) string {
		t.Helper()
		resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.edn", WorkBudget: budget})
		if err != nil {
			t.Fatalf("CheckDesign with budget %d: %v", budget, err)
		}
		var out []string
		for _, f := range resp.GetFindings() {
			out = append(out, f.GetMessage())
		}
		return strings.Join(out, " | ")
	}
	if got := ask(5); !strings.Contains(got, "budget of 5") {
		t.Errorf("findings past the budget = %q, want one naming the budget of 5", got)
	}
	if got := ask(10_000_000); strings.Contains(got, "budget") || got == "" {
		t.Errorf("findings under a budget the rule fits in = %q, want the rule's own findings", got)
	}
}

// findingText joins the messages of an item's findings.
func findingText(rv *webapi.Review, id string) string {
	var out []string
	for _, a := range rv.GetResults().GetAreas() {
		for _, it := range a.GetItems() {
			if it.GetId() == id {
				for _, f := range it.GetFindings() {
					out = append(out, f.GetMessage())
				}
			}
		}
	}
	return strings.Join(out, " | ")
}
