package service

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The tests here cover agni issue 896: a request's limit caps an answer inside evaluation, and the
// answer says how many rows the limit left out.

// netPreset is the served click-to-ask query for a net.
func netPreset(t *testing.T) string {
	t.Helper()
	for _, e := range query.EntityQueries() {
		if e.Kind == "net" {
			return e.Query
		}
	}
	t.Fatal("no net preset is served")
	return ""
}

func TestALimitedAnswerSaysHowManyRowsItLeftOut(t *testing.T) {
	svc, uri := tutorialQueries()
	q := netPreset(t)
	ask := func(limit int32) *webapi.RunQueryResponse {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
			Uri: uri, Query: q, Bindings: map[string]*webapi.QueryValue{"net": text("GND")}, Limit: limit, OmitLocations: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	whole := ask(0)
	all := len(whole.GetRows())
	if all < 4 {
		t.Fatalf("GND has %d pins on the fixture, too few for a limit to cut", all)
	}
	if whole.GetTotal() != 0 {
		t.Errorf("an answer with no limit reports total %d, want 0", whole.GetTotal())
	}
	page := ask(2)
	if len(page.GetRows()) != 2 {
		t.Errorf("limit 2 answered %d rows", len(page.GetRows()))
	}
	if int(page.GetTotal()) != all {
		t.Errorf("limit 2 reports total %d, want the %d rows the whole answer has", page.GetTotal(), all)
	}
	// Only the rows returned are assembled: the page names fewer entities than the whole answer. The
	// goal is still solved whole, since counting the total needs every row.
	if len(page.GetEntities()) >= len(whole.GetEntities()) {
		t.Errorf("limit 2 names %d entities against %d for the whole answer, want fewer", len(page.GetEntities()), len(whole.GetEntities()))
	}
}

func TestTheSmallerOfTwoLimitsApplies(t *testing.T) {
	svc, uri := tutorialQueries()
	q := netPreset(t)
	for _, c := range []struct {
		written string
		limit   int32
		want    int
	}{
		{" limit 3", 2, 2},
		{" limit 2", 3, 2},
		{" limit 2", 0, 2},
	} {
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
			Uri: uri, Query: q + c.written, Bindings: map[string]*webapi.QueryValue{"net": text("GND")}, Limit: c.limit, OmitLocations: true,
		})
		if err != nil {
			t.Fatalf("%q with limit %d: %v", c.written, c.limit, err)
		}
		if len(resp.GetRows()) != c.want {
			t.Errorf("%q with request limit %d answered %d rows, want %d", c.written, c.limit, len(resp.GetRows()), c.want)
		}
		if resp.GetTotal() <= int32(c.want) {
			t.Errorf("%q with request limit %d reports total %d, want more than %d", c.written, c.limit, resp.GetTotal(), c.want)
		}
	}
}

func TestANegativeLimitIsInvalid(t *testing.T) {
	svc, uri := tutorialQueries()
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: uri, Query: netPreset(t), Bindings: map[string]*webapi.QueryValue{"net": text("GND")}, Limit: -1})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("err = %v, want an invalid argument", err)
	}
}

// TestTheNetPresetAnswersAsTheJoinItReplaced: the net preset dropped a component.net atom that added
// no row (agni issue 896), so it must answer exactly what the join did.
func TestTheNetPresetAnswersAsTheJoinItReplaced(t *testing.T) {
	svc, uri := tutorialQueries()
	bind := map[string]*webapi.QueryValue{"net": text("GND")}
	ask := func(q string) string {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: uri, Query: q, Bindings: bind, OmitLocations: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.GetRows()) == 0 {
			t.Fatalf("%s answers nothing on the fixture, so the comparison proves nothing", q)
		}
		return rowCells(resp)
	}
	if got, want := ask(netPreset(t)), ask(`component.net(?ref, ?net), pin.net(?ref, ?pin, ?net) => ?ref, ?pin`); got != want {
		t.Errorf("the preset answers %s, the join answered %s", got, want)
	}
}
