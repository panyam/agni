package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// probedDesign has one two-terminal part, R1, with a test point on one of its nets (VBUS) and none on
// the other (SENSE), so component.probed_one answers R1 through a `not` that held.
func probedDesign() *ir.Design {
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "TP1"}, {RefDes: "U1"}},
		Nets: []*ir.Net{
			{Name: "VBUS", Prov: &ir.Provenance{SourceFile: "x.edn", NativeId: "VBUS"}, Connections: []*ir.Connection{conn("R1", "1"), conn("TP1", "1")}},
			{Name: "SENSE", Prov: &ir.Provenance{SourceFile: "x.edn", NativeId: "SENSE"}, Connections: []*ir.Connection{conn("R1", "2"), conn("U1", "3")}},
		},
	}
}

const probedOneQuery = `component.probed_one(?r, ?probed, ?unprobed) => ?r, ?unprobed`

func runProbed(t *testing.T, witnesses bool) *webapi.RunQueryResponse {
	t.Helper()
	svc := NewQueryService(fakeLoader{design: probedDesign()}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.edn"), Query: probedOneQuery, Witnesses: witnesses,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rowCells(resp); got != "R1|SENSE" {
		t.Fatalf("rows = %q, want R1|SENSE, so the fixture asks what the test means it to", got)
	}
	return resp
}

// A row over a library member is witnessed by the member's own rule, with its body facts in the order
// the rule is written, and the `not` that held is a negated leaf (agni issue 794's acceptance).
func TestRunQueryWitnessesALibraryMember(t *testing.T) {
	resp := runProbed(t, true)
	row := resp.GetRows()[0]
	if len(row.GetWitnesses()) != 1 {
		t.Fatalf("the row has %d witnesses, want one for the goal's one literal", len(row.GetWitnesses()))
	}
	top := row.GetWitnesses()[0]
	if top.GetRelation() != "component.probed_one" || strings.Join(top.GetValues(), ",") != "R1,VBUS,SENSE" {
		t.Errorf("top witness %s(%v), want component.probed_one(R1, VBUS, SENSE)", top.GetRelation(), top.GetValues())
	}
	if !strings.Contains(top.GetRule(), ":-") || !strings.Contains(top.GetRule(), "not net.has_test_point(?unprobed)") {
		t.Errorf("top witness rule %q, want the member's written rule", top.GetRule())
	}
	var body []string
	for _, c := range top.GetChildren() {
		s := c.GetRelation() + "(" + strings.Join(c.GetValues(), ",") + ")"
		if c.GetNegated() {
			s = "not " + s
		}
		body = append(body, s)
	}
	want := "component.two_terminal(R1,SENSE,VBUS) net.has_test_point(VBUS) not net.has_test_point(SENSE)"
	if got := strings.Join(body, " "); got != want {
		t.Errorf("body witnesses %q, want %q", got, want)
	}
	if len(top.GetChildren()) != 3 {
		t.Fatalf("%d body witnesses, so the checks below would read past them", len(top.GetChildren()))
	}
	neg := top.GetChildren()[2]
	if len(neg.GetCiteIndex()) != 0 || len(neg.GetChildren()) != 0 {
		t.Errorf("the negated leaf cites %v and has %d children; an absence has neither", neg.GetCiteIndex(), len(neg.GetChildren()))
	}
	// net.has_test_point is itself a member, so its node holds the test point fact, cited through the
	// response's sources like a row's citations.
	tp := top.GetChildren()[1]
	if tp.GetRule() == "" || len(tp.GetChildren()) != 2 {
		t.Fatalf("net.has_test_point(VBUS) is derived, want its rule and two body facts, got rule %q and %d", tp.GetRule(), len(tp.GetChildren()))
	}
	leaf := tp.GetChildren()[0]
	if leaf.GetRelation() != "component.net" || strings.Join(leaf.GetValues(), ",") != "TP1,VBUS" {
		t.Errorf("first body fact %s(%v), want component.net(TP1, VBUS)", leaf.GetRelation(), leaf.GetValues())
	}
	if len(leaf.GetCiteIndex()) == 0 {
		t.Fatal("the leaf cites nothing")
	}
	for _, i := range leaf.GetCiteIndex() {
		if int(i) >= len(resp.GetSources()) || !strings.Contains(resp.GetSources()[i], "x.edn") {
			t.Errorf("cite_index %d does not point at a source of the response (%v)", i, resp.GetSources())
		}
	}
	// A value naming an entity points at the entity a cell naming it points at.
	if ve := top.GetValueEntity(); len(ve) != 3 || ve[0] == 0 || ve[1] == 0 || ve[0] != row.GetCellEntity()[0] || ve[2] != row.GetCellEntity()[1] {
		t.Fatalf("value_entity %v, want R1 and SENSE at the cells' entities %v, and VBUS typed", ve, row.GetCellEntity())
	}
	if e := resp.GetEntities()[top.GetValueEntity()[1]-1]; e.GetKind() != "net" || e.GetRef() != "VBUS" {
		t.Errorf("VBUS types as %s %s, want the net VBUS", e.GetKind(), e.GetRef())
	}
	if ve := leaf.GetValueEntity(); len(ve) != 2 || ve[0] == 0 || resp.GetEntities()[ve[0]-1].GetKind() != "component" {
		t.Errorf("TP1 in component.net(TP1, VBUS) types as %v, want a component", ve)
	}
}

// Witnesses cost nothing unasked: an answer without them carries none, and its work is what the same
// evaluation costs with no witness option at all.
func TestRunQueryWithoutWitnessesCostsWhatItDid(t *testing.T) {
	resp := runProbed(t, false)
	for _, r := range resp.GetRows() {
		if len(r.GetWitnesses()) != 0 {
			t.Errorf("row %v carries witnesses nobody asked for", r.GetCells())
		}
	}
	svc := NewQueryService(fakeLoader{design: probedDesign()}, nil, nil)
	ctx, snap, err := svc.projects.Snapshot(context.Background(), snapOf(t, svc.Snapshots(), "mount://m/x.edn"))
	if err != nil {
		t.Fatal(err)
	}
	q, err := query.Parse(probedOneQuery)
	if err != nil {
		t.Fatal(err)
	}
	d, err := svc.read(ctx, snap, q)
	if err != nil {
		t.Fatal(err)
	}
	cost := func() int64 {
		before := query.Work(d.base)
		if _, err := query.Default.EvalTable(ctx, q, d.base); err != nil {
			t.Fatal(err)
		}
		return query.Work(d.base) - before
	}
	plain := cost()
	if resp.GetWork() <= 0 || resp.GetWork() != plain {
		t.Errorf("work without witnesses = %d, want %d, what the evaluation costs with no witness option", resp.GetWork(), plain)
	}
	// The positive control: recording witnesses does change the work here, so equal work above means
	// none were recorded.
	if w := runProbed(t, true).GetWork(); w == plain {
		t.Errorf("work with witnesses = %d, the same as without, so the comparison above proves nothing", w)
	}
}

// RunQueries asks every query of a set for witnesses when the request does.
func TestRunQueriesWitnessesEachQuery(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: probedDesign()}, nil, nil)
	resp, err := svc.RunQueries(context.Background(), &webapi.RunQueriesRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.edn"), Witnesses: true,
		Set: &webapi.QuerySet{Queries: []*webapi.NamedQuery{{Name: "one", Query: probedOneQuery}, {Name: "nets", Query: `net.has_test_point(?n) => ?n`}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetResults()) != 2 {
		t.Fatalf("%d results, want 2", len(resp.GetResults()))
	}
	for _, r := range resp.GetResults() {
		rows := r.GetResult().GetRows()
		if len(rows) == 0 {
			t.Fatalf("%s answered no rows (%s)", r.GetName(), r.GetError())
		}
		for _, row := range rows {
			if len(row.GetWitnesses()) == 0 {
				t.Errorf("%s row %v has no witnesses", r.GetName(), row.GetCells())
			}
		}
	}
}
