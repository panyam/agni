package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The tests here cover agni issue 793: a caller binds goal variables rather than splicing values
// into query text, and a bound variable answers exactly as the constant written in.

func text(s string) *webapi.QueryValue {
	return &webapi.QueryValue{Kind: &webapi.QueryValue_Text{Text: s}}
}
func number(f float64) *webapi.QueryValue {
	return &webapi.QueryValue{Kind: &webapi.QueryValue_Number{Number: f}}
}

func tutorialQueries() (*QueryService, string) {
	return NewQueryService(fsQueryLoader{base: filepath.Join("..", "examples", "tutorial-project")}, nil, nil),
		"mount://m/designs/gateway/gateway.edn"
}

func TestABoundQueryAnswersAsTheWrittenConstant(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	for _, c := range []struct {
		name, bound, written string
		bind                 map[string]*webapi.QueryValue
	}{
		{"text", `component.net(?r, ?n) => ?n`, `component.net("U1", ?n) => ?n`, map[string]*webapi.QueryValue{"r": text("U1")}},
		{"number", `net.pin_count(?n, ?c), ?c >= ?min => ?n`, `net.pin_count(?n, ?c), ?c >= 3 => ?n`, map[string]*webapi.QueryValue{"min": number(3)}},
	} {
		got, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: c.bound, Bindings: c.bind})
		if err != nil {
			t.Fatalf("%s: bound: %v", c.name, err)
		}
		want, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: c.written})
		if err != nil {
			t.Fatalf("%s: written: %v", c.name, err)
		}
		if len(want.GetRows()) == 0 {
			t.Fatalf("%s: the written query answers nothing on the fixture, so the comparison proves nothing", c.name)
		}
		if g, w := rowCells(got), rowCells(want); g != w {
			t.Errorf("%s: bound rows %s, written rows %s", c.name, g, w)
		}
		if len(got.GetBindings()) != len(c.bind) {
			t.Errorf("%s: response echoes %d bindings, want %d", c.name, len(got.GetBindings()), len(c.bind))
		}
	}
}

func TestABindingTheGoalDoesNotUseIsInvalid(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Snapshot: snap, Query: `component.net(?r, ?n) => ?n`, Bindings: map[string]*webapi.QueryValue{"zz": text("U1")},
	})
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "?zz") {
		t.Errorf("err = %v, want an invalid argument naming ?zz", err)
	}
}

// TestEachQueryInASetBindsItsOwnVariables: two queries with the same text and different bindings
// answer differently, so bindings are per query rather than per set.
func TestEachQueryInASetBindsItsOwnVariables(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	const q = `component.net(?r, ?n) => ?n`
	set := &webapi.QuerySet{Title: "t", Queries: []*webapi.NamedQuery{
		{Name: "u1", Query: q, Bindings: map[string]*webapi.QueryValue{"r": text("U1")}},
		{Name: "j1", Query: q, Bindings: map[string]*webapi.QueryValue{"r": text("J1")}},
	}}
	resp, err := svc.RunQueries(context.Background(), &webapi.RunQueriesRequest{Snapshot: snap, Set: set})
	if err != nil {
		t.Fatal(err)
	}
	res := resp.GetResults()
	if len(res) != 2 || res[0].GetError() != "" || res[1].GetError() != "" {
		t.Fatalf("results = %v", res)
	}
	a, b := rowCells(res[0].GetResult()), rowCells(res[1].GetResult())
	if a == "" || b == "" || a == b {
		t.Errorf("U1 answered %q and J1 %q, want two different non-empty answers", a, b)
	}
}

// TestABindingInANumberPositionIsReadAsANumber: a value bound where the schema declares a number is
// read as one whatever type it arrived as (panyam/jaala#65), so a caller holding only text, such as a
// viewer's input box, binds a count correctly, and text that is not a number is refused rather than
// answering nothing.
func TestABindingInANumberPositionIsReadAsANumber(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	const q = `net.pin_count(?n, ?c), ?c >= ?min => ?n`
	asked := func(v *webapi.QueryValue) (*webapi.RunQueryResponse, error) {
		return svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: q, Bindings: map[string]*webapi.QueryValue{"min": v}})
	}
	asNumber, err := asked(number(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(asNumber.GetRows()) == 0 {
		t.Fatal("no net has three or more pins on the fixture, so the comparison proves nothing")
	}
	asText, err := asked(text("3"))
	if err != nil {
		t.Fatal(err)
	}
	if g, w := rowCells(asText), rowCells(asNumber); g != w {
		t.Errorf("the text 3 answered %s, the number 3 answered %s", g, w)
	}
	if _, err := asked(text("abc")); !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "number") {
		t.Errorf("binding abc to a number: err = %v, want an invalid argument saying it is a number", err)
	}
}

// TestAnUntypedBindingKeepsItsOwnType: where nothing in the query types a variable, the binding's own
// type decides, which is what the typed wire value is for. As numbers 10 < 9 is false; as text "10"
// sorts before "9".
func TestAnUntypedBindingKeepsItsOwnType(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	const q = `entity(?n, "net"), ?a < ?b => ?n`
	asked := func(a, b *webapi.QueryValue) int {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: q, Bindings: map[string]*webapi.QueryValue{"a": a, "b": b}})
		if err != nil {
			t.Fatal(err)
		}
		return len(resp.GetRows())
	}
	if n := asked(number(10), number(9)); n != 0 {
		t.Errorf("as numbers 10 < 9 held for %d nets, want none", n)
	}
	if n := asked(text("10"), text("9")); n == 0 {
		t.Error(`as text "10" < "9" held for no net, want every net`)
	}
}

func list(vals ...*webapi.QueryValue) *webapi.QueryValue {
	return &webapi.QueryValue{Kind: &webapi.QueryValue_List{List: &webapi.QueryValueList{Values: vals}}}
}

// A variable bound to a list answers as the union of the query asked once per value, echoes the list,
// and matches nothing when the list is empty (agni issue 1121).
func TestAListBindingAnswersForEachValue(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	const q = `component.net(?r, ?n) => ?r, ?n`
	ask := func(v *webapi.QueryValue) *webapi.RunQueryResponse {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: q, Bindings: map[string]*webapi.QueryValue{"r": v}})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	u1, u2 := ask(text("U1")), ask(text("U2"))
	if len(u1.GetRows()) == 0 || len(u2.GetRows()) == 0 {
		t.Fatal("U1 or U2 answers nothing on the fixture, so the union proves nothing")
	}
	both := ask(list(text("U1"), text("U2")))
	want := map[string]bool{}
	for _, r := range append(u1.GetRows(), u2.GetRows()...) {
		want[strings.Join(r.GetCells(), "|")] = true
	}
	got := map[string]bool{}
	for _, r := range both.GetRows() {
		got[strings.Join(r.GetCells(), "|")] = true
	}
	if len(got) != len(want) || len(both.GetRows()) != len(want) {
		t.Errorf("the list answered %d rows, want the %d of U1 and U2 together", len(both.GetRows()), len(want))
	}
	for k := range want {
		if !got[k] {
			t.Errorf("the list's answer lacks %s", k)
		}
	}
	if l := both.GetBindings()["r"].GetList().GetValues(); len(l) != 2 || l[0].GetText() != "U1" || l[1].GetText() != "U2" {
		t.Errorf("echoed %v, want the list U1, U2", both.GetBindings()["r"])
	}
	if n := len(ask(list()).GetRows()); n != 0 {
		t.Errorf("an empty list answered %d rows, want none", n)
	}
}

func TestAListInsideAListIsInvalid(t *testing.T) {
	svc, uri := tutorialQueries()
	snap := snapOf(t, svc.Snapshots(), uri)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: `component.net(?r, ?n) => ?n`,
		Bindings: map[string]*webapi.QueryValue{"r": list(list(text("U1")))}})
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "not another list") {
		t.Errorf("a nested list: %v, want an invalid argument saying a list holds no list", err)
	}
}
