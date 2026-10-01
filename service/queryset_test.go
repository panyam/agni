package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
)

// countingLoader counts how often a design and its geometry are read, since a query set claims
// one read however many questions.
type countingLoader struct {
	fakeLoader
	designs, geoms *int
}

func (c countingLoader) Design(ctx context.Context, u artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	*c.designs++
	return c.fakeLoader.Design(ctx, u, opts...)
}

func (c countingLoader) Geometry(ctx context.Context, u artifact.URI, s string, b bool, opts ...ReadOption) (*geom.SchematicGeometry, error) {
	*c.geoms++
	return c.fakeLoader.Geometry(ctx, u, s, b, opts...)
}

func newCounting(g *geom.SchematicGeometry) (countingLoader, *int, *int) {
	var d, gm int
	return countingLoader{fakeLoader: fakeLoader{design: queryDesign(), geom: g}, designs: &d, geoms: &gm}, &d, &gm
}

func setReq(queries ...string) *webapi.RunQueriesRequest {
	s := &webapi.QuerySet{Title: "t"}
	for i, q := range queries {
		s.Queries = append(s.Queries, &webapi.NamedQuery{Name: string(rune('a' + i)), Query: q})
	}
	return &webapi.RunQueriesRequest{Uri: "mount://m/x.kicad_sch", Set: s}
}

var setGeometry = &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "s1", Placements: []*geom.SymbolPlacement{{RefDes: "R1"}}}}}

// Each result is exactly what RunQuery answers for that query alone, down to sheet badges and locate
// reasons, so a set is a batch of RunQuery calls and not a second, drifting implementation.
func TestRunQueriesMatchesRunQueryPerQuery(t *testing.T) {
	queries := []string{
		`component-on-net(?r,?n) => ?r, ?n`,
		`component-on-net(?r,?n) => ?n, count(distinct ?r)`,
		`component-on-net(?r,"nope") => count(?r)`,
	}
	svc := NewQueryService(fakeLoader{design: queryDesign(), geom: setGeometry}, nil, nil)
	got, err := svc.RunQueries(context.Background(), setReq(queries...))
	if err != nil {
		t.Fatalf("RunQueries: %v", err)
	}
	if len(got.GetResults()) != len(queries) {
		t.Fatalf("results = %d, want %d", len(got.GetResults()), len(queries))
	}
	for i, q := range queries {
		want, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.kicad_sch", Query: q})
		if err != nil {
			t.Fatalf("RunQuery(%s): %v", q, err)
		}
		r := got.GetResults()[i]
		if r.GetError() != "" || !proto.Equal(r.GetResult(), want) {
			t.Errorf("%s: set answered %v (error %q), RunQuery answered %v", q, r.GetResult(), r.GetError(), want)
		}
	}
}

func TestRunQueriesReadsTheDesignOnce(t *testing.T) {
	one, d1, _ := newCounting(setGeometry)
	if _, err := NewQueryService(one, nil, nil).RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.kicad_sch", Query: `component-on-net(?r,?n) => ?r`}); err != nil {
		t.Fatal(err)
	}
	set, dN, gN := newCounting(setGeometry)
	_, err := NewQueryService(set, nil, nil).RunQueries(context.Background(), setReq(
		`component-on-net(?r,?n) => ?r`, `component-on-net(?r,?n) => ?n`, `component-on-net(?r,?n) => ?r, ?n`))
	if err != nil {
		t.Fatal(err)
	}
	if *d1 == 0 || *dN != *d1 {
		t.Errorf("three queries read the design %d times, one query reads it %d; want the same", *dN, *d1)
	}
	if *gN != 1 {
		t.Errorf("geometry loaded %d times for three entity queries, want once", *gN)
	}
}

func TestRunQueriesOfScalarsLoadsNoGeometry(t *testing.T) {
	l, _, g := newCounting(setGeometry)
	if _, err := NewQueryService(l, nil, nil).RunQueries(context.Background(), setReq(`component-on-net(?r,?n) => count(?r)`)); err != nil {
		t.Fatal(err)
	}
	if *g != 0 {
		t.Errorf("geometry loaded %d times for a scalar-only set, want none", *g)
	}
}

func TestRunQueriesReportsABadQueryByName(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	got, err := svc.RunQueries(context.Background(), setReq(`component-on-net(?r,?n) => ?r`, `compnent-on-net(?r,?n)`, `component-on-net(?r`))
	if err != nil {
		t.Fatalf("a bad query failed the whole set: %v", err)
	}
	res := got.GetResults()
	if res[0].GetResult() == nil || res[0].GetError() != "" {
		t.Errorf("good query: %v", res[0])
	}
	if !strings.Contains(res[1].GetError(), "did you mean") || res[1].GetResult() != nil {
		t.Errorf("unknown relation: %v, want an error with a suggestion and no result", res[1])
	}
	if res[2].GetError() == "" || res[2].GetResult() != nil {
		t.Errorf("parse error: %v, want an error and no result", res[2])
	}
	if res[1].GetName() != "b" || res[2].GetName() != "c" {
		t.Errorf("names = %q, %q; want b, c in request order", res[1].GetName(), res[2].GetName())
	}
}

func TestRunQueriesPreambleReachesEveryQuery(t *testing.T) {
	req := setReq(`on(?r) => ?r`, `on(?r) => count(?r)`)
	req.Set.Preamble = `on(?r) :- component-on-net(?r, ?_);`
	got, err := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil).RunQueries(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.GetResults() {
		if r.GetError() != "" || len(r.GetResult().GetRows()) == 0 {
			t.Errorf("%s: %v, want rows through the preamble's relation", r.GetName(), r)
		}
	}
	if got.GetPreamble() != req.Set.Preamble || got.GetTitle() != "t" {
		t.Errorf("response does not echo its set: title %q preamble %q", got.GetTitle(), got.GetPreamble())
	}
}

func TestRunQueriesUnusableSetIsInvalidArgument(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	req := setReq(`component-on-net(?r,?n)`, `component-on-net(?r,?n)`)
	req.Set.Queries[1].Name = "a"
	if _, err := svc.RunQueries(context.Background(), req); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("repeated name: err = %v, want invalid argument", err)
	}
	if _, err := svc.RunQueries(context.Background(), &webapi.RunQueriesRequest{Uri: "mount://m/x.kicad_sch"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty set: err = %v, want invalid argument", err)
	}
}
